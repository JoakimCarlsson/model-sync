package google

import (
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Keys the thinking guide populates.
const (
	ListThinkingLevels  = "thinking_levels"
	AttrDefaultThinking = "default_thinking_level"
)

// applyThinking reads the thinking guide's table of how much each model
// reasons by default and which levels it accepts.
//
// Google states no budget in tokens any more. The parameter is thinking_level
// and the guide enumerates the levels a model takes, so the bound is a set of
// names rather than a pair of numbers and is recorded as the list it is.
//
// The rows name endpoints exactly, so no other table on the page can be
// mistaken for this one: a row whose first cell is not an endpoint the pricing
// page priced matches nothing.
func (b *builder) applyThinking(doc catalog.Document) {
	for _, row := range pageRowRe.FindAllStringSubmatch(string(doc.Body), -1) {
		cells := rowCells(row[1])
		if len(cells) != 3 {
			continue
		}
		m := b.models[strings.TrimSpace(cells[0])]
		if m == nil {
			continue
		}
		m.AddSource(doc.URL)
		m.SetAttr(AttrDefaultThinking, defaultLevel(cells[1]))
		m.AddList(ListFeatures, catalog.CapabilityReasoning)
		addThinkingLevels(m, cells[2])
	}
}

// defaultLevel reads how much a model reasons when it is not told. Google
// writes the level in a parenthesis after the word saying whether reasoning is
// on at all, and writes the word alone where the model has one setting.
func defaultLevel(cell string) string {
	value := strings.ToLower(strings.TrimSpace(cell))
	if aside := parenRe.FindString(value); aside != "" {
		return strings.TrimSpace(strings.Trim(aside, "()"))
	}
	return value
}

// addThinkingLevels records the levels a cell enumerates.
func addThinkingLevels(m *catalog.Model, cell string) {
	for _, level := range splitProse(strings.ToLower(cell)) {
		m.AddList(ListThinkingLevels, strings.TrimSpace(level))
	}
}
