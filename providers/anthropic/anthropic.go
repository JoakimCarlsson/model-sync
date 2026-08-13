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

// Parse reads the documents in dependency order regardless of the order they
// arrive in. The pricing tables name models only by display name, so both
// pages that state API identifiers are read first: deprecations, which lists
// every model that has ever existed, then the overview, which is authoritative
// for the current ones. The two capability guides come last, because each
// attaches a capability to models the earlier pages established and neither
// establishes one of its own.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	for _, stage := range []struct {
		match string
		apply func(catalog.Document)
	}{
		{"/model-deprecations", b.applyDeprecations},
		{"/models/", b.applyOverview},
		{"/pricing", b.applyPricing},
		{"/structured-outputs", func(doc catalog.Document) {
			b.applySupportedModels(
				doc,
				catalog.CapabilityStructuredOutputs,
			)
		}},
		{"/tool-use/", b.applyToolUse},
	} {
		for _, doc := range docs {
			if strings.Contains(doc.URL, stage.match) {
				stage.apply(doc)
			}
		}
	}
	b.applyModalities()
	b.applySharedSpecs()
	return b.result(), nil
}

// builder accumulates models across documents.
//
// Two indexes exist because Anthropic identifies models three different ways.
// nameToID holds the display name to identifier mapping the overview states
// outright. byTokens holds the identifiers the deprecations page lists, keyed
// on their version tokens, which is the only way to reach a retired model
// whose display name and identifier order their tokens differently.
type builder struct {
	models    map[string]*catalog.Model
	order     []string
	nameToID  map[string]string
	byTokens  map[string]string
	ambiguous map[string]bool
	// inputModalities and outputModalities are what the overview states every
	// current model takes and returns, held until every document has been read
	// because the last of them names a model the overview does not tabulate.
	inputModalities  []string
	outputModalities []string
	// sharedSpecs pairs a model with the one the overview gives its bounds by
	// naming, held for the same reason: the models it names are tabulated
	// nowhere and arrive with the pricing page.
	sharedSpecs [][2]string
}

func newBuilder() *builder {
	return &builder{
		models:    map[string]*catalog.Model{},
		nameToID:  map[string]string{},
		byTokens:  map[string]string{},
		ambiguous: map[string]bool{},
	}
}

// resolve turns a display name from a pricing row into an identifier, in
// descending order of how directly Anthropic states it: the overview names the
// identifier for current models, the deprecations page lists it for every
// model including retired ones, and only a model on neither page falls back to
// a slug of its display name.
func (b *builder) resolve(name string) string {
	if id, ok := b.nameToID[strings.ToLower(name)]; ok {
		return id
	}
	if id, ok := b.lookup(name); ok {
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
