package anthropic

import (
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// labelAPIID is the row of the transposed table holding the identifier every
// other document should be keyed by.
const labelAPIID = "claude api id"

// applyOverview reads the model comparison table, in which each column is a
// model and each row is one fact about every model.
//
// It runs before the pricing page because the pricing tables name models only
// by display name, and this table is where the display name is tied to the API
// identifier.
func (b *builder) applyOverview(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		ids := columnIDs(t)
		if len(ids) == 0 {
			continue
		}
		for column, id := range ids {
			b.nameToID[strings.ToLower(clean(t.Headers[column]))] = id
			b.model(id, KindChat).AddSource(t.Source)
		}
		for _, row := range t.Rows {
			b.applyOverviewRow(rowLabel(row), row, ids)
		}
	}
}

// columnIDs locates the API identifier of each model column, and reports none
// for a table that is not the comparison table.
func columnIDs(t mdTable) map[int]string {
	for _, row := range t.Rows {
		if rowLabel(row) != labelAPIID {
			continue
		}
		ids := map[int]string{}
		for i := 1; i < len(t.Headers); i++ {
			if id := clean(cellAt(row, i)); id != "" {
				ids[i] = id
			}
		}
		return ids
	}
	return nil
}

// rowLabel normalizes the leading cell that names what a row states.
func rowLabel(row []string) string {
	return strings.ToLower(dropFootnote(clean(cellAt(row, 0))))
}

// applyOverviewRow records one fact about every model in the table.
//
// Rows whose value is Yes or No are treated as capabilities generically rather
// than by name, so a capability Anthropic adds later is picked up without a
// change here.
func (b *builder) applyOverviewRow(
	label string,
	row []string,
	ids map[int]string,
) {
	if label == labelAPIID || label == "pricing" {
		return
	}
	for column, id := range ids {
		value := clean(cellAt(row, column))
		if value == "" || value == "-" {
			continue
		}
		m := b.model(id, KindChat)
		switch label {
		case "description":
			m.SetAttr(AttrSummary, value)
		case "claude api alias":
			m.AddList(ListAliases, value)
		case "aws bedrock id":
			m.SetAttr(AttrBedrockID, dropIDFootnote(value, id))
		case "google cloud id":
			m.SetAttr(AttrVertexID, dropIDFootnote(value, id))
		case "context window":
			m.SetLimit(LimitContextWindow, parseTokenCount(value))
		case "max output":
			m.SetLimit(LimitMaxOutputTokens, parseTokenCount(value))
		case "comparative latency":
			m.SetAttr(AttrLatency, value)
		case "reliable knowledge cutoff":
			m.SetAttr(AttrKnowledgeCutoff, isoDate(dropFootnote(value)))
		case "training data cutoff":
			m.SetAttr(AttrTrainingCutoff, isoDate(dropFootnote(value)))
		default:
			applyCapability(m, label, value)
		}
	}
}

// applyCapability records a Yes or No row as a feature the model has or lacks.
//
// Anthropic writes a capability as prose with the parameter that turns it on
// in the middle of it, so the row's own wording is not an identifier and is
// translated into one. Where the wording is lost by that — Anthropic offers
// two kinds of thinking and the catalog has one word for both — the row's
// heading is kept as a note.
//
// Some answers are qualified, as in "Yes (always on)", which is kept as a note
// too so the qualification is not lost.
func applyCapability(m *catalog.Model, label, value string) {
	answer, qualifier, _ := strings.Cut(value, " ")
	if !strings.EqualFold(answer, "yes") {
		return
	}
	feature, renamed := featureName(label)
	m.AddList(ListFeatures, feature)
	if renamed {
		m.AddNote(feature + " is " + strings.ToLower(clean(label)))
	}
	if qualifier != "" {
		m.AddNote(feature + " " + strings.Trim(qualifier, "()"))
	}
}

// featureName translates a capability row's heading into the catalog's
// vocabulary, reporting whether the translation lost anything.
//
// Both of Anthropic's thinking rows describe a model that reasons before it
// answers, which every other provider spells "reasoning". A model has one or
// the other and never both, so the two collapse onto that one word.
func featureName(label string) (name string, renamed bool) {
	text := strings.ToLower(clean(label))
	if strings.Contains(text, "thinking") {
		return FeatureReasoning, true
	}
	return strings.ReplaceAll(slugID(label), "-", "_"), false
}
