package openai

import (
	"net/http"
	"slices"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// currency is the only currency OpenAI quotes.
const currency = "USD"

// providerID and providerName identify this source in the catalog.
const (
	providerID   = "openai"
	providerName = "OpenAI"
)

// Provider reads OpenAI's documentation. The zero value is not usable; call
// New.
type Provider struct {
	// Client performs the fetches.
	Client *http.Client
	// CacheDir, when set, backs every fetch with a file on disk so repeated
	// runs and offline work do not re-request roughly sixty documents.
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

// Parse routes each document to the reader for its shape and merges what they
// find into one model per identifier.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	for _, doc := range docs {
		switch {
		case strings.Contains(doc.URL, "/docs/models/"):
			b.applyModelPage(doc)
		case strings.HasSuffix(doc.URL, "/pricing.md"):
			for _, t := range scanMarkdownTables(doc) {
				b.applyPricingTable(t)
			}
		default:
			for _, t := range scanJSXTables(doc) {
				b.applyImageTable(t)
			}
		}
	}
	return b.result(), nil
}

// builder accumulates models across documents, keyed by identifier, so that a
// pricing table and a model page describing the same model produce one entry.
type builder struct {
	models map[string]*catalog.Model
	order  []string
}

func newBuilder() *builder {
	return &builder{models: map[string]*catalog.Model{}}
}

// model returns the entry for id, creating it if absent. A kind already
// established by a more specific document is never replaced.
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
