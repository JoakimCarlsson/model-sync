package voyage

import (
	"net/http"
	"slices"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// currency is the only currency Voyage quotes.
const currency = "USD"

// providerID and providerName identify this source in the catalog.
const (
	providerID   = "voyage"
	providerName = "Voyage AI"
)

// Provider reads Voyage's documentation. The zero value is not usable; call
// New.
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

// Parse reads the pricing page first, so that the capability pages add to
// models already priced rather than creating them in an order that depends on
// which document arrived first. The overview follows them for the same reason
// in reverse: it is read for what Voyage's own pages have stopped stating, and
// a scalar already set from those pages is not overwritten.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	for _, stage := range []struct {
		match func(string) bool
		apply func(catalog.Document)
	}{
		{isPricing, b.applyPricing},
		{isCapabilityPage, b.applyModelPage},
		{isOverview, b.applyModelPage},
		{isBatch, b.applyBatch},
		{isPricing, b.applyPixelBands},
		{isTokenization, b.applyTokenizer},
		{isReference, b.applyReference},
		{isRateLimits, b.applyRateLimits},
		{isListing, b.applyListing},
		{isAnnouncement, b.applyAnnouncement},
	} {
		for _, doc := range docs {
			if stage.match(doc.URL) {
				stage.apply(doc)
			}
		}
	}
	return b.result(), nil
}

func isPricing(url string) bool {
	return strings.HasSuffix(url, "/pricing.md")
}

func isBatch(url string) bool {
	return strings.HasSuffix(url, "/batch-inference.md")
}

func isRateLimits(url string) bool {
	return strings.HasSuffix(url, "/rate-limits.md")
}

func isTokenization(url string) bool {
	return strings.HasSuffix(url, "/tokenization.md")
}

func isReference(url string) bool {
	return strings.HasPrefix(url, refURL+"/")
}

func isListing(url string) bool {
	return strings.HasPrefix(url, marketplaceURL)
}

func isAnnouncement(url string) bool {
	return strings.HasPrefix(url, blogURL)
}

func isOverview(url string) bool { return url == overviewURL }

// isCapabilityPage reports whether a URL is one of the pages describing what a
// family of models can do, which is every guide page that is not one of the
// pages read for something else.
func isCapabilityPage(url string) bool {
	switch {
	case isPricing(url), isBatch(url), isOverview(url),
		isRateLimits(url), isTokenization(url), isReference(url),
		isListing(url), isAnnouncement(url):
		return false
	}
	return true
}

// builder accumulates models across documents.
type builder struct {
	models map[string]*catalog.Model
	order  []string
	// pageModels records which models each guide page's tables listed, in
	// table order. The request bounds and the parameter list are stated once
	// per endpoint rather than per model, and this is what says which models
	// an endpoint serves.
	pageModels map[string][]string
}

func newBuilder() *builder {
	return &builder{
		models:     map[string]*catalog.Model{},
		pageModels: map[string][]string{},
	}
}

// servedBy returns the models a guide page listed, skipping any whose weights
// Voyage publishes rather than serving. A request bound is a bound on Voyage's
// API, and a model it does not host is not called through that API.
func (b *builder) servedBy(page string) []*catalog.Model {
	var out []*catalog.Model
	for _, id := range b.pageModels[page] {
		m, ok := b.models[id]
		if !ok || m.Attrs[AttrOpenWeights] == "true" {
			continue
		}
		out = append(out, m)
	}
	return out
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
