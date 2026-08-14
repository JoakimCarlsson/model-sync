package google

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

const baseURL = "https://ai.google.dev"

// Documents Google publishes that this parser reads.
const (
	// PricingURL states the Gemini API's rates.
	PricingURL = baseURL + "/gemini-api/docs/pricing"
	// ModelsURL indexes the page each model has of its own.
	ModelsURL = baseURL + "/gemini-api/docs/models"
	// modelPagePre prefixes one model's page.
	modelPagePre = baseURL + "/gemini-api/docs/models/"
)

// fetchWorkers bounds the concurrent requests made for the per-model pages.
const fetchWorkers = 8

// modelHrefRe matches a link from the index to one model's page.
var modelHrefRe = regexp.MustCompile(
	`href="/gemini-api/docs/models/([a-z0-9._-]+)"`,
)

// errNoPage reports that Google publishes nothing at a URL this parser
// derived from a model code rather than read from a link. Most codes have a
// page and the ones that do not answer with a 404, which is an answer and not
// a fault.
var errNoPage = errors.New("no model page")

// Fetch retrieves the pricing page, the model index and one page per model,
// both the ones the index links to and the ones only the pricing page names.
//
// The pricing page is the authority on rates and names every model that costs
// anything, but states nothing a model holds or can do. That is on the model's
// own page, which states no rate. The index is the third: it is the only
// document pairing the name a model is sold under with the endpoint it answers
// to, and the only one saying which models Google has withdrawn.
//
// The index does not link every page. Four models it either omits or lists
// without a link have a page all the same, addressed by the identifier the
// pricing page states beneath the heading, so those addresses are tried too
// and a 404 among them is read as Google having no page rather than as a
// failed fetch.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	pricing, err := p.get(ctx, PricingURL)
	if err != nil {
		return nil, err
	}
	docs := []catalog.Document{pricing}
	index, err := p.get(ctx, ModelsURL)
	if err != nil {
		return docs, err
	}
	docs = append(docs, index)
	linked := modelPageURLs(index)
	pages, failures := p.getAll(ctx, linked)
	docs = append(docs, pages...)
	guessed, missing := p.getAll(ctx, codePageURLs(pricing, linked))
	docs = append(docs, guessed...)
	return docs, errors.Join(append(failures, published(missing)...)...)
}

// modelPageURLs derives the per-model pages the index links to.
func modelPageURLs(index catalog.Document) []string {
	var urls []string
	for _, match := range modelHrefRe.FindAllStringSubmatch(
		string(index.Body),
		-1,
	) {
		url := modelPagePre + match[1]
		if !slices.Contains(urls, url) {
			urls = append(urls, url)
		}
	}
	slices.Sort(urls)
	return urls
}

// codePageURLs derives the page each endpoint the pricing page names would
// have, less the ones the index already links.
func codePageURLs(pricing catalog.Document, linked []string) []string {
	var urls []string
	for _, code := range pricingCodes(pricing) {
		url := modelPagePre + code
		if slices.Contains(linked, url) || slices.Contains(urls, url) {
			continue
		}
		urls = append(urls, url)
	}
	return urls
}

// published drops the failures that are Google having no page at an address
// this parser derived rather than read.
func published(errs []error) []error {
	var out []error
	for _, err := range errs {
		if !errors.Is(err, errNoPage) {
			out = append(out, err)
		}
	}
	return out
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
	if resp.StatusCode == http.StatusNotFound {
		return catalog.Document{}, fmt.Errorf("fetch %s: %w", url, errNoPage)
	}
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
	trimmed := strings.TrimPrefix(url, baseURL+"/gemini-api/docs/")
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
