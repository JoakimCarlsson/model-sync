package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// providerID and providerName identify this source in the catalog.
const (
	providerID   = "azure"
	providerName = "Azure AI Foundry"
)

const retailPricesURL = "https://prices.azure.com/api/retail/prices"

// meterFilter selects the model serving meters in their primary region.
//
// Foundry Models is the service that serves models; the rest of Azure's AI
// family is vision, speech and compute, which are not models. Restricting to
// the primary region keeps one row per meter rather than one per region.
const meterFilter = "serviceName eq 'Foundry Models'" +
	" and type eq 'Consumption'" +
	" and isPrimaryMeterRegion eq true"

// Pagination limits. Azure returns a thousand meters a page and rejects
// requests that arrive too quickly, so pages are spaced and retried.
const (
	requestSpacing = 300 * time.Millisecond
	maxAttempts    = 5
	retryBackoff   = 4 * time.Second
	maxPages       = 60
)

// documentationURLs are the pages stating what a model holds and can do, in
// the order they are read.
var documentationURLs = []string{
	ModelsURL,
	PartnersURL,
	ImagesURL,
	VideoURL,
	ScheduleURL,
	GalleryURL,
}

// defaultCurrency is what a meter is read in when it states none.
const defaultCurrency = "USD"

// Provider reads Azure's retail prices. The zero value is not usable; call
// New.
type Provider struct {
	// Client performs the fetches.
	Client *http.Client
	// Sleep waits between pages. It exists so a caller can run the pagination
	// without delay.
	Sleep func(time.Duration)
}

// New returns a Provider using the default HTTP client.
func New() *Provider {
	return &Provider{Client: http.DefaultClient, Sleep: time.Sleep}
}

// ID implements catalog.Source.
func (p *Provider) ID() string { return providerID }

// Name implements catalog.Source.
func (p *Provider) Name() string { return providerName }

// page is one response from the retail price API.
type page struct {
	Items        []meter `json:"Items"`
	NextPageLink string  `json:"NextPageLink"`
}

// meter is one billable rate.
type meter struct {
	CurrencyCode  string  `json:"currencyCode"`
	RetailPrice   float64 `json:"retailPrice"`
	ArmRegionName string  `json:"armRegionName"`
	MeterName     string  `json:"meterName"`
	ProductName   string  `json:"productName"`
	SkuName       string  `json:"skuName"`
	UnitOfMeasure string  `json:"unitOfMeasure"`
}

// Fetch walks every page of the meter listing and returns them as one
// document, so that parsing does not depend on how the pages were split.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	meters, err := p.fetchMeters(ctx)
	if err != nil {
		return nil, err
	}
	docs := []catalog.Document{meters}
	var failures []error
	for _, url := range documentationURLs {
		doc, err := p.fetchDocument(ctx, url)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		docs = append(docs, doc)
	}
	return docs, errors.Join(failures...)
}

// fetchDocument retrieves one document, by whichever means it answers to.
func (p *Provider) fetchDocument(
	ctx context.Context,
	url string,
) (catalog.Document, error) {
	if url == GalleryURL {
		return p.fetchGallery(ctx)
	}
	return p.fetchPage(ctx, url)
}

// fetchGallery walks every page of the catalog listing and returns them as one
// document, the same as the meter listing, so that parsing does not depend on
// how the pages were split.
func (p *Provider) fetchGallery(ctx context.Context) (catalog.Document, error) {
	var (
		summaries []gallerySummary
		token     string
	)
	for pages := 0; pages < galleryMaxPages; pages++ {
		request, err := json.Marshal(galleryRequest{
			Filters: []galleryFilter{{
				Field:    "Publisher",
				Values:   []string{galleryExcluded},
				Operator: "ne",
			}},
			PageSize:          galleryPageSize,
			ContinuationToken: token,
		})
		if err != nil {
			return catalog.Document{}, err
		}
		body, err := p.post(ctx, GalleryURL, request)
		if err != nil {
			return catalog.Document{}, err
		}
		var current galleryPage
		if err := json.Unmarshal(body, &current); err != nil {
			return catalog.Document{}, fmt.Errorf(
				"decode %s: %w",
				GalleryURL,
				err,
			)
		}
		summaries = append(summaries, current.Summaries...)
		token = current.ContinuationToken
		if token == "" {
			break
		}
		p.wait(requestSpacing)
	}
	body, err := json.Marshal(galleryPage{Summaries: summaries})
	if err != nil {
		return catalog.Document{}, err
	}
	return catalog.Document{URL: GalleryURL, Body: body}, nil
}

// fetchPage retrieves one documentation page, which needs none of the
// pagination the price list does.
func (p *Provider) fetchPage(
	ctx context.Context,
	url string,
) (catalog.Document, error) {
	body, err := p.get(ctx, url)
	if err != nil {
		return catalog.Document{}, err
	}
	return catalog.Document{URL: url, Body: body}, nil
}

