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
//
// The order is a dependency order, not the order documents arrive in. Pricing
// comes first because it states rates by tier and context band, which a model
// page cannot, and a model page defers to what is already recorded.
// Deprecations come before the model pages so that a withdrawn model is not
// then marked current by the page that still documents it.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	for _, stage := range []struct {
		match func(string) bool
		apply func(catalog.Document)
	}{
		{isPricing, b.applyPricingDoc},
		{isDeprecations, b.applyDeprecations},
		{isGuide, b.applyGuide},
		{isModelPage, b.applyModelPage},
	} {
		for _, doc := range docs {
			if stage.match(doc.URL) {
				stage.apply(doc)
			}
		}
	}
	b.fillKinds()
	b.applyAliasRates()
	b.noteUnpriced()
	return b.result(), nil
}

// noteUnpriced marks a model OpenAI documents and serves but states no rate
// for, so that it does not read as a free one.
//
// The tables leave a served model out in two ways. gpt-5.4-cyber has a row
// whose every amount is the dash its own tables use for "not offered", and the
// two open-weight gpt-oss models have no row at all: their weights are
// published for the reader to run, so there is no rate for OpenAI to state.
//
// Only models OpenAI still serves are marked. A deprecated model's missing rate
// is a rate OpenAI stopped stating rather than one it failed to state, so the
// standing is checked and not just the documents. This runs after the aliases
// are priced, so a model that borrows its target's rate is not marked as
// unpriced.
func (b *builder) noteUnpriced() {
	for _, id := range b.order {
		m := b.models[id]
		if len(m.Prices) > 0 || !served(m) {
			continue
		}
		m.AddNote(noteNoRate)
	}
}

// served reports whether OpenAI still documents and sells a model as current. A
// model reaches the catalog from a page of its own or from a row in the rate
// table, and one that arrives only through the deprecations table is on its way
// out.
func served(m *catalog.Model) bool {
	if m.Attrs[AttrState] == StateDeprecated {
		return false
	}
	return slices.ContainsFunc(m.Sources, func(url string) bool {
		return isModelPage(url) || isPricing(url)
	})
}

// applyAliasRates prices an alias from the model it points at.
//
// OpenAI sells two models under names that are aliases rather than models:
// Daybreak Blue and Daybreak Red point at whichever frontier model the program
// has reached, and the pricing page states their rates only as a sentence
// saying the alias is priced as its target. The rate table leaves their rows
// out entirely, so without this they read as free. The target is the snapshot
// the model page already names, and the borrowing is recorded as a note so a
// reader is not left thinking the table stated it.
func (b *builder) applyAliasRates() {
	for _, id := range b.order {
		m := b.models[id]
		target, ok := b.models[m.Attrs[AttrDefaultSnapshot]]
		if len(m.Prices) > 0 || !ok || target.ID == id ||
			len(target.Prices) == 0 {
			continue
		}
		for _, price := range target.Prices {
			m.AddPrice(price)
		}
		m.AddNote(notePricedAs + target.ID)
		for _, source := range target.Sources {
			m.AddSource(source)
		}
	}
}

// fillKinds settles what the models left without one are.
//
// A deprecated model whose page OpenAI has already taken down appears only in
// the deprecations table, which lists an identifier and a date and nothing
// about what the model did. Its name is the only evidence there is, and leaving
// the kind empty would be read as a parsing gap rather than as the absence of a
// model page.
func (b *builder) fillKinds() {
	for _, m := range b.models {
		if m.Kind == "" {
			m.Kind = kindFor(m.ID, m.Lists[ListEndpoints])
		}
	}
}

func isPricing(url string) bool { return strings.HasSuffix(url, "/pricing.md") }

func isDeprecations(url string) bool {
	return strings.HasSuffix(url, "/deprecations.md")
}

func isModelPage(url string) bool {
	return strings.Contains(url, "/docs/models/")
}

func isGuide(url string) bool { return strings.Contains(url, "/docs/guides/") }

// applyPricingDoc reads every rate table on the pricing page.
func (b *builder) applyPricingDoc(doc catalog.Document) {
	for _, t := range scanMarkdownTables(doc) {
		b.applyPricingTable(t)
	}
}

// applyGuide reads a guide. Each states something no other document does: the
// image guide holds the per-image rate matrix, the embeddings guide the vector
// width and the longest accepted input, the web search guide the context
// window of the search models, and the two transcription guides what a
// listening model can do.
func (b *builder) applyGuide(doc catalog.Document) {
	switch doc.URL {
	case EmbeddingsGuideURL:
		b.applyEmbeddingsGuide(doc)
	case WebSearchGuideURL:
		b.applyWebSearchGuide(doc)
	case TranscriptionGuideURL:
		b.applyTranscriptionGuide(doc)
	case SpeechToTextGuideURL:
		b.applySpeechToTextGuide(doc)
	default:
		for _, t := range scanJSXTables(doc) {
			b.applyImageTable(t)
		}
	}
}

// builder accumulates models across documents, keyed by identifier, so that a
// pricing table and a model page describing the same model produce one entry.
type builder struct {
	models map[string]*catalog.Model
	order  []string
	// withdrawn holds the identifiers OpenAI has stopped serving, which are
	// dropped from the result however many documents describe them.
	withdrawn map[string]bool
}

func newBuilder() *builder {
	return &builder{
		models:    map[string]*catalog.Model{},
		withdrawn: map[string]bool{},
	}
}

// withdraw records that OpenAI has shut a model down, so that it is left out of
// the result even when a page of its own still documents it. Two of the shut
// down moderation models keep such a page, as do several dated snapshots.
func (b *builder) withdraw(id string) {
	b.withdrawn[id] = true
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

// result returns the accumulated models in identifier order, less the ones
// OpenAI has withdrawn.
func (b *builder) result() []catalog.Model {
	ids := slices.Clone(b.order)
	slices.Sort(ids)
	out := make([]catalog.Model, 0, len(ids))
	for _, id := range ids {
		if b.withdrawn[id] {
			continue
		}
		out = append(out, *b.models[id])
	}
	return out
}
