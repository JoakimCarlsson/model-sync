package vertexai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// providerID and providerName identify this source in the catalog.
const (
	providerID   = "vertexai"
	providerName = "Google Vertex AI"
)

// vertexServiceID is Vertex AI's service in the billing catalog.
const vertexServiceID = "C7E2-9256-1C43"

// catalogURL is the billing catalog endpoint listing a service's SKUs.
const catalogURL = "https://cloudbilling.googleapis.com/v1/services/" +
	vertexServiceID + "/skus"

// tokenEnv is where a caller may put an OAuth token, so that a machine with no
// gcloud can still sync.
const tokenEnv = "GOOGLE_OAUTH_TOKEN"

// cacheFile is where a fetched listing is kept.
const cacheFile = "vertexai_skus.json"

// pageSize is how many SKUs to request at a time.
const pageSize = 5000

// maxPages bounds the walk.
const maxPages = 20

// ErrUnconfigured reports that no Google credential is available. It wraps the
// catalog's sentinel so a caller can carry on without this provider.
var ErrUnconfigured = fmt.Errorf(
	"vertexai: no credential; set %s or authenticate gcloud: %w",
	tokenEnv,
	catalog.ErrUnconfigured,
)

// Provider reads Vertex AI's billing catalog. The zero value is not usable;
// call New.
type Provider struct {
	// Client performs the fetches.
	Client *http.Client
	// CacheDir, when set, backs the fetch with a file on disk.
	CacheDir string
	// Token is the OAuth token to present. When empty it is taken from the
	// environment, and failing that from gcloud.
	Token string
}

// New returns a Provider using the default HTTP client.
func New() *Provider {
	return &Provider{Client: http.DefaultClient}
}

// ID implements catalog.Source.
func (p *Provider) ID() string { return providerID }

// Name implements catalog.Source.
func (p *Provider) Name() string { return providerName }

// page is one response from the catalog endpoint.
type page struct {
	SKUs          []sku  `json:"skus"`
	NextPageToken string `json:"nextPageToken"`
}

// sku is one billable rate.
type sku struct {
	SKUID          string       `json:"skuId"`
	Description    string       `json:"description"`
	Category       category     `json:"category"`
	ServiceRegions []string     `json:"serviceRegions"`
	PricingInfo    []pricingRow `json:"pricingInfo"`
}

type category struct {
	ResourceGroup string `json:"resourceGroup"`
	UsageType     string `json:"usageType"`
}

type pricingRow struct {
	PricingExpression pricingExpression `json:"pricingExpression"`
}

type pricingExpression struct {
	UsageUnit       string       `json:"usageUnit"`
	DisplayQuantity int64        `json:"displayQuantity"`
	TieredRates     []tieredRate `json:"tieredRates"`
}

type tieredRate struct {
	StartUsageAmount float64   `json:"startUsageAmount"`
	UnitPrice        unitPrice `json:"unitPrice"`
}

type unitPrice struct {
	CurrencyCode string `json:"currencyCode"`
	Units        string `json:"units"`
	Nanos        int64  `json:"nanos"`
}

// Fetch walks the billing catalog, then reads the model documentation.
//
// The catalog needs a credential and the documentation does not, but the
// documentation alone describes no rate and names no model this catalog
// holds, so the credential is still what the run depends on.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	skus, err := p.fetchSKUs(ctx)
	if err != nil {
		return nil, err
	}
	docs := []catalog.Document{skus}
	index, err := p.getPage(ctx, ModelsURL)
	if err != nil {
		return docs, err
	}
	docs = append(docs, index)
	var failures []error
	for _, url := range append(modelPageURLs(index), sidePages()...) {
		page, err := p.getPage(ctx, url)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		docs = append(docs, page)
	}
	return docs, errors.Join(failures...)
}

