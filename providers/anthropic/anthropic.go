package anthropic

import (
	"net/http"
	"slices"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// currency is the only currency Anthropic quotes.
const currency = "USD"

// providerID and providerName identify this source in the catalog.
const (
	providerID   = "anthropic"
	providerName = "Anthropic"
)

// Provider reads Anthropic's documentation. The zero value is not usable;
// call New.
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

// Parse reads the overview before the pricing page regardless of the order the
// documents arrive in, because the pricing tables name models by display name
// and the overview is what maps those names onto API identifiers.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	for _, doc := range docs {
		if strings.Contains(doc.URL, "/models/") {
			b.applyOverview(doc)
		}
	}
	for _, doc := range docs {
		if strings.Contains(doc.URL, "/pricing") {
			b.applyPricing(doc)
		}
	}
	return b.result(), nil
}

// builder accumulates models across documents. nameToID carries the display
// name to identifier mapping the overview establishes, which the pricing
// tables then depend on.
type builder struct {
	models   map[string]*catalog.Model
	order    []string
	nameToID map[string]string
}

func newBuilder() *builder {
	return &builder{
		models:   map[string]*catalog.Model{},
		nameToID: map[string]string{},
	}
}

// resolve turns a display name from a pricing row into an identifier. Models
// the overview does not list, which are the retired ones, fall back to the
// alias form of their name.
func (b *builder) resolve(name string) string {
	if id, ok := b.nameToID[strings.ToLower(name)]; ok {
		return id
	}
	return slugID(name)
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
