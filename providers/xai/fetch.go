package xai

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

const baseURL = "https://docs.x.ai"

// Documents xAI publishes that this parser reads. Appending .md to a doc URL
// returns its markdown source.
const (
	PricingURL   = baseURL + "/developers/pricing.md"
	ModelsURL    = baseURL + "/developers/models.md"
	modelPagePre = baseURL + "/developers/models/"
)

// fetchWorkers bounds the concurrent requests made for the per-model pages.
const fetchWorkers = 8

// Fetch retrieves the pricing and models pages, then one page per model named
// by their rate tables.
//
// xAI publishes no index of model pages, so the list is derived from the
// models it charges for. A model absent from every rate table has no page
// fetched, which is correct: there is nothing to bill it by.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	pricing, err := p.get(ctx, PricingURL)
	if err != nil {
		return nil, err
	}
	docs := []catalog.Document{pricing}
	var failures []error
	if models, err := p.get(ctx, ModelsURL); err == nil {
		docs = append(docs, models)
	} else {
		failures = append(failures, err)
	}
	pages, pageErrs := p.getAll(ctx, modelPageURLs(pricing))
	docs = append(docs, pages...)
	return docs, errors.Join(append(failures, pageErrs...)...)
}

// modelPageURLs derives the per-model page of every model the pricing tables
// name.
func modelPageURLs(pricing catalog.Document) []string {
	var urls []string
	add := func(id string) {
		if id == "" || strings.Contains(id, " ") {
			return
		}
		url := modelPagePre + id + ".md"
		if !slices.Contains(urls, url) {
			urls = append(urls, url)
		}
	}
	for _, t := range scanTables(string(pricing.Body), pricing.URL) {
		switch t.Section {
		case sectionText:
			at := columnOf(t.Headers, "model")
			for _, row := range t.Rows {
				add(splitModelCell(cellAt(row, at)).ID)
			}
		case sectionImagine:
			at := columnOf(t.Headers, "model")
			for _, row := range t.Rows {
				add(clean(cellAt(row, at)))
			}
		case sectionVoice:
			at := columnOf(t.Headers, "mode")
			for _, row := range t.Rows {
				id, _ := voiceID(clean(cellAt(row, at)))
				add(id)
			}
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
		return catalog.Document{}, fmt.Errorf(
			"fetch %s: %s",
			url,
			resp.Status,
		)
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
