package anthropic

import (
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// ThinkingURL is the troubleshooting page, whose first section is a per-model
// table of thinking configurations. The comparison table answers Yes or No to
// two thinking rows and stops there; this table states which modes a model
// accepts and which one it gets when the request asks for nothing, which is a
// different fact and the only place Anthropic states it.
const ThinkingURL = baseURL + "/build-with-claude/thinking-troubleshooting.md"

// applyThinking records what each model does about thinking.
//
// The table's Rejected column is not read. It states the same fact as the
// Thinking types column, from the other side and in the API's vocabulary, and
// a catalog carrying both would carry one fact twice and have to keep them
// agreeing.
func (b *builder) applyThinking(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		types, fallback := columnOf(t, "thinking types"), columnOf(t, "default")
		if !headerIs(t, 0, "model") || types < 0 || fallback < 0 {
			continue
		}
		for _, row := range t.Rows {
			b.applyThinkingRow(t, row, types, fallback)
		}
	}
}

// applyThinkingRow records one model's thinking configuration.
func (b *builder) applyThinkingRow(
	t mdTable,
	row []string,
	types, fallback int,
) {
	m, ok := b.models[b.resolve(clean(cellAt(row, 0)))]
	if !ok {
		return
	}
	m.AddSource(t.Source)
	if modes := clean(cellAt(row, types)); modes != "" {
		m.AddList(ListFeatures, FeatureReasoning)
	}
	m.SetAttr(
		AttrThinkingTypes,
		strings.ToLower(dropFootnote(clean(cellAt(row, types)))),
	)
	m.SetAttr(
		AttrThinkingDefault,
		strings.ToLower(dropFootnote(clean(cellAt(row, fallback)))),
	)
}