// fetchMeters walks every page of the meter listing and returns them as one
// document, so that parsing does not depend on how the pages were split.
func (p *Provider) fetchMeters(ctx context.Context) (catalog.Document, error) {
	var (
		meters []meter
		next   = retailPricesURL + "?$filter=" + url.QueryEscape(meterFilter)
	)
	for pages := 0; next != "" && pages < maxPages; pages++ {
		body, err := p.get(ctx, next)
		if err != nil {
			return catalog.Document{}, err
		}
		var current page
		if err := json.Unmarshal(body, &current); err != nil {
			return catalog.Document{}, fmt.Errorf("decode %s: %w", next, err)
		}
		meters = append(meters, current.Items...)
		next = current.NextPageLink
		if next != "" {
			p.wait(requestSpacing)
		}
	}
	body, err := json.Marshal(page{Items: meters})
	if err != nil {
		return catalog.Document{}, err
	}
	return catalog.Document{URL: retailPricesURL, Body: body}, nil
}

// get retrieves one page, retrying when Azure asks for a slower pace.
func (p *Provider) get(ctx context.Context, target string) ([]byte, error) {
	var failures []error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		body, err := p.attempt(ctx, target)
		if err == nil {
			return body, nil
		}
		failures = append(failures, err)
		if ctx.Err() != nil {
			break
		}
		p.wait(time.Duration(attempt) * retryBackoff)
	}
	return nil, errors.Join(failures...)
}

// post retrieves one page of a listing that is queried rather than addressed,
// retrying on the same terms a fetch is.
func (p *Provider) post(
	ctx context.Context,
	target string,
	request []byte,
) ([]byte, error) {
	var failures []error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		body, err := p.attemptPost(ctx, target, request)
		if err == nil {
			return body, nil
		}
		failures = append(failures, err)
		if ctx.Err() != nil {
			break
		}
		p.wait(time.Duration(attempt) * retryBackoff)
	}
	return nil, errors.Join(failures...)
}

// attemptPost performs one queried request.
func (p *Provider) attemptPost(
	ctx context.Context,
	target string,
	request []byte,
) ([]byte, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		target,
		bytes.NewReader(request),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return p.send(req)
}

// attempt performs one request.
func (p *Provider) attempt(ctx context.Context, target string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	return p.send(req)
}

// send performs one prepared request.
func (p *Provider) send(req *http.Request) ([]byte, error) {
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", req.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: %s", req.URL, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// wait pauses between requests.
func (p *Provider) wait(d time.Duration) {
	if p.Sleep != nil {
		p.Sleep(d)
	}
}

// Parse reads the meters first, because they are the only document naming the
// models, then the documentation onto what they established.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	var failures []error
	for _, doc := range docs {
		if doc.URL != retailPricesURL {
			continue
		}
		var current page
		if err := json.Unmarshal(doc.Body, &current); err != nil {
			failures = append(failures, fmt.Errorf("decode: %w", err))
			continue
		}
		for _, m := range current.Items {
			b.applyMeter(m, doc.URL)
		}
	}
	b.applyCatalog(documentation(docs))
	return b.result(), errors.Join(failures...)
}

// documentation returns the fetched documentation pages, in the order they are
// read, so that a run missing one still applies the rest.
func documentation(docs []catalog.Document) []catalog.Document {
	var out []catalog.Document
	for _, url := range documentationURLs {
		for _, doc := range docs {
			if doc.URL == url {
				out = append(out, doc)
			}
		}
	}
	return out
}

// applyMeter records one meter against the model named inside its SKU.
func (b *builder) applyMeter(m meter, source string) {
	if m.RetailPrice == 0 {
		return
	}
	unit, ok := unitsOfMeasure[strings.ToLower(strings.TrimSpace(m.UnitOfMeasure))]
	if !ok {
		return
	}
	if nonModel(m.SkuName, m.ProductName) {
		return
	}
	read := readSKU(m.SkuName, m.ProductName)
	id := slugID(read.model)
	if id == "" {
		return
	}
	model := b.model(id, kindFor(read.model, m.ProductName))
	model.AddSource(source)
	model.SetAttr(AttrProduct, m.ProductName)
	currency := m.CurrencyCode
	if currency == "" {
		currency = defaultCurrency
	}
	model.AddPrice(catalog.Price{
		Metric:   metricFor(read),
		Unit:     unit,
		Amount:   m.RetailPrice,
		Currency: currency,
		Dims: catalog.Dims{}.
			With(DimDeployment, read.deployment).
			With(DimTier, read.tier).
			With(DimContext, read.context).
			With(DimModality, read.modality).
			With(DimFineTuned, read.fineTuned).
			With(DimModelGrader, read.grader).
			With(DimQuality, read.quality).
			With(DimResolution, read.resolution).
			With(DimAspect, read.aspect).
			With(DimDuration, read.duration).
			With(DimRegion, m.ArmRegionName),
	})
}

// builder accumulates models.
type builder struct {
	models map[string]*catalog.Model
	order  []string
}

func newBuilder() *builder {
	return &builder{models: map[string]*catalog.Model{}}
}

// model returns the entry for id, creating it if absent.
func (b *builder) model(id string, kind catalog.Kind) *catalog.Model {
	m, ok := b.models[id]
	if !ok {
		m = &catalog.Model{ID: id, Provider: providerID, Kind: kind}
		b.models[id] = m
		b.order = append(b.order, id)
		return m
	}
	if m.Kind == "" {
		m.Kind = kind
	}
	return m
}

// result returns the accumulated models in identifier order.
func (b *builder) result() []catalog.Model {
	ids := slices.Clone(b.order)
	slices.Sort(ids)
	out := make([]catalog.Model, 0, len(ids))
	for _, id := range ids {
		out = append(out, *b.models[id])
	}
	return out
}