// getPage retrieves one documentation page, which needs no credential.
func (p *Provider) getPage(
	ctx context.Context,
	url string,
) (catalog.Document, error) {
	if body, ok := p.readCacheFile(cacheName(url)); ok {
		return catalog.Document{URL: url, Body: body}, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return catalog.Document{}, err
	}
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return catalog.Document{}, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return catalog.Document{}, fmt.Errorf("fetch %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return catalog.Document{}, fmt.Errorf("read %s: %w", url, err)
	}
	p.writeCacheFile(cacheName(url), body)
	return catalog.Document{URL: url, Body: body}, nil
}

// cacheName turns a documentation URL into a flat filename.
func cacheName(url string) string {
	trimmed := strings.TrimPrefix(url, docsBase+"/")
	return providerID + "_" + strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '.', r == '-', r == '_':
			return r
		}
		return '_'
	}, trimmed)
}

// fetchSKUs walks the catalog and returns every SKU as one document.
func (p *Provider) fetchSKUs(ctx context.Context) (catalog.Document, error) {
	if body, ok := p.readCache(); ok {
		return catalog.Document{URL: catalogURL, Body: body}, nil
	}
	token, err := p.accessToken(ctx)
	if err != nil {
		return catalog.Document{}, err
	}
	var (
		skus []sku
		next string
	)
	for pages := 0; pages < maxPages; pages++ {
		body, err := p.get(ctx, token, next)
		if err != nil {
			return catalog.Document{}, err
		}
		var current page
		if err := json.Unmarshal(body, &current); err != nil {
			return catalog.Document{}, fmt.Errorf(
				"decode %s: %w",
				catalogURL,
				err,
			)
		}
		skus = append(skus, current.SKUs...)
		next = current.NextPageToken
		if next == "" {
			break
		}
	}
	body, err := json.Marshal(page{SKUs: skus})
	if err != nil {
		return catalog.Document{}, err
	}
	p.writeCache(body)
	return catalog.Document{URL: catalogURL, Body: body}, nil
}

// accessToken finds a credential, preferring one given outright, then the
// environment, then whatever gcloud is signed in as.
func (p *Provider) accessToken(ctx context.Context) (string, error) {
	if p.Token != "" {
		return p.Token, nil
	}
	if token := strings.TrimSpace(os.Getenv(tokenEnv)); token != "" {
		return token, nil
	}
	out, err := exec.CommandContext(
		ctx, "gcloud", "auth", "print-access-token",
	).Output()
	if token := strings.TrimSpace(string(out)); err == nil && token != "" {
		return token, nil
	}
	return "", ErrUnconfigured
}

