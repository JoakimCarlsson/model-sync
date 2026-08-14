package ollama

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// providerID and providerName identify this source in the catalog.
const (
	providerID   = "ollama"
	providerName = "Ollama"
)

const baseURL = "https://ollama.com"

// LibraryURL is the page listing every model Ollama distributes.
const LibraryURL = baseURL + "/library"

// StructuredOutputsURL documents the one capability the library has no tag
// for. It belongs to the runtime rather than to a model, which is why no model
// is tagged with it.
const StructuredOutputsURL = "https://docs.ollama.com/capabilities/" +
	"structured-outputs.md"

// fetchWorkers bounds the concurrent requests made for the tag listings.
const fetchWorkers = 8

// Provider reads Ollama's model library. The zero value is not usable; call
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

// Fetch retrieves the library page, then the tag listing of every model on it,
// then the pages that answer what the library and the listings leave open.
//
// The library says which models exist, what each can do and where each runs,
// and states no bound on any of them. A model's tag listing states one per
// build, so the listings are fetched too, one per model.
//
// Two sets of pages are fetched on top of those, each for the models it says
// anything about. A model Ollama runs itself is the only kind with a price, so
// its own page is fetched for the rate or the usage level it quotes. And an
// embedding model's width is on the metadata page of its model layer, which is
// reached through the page of the build that layer belongs to.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	library, err := p.get(ctx, LibraryURL)
	if err != nil {
		return nil, err
	}
	index := newBuilder()
	index.applyLibrary(library)

	urls := append(tagListingURLs(library), StructuredOutputsURL)
	urls = append(urls, index.urlsWithAttr(AttrCloud)...)
	docs, failures := p.getAll(ctx, urls)

	widths, widthFailures := p.fetchWidths(ctx, index, docs)
	docs = append(docs, widths...)
	failures = append(failures, widthFailures...)

	return append([]catalog.Document{library}, docs...),
		errors.Join(failures...)
}

// urlsWithAttr gives the page of every model the library marked with attr.
func (b *builder) urlsWithAttr(attr string) []string {
	var urls []string
	for _, id := range b.order {
		if b.models[id].Attrs[attr] != "" {
			urls = append(urls, baseURL+"/library/"+id)
		}
	}
	return urls
}

// fetchWidths retrieves the metadata page of every embedding model's model
// layer, which takes two rounds: the build's page is what names the layer, and
// the layer's page is what states the width. The listings are the ones already
// fetched, since they are what name the build.
func (p *Provider) fetchWidths(
	ctx context.Context,
	index *builder,
	listings []catalog.Document,
) ([]catalog.Document, []error) {
	var builds []string
	for _, doc := range listings {
		if !strings.HasSuffix(doc.URL, tagsPath) {
			continue
		}
		m, ok := index.models[path.Base(strings.TrimSuffix(doc.URL, tagsPath))]
		if !ok || m.Kind != KindEmbedding {
			continue
		}
		if row := defaultRow(doc.Body); row != nil {
			builds = append(builds, baseURL+"/library/"+row[1])
		}
	}
	docs, failures := p.getAll(ctx, builds)

	var layers []string
	for _, doc := range docs {
		if match := modelBlobRe.FindSubmatch(doc.Body); match != nil {
			layers = append(layers, baseURL+string(match[1]))
		}
	}
	pages, layerFailures := p.getAll(ctx, layers)
	return pages, append(failures, layerFailures...)
}

// tagListingURLs derives the tag listing of every model the library names.
func tagListingURLs(library catalog.Document) []string {
	var urls []string
	for _, entry := range entryRe.FindAllStringSubmatch(
		string(library.Body),
		-1,
	) {
		id := strings.TrimSpace(entry[1])
		if id == "" {
			continue
		}
		url := baseURL + "/library/" + id + tagsPath
		if !slices.Contains(urls, url) {
			urls = append(urls, url)
		}
	}
	slices.Sort(urls)
	return urls
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

// Parse reads the library first, because it is the only document naming the
// models, then every other document onto the model it belongs to, which its
// URL says: a tag listing is filed under the model, a layer's metadata under
// the build it came from, and the one page stating a capability the library
// has no tag for is filed under neither.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	for _, doc := range docs {
		if doc.URL == LibraryURL {
			b.applyLibrary(doc)
		}
	}
	for _, doc := range docs {
		switch {
		case doc.URL == LibraryURL:
		case doc.URL == StructuredOutputsURL:
			b.applyStructuredOutputs(doc)
		case strings.Contains(doc.URL, blobsPath):
			b.applyBlob(doc)
		case strings.HasSuffix(doc.URL, tagsPath):
			b.applyTagListing(doc)
		default:
			b.applyModelPage(doc)
		}
	}
	return b.result(), nil
}

// readCache returns a previously fetched body.
func (p *Provider) readCache(url string) ([]byte, bool) {
	if p.CacheDir == "" {
		return nil, false
	}
	body, err := os.ReadFile(filepath.Join(p.CacheDir, cacheName(url)))
	return body, err == nil
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
