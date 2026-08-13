package google

import (
	"maps"
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

// Parse reads the pricing page first, because it is the only document naming
// every model the catalog holds, then attaches each model page to the model it
// describes.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	for _, doc := range docs {
		if doc.URL == PricingURL {
			b.applyPricing(doc)
		}
	}
	pages := make([]catalog.Document, 0, len(docs))
	for _, doc := range docs {
		if strings.HasPrefix(doc.URL, modelPagePre) {
			pages = append(pages, doc)
		}
	}
	for i, id := range b.pair(pages) {
		b.applyModelPage(pages[i], id)
	}
	return b.result(), nil
}

// pair decides which model each page describes.
//
// The pricing page heads a model with the name it sells under and the model
// page addresses it by the identifier the API answers to, and the two drift:
// what pricing heads "Gemini 3.1 Flash Image (Nano Banana 2)" the API calls
// gemini-3.1-flash-image. One is therefore a prefix of the other as often as
// they are equal, in either direction, and matching on that alone would let
// one model's page attach to another model, since gemini-2.5-flash prefixes
// several models that are not it.
//
// The pairing is made one to one to prevent that. Pages that name a model
// exactly are settled first, then those whose model extends the page's
// address, then those whose address extends the model, and each round can only
// claim a model no earlier round took. A page matching nothing left describes a
// model the pricing page does not price, and is left unpaired.
//
// The result is indexed alongside pages, holding the empty string where a page
// paired with nothing.
func (b *builder) pair(pages []catalog.Document) []string {
	out := make([]string, len(pages))
	taken := map[string]bool{}
	var rest []int
	for i, doc := range pages {
		id := pageID(doc)
		if b.models[id] == nil || taken[id] {
			rest = append(rest, i)
			continue
		}
		out[i], taken[id] = id, true
	}
	for _, extends := range []func(page, id string) bool{
		func(page, id string) bool { return strings.HasPrefix(id, page+"-") },
		func(page, id string) bool { return strings.HasPrefix(page, id+"-") },
	} {
		remaining := rest[:0]
		for _, i := range rest {
			best := b.longestUnclaimed(pageID(pages[i]), taken, extends)
			if best == "" {
				remaining = append(remaining, i)
				continue
			}
			out[i], taken[best] = best, true
		}
		rest = remaining
	}
	return out
}

// longestUnclaimed returns the longest model identifier still free that stands
// in the given relation to a page's address, and the first in identifier order
// where two are equally long, so that a run is reproducible.
func (b *builder) longestUnclaimed(
	page string,
	taken map[string]bool,
	extends func(page, id string) bool,
) string {
	best := ""
	for _, id := range slices.Sorted(maps.Keys(b.models)) {
		if taken[id] || !extends(page, id) {
			continue
		}
		if len(id) > len(best) {
			best = id
		}
	}
	return best
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
