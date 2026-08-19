package google

import (
	"net/http"
	"slices"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// currency is the only currency Google quotes here.
const currency = "USD"

// providerID and providerName identify this source in the catalog.
const (
	providerID   = "google"
	providerName = "Google"
)

// Provider reads Google's Gemini pricing and model documentation. The zero
// value is not usable; call New.
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

// Parse reads the index first, since it names and dates every model the other
// two documents only identify, then the pricing page, which is the only
// document naming every model the catalog holds, and finally attaches each
// model page to the models it describes.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	for _, doc := range docs {
		if doc.URL == ModelsURL {
			b.applyIndex(doc)
		}
	}
	for _, doc := range docs {
		if doc.URL == PricingURL {
			b.applyPricing(doc)
		}
	}
	b.applyPages(docs)
	for _, doc := range docs {
		b.applyGuide(doc)
	}
	return b.result(), nil
}

// applyPages attaches the per-model pages, which is the one document set that
// has to be paired with the models before any of it is read.
func (b *builder) applyPages(docs []catalog.Document) {
	pages := make([]catalog.Document, 0, len(docs))
	for _, doc := range docs {
		if strings.HasPrefix(doc.URL, modelPagePre) {
			pages = append(pages, doc)
		}
	}
	for i, ids := range b.pair(pages) {
		for _, id := range ids {
			b.applyModelPage(pages[i], id)
		}
	}
}

// applyGuide reads one of the documents that describe models the model pages
// leave incomplete, and does nothing for a document that is not one.
//
// Each is read after the pages rather than before, because a page states a
// model's own bounds and a guide states a family's: where the two disagree the
// page is the one written about the model, and the first document to state a
// field is the one kept.
func (b *builder) applyGuide(doc catalog.Document) {
	switch {
	case doc.URL == DeprecationsURL:
		b.applyDeprecations(doc)
	case doc.URL == RateLimitsURL:
		b.applyRateLimits(doc)
	case doc.URL == ThinkingURL:
		b.applyThinking(doc)
	case doc.URL == VideoGuideURL:
		b.applyVideoGuide(doc)
	case doc.URL == ImageGuideURL:
		b.applyImageGuide(doc)
	case doc.URL == ImagenGuideURL:
		b.applyImagenGuide(doc)
	case strings.HasPrefix(doc.URL, cardPre):
		b.applyModelCard(doc)
	}
}

// pair decides which models each page describes, in three rounds, each able to
// claim only what no earlier round took.
//
// A page is addressed by the identifier the API answers to, so the first round
// is that identifier and settles almost everything. It is settled first
// because a page also states the identifiers it covers and a few pages state
// the wrong one: the streaming robotics page and the Lyria Pro page both name
// their sibling's endpoint, and taking that at face value would give one model
// the other's page.
//
// The second round is those stated identifiers, which is what attaches the one
// page Google publishes for a family to every endpoint in it: the Imagen page
// names all three of its sizes and the Veo 3.1 page names both its own and the
// fast build.
//
// The third round hands the endpoints still unclaimed to the page describing
// the rest of the family the pricing page groups them with, which is how the
// custom-tools endpoint of Gemini 3.1 Pro, sold under the same heading and
// documented nowhere else, gets a page. Where two pages already split a family
// the round does nothing, so the Veo 3.1 Lite page keeps its own model.
//
// The result is indexed alongside pages, holding nothing where a page
// describes a model the pricing page does not price.
func (b *builder) pair(pages []catalog.Document) [][]string {
	out := make([][]string, len(pages))
	taken := map[string]int{}
	claim := func(i int, id string) {
		if _, ok := b.models[id]; !ok {
			return
		}
		if _, ok := taken[id]; ok {
			return
		}
		taken[id] = i
		out[i] = append(out[i], id)
	}
	for i, doc := range pages {
		claim(i, pageID(doc))
	}
	for i, doc := range pages {
		for _, id := range pageCodes(doc) {
			claim(i, id)
		}
	}
	for _, group := range b.groups {
		owner, ok := groupOwner(group, taken)
		if !ok {
			continue
		}
		for _, id := range group {
			claim(owner, id)
		}
	}
	return out
}

// groupOwner returns the one page already describing an endpoint of a family
// the pricing page groups under a single heading, and reports false where none
// does or where two pages split the family between them.
func groupOwner(group []string, taken map[string]int) (int, bool) {
	owner, found := 0, false
	for _, id := range group {
		at, ok := taken[id]
		if !ok {
			continue
		}
		if found && at != owner {
			return 0, false
		}
		owner, found = at, true
	}
	return owner, found
}

// pageID returns the identifier a model page is addressed by, which is the
// identifier the API answers to.
func pageID(doc catalog.Document) string {
	return strings.TrimPrefix(doc.URL, modelPagePre)
}

// builder accumulates models.
type builder struct {
	models map[string]*catalog.Model
	order  []string
	// groups holds the endpoints of each pricing heading, in the order the
	// heading states them, which is what lets one page stand for a family.
	groups [][]string
	// index holds what the model index states, keyed by endpoint and by the
	// name of the family the endpoint belongs to.
	index map[string]indexEntry
	// byName maps the name the index lists a model under onto the endpoint it
	// answers to, which is what lets a document naming models rather than
	// endpoints reach them.
	byName map[string]string
	// cardState holds the availability the index's grid marks an endpoint
	// with, which is the only place most models still in preview are said to
	// be.
	cardState map[string]string
}

func newBuilder() *builder {
	return &builder{
		models:    map[string]*catalog.Model{},
		index:     map[string]indexEntry{},
		byName:    map[string]string{},
		cardState: map[string]string{},
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
