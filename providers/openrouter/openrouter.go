package openrouter

import (
	"context"
	"encoding/json"
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

// currency is the only currency OpenRouter quotes.
const currency = "USD"

// providerID and providerName identify this source in the catalog.
const (
	providerID   = "openrouter"
	providerName = "OpenRouter"
)

// ModelsURL is the endpoint listing every model OpenRouter brokers.
const ModelsURL = "https://openrouter.ai/api/v1/models"

// baseURL is the host the listing's own links are relative to.
const baseURL = "https://openrouter.ai"

// fetchWorkers bounds the concurrent requests made for the endpoint documents.
const fetchWorkers = 8

// Provider reads OpenRouter's model API. The zero value is not usable; call
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

// Fetch retrieves the model listing, then the endpoint document of every model
// the listing left short.
//
// The listing describes a model through the one upstream OpenRouter currently
// fronts for it, and that upstream does not always state a completion ceiling
// or forward a parameter that implies a capability. The rest of the upstreams
// serving the same model are a document away, so the listing is read first for
// the models it answers and the documents are fetched only for the ones it does
// not.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	models, err := p.get(ctx, ModelsURL)
	if err != nil {
		return nil, err
	}
	urls, err := detailURLs(models)
	if err != nil {
		return []catalog.Document{models}, err
	}
	details, failures := p.getAll(ctx, urls)
	return append([]catalog.Document{models}, details...),
		errors.Join(failures...)
}

// detailURLs derives the endpoint document of every model whose listing entry
// stated no completion ceiling or no capability at all. Several entries share
// one document, because a model and its batch and free variants are one model
// served three ways, so each document is fetched once.
func detailURLs(models catalog.Document) ([]string, error) {
	var list listing
	if err := json.Unmarshal(models.Body, &list); err != nil {
		return nil, fmt.Errorf("decode %s: %w", models.URL, err)
	}
	var urls []string
	for _, e := range list.Data {
		url := detailURL(e)
		if url == "" || !needsDetail(e) || slices.Contains(urls, url) {
			continue
		}
		urls = append(urls, url)
	}
	slices.Sort(urls)
	return urls, nil
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

// Parse reads the listing first, because it is the only document naming the
// models and the only one linking them to the endpoint documents that fill
// what it left out.
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
		if doc.URL == ModelsURL {
			continue
		}
		if err := b.applyDetail(doc); err != nil {
			failures = append(failures, err)
		}
	}
	return b.result(), errors.Join(failures...)
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
	// details maps an endpoint document to the models the listing linked to
	// it. The document names the model it describes, but under the identifier
	// of the base model rather than of the variant that linked to it, so the
	// link is what an endpoint document is attributed by.
	details map[string][]string
}

func newBuilder() *builder {
	return &builder{
		models:  map[string]*catalog.Model{},
		details: map[string][]string{},
	}
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
