package cohere

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"

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
}

// New returns a Provider using the default HTTP client.
func New() *Provider {
	return &Provider{Client: http.DefaultClient}
}

// ID implements catalog.Source.
func (p *Provider) ID() string { return providerID }

// Name implements catalog.Source.
func (p *Provider) Name() string { return providerName }

// documents are everything Cohere publishes that this parser reads, less the
// overview, which is fetched first and on its own, and less the release notes,
// which are not known until their index has been read.
//
// The order is the order they are listed in, which is the order they are
// fetched in and has no bearing on the order they are parsed in.
func documents() []string {
	urls := []string{
		PricingURL,
		VaultPricingURL,
		DeprecationsURL,
		TranscribeURL,
		TranscribeArabicURL,
		StructuredOutputsURL,
		ToolUseURL,
		StreamingURL,
		RateLimitsURL,
		ChatReferenceURL,
		EmbedReferenceURL,
		RerankReferenceURL,
		AudioReferenceURL,
		ChangelogURL,
	}
	for url := range modelPages {
		urls = append(urls, url)
	}
	slices.Sort(urls)
	return urls
}

// Fetch retrieves the overview, the two pricing pages, the deprecation
// announcements, the pages of the two transcription models, the pages of the
// six Command models that have one, the three capability guides, the rate
// limit page, the four endpoint references and every release note. Only the
// overview is required: it is the one document naming the identifiers the API
// answers to, and without it nothing the others say can be attached to
// anything. A document that cannot be read costs what it alone states, so the
// rest are returned with the failure rather than instead of it.
//
// The release notes are fetched last and in two rounds, because Cohere numbers
// them by nothing: the index names each entry and dates it, and the entry
// itself carries no date, so which notes exist is not known until the index
// has been read.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	overview, err := p.get(ctx, ModelsURL)
	if err != nil {
		return nil, err
	}
	docs := []catalog.Document{overview}
	var failures []error
	for _, url := range documents() {
		doc, err := p.get(ctx, url)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		docs = append(docs, doc)
		if url != ChangelogURL {
			continue
		}
		for _, entry := range changelogEntries(doc.Body) {
			note, err := p.get(ctx, entry)
			if err != nil {
				failures = append(failures, err)
				continue
			}
			docs = append(docs, note)
		}
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
	dates := map[string]string{}
	for _, doc := range docs {
		switch doc.URL {
		case ModelsURL:
			b.applyOverview(doc)
		case ChangelogURL:
			dates = changelogDates(doc.Body)
		}
	}
	b.linkAliases()
	for _, doc := range docs {
		switch doc.URL {
		case TranscribeURL, TranscribeArabicURL:
			b.applyTranscribe(doc)
		case DeprecationsURL:
			b.applyLifecycle(doc)
		default:
			b.applyModelPage(doc)
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
		case RateLimitsURL:
			b.applyRateLimits(doc)
		default:
			b.applyReference(doc)
			b.applyChangelog(doc, dates[doc.URL])
		}
	}
	b.aliasPrices()
	if priced[PricingURL] && priced[VaultPricingURL] {
		b.noteUnpriced()
	}
	return b.result(), nil
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
		return catalog.Document{}, fmt.Errorf("fetch %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return catalog.Document{}, fmt.Errorf("read %s: %w", url, err)
	}
	return catalog.Document{URL: url, Body: body}, nil
}

// builder accumulates models across documents.
type builder struct {
	models map[string]*catalog.Model
	order  []string
	// noTools holds the models whose own page withholds a capability the tool
	// use guide claims for the family they belong to.
	noTools map[string]bool
}

func newBuilder() *builder {
	return &builder{
		models:  map[string]*catalog.Model{},
		noTools: map[string]bool{},
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
