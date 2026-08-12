package voyage

import (
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// sectionOpenModels is the heading under which Voyage lists the models whose
// weights it publishes.
const sectionOpenModels = "open models"

// applyModelPage reads a capability page: the embeddings, multimodal,
// contextualized-chunk or reranker page.
//
// A model found here but not in any rate table is still recorded. Voyage's
// open-weight model is documented and usable without appearing in the pricing
// tables at all, because running it yourself costs Voyage nothing to state.
func (b *builder) applyModelPage(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		idCol := columnOf(t.Headers, "model")
		contextCol := columnOf(t.Headers, "context length (tokens)")
		if idCol < 0 || contextCol < 0 {
			continue
		}
		dimCol := columnOf(t.Headers, "embedding dimension")
		descCol := columnOf(t.Headers, "description")
		chunkCol := columnOf(t.Headers, "per chunk context window")
		for _, row := range t.Rows {
			for _, id := range splitModels(cellAt(row, idCol)) {
				m := b.model(id, kindFor(id))
				m.AddSource(t.Source)
				m.SetLimit(
					LimitContextWindow,
					parseCount(cellAt(row, contextCol)),
				)
				m.SetLimit(
					LimitChunkContext,
					parseCount(cellAt(row, chunkCol)),
				)
				m.SetAttr(AttrSummary, summaryOf(cellAt(row, descCol)))
				applyDimensions(m, cellAt(row, dimCol))
				if t.Section == sectionOpenModels {
					m.SetAttr(AttrOpenWeights, "true")
				}
			}
		}
	}
}

// applyDimensions records the embedding widths a model offers and the one it
// uses when none is asked for.
func applyDimensions(m *catalog.Model, cell string) {
	dimensions, defaultDim := parseDimensions(cell)
	m.AddList(ListDimensions, dimensions...)
	m.SetAttr(AttrDefaultDimension, defaultDim)
}

// summaryOf reduces a description cell to its first sentence. Voyage's
// descriptions run to several sentences of links and compatibility notes, and
// only the first says what the model is.
func summaryOf(cell string) string {
	text := clean(cell)
	if sentence, _, ok := strings.Cut(text, ". "); ok {
		return sentence + "."
	}
	return text
}
