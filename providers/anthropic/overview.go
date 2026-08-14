package anthropic

import (
	"slices"
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
	b.readModalities(string(doc.Body))
	b.readSharedSpecs(string(doc.Body))
	b.readBatchOutput(string(doc.Body))
}

// readBatchOutput records the output ceiling the Batches API allows, which the
// comparison table's Max output row is not: that row is the synchronous
// Messages API's, and a note under the table names the models that go further
// on batch and the beta header that lets them.
//
// It is applied here rather than held, because the note names models the table
// above it has already established. A name it cannot place is skipped rather
// than created, for the reason the capability guides skip one: this note states
// a bound of a model, never that a model exists.
func (b *builder) readBatchOutput(body string) {
	match := batchOutputRe.FindStringSubmatch(body)
	if match == nil {
		return
	}
	limit := parseTokenCount(match[2] + " tokens")
	if limit == 0 {
		return
	}
	for _, name := range splitNames(match[1]) {
		if m, ok := b.models[b.resolve(name)]; ok {
			m.SetLimit(LimitMaxOutputTokensBatch, limit)
		}
	}
}

// readSharedSpecs notes every model whose bounds the overview gives by naming
// another model rather than by stating them.
//
// It is kept rather than applied, for the same reason the modality sentence is:
// the model it describes is absent from the comparison table and reaches the
// catalog only when the pricing page read after this one names it.
func (b *builder) readSharedSpecs(body string) {
	for _, match := range sharedSpecsRe.FindAllStringSubmatch(body, -1) {
		b.sharedSpecs = append(b.sharedSpecs, [2]string{match[1], match[2]})
	}
}

// applySharedSpecs records the bounds of a model the comparison table has no
// column for, which Anthropic states by naming the model it matches.
//
// Claude Mythos 5 is offered to approved customers rather than generally, so it
// is absent from the table and documented in one sentence: it "shares Claude
// Fable 5's specs and pricing". That is Anthropic stating the bounds, in the
// only place it states them, so they are taken from the model named.
//
// Only what the sentence claims is copied. The rates are not, because the
// pricing page lists the model itself and is the authority on those, and the
// modalities are not, because the sentence covering every current model already
// reaches it.
func (b *builder) applySharedSpecs() {
	for _, pair := range b.sharedSpecs {
		m, ok := b.models[pair[0]]
		if !ok {
			continue
		}
		source, ok := b.models[b.nameToID[strings.ToLower(clean(pair[1]))]]
		if !ok || source == m {
			continue
		}
		for key, value := range source.Limits {
			m.SetLimit(key, value)
		}
		for key, values := range source.Lists {
			if key == ListInputModalities || key == ListOutputModalities {
				continue
			}
			m.AddList(key, values...)
		}
	}
}

// readModalities reads the sentence stating what a current model takes and
// returns, which the comparison table has no row for and which is the only
// place Anthropic states it outside the Models API, which needs a key.
//
// It is kept rather than applied, because the pricing page read after this one
// names a model the overview describes only in prose, and that model is as
// current as the rest.
func (b *builder) readModalities(body string) {
	sentence := modalitySentenceRe.FindStringSubmatch(body)
	if sentence == nil {
		return
	}
	b.inputModalities, b.outputModalities = modalitiesOf(sentence[1])
}

// applyModalities records what the overview's sentence stated against every
// model it covers. Its scope is every model still served, which is every chat
// model the documents name that has not retired; the server-side tools are not
// models and are left alone.
func (b *builder) applyModalities() {
	in, out := b.inputModalities, b.outputModalities
	for _, id := range b.order {
		m := b.models[id]
		if m.Kind != KindChat || withdrawn(m) {
			continue
		}
		m.AddList(ListInputModalities, in...)
		m.AddList(ListOutputModalities, out...)
	}
}

// modalitiesOf reads the clauses of that sentence, which name each direction
// after the modalities travelling in it: "text and image input, text output".
func modalitiesOf(sentence string) (in, out []string) {
	for _, clause := range modalityClauseRe.FindAllStringSubmatch(sentence, -1) {
		var named []string
		for _, word := range wordRe.FindAllString(
			strings.ToLower(clause[1]),
			-1,
		) {
			if name, ok := modalityWords[word]; ok &&
				!slices.Contains(named, name) {
				named = append(named, name)
			}
		}
		if strings.EqualFold(clause[2], "input") {
			in = append(in, named...)
			continue
		}
		out = append(out, named...)
	}
	return in, out
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
