package together

import (
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// sectionSupported is the heading the reasoning page lists its models under.
const sectionSupported = "supported models"

// colAPIString is the column of that table holding the identifier the API
// answers to, which the catalog page heads differently.
const colAPIString = "api string"

// colType is the column saying how the reasoning is controlled.
const colType = "type"

// AttrReasoningMode records what that column says: whether a model always
// reasons, can be switched between reasoning and not, or takes an effort
// setting. The capability is one word and this is the rest of what Together
// states about it.
const AttrReasoningMode = "reasoning_mode"

// applyReasoning reads the table of models that reason.
//
// The catalog page has a column for tool calling and one for structured output
// and none for this, so a model's reasoning is stated on a page of its own,
// listed rather than flagged. Only the one table under the supported models
// heading is read: the rest of the page is worked examples, and its other
// tables are the effort settings and their meanings, which name no model.
//
// A row naming a model the catalog did not establish is skipped rather than
// creating one. The page lists what reasons on serverless inference, and a
// model the catalog does not carry is one Together documents somewhere else.
func (b *builder) applyReasoning(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		if t.Section != sectionSupported {
			continue
		}
		idCol := columnOf(t.Headers, colAPIString)
		if idCol < 0 {
			continue
		}
		typeCol := columnOf(t.Headers, colType)
		for _, row := range t.Rows {
			m, ok := b.models[clean(cellAt(row, idCol))]
			if !ok {
				continue
			}
			m.AddList(ListFeatures, catalog.CapabilityReasoning)
			m.SetAttr(
				AttrReasoningMode,
				strings.ToLower(clean(cellAt(row, typeCol))),
			)
			m.AddSource(doc.URL)
		}
	}
}

// columnOf locates a column by the name the table heads it with.
func columnOf(headers []string, name string) int {
	for i, h := range headers {
		if strings.EqualFold(clean(h), name) {
			return i
		}
	}
	return -1
}
