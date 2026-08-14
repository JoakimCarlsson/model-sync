package mistral

import (
	"regexp"
	"slices"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Keys the embedding guides populate.
const (
	AttrDefaultDimension = "default_embedding_dimension"
	AttrMaxDimension     = "max_embedding_dimension"
	FeatureReducibleDims = "reducible_embedding_dimensions"
)

// Patterns over an embedding guide's flight payload.
var (
	// embeddingWidthRe matches the sentence naming a model and the width of
	// the vector it answers with. The guide writes the model into a code
	// element and the width into the prose that follows it, one way for a
	// model of a fixed width and another for one that can be shortened.
	embeddingWidthRe = regexp.MustCompile(
		`"children":"([a-z0-9.-]+)"\}\]," model generates embedding vectors ` +
			`(?:of dimension|up to dimensions of) (\d+)`,
	)
	// embeddingDefaultRe matches the width a request that asks for none gets.
	embeddingDefaultRe = regexp.MustCompile(`"children":"defaults to (\d+)"`)
	// embeddingMaxRe matches the longest vector a request may ask for.
	embeddingMaxRe = regexp.MustCompile(`"children":"maximum value of (\d+)"`)
)

// applyEmbeddings reads one embedding guide.
//
// A model page states the context window and the rates of an embedding model
// but never the width of the vector it returns. The guide states it, in the
// prose of the section that walks through a call, naming the model in the same
// sentence, so that is where it is read from.
//
// Where the width can be asked for, the guide states a default and a maximum
// and no set of options in between, so the default is what is recorded as the
// width, alongside the fact that a caller may shorten it. Recording a list
// would invent a set of discrete choices Mistral does not publish.
func (b *builder) applyEmbeddings(doc catalog.Document) {
	body := flight(doc.Body)
	match := embeddingWidthRe.FindStringSubmatch(body)
	if match == nil {
		return
	}
	m := b.byName(match[1])
	if m == nil {
		return
	}
	m.AddSource(doc.URL)
	width := match[2]
	if def := first(embeddingDefaultRe, body); def != "" {
		width = def
		m.AddList(ListFeatures, FeatureReducibleDims)
	}
	m.SetAttr(AttrDefaultDimension, width)
	m.SetAttr(AttrMaxDimension, first(embeddingMaxRe, body))
}

// byName returns the model an identifier names, or nil.
//
// The guides call a model by its undated identifier, which a model page
// records as an alias of the dated one the catalog is keyed by, so both are
// searched. Nothing is created for an identifier no page named: a guide names
// a model Mistral serves, and one absent from the pages would be a second
// entry for a model already held under its dated name.
func (b *builder) byName(id string) *catalog.Model {
	if m, ok := b.models[id]; ok {
		return m
	}
	for _, m := range b.models {
		if slices.Contains(m.Lists[ListAliases], id) {
			return m
		}
	}
	return nil
}
