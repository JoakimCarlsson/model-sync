package mistral

import (
	"context"
	"encoding/json"
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

const baseURL = "https://docs.mistral.ai"

// Documents Mistral publishes that this parser reads.
const (
	// ModelsURL indexes every model page and carries the deprecation table.
	ModelsURL = baseURL + "/models"
	// modelPagePre prefixes one model's page.
	modelPagePre = baseURL + "/models/"
)

// notModels are the pages filed beside the model pages that describe no model.
var notModels = []string{"overview", "model-selection-guide"}

// fetchWorkers bounds the concurrent requests made for the per-model pages.
const fetchWorkers = 8

// modelHrefRe matches a link to one model's page in the index's navigation.
var modelHrefRe = regexp.MustCompile(`"href":"/models/([a-z0-9._-]+)"`)

// Fetch retrieves the index, then one page per model it links to.
//
// The index names every model but describes none of them: what a model costs,
// holds and can do is stated only on its own page. Fetching all of them is
// therefore the whole of the data, not an optimization over a summary.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	index, err := p.get(ctx, ModelsURL, false)
	if err != nil {
		return nil, err
	}
	pages, failures := p.getAll(ctx, modelPageURLs(index))
	return append([]catalog.Document{index}, pages...), errors.Join(failures...)
}

// modelPageURLs derives the per-model pages the index links to.
func modelPageURLs(index catalog.Document) []string {
	body := flight(index.Body)
	var slugs []string
	for _, match := range modelHrefRe.FindAllStringSubmatch(body, -1) {
		if slices.Contains(notModels, match[1]) ||
			slices.Contains(slugs, match[1]) {
			continue
		}
		slugs = append(slugs, match[1])
	}
	slices.Sort(slugs)
	urls := make([]string, 0, len(slugs))
	for _, slug := range slugs {
		urls = append(urls, modelPagePre+slug)
	}
	return urls
}

// flight returns the React flight payload a page carries.
//
// Requesting a page with the RSC header serves the payload on its own, which
// is what this asks for. A response that arrives as a rendered document
// instead has the same payload embedded in it a piece at a time, each piece a
// JSON string, so those are decoded and rejoined.
func flight(body []byte) string {
	text := string(body)
	if !strings.Contains(text, "self.__next_f.push") {
		return text
	}
	var out strings.Builder
	for _, match := range pushRe.FindAllStringSubmatch(text, -1) {
		var piece string
		if err := json.Unmarshal([]byte(match[1]), &piece); err != nil {
			continue
		}
		out.WriteString(piece)
	}
	return out.String()
}

// pushRe matches one piece of an embedded flight payload.
var pushRe = regexp.MustCompile(
	`(?s)self\.__next_f\.push\(\[1,("(?:[^"\\]|\\.)*")\]\)`,
)

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
				docs[i], errs[i] = p.get(ctx, urls[i], true)
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
//
// The RSC header asks for the flight payload without the document rendered
// around it, which is the same data a fifth smaller, and is asked for wherever
// only the payload is read. The index is fetched rendered because its
// deprecation table is markup rather than payload. A server that ignores the
// header returns the rendered document and the parser reads that instead.
func (p *Provider) get(
	ctx context.Context,
	url string,
	flightOnly bool,
) (catalog.Document, error) {
	if body, ok := p.readCache(url); ok {
		return catalog.Document{URL: url, Body: body}, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return catalog.Document{}, err
	}
	if flightOnly {
		req.Header.Set("RSC", "1")
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
