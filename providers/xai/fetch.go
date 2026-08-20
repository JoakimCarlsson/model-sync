package xai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
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
	markdown, variants := modelPageURLs(pricing)
	pages, pageErrs := p.getAll(ctx, append(markdown, variants...))
	docs = append(docs, pages...)
	return docs, errors.Join(append(failures, pageErrs...)...)
}

// modelPageURLs derives the per-model pages of every model the pricing tables
// name, returning the markdown pages and, separately, the rendered pages of
// the generation models.
//
// A voice row names two pages, not one. xAI generates a page under the model's
// identifier and writes the documentation under the mode's name, and only the
// second states the modalities and capabilities, so both are fetched.
//
// The rendered page is fetched only for the Imagine models, and only because
// their markdown is lossy: it states one headline rate where the page states a
// matrix by resolution and quality. Every other model's markdown is complete,
// and fetching its page too would double the requests for nothing.
func modelPageURLs(pricing catalog.Document) (markdown, variants []string) {
	add := func(urls *[]string, url string) {
		if !slices.Contains(*urls, url) {
			*urls = append(*urls, url)
		}
	}
	for _, t := range scanTables(string(pricing.Body), pricing.URL) {
		at := columnOf(t.Headers, "model", "mode")
		if at < 0 {
			continue
		}
		for _, row := range t.Rows {
			id := ""
			switch t.Section {
			case sectionText:
				id = splitModelCell(cellAt(row, at)).ID
			case sectionImagine:
				id = clean(cellAt(row, at))
			case sectionVoice:
				model, label, _ := voiceID(clean(cellAt(row, at)))
				id = model
				add(&markdown, modelPagePre+slugID(label)+".md")
			}
			if id == "" || strings.Contains(id, " ") {
				continue
			}
			add(&markdown, modelPagePre+id+".md")
			if t.Section == sectionImagine {
				add(&variants, modelPagePre+id)
			}
		}
	}
	slices.Sort(markdown)
	slices.Sort(variants)
	return markdown, variants
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

// get retrieves one document.
func (p *Provider) get(
	ctx context.Context,
	url string,
) (catalog.Document, error) {
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
	return catalog.Document{URL: url, Body: body}, nil
}
