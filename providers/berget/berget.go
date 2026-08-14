package berget

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// defaultCurrency is what Berget quotes when it states no currency of its
// own. It is euros, not dollars, which is why every price carries the
// currency it was quoted in.
const defaultCurrency = "EUR"

// providerID and providerName identify this source in the catalog.
const (
	providerID   = "berget"
	providerName = "Berget"
)

// Documents Berget publishes that this parser reads.
const (
	// ModelsURL lists every model Berget serves, with its rate and what it
	// can do.
	ModelsURL = "https://api.berget.ai/v1/models"
	// OverviewURL states each model's context window, which the endpoint
	// does not.
	OverviewURL = "https://docs.berget.ai/models/overview"
	// OpenAPIURL is the API's own specification, which states what each
	// endpoint accepts.
	OpenAPIURL = "https://api.berget.ai/openapi.json"
)

// cacheFiles are where a fetched document is kept, one per document.
var cacheFiles = map[string]string{
	ModelsURL:   "berget_models.json",
	OverviewURL: "berget_overview.html",
	OpenAPIURL:  "berget_openapi.json",
}

// Provider reads Berget's model API. The zero value is not usable; call New.
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

// Fetch retrieves the model listing, the overview and the specification. The
// listing is the only one a model cannot be built without: a missing overview
// costs the context windows and a missing specification the request
// parameters, so what did arrive is returned rather than the run failing.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	listing, err := p.get(ctx, ModelsURL)
	if err != nil {
		return nil, err
	}
	docs := []catalog.Document{listing}
	var failures []error
	for _, url := range []string{OverviewURL, OpenAPIURL} {
		doc, err := p.get(ctx, url)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		docs = append(docs, doc)
	}
	return docs, errors.Join(failures...)
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

// Parse decodes the listing, then reads the overview and the specification
// onto the models it established.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	var failures []error
	for _, doc := range docs {
		if doc.URL != ModelsURL {
			continue
		}
		if err := b.applyListing(doc); err != nil {
			failures = append(failures, err)
		}
	}
	for _, doc := range docs {
		switch doc.URL {
		case OverviewURL:
			b.applyOverview(doc)
		case OpenAPIURL:
			if err := b.applySpec(doc); err != nil {
				failures = append(failures, err)
			}
		}
	}
	return b.result(), errors.Join(failures...)
}

// readCache returns a previously fetched body.
func (p *Provider) readCache(url string) ([]byte, bool) {
	if p.CacheDir == "" {
		return nil, false
	}
	body, err := os.ReadFile(filepath.Join(p.CacheDir, cacheFiles[url]))
	return body, err == nil
}

// writeCache stores a response, ignoring failures because the cache is an
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