// get retrieves one page of the catalog.
func (p *Provider) get(
	ctx context.Context,
	token, pageToken string,
) ([]byte, error) {
	target := fmt.Sprintf("%s?pageSize=%d", catalogURL, pageSize)
	if pageToken != "" {
		target += "&pageToken=" + pageToken
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
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

// Parse reads the SKUs first, because they are the only document naming the
// models Vertex bills for, then the model pages onto what they established.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	var failures []error
	var pages []catalog.Document
	var skus []sku
	source := ""
	for _, doc := range docs {
		if doc.URL != catalogURL {
			pages = append(pages, doc)
			continue
		}
		var current page
		if err := json.Unmarshal(doc.Body, &current); err != nil {
			failures = append(failures, fmt.Errorf("decode: %w", err))
			continue
		}
		skus = append(skus, current.SKUs...)
		source = doc.URL
	}
	documented := readDocumented(pages)
	b.applySKUs(skus, source, documented)
	b.applyModelPages(pages, documented)
	return b.result(), errors.Join(failures...)
}

// applySKUs records every rate the catalog states.
//
// The descriptions that say what they count are read first, because a
// description that stops before saying it names a model only by what is left
// once every other word is taken out, and the leftovers of Google's
// per-product meters read as models too: "CodeMender Gemini 3.1 Pro Global
// Text Output" leaves a CodeMender Gemini 3.1 Pro that Vertex does not serve.
// Such a description is read only where some other document has already named
// the model it leaves, which is what tells a caching rate on a model Google
// documents from a meter on a product of Google's own. Without reading them at
// all, every caching rate and every batch rate Vertex charges for a Gemini
// model went unrecorded.
func (b *builder) applySKUs(
	skus []sku,
	source string,
	pages map[string]*documented,
) {
	var deferred []sku
	for _, s := range skus {
		if _, ok := tokenRate(s); !ok {
			continue
		}
		read, ok := readDescription(s.Description)
		if !ok {
			continue
		}
		if read.bare {
			deferred = append(deferred, s)
			continue
		}
		b.applySKU(s, read, source)
	}
	names := slices.Sorted(maps.Keys(pages))
	for _, s := range deferred {
		read, ok := readDescription(s.Description)
		if !ok || !b.named(slugID(read.model), names, pages) {
			continue
		}
		b.applySKU(s, read, source)
	}
}

// named reports a model some other document has already named, either a rate
// whose description says what it counts or a page of the model's own.
func (b *builder) named(
	id string,
	names []string,
	pages map[string]*documented,
) bool {
	if _, ok := b.models[id]; ok {
		return true
	}
	return matchPage(id, names, pages) != ""
}

// tokenRate returns the pricing expression of a SKU quoted per token,
// reporting false for a SKU quoted for anything else.
func tokenRate(s sku) (pricingExpression, bool) {
	if len(s.PricingInfo) == 0 {
		return pricingExpression{}, false
	}
	expression := s.PricingInfo[0].PricingExpression
	if expression.UsageUnit != "count" ||
		expression.DisplayQuantity != tokenQuantity ||
		len(expression.TieredRates) == 0 {
		return pricingExpression{}, false
	}
	return expression, true
}

// applySKU records one rate against the model named in its description.
func (b *builder) applySKU(s sku, read reading, source string) {
	expression, ok := tokenRate(s)
	if !ok {
		return
	}
	metric, ok := metricFor(read)
	if !ok {
		return
	}
	rate := expression.TieredRates[len(expression.TieredRates)-1]
	amount, ok := amountOf(
		rate.UnitPrice.Units,
		rate.UnitPrice.Nanos,
		expression.DisplayQuantity,
	)
	if !ok {
		return
	}
	id := slugID(read.model)
	m := b.model(id, kindFor(read.model))
	m.AddSource(source)
	if m.Name == "" {
		m.Name = read.model
	}
	currency := rate.UnitPrice.CurrencyCode
	if currency == "" {
		currency = defaultCurrency
	}
	context := ""
	if read.longCtx {
		context = "long"
	}
	m.AddPrice(catalog.Price{
		Metric:   metric,
		Unit:     UnitPer1MTokens,
		Amount:   amount,
		Currency: currency,
		Dims: catalog.Dims{}.
			With(DimDeployment, read.deployment).
			With(DimTier, read.tier).
			With(DimModality, read.modality).
			With(DimStage, read.stage).
			With(DimContext, context),
	})
}

// defaultCurrency is what the catalog quotes when a rate states none.
const defaultCurrency = "USD"

// readCache returns a previously fetched listing.
func (p *Provider) readCache() ([]byte, bool) {
	return p.readCacheFile(cacheFile)
}

// writeCache stores a listing.
func (p *Provider) writeCache(body []byte) {
	p.writeCacheFile(cacheFile, body)
}

// readCacheFile returns a previously fetched body.
func (p *Provider) readCacheFile(name string) ([]byte, bool) {
	if p.CacheDir == "" {
		return nil, false
	}
	body, err := os.ReadFile(filepath.Join(p.CacheDir, name))
	return body, err == nil
}

// writeCacheFile stores a body, ignoring failures because the cache is an
// optimization and never the source of truth.
func (p *Provider) writeCacheFile(name string, body []byte) {
	if p.CacheDir == "" {
		return
	}
	if err := os.MkdirAll(p.CacheDir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(p.CacheDir, name), body, 0o644)
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
