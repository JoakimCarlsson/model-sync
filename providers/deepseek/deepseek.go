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

// The pages read. The trailing slash is load bearing on all of the
// api-docs.deepseek.com ones. Without it the site answers with its home page,
// the guide to a first API call, at a 200 and with no redirect, so the fetch
// succeeds and the document carries one table of base URLs and no model. That
// is indistinguishable from a parser failure except by reading the page.
const (
	// PricingURL is the page stating DeepSeek's models and rates.
	PricingURL = baseURL + "/quick_start/pricing/"
	// ChangeLogURL is where DeepSeek writes each model's name, dates its
	// release and says what it is, since the pricing table heads its columns
	// with the identifier and states nothing else in prose.
	ChangeLogURL = baseURL + "/updates/"
	// ResponsesGuideURL states which content parts a request and a response
	// carry, which is where DeepSeek says what its models take and return, and
	// which request parameters it honours.
	ResponsesGuideURL = baseURL + "/guides/responses_api/"
	// ThinkingGuideURL states the effort levels the thinking mode accepts and
	// what it does when the caller asks for none of them.
	ThinkingGuideURL = baseURL + "/guides/thinking_mode/"
	// FIMGuideURL states the beta endpoint's own base URL and its own output
	// ceiling, neither of which the pricing table carries.
	FIMGuideURL = baseURL + "/guides/fim_completion/"
	// CacheGuideURL states what the two input rates on the pricing table are
	// distinguished by.
	CacheGuideURL = baseURL + "/guides/kv_cache/"
	// RateLimitURL is the only page stating a limit on how hard the API may be
	// called, which the pricing table's concurrency row points at.
	RateLimitURL = baseURL + "/quick_start/rate_limit/"
)

// The Hugging Face cards. DeepSeek releases the weights of both models and
// links them from its release announcement; the card is where it states the
// licence, the parameter counts and the precision the weights carry.
const (
	ProCardURL   = cardBase + "DeepSeek-V4-Pro/raw/main/README.md"
	FlashCardURL = cardBase + "DeepSeek-V4-Flash/raw/main/README.md"
)

// modelCards names the model each card is the card of. A card states one
// licence and it is the licence of the repository serving the card, so the
// card has to be attributed to a model rather than to the series it describes.
var modelCards = map[string]string{
	ProCardURL:   "deepseek-v4-pro",
	FlashCardURL: "deepseek-v4-flash",
}

const (
	baseURL  = "https://api-docs.deepseek.com"
	cardBase = "https://huggingface.co/deepseek-ai/"
)

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

// secondaryURLs are the pages read after the pricing page, in the order they
// are read. None of them names a model, so each one failing costs the models
// one group of facts and nothing else.
var secondaryURLs = []string{
	ChangeLogURL,
	ResponsesGuideURL,
	ThinkingGuideURL,
	FIMGuideURL,
	CacheGuideURL,
	RateLimitURL,
	ProCardURL,
	FlashCardURL,
}

// Fetch retrieves the pricing page, then the pages stating what the pricing
// page leaves out.
//
// Only the pricing page says which models exist, so its failure ends the run.
// A page that cannot be read costs those models a group of facts and nothing
// else, so the failure is reported and the rest of the run continues.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	pricing, err := p.get(ctx, PricingURL)
	if err != nil {
		return nil, err
	}
	docs := []catalog.Document{pricing}
	var failures []error
	for _, url := range secondaryURLs {
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
// which models DeepSeek serves; every other document states facts about models
// it has already named.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	for _, doc := range docs {
		if doc.URL == PricingURL {
			b.applyPricing(doc)
		}
	}
	for _, doc := range docs {
		if id, ok := modelCards[doc.URL]; ok {
			b.applyModelCard(doc, id)
			continue
		}
		switch doc.URL {
		case ChangeLogURL:
			b.applyChangeLog(doc)
		case ResponsesGuideURL:
			b.applyResponsesGuide(doc)
		case ThinkingGuideURL:
			b.applyThinkingGuide(doc)
		case FIMGuideURL:
			b.applyFIMGuide(doc)
		case CacheGuideURL:
			b.applyCacheGuide(doc)
		case RateLimitURL:
			b.applyRateLimitPage(doc)
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

// cacheName turns a URL into a flat filename. The documentation host is
// trimmed off because every path under it is distinct on its own; anything
// else keeps its host, so that two cards differing only in repository do not
// collide.
func cacheName(url string) string {
	trimmed := strings.Trim(strings.TrimPrefix(url, baseURL), "/")
	trimmed = strings.TrimPrefix(trimmed, "https://")
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
	// rate is what the pricing rows beneath the current section label are
	// charged for, carried because that label spans two rows and the row
	// beneath it states only a period and its amounts.
	rate catalog.Metric
	// priceNote is the pricing table's footnote naming the hours the peak
	// period covers, which qualifies every rate on the table.
	priceNote string
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

// applyAll records a fact stated of the API against every model, in
// identifier order so that no map iteration reaches the output.
func (b *builder) applyAll(source string, set func(*catalog.Model)) {
	for _, id := range b.order {
		m := b.models[id]
		set(m)
		m.AddSource(source)
	}
}
