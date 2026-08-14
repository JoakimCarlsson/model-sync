package cerebras

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// providerID and providerName identify this source in the catalog.
const (
	providerID   = "cerebras"
	providerName = "Cerebras"
)

const baseURL = "https://inference-docs.cerebras.ai"

// CatalogURL lists every model Cerebras serves and links to its own page.
const CatalogURL = baseURL + "/models/overview.md"

// Provider reads Cerebras' model catalog. The zero value is not usable; call
// New.
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

// Fetch retrieves the catalog, the public model list, then one page per model
// the catalog links to.
//
// The catalog states what a model holds; its own page states what it costs,
// what it can do and what it takes, none of which the catalog repeats; and the
// public list is Cerebras answering which models it sells, with the two
// ceilings both documents round. A page that cannot be read costs that model
// its rates and nothing else, so the failure is reported and the rest of the
// run continues.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	index, err := p.get(ctx, CatalogURL)
	if err != nil {
		return nil, err
	}
	docs := []catalog.Document{index}
	var failures []error
	urls := append([]string{PublicModelsURL}, modelPageURLs(index)...)
	for _, url := range urls {
		page, err := p.get(ctx, url)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		docs = append(docs, page)
	}
	return docs, errors.Join(failures...)
}

// Parse reads the public list first and the catalog second, because between
// them they say which models Cerebras serves, how exactly each is bounded and
// under which standing it is offered. A model page adds to a model one of them
// has already named.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	for _, doc := range docs {
		if doc.URL != PublicModelsURL {
			continue
		}
		if err := b.applyPublicModels(doc); err != nil {
			return nil, err
		}
	}
	for _, doc := range docs {
		if doc.URL == CatalogURL {
			b.applyCatalog(doc)
			b.applyDeprecations(doc)
		}
	}
	for _, doc := range docs {
		if doc.URL != CatalogURL && doc.URL != PublicModelsURL {
			b.applyModelPage(doc)
		}
	}
	return b.result(), nil
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

// readCache returns a previously fetched body.
func (p *Provider) readCache(url string) ([]byte, bool) {
	if p.CacheDir == "" {
		return nil, false
	}
	body, err := os.ReadFile(filepath.Join(p.CacheDir, cacheName(url)))
	return body, err == nil
}

// writeCache stores a body, ignoring failures because the cache is an
// optimization and never the source of truth.
func (p *Provider) writeCache(url string, body []byte) {
	if p.CacheDir == "" {
		return
	}
	if err := os.MkdirAll(p.CacheDir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(p.CacheDir, cacheName(url)), body, 0o644)
}

// cacheName turns a URL into a flat filename.
func cacheName(url string) string {
	trimmed := strings.TrimPrefix(url, baseURL+"/")
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
