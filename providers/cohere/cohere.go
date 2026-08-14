package cohere

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

// providerID and providerName identify this source in the catalog.
const (
	providerID   = "cohere"
	providerName = "Cohere"
)

// Documents Cohere publishes that this parser reads.
const (
	// ModelsURL lists every model Cohere serves and what it holds.
	ModelsURL = "https://docs.cohere.com/docs/models.md"
	// PricingURL states the rates. They are on the marketing site rather than
	// in the documentation, which publishes none.
	PricingURL = "https://cohere.com/pricing"
)

// Provider reads Cohere's model overview and pricing page. The zero value is
// not usable; call New.
type Provider struct {
	// Client performs the fetches.
	Client *http.Client
	// CacheDir, when set, backs every fetch with a file on disk.
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

// Fetch retrieves the overview, the two pricing pages, the deprecation
// announcements, the pages of the two transcription models and the three
// capability guides. Only the overview is required: it is the one document
// naming the identifiers the API answers to, and without it nothing the others
// say can be attached to anything. A document that cannot be read costs what
// it alone states, so the rest are returned with the failure rather than
// instead of it.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	overview, err := p.get(ctx, ModelsURL)
	if err != nil {
		return nil, err
	}
	docs := []catalog.Document{overview}
	var failures []error
	for _, url := range []string{
		PricingURL,
		VaultPricingURL,
		DeprecationsURL,
		TranscribeURL,
		TranscribeArabicURL,
		StructuredOutputsURL,
		ToolUseURL,
		StreamingURL,
	} {
		doc, err := p.get(ctx, url)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		docs = append(docs, doc)
	}
	return docs, errors.Join(failures...)
}

// Parse reads the overview first, because it is the only document naming the
// identifiers the API answers to, then the documents that say which models
// exist and which are still served, and the rest last: the two pricing pages
// attach rates to models the earlier documents established, and the three
// guides attach capabilities to them the same way.
//
// A rate is never recorded against a withdrawn model, so the announcements
// have to be read before the amounts are.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	for _, doc := range docs {
		if doc.URL == ModelsURL {
			b.applyOverview(doc)
		}
	}
	for _, doc := range docs {
		switch doc.URL {
		case TranscribeURL, TranscribeArabicURL:
			b.applyTranscribe(doc)
		case DeprecationsURL:
			b.applyLifecycle(doc)
		}
	}
	priced := map[string]bool{}
	for _, doc := range docs {
		switch doc.URL {
		case PricingURL:
			b.applyPricing(doc)
			priced[doc.URL] = true
		case VaultPricingURL:
			b.applyVaultPricing(doc)
			priced[doc.URL] = true
		case StructuredOutputsURL:
			b.applyStructuredOutputs(doc)
		case ToolUseURL:
			b.applyToolUse(doc)
		case StreamingURL:
			b.applyStreaming(doc)
		}
	}
	if priced[PricingURL] && priced[VaultPricingURL] {
		b.noteUnpriced()
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

// builder accumulates models across documents.
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

// result returns the accumulated models in identifier order, less the ones
// Cohere no longer serves.
//
// A retired model is dropped rather than published with its standing attached.
// The catalog says what can be called and what it costs, and a model that has
// been shut down can be neither called nor bought; publishing it would offer a
// reader something to choose that no longer exists. A deprecated model stays,
// because Cohere goes on serving it and goes on stating its rate.
//
// The standing is read after every document has spoken, so a model the
// announcements withdraw is dropped whichever document established it.
func (b *builder) result() []catalog.Model {
	ids := slices.Clone(b.order)
	slices.Sort(ids)
	out := make([]catalog.Model, 0, len(ids))
	for _, id := range ids {
		if m := b.models[id]; served(m) {
			out = append(out, *m)
		}
	}
	return out
}
