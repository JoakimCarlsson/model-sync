package deepseek

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

// currency is the only currency DeepSeek quotes.
const currency = "USD"

// providerID and providerName identify this source in the catalog.
const (
	providerID   = "deepseek"
	providerName = "DeepSeek"
)

// The pages read. The trailing slash is load bearing on all of them. Without
// it the site answers with its home page, the guide to a first API call, at a
// 200 and with no redirect, so the fetch succeeds and the document carries one
// table of base URLs and no model. That is indistinguishable from a parser
// failure except by reading the page.
const (
	// PricingURL is the page stating DeepSeek's models and rates.
	PricingURL = baseURL + "/quick_start/pricing/"
	// ChangeLogURL is where DeepSeek writes each model's name, since the
	// pricing table heads its columns with the identifier instead.
	ChangeLogURL = baseURL + "/updates/"
	// ResponsesGuideURL states which content parts a request and a response
	// carry, which is where DeepSeek says what its models take and return.
	ResponsesGuideURL = baseURL + "/guides/responses_api/"
)

const baseURL = "https://api-docs.deepseek.com"

// Provider reads DeepSeek's pricing page. The zero value is not usable; call
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

// Fetch retrieves the pricing page, then the two pages stating what the
// pricing page leaves out.
//
// Only the pricing page says which models exist, so its failure ends the run.
// A guide that cannot be read costs those models a name or a modality and
// nothing else, so the failure is reported and the rest of the run continues.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	pricing, err := p.get(ctx, PricingURL)
	if err != nil {
		return nil, err
	}
	docs := []catalog.Document{pricing}
	var failures []error
	for _, url := range []string{ChangeLogURL, ResponsesGuideURL} {
		doc, err := p.get(ctx, url)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		docs = append(docs, doc)
	}
	return docs, errors.Join(failures...)
}

// Parse reads the pricing page first, because it is the only document saying
// which models DeepSeek serves; the guides state facts about models it has
// already named.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	for _, doc := range docs {
		if doc.URL == PricingURL {
			b.applyPricing(doc)
		}
	}
	for _, doc := range docs {
		switch doc.URL {
		case ChangeLogURL:
			b.applyChangeLog(doc)
		case ResponsesGuideURL:
			b.applyResponsesGuide(doc)
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
	trimmed := strings.Trim(strings.TrimPrefix(url, baseURL), "/")
	return providerID + "_" + strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '.', r == '-', r == '_':
			return r
		}
		return '_'
	}, trimmed) + ".html"
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
