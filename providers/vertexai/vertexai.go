package vertexai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// Fetch walks the catalog and returns every SKU as one document.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	if body, ok := p.readCache(); ok {
		return []catalog.Document{{URL: catalogURL, Body: body}}, nil
	}
	token, err := p.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	var (
		skus []sku
		next string
	)
	for pages := 0; pages < maxPages; pages++ {
		body, err := p.get(ctx, token, next)
		if err != nil {
			return nil, err
		}
		var current page
		if err := json.Unmarshal(body, &current); err != nil {
			return nil, fmt.Errorf("decode %s: %w", catalogURL, err)
		}
		skus = append(skus, current.SKUs...)
		next = current.NextPageToken
		if next == "" {
			break
		}
	}
	body, err := json.Marshal(page{SKUs: skus})
	if err != nil {
		return nil, err
	}
	p.writeCache(body)
	return []catalog.Document{{URL: catalogURL, Body: body}}, nil
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

// Parse reads the collected SKUs.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	var failures []error
	for _, doc := range docs {
		var current page
		if err := json.Unmarshal(doc.Body, &current); err != nil {
			failures = append(failures, fmt.Errorf("decode: %w", err))
			continue
		}
		for _, s := range current.SKUs {
			b.applySKU(s, doc.URL)
		}
	}
	return b.result(), errors.Join(failures...)
}

// applySKU records one rate against the model named in its description.
func (b *builder) applySKU(s sku, source string) {
	if len(s.PricingInfo) == 0 {
		return
	}
	expression := s.PricingInfo[0].PricingExpression
	if expression.UsageUnit != "count" ||
		expression.DisplayQuantity != tokenQuantity ||
		len(expression.TieredRates) == 0 {
		return
	}
	read, ok := readDescription(s.Description)
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
	m := b.model(id, KindChat)
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
	if p.CacheDir == "" {
		return nil, false
	}
	body, err := os.ReadFile(filepath.Join(p.CacheDir, cacheFile))
	return body, err == nil
}

// writeCache stores a listing, ignoring failures because the cache is an
// optimization and never the source of truth.
func (p *Provider) writeCache(body []byte) {
	if p.CacheDir == "" {
		return
	}
	if err := os.MkdirAll(p.CacheDir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(p.CacheDir, cacheFile), body, 0o644)
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
