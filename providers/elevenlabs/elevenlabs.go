package elevenlabs

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// providerID and providerName identify this source in the catalog.
const (
	providerID   = "elevenlabs"
	providerName = "ElevenLabs"
)

// ModelsURL is the page listing every model ElevenLabs serves.
const ModelsURL = "https://elevenlabs.io/docs/overview/models.md"

// PricingURL is the only page stating a rate in dollars. The documentation
// quotes credits, which are a plan allowance rather than a price.
const PricingURL = "https://elevenlabs.io/pricing/api"

// sourceURLs are the documents this provider reads, models page first because
// it is the only one naming every identifier and the rest are read onto it.
var sourceURLs = slices.Concat(
	[]string{ModelsURL, NamesURL},
	endpointURLs,
	guideURLs,
	[]string{PricingURL},
)

// Provider reads ElevenLabs' models and pricing pages. The zero value is not
// usable; call New.
type Provider struct {
	// Client performs the fetch.
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

// Fetch retrieves every document, keeping the ones it got when one fails so a
// page ElevenLabs moved costs that page's facts rather than the whole sync.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	var docs []catalog.Document
	for _, url := range sourceURLs {
		doc, err := p.get(ctx, url)
		if err != nil {
			return docs, err
		}
		docs = append(docs, doc)
	}
	return docs, nil
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

// Parse indexes the language lists of every document first, because a languages
// cell on the models page is often a link to one of them and the cell is read
// through the link. It then reads the models page, which is the only document
// naming an identifier and onto whose models every other document is read, and
// prices the families the pricing page quotes last.
//
// The models the cards leave out are marked only when the pricing page was one
// of the documents. A fetch that lost it would otherwise mark every model as
// uncovered, which would be this parser reporting its own missing document as a
// fact about ElevenLabs.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	for _, doc := range docs {
		b.indexLanguages(doc)
	}
	for _, doc := range docs {
		if doc.URL == ModelsURL {
			b.applyModels(doc)
			b.applyCards(doc)
		}
	}
	for _, doc := range docs {
		switch {
		case doc.URL == NamesURL:
			b.applyNames(doc)
		case slices.Contains(endpointURLs, doc.URL):
			b.applyEndpoint(doc)
		case slices.Contains(guideURLs, doc.URL):
			b.applyGuide(doc)
		}
	}
	priced := false
	for _, doc := range docs {
		if doc.URL == PricingURL {
			b.applyPricing(doc)
			b.applyRates(doc)
			priced = true
		}
	}
	if priced {
		b.noteUnpriced()
	}
	return b.result(), nil
}

// builder accumulates models.
type builder struct {
	models map[string]*catalog.Model
	order  []string
	// sections holds the language list of every section a languages cell can
	// link to, keyed by the path and anchor a link names it by.
	sections map[string][]string
}

func newBuilder() *builder {
	return &builder{
		models:   map[string]*catalog.Model{},
		sections: map[string][]string{},
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
