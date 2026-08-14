package xai

import (
	"net/http"
	"slices"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// currency is the only currency xAI quotes.
const currency = "USD"

// providerID and providerName identify this source in the catalog.
const (
	providerID   = "xai"
	providerName = "xAI"
)

// Provider reads xAI's documentation. The zero value is not usable; call New.
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

// Parse reads the pricing page first, because it is the only document listing
// every model and the per-model pages are fetched from what it names. The
// rendered pages come last: they supersede the headline rate the earlier two
// documents give for a generation model.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	for _, stage := range []struct {
		match func(string) bool
		apply func(catalog.Document)
	}{
		{isSummary, b.applyPricing},
		{isMarkdownPage, b.applyModelPage},
		{isVariantPage, b.applyVariantPage},
	} {
		for _, doc := range docs {
			if stage.match(doc.URL) {
				stage.apply(doc)
			}
		}
	}
	return b.result(), nil
}

// isModelPage reports whether a URL is one model's detail page.
func isModelPage(url string) bool {
	return strings.Contains(url, "/developers/models/")
}

// isSummary reports whether a URL is the pricing or models page.
func isSummary(url string) bool { return !isModelPage(url) }

// isMarkdownPage reports whether a URL is a model's markdown page.
func isMarkdownPage(url string) bool {
	return isModelPage(url) && strings.HasSuffix(url, ".md")
}

// isVariantPage reports whether a URL is a model's rendered page.
func isVariantPage(url string) bool {
	return isModelPage(url) && !strings.HasSuffix(url, ".md")
}

// builder accumulates models across documents.
type builder struct {
	models map[string]*catalog.Model
	order  []string
	// pages maps the slug of a page named after a mode onto the model that
	// mode runs on. xAI documents its voice modes under the mode's name and
	// names the model only in the pricing table, so without this the mode page
	// would land on a model of its own and the model it describes would keep
	// the near-empty page xAI generates under its identifier.
	pages map[string]string
}

func newBuilder() *builder {
	return &builder{
		models: map[string]*catalog.Model{},
		pages:  map[string]string{},
	}
}

// linkPage records that a page published under slug describes id. The first
// model claiming a slug keeps it, which is what leaves a mode's page with the
// model still served where two models share one mode.
func (b *builder) linkPage(slug, id string) {
	if slug == "" || slug == id {
		return
	}
	if _, ok := b.pages[slug]; !ok {
		b.pages[slug] = id
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
