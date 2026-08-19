package assemblyai

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

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

// cacheFile names the file one document is kept in, derived from the document
// so that adding a source does not mean maintaining a second list. Two
// documents of the same name under different sections keep their sections in
// the file name, which is why the whole path is used and not the last segment.
func cacheFile(url string) string {
	trimmed := strings.TrimPrefix(url, "https://www.assemblyai.com/")
	name := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' {
			return r
		}
		return '_'
	}, strings.ToLower(trimmed))
	if !strings.Contains(name, ".") {
		name += ".html"
	}
	return providerID + "_" + name
}

// Provider reads AssemblyAI's published documentation. The zero value is not
// usable; call New.
type Provider struct {
	// Client performs the fetch.
	Client *http.Client
	// CacheDir, when set, backs the fetch with a file on disk.
	CacheDir string
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

// get retrieves one document, reading from and writing to the cache directory
// when one is configured.
func (p *Provider) get(
	ctx context.Context,
	url string,
) (catalog.Document, error) {
	if body, ok := p.readCache(url); ok {
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
	p.writeCache(url, body)
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

// readCache returns a previously fetched document.
func (p *Provider) readCache(url string) ([]byte, bool) {
	if p.CacheDir == "" {
		return nil, false
	}
	body, err := os.ReadFile(filepath.Join(p.CacheDir, cacheFile(url)))
	return body, err == nil
}

// writeCache stores a document, ignoring failures because the cache is an
// optimization and never the source of truth.
func (p *Provider) writeCache(url string, body []byte) {
	if p.CacheDir == "" {
		return
	}
	if err := os.MkdirAll(p.CacheDir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(p.CacheDir, cacheFile(url)), body, 0o644)
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
