package mistral

import (
	"net/http"
	"slices"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// providerID and providerName identify this source in the catalog.
const (
	providerID   = "mistral"
	providerName = "Mistral AI"
)

// Provider reads Mistral's model documentation. The zero value is not usable;
// call New.
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

// Parse reads the model pages first, because they state the identifier a model
// is billed and called under and everything known about it. Every other
// document adds to models the pages already established, and none of them
// creates one: the index supplies lifecycle dates, the guides supply the facts
// stated of a capability rather than of a model, and the pricing page supplies
// the ratios a published rate may be adjusted by.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	for _, doc := range docs {
		if isModelPage(doc.URL) {
			b.applyModelPage(doc)
		}
	}
	for _, doc := range docs {
		switch doc.URL {
		case TextEmbeddingsURL, CodeEmbeddingsURL:
			b.applyEmbeddings(doc)
		case OCRGuideURL:
			b.applyOCRGuide(doc)
		case LanguagesURL:
			b.applyLanguages(doc)
		case ReasoningURL:
			b.applyReasoning(doc)
		case PricingURL:
			b.applyPricingPage(doc)
		case ModelsURL:
			b.applyDeprecations(doc)
		}
	}
	return b.result(), nil
}

// isModelPage reports whether a URL is one model's page rather than the index.
func isModelPage(url string) bool {
	return strings.HasPrefix(url, modelPagePre)
}

// builder accumulates models across documents.
type builder struct {
	models map[string]*catalog.Model
	order  []string
	// slugs maps the last segment of a model page's URL onto the identifier
	// the page names, so a document that links to a page rather than naming a
	// model can be matched to one.
	slugs map[string]string
}

func newBuilder() *builder {
	return &builder{
		models: map[string]*catalog.Model{},
		slugs:  map[string]string{},
	}
}

// sortedIDs returns the identifiers held, in order.
func (b *builder) sortedIDs() []string {
	ids := slices.Clone(b.order)
	slices.Sort(ids)
	return ids
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

// result returns the models Mistral still serves, in identifier order.
//
// A model whose badge says it has been withdrawn is dropped here rather than
// where it is read, because two documents name it: its own page states the
// standing and the index's deprecation table lists the same identifier again,
// so the standing is only settled once both have been read. Dropping it is
// what removes its file, since the store deletes what the parser stops
// emitting.
func (b *builder) result() []catalog.Model {
	ids := b.sortedIDs()
	out := make([]catalog.Model, 0, len(ids))
	for _, id := range ids {
		m := b.models[id]
		if slices.Contains(withdrawnStates, m.Attrs[AttrState]) {
			continue
		}
		out = append(out, *m)
	}
	return out
}
