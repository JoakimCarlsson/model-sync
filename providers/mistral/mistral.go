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

// Parse reads the model pages first, because they state the identifier a model
// is billed and called under and everything known about it. The index and the
// guides come last and add to models the pages already established: lifecycle
// dates from the one, embedding widths from the others.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	for _, doc := range docs {
		if isModelPage(doc.URL) {
			b.applyModelPage(doc)
		}
	}
	for _, doc := range docs {
		switch {
		case isModelPage(doc.URL):
		case slices.Contains(guideURLs, doc.URL):
			b.applyEmbeddings(doc)
		default:
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

// result returns the models Mistral still serves, in identifier order.
//
// A model whose badge says it has been withdrawn is dropped here rather than
// where it is read, because two documents name it: its own page states the
// standing and the index's deprecation table lists the same identifier again,
// so the standing is only settled once both have been read. Dropping it is
// what removes its file, since the store deletes what the parser stops
// emitting.
func (b *builder) result() []catalog.Model {
	ids := slices.Clone(b.order)
	slices.Sort(ids)
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
