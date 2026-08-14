package openai

import (
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// applyWebSearchGuide reads the limitations table of the web search guide.
//
// OpenAI sells three search models through Chat Completions and gives none of
// them a page, so this table is the only place it states how much context they
// take. The table beside it, listing the models the Responses tool runs on,
// heads its column "Model context window" and rounds what it holds, so only the
// exactly headed column is read; those models have pages of their own, which
// are read after the guides and restate the figure exactly.
func (b *builder) applyWebSearchGuide(doc catalog.Document) {
	for _, t := range scanMarkdownTables(doc) {
		idCol := columnOf(t.Headers, []string{"model"})
		windowCol := columnOf(t.Headers, []string{"context window"})
		if idCol < 0 || windowCol < 0 {
			continue
		}
		for _, row := range t.Rows {
			b.setSearchContext(
				t.Source,
				cellAt(row, idCol),
				cellAt(row, windowCol),
			)
		}
	}
}

// setSearchContext records the context window one row states.
func (b *builder) setSearchContext(source, cell, window string) {
	id := unquote(cell)
	count := parseShorthand(window)
	if id == "" || count == 0 {
		return
	}
	m := b.model(id, KindChat)
	m.AddSource(source)
	m.SetLimit(LimitContextWindow, count)
}

// parseShorthand reads a token count abbreviated by magnitude, which is how the
// guides write one: "200k" for two hundred thousand and "1M" for a million. A
// count written out in full is read as it stands.
func parseShorthand(cell string) int64 {
	text := strings.ToLower(strings.TrimSpace(cell))
	scale := int64(1)
	switch {
	case strings.HasSuffix(text, "k"):
		scale, text = 1_000, strings.TrimSuffix(text, "k")
	case strings.HasSuffix(text, "m"):
		scale, text = 1_000_000, strings.TrimSuffix(text, "m")
	}
	return parseCount(text) * scale
}
