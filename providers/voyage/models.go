package voyage

import (
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// sectionOpenModels is the heading under which Voyage lists the models whose
// weights it publishes.
const sectionOpenModels = "open models"

// applyModelPage reads a capability page: the embeddings, multimodal,
// contextualized-chunk or reranker page, or MongoDB's overview, which states
// the same tables in the same shape under its own column headings.
//
// A model found here but not in any rate table is still recorded. Voyage's
// open-weight model is documented and usable without appearing in the pricing
// tables at all, because running it yourself costs Voyage nothing to state.
func (b *builder) applyModelPage(doc catalog.Document) {
	b.applyModelTables(doc)
	b.applyQuantization(doc)
	b.applyVideoInputs(doc)
}

// applyModelTables reads the model tables of a capability page.
func (b *builder) applyModelTables(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		idCol := columnOf(t.Headers, "model")
		contextCol := columnOf(
			t.Headers,
			"context length (tokens)",
			"context length",
		)
		if idCol < 0 || contextCol < 0 {
			continue
		}
		dimCol := columnOf(t.Headers, "embedding dimension", "dimensions")
		descCol := columnOf(t.Headers, "description")
		chunkCol := columnOf(t.Headers, "per chunk context window")
		for _, row := range t.Rows {
			for _, id := range splitModels(cellAt(row, idCol)) {
				m := b.model(id, kindFor(id))
				m.AddSource(t.Source)
				addModalities(m, modalitiesFor(doc.URL, t.Section))
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
					m.AddNote(noteOpenWeights)
				}
			}
		}
	}
}

// applyDimensions records the embedding widths a model offers, the one it uses
// when none is asked for, and the capability that having more than one is.
//
// The first document to state the widths wins, as it does for every scalar
// here. Voyage's own pages and MongoDB's overview disagree about voyage-4-nano,
// and merging two sets of widths would report a set that neither of them
// states.
func applyDimensions(m *catalog.Model, cell string) {
	if len(m.Lists[ListDimensions]) > 0 {
		return
	}
	dimensions, defaultDim := parseDimensions(cell)
	m.AddList(ListDimensions, dimensions...)
	m.SetAttr(AttrDefaultDimension, defaultDim)
	if len(dimensions) > 1 {
		m.AddList(ListFeatures, FeatureReducibleDims)
	}
}

// applyQuantization records the models that can return a vector narrower than a
// 32-bit float, which Voyage states in one sentence naming them rather than in
// the model table.
func (b *builder) applyQuantization(doc catalog.Document) {
	for _, match := range quantizedRe.FindAllStringSubmatch(
		string(doc.Body),
		-1,
	) {
		for _, id := range modelIDs(match[1]) {
			m := b.model(id, kindFor(id))
			m.AddSource(doc.URL)
			m.AddList(ListFeatures, FeatureQuantizedOutput)
		}
	}
}

// applyVideoInputs records the models that take video, which the multimodal
// page states in a sentence withholding it from the rest of its own table.
func (b *builder) applyVideoInputs(doc catalog.Document) {
	for _, match := range videoInputRe.FindAllStringSubmatch(
		string(doc.Body),
		-1,
	) {
		for _, id := range modelIDs(match[1]) {
			m := b.model(id, kindFor(id))
			m.AddSource(doc.URL)
			m.AddList(ListInputModalities, ModalityVideo)
		}
	}
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
