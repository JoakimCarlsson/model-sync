package azure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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

// cacheFiles are where a fetched document is kept, one per document.
var cacheFiles = map[string]string{
	retailPricesURL: "azure_foundry_meters.json",
	ModelsURL:       "azure_foundry_models.html",
}

// defaultCurrency is what a meter is read in when it states none.
const defaultCurrency = "USD"

// Provider reads Azure's retail prices. The zero value is not usable; call
// New.
type Provider struct {
	// Client performs the fetches.
	Client *http.Client
	// CacheDir, when set, backs the fetch with a file on disk.
	CacheDir string
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
	models, err := p.fetchModels(ctx)
	if err != nil {
		return []catalog.Document{meters}, err
	}
	return []catalog.Document{meters, models}, nil
}

// fetchModels retrieves the model documentation, which is one page and needs
// none of the pagination the price list does.
func (p *Provider) fetchModels(ctx context.Context) (catalog.Document, error) {
	if body, ok := p.readCache(ModelsURL); ok {
		return catalog.Document{URL: ModelsURL, Body: body}, nil
	}
	body, err := p.get(ctx, ModelsURL)
	if err != nil {
		return catalog.Document{}, err
	}
	p.writeCache(ModelsURL, body)
	return catalog.Document{URL: ModelsURL, Body: body}, nil
}

// fetchMeters walks every page of the meter listing and returns them as one
// document, so that parsing does not depend on how the pages were split.
func (p *Provider) fetchMeters(ctx context.Context) (catalog.Document, error) {
	if body, ok := p.readCache(retailPricesURL); ok {
		return catalog.Document{URL: retailPricesURL, Body: body}, nil
	}
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
	p.writeCache(retailPricesURL, body)
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

// attempt performs one request.
func (p *Provider) attempt(ctx context.Context, target string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", target, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: %s", target, resp.Status)
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
	for _, doc := range docs {
		if doc.URL == ModelsURL {
			b.applyCatalog(doc)
		}
	}
	return b.result(), errors.Join(failures...)
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
	read := readSKU(m.SkuName, m.ProductName)
	id := slugID(read.model)
	if id == "" {
		return
	}
	model := b.model(id, kindFor(m.SkuName, m.ProductName))
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
			With(DimRegion, m.ArmRegionName),
	})
}

// readCache returns a previously fetched listing.
func (p *Provider) readCache(url string) ([]byte, bool) {
	if p.CacheDir == "" {
		return nil, false
	}
	body, err := os.ReadFile(filepath.Join(p.CacheDir, cacheFiles[url]))
	return body, err == nil
}

// writeCache stores a listing, ignoring failures because the cache is an
// optimization and never the source of truth.
func (p *Provider) writeCache(url string, body []byte) {
	if p.CacheDir == "" {
		return
	}
	if err := os.MkdirAll(p.CacheDir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(p.CacheDir, cacheFiles[url]), body, 0o644)
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
