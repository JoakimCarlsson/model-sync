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

func isOverview(url string) bool { return url == overviewURL }

// isCapabilityPage reports whether a URL is one of the pages describing what a
// family of models can do.
func isCapabilityPage(url string) bool {
	return !isPricing(url) && !isBatch(url) && !isOverview(url)
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
