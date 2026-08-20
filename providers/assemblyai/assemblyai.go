package assemblyai

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// currency is the only currency AssemblyAI quotes.
const currency = "USD"

// providerID and providerName identify this source in the catalog.
const (
	providerID   = "assemblyai"
	providerName = "AssemblyAI"
)

// ModelsURL is the page describing every model and its rate.
const ModelsURL = "https://www.assemblyai.com/docs/getting-started/models.md"

// sourceURLs are the documents this provider reads. The two documents naming
// models come first, and the rest are read onto what they established.
var sourceURLs = append([]string{
	ModelsURL,
	GatewayModelsURL,
	PrerecordedModelsURL,
	StreamingModelsURL,
	LanguagesURL,
	LimitsURL,
	TimestampsURL,
	GatewaySpecURL,
	PrerecordedLimitsURL,
	StreamingLimitsURL,
	GatewayLimitsURL,
	PricingURL,
}, featureURLs...)

// Provider reads AssemblyAI's published documentation. The zero value is not
// usable; call New.
type Provider struct {
	// Client performs the fetch.
	Client *http.Client
}

// New returns a Provider using the default HTTP client.
func New() *Provider {
	return &Provider{Client: http.DefaultClient}
}

// ID implements catalog.Source.
func (p *Provider) ID() string { return providerID }

// Name implements catalog.Source.
func (p *Provider) Name() string { return providerName }

// Fetch retrieves every document this provider reads.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	docs := make([]catalog.Document, 0, len(sourceURLs))
	for _, url := range sourceURLs {
		doc, err := p.get(ctx, url)
		if err != nil {
			return docs, err
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

// get retrieves one document.
func (p *Provider) get(
	ctx context.Context,
	url string,
) (catalog.Document, error) {
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
	return catalog.Document{URL: url, Body: body}, nil
}

// Parse reads the documents in three passes, because what AssemblyAI states
// about a model is spread across documents that only make sense in an order.
//
// The first pass is the two rosters, the models page and the gateway models
// page, since every other document says something about a model one of them
// named. The second reads onto those: the identifier a request uses, the
// bounds, the capability matrix, the feature pages. The pricing page is last,
// because a feature page is what says a feature exists and the pricing page
// only says what it costs.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	for _, doc := range docs {
		switch doc.URL {
		case ModelsURL:
			b.applyModels(doc)
		case GatewayModelsURL:
			b.applyGateway(doc)
		}
	}
	for _, doc := range docs {
		switch doc.URL {
		case PrerecordedModelsURL, StreamingModelsURL:
			b.applySelection(doc)
		case LanguagesURL:
			b.applyLanguages(doc)
		case LimitsURL:
			b.applyLimits(doc)
		case TimestampsURL:
			b.applyTimestamps(doc)
		case GatewaySpecURL:
			b.applyGatewaySpec(doc)
		case PrerecordedLimitsURL, StreamingLimitsURL, GatewayLimitsURL:
			b.applyRateLimits(doc)
		default:
			if slices.Contains(featureURLs, doc.URL) {
				b.applyFeature(doc)
			}
		}
	}
	for _, doc := range docs {
		if doc.URL == PricingURL {
			b.applyPricing(doc)
		}
	}
	return b.result(), nil
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
