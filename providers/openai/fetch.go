package openai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/joakimcarlsson/model-sync/catalog"
)

const baseURL = "https://developers.openai.com"

// Documents OpenAI publishes that this parser reads.
const (
	PricingURL     = baseURL + "/api/docs/pricing.md"
	ModelsIndexURL = baseURL + "/api/docs/models.md"
)

// GuideURLs are guides carrying rates stated nowhere else. The image
// generation guide holds the per-image dollar prices, which the pricing page
// states only per token.
var GuideURLs = []string{
	baseURL + "/api/docs/guides/image-generation.md",
}

// fetchWorkers bounds the concurrent requests made for the per-model pages.
const fetchWorkers = 8

var modelLinkRe = regexp.MustCompile(`\(/api/docs/models/([^)]+?)\.md\)`)

// Fetch retrieves the pricing page, the guides, and one page per model listed
// in the model index.
//
// A failure to retrieve the pricing page or the index is fatal and returns no
// documents. A failure on an individual model page is not: the remaining
// documents are returned along with a joined error naming what was missed, so
// a single removed page degrades the run instead of ending it.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	pricing, err := p.get(ctx, PricingURL)
	if err != nil {
		return nil, err
	}
	index, err := p.get(ctx, ModelsIndexURL)
	if err != nil {
		return nil, err
	}
	docs := []catalog.Document{pricing}
	var failures []error
	for _, url := range GuideURLs {
		doc, err := p.get(ctx, url)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		docs = append(docs, doc)
	}
	pages, pageErrs := p.getAll(ctx, modelPageURLs(index.Body))
	docs = append(docs, index)
	docs = append(docs, pages...)
	return docs, errors.Join(append(failures, pageErrs...)...)
}

// modelPageURLs returns the markdown page for every model the index links to.
func modelPageURLs(index []byte) []string {
	var urls []string
	for _, m := range modelLinkRe.FindAllStringSubmatch(string(index), -1) {
		url := baseURL + "/api/docs/models/" + m[1] + ".md"
		if !slices.Contains(urls, url) {
			urls = append(urls, url)
		}
	}
	slices.Sort(urls)
	return urls
}

// getAll retrieves urls concurrently and returns the documents in the order
// the urls were given, so a run is reproducible.
func (p *Provider) getAll(ctx context.Context, urls []string) ([]catalog.Document, []error) {
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
func (p *Provider) get(ctx context.Context, url string) (catalog.Document, error) {
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
	defer resp.Body.Close()
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
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-' {
			return r
		}
		return '_'
	}, trimmed)
}
