package together

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
	"sync"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// currency is the only currency Together quotes.
const currency = "USD"

// providerID and providerName identify this source in the catalog.
const (
	providerID   = "together"
	providerName = "Together AI"
)

// CatalogURL is the page listing every model Together serves and its rate.
const CatalogURL = "https://docs.together.ai/docs/serverless/models.md"

// ReasoningURL is the page listing which of those models reason. The catalog
// has a column for tool calling and one for structured output and none for
// this, so it is stated here instead, in a table of its own.
const ReasoningURL = "https://docs.together.ai/docs/inference/chat/reasoning.md"

// fetchWorkers bounds the concurrent requests made for the pages named by an
// index.
const fetchWorkers = 8

// Provider reads Together's model catalog. The zero value is not usable; call
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

// Fetch retrieves the catalog page, then everything that says more about the
// models it names: the reasoning page, the per-model guides the documentation
// index lists, and the page each model has in the model library.
//
// Only the catalog is required. It is the one document naming the models, and
// every other page here answers about a model the catalog already established,
// so one that cannot be read costs a field rather than the whole provider.
//
// Neither index is returned. A sitemap and a list of pages name documents and
// state nothing about a model, so they are read here and no further.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	cat, err := p.get(ctx, CatalogURL)
	if err != nil {
		return nil, err
	}
	docs := []catalog.Document{cat}
	var failures []error
	for _, url := range []string{
		ReasoningURL,
		DeprecationsURL,
		ChangelogURL,
		DedicatedURL,
	} {
		doc, err := p.get(ctx, url)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		docs = append(docs, doc)
	}
	for _, source := range []struct {
		index string
		urls  func(catalog.Document) []string
	}{
		{GuideIndexURL, guideURLs},
		{LibraryIndexURL, libraryURLs},
	} {
		index, err := p.get(ctx, source.index)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		pages, errs := p.getAll(ctx, source.urls(index))
		docs = append(docs, pages...)
		failures = append(failures, errs...)
	}
	return docs, errors.Join(failures...)
}

// getAll retrieves urls concurrently, returning the documents in the order the
// urls were given so a run is reproducible.
func (p *Provider) getAll(
	ctx context.Context,
	urls []string,
) ([]catalog.Document, []error) {
	docs := make([]catalog.Document, len(urls))
	errs := make([]error, len(urls))
	var wg sync.WaitGroup
	work := make(chan int)
	for range min(fetchWorkers, len(urls)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				docs[i], errs[i] = p.get(ctx, urls[i])
			}
		}()
	}
	for i := range urls {
		work <- i
	}
	close(work)
	wg.Wait()

	out := make([]catalog.Document, 0, len(urls))
	var failures []error
	for i := range urls {
		if errs[i] != nil {
			failures = append(failures, errs[i])
			continue
		}
		out = append(out, docs[i])
	}
	return out, failures
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

// Parse reads the catalog page first, because it is the only document naming
// the models, then every other page onto the models it established.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	for _, doc := range docs {
		if doc.URL == CatalogURL {
			b.applyCatalog(doc)
		}
	}
	for _, doc := range docs {
		switch {
		case doc.URL == CatalogURL:
		case doc.URL == ReasoningURL:
			b.applyReasoning(doc)
		case doc.URL == DeprecationsURL:
			b.applyDeprecations(doc)
		case doc.URL == ChangelogURL:
			b.applyChangelog(doc)
		case doc.URL == DedicatedURL:
			b.applyDedicated(doc)
		case strings.HasPrefix(doc.URL, LibraryPre):
			b.applyLibrary(doc)
		default:
			b.applyGuide(doc)
		}
	}
	return b.result(), nil
}

// readCache returns a previously fetched document.
func (p *Provider) readCache(url string) ([]byte, bool) {
	if p.CacheDir == "" {
		return nil, false
	}
	body, err := os.ReadFile(filepath.Join(p.CacheDir, cacheName(url)))
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
	_ = os.WriteFile(filepath.Join(p.CacheDir, cacheName(url)), body, 0o644)
}

// cacheName turns a URL into a flat filename.
func cacheName(url string) string {
	trimmed := strings.TrimPrefix(url, "https://")
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
