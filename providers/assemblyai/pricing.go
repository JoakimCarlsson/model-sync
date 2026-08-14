package assemblyai

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// PricingURL is the page carrying the rates the documentation only points at.
// The models page states what the transcription models cost and says of an
// add-on only that it is billed separately.
const PricingURL = "https://www.assemblyai.com/pricing"

// addOnHeading is the word the add-on tables head their first column with,
// which is what separates them from the model tables beside them.
const addOnHeading = "add-on features"

// notSupported is what a cell says where the add-on cannot be had at all,
// which is neither a rate nor a rate of nothing.
const notSupported = "not supported"

// noteIncluded marks a rate of zero the page writes as "Included", meaning the
// add-on costs nothing beyond the model it runs on.
const noteIncluded = "included in the model's rate"

// pricingNames join the two vocabularies AssemblyAI names a streaming model
// in: the documentation calls it streaming and the pricing page sells it as
// realtime, and the shorter of the two names is the English-only model the
// documentation names in full. Neither name is a variant of the other, so they
// are mapped here rather than matched loosely.
var pricingNames = map[string]string{
	"universal-3.5-pro-realtime": "universal-3.5-pro-streaming",
	"universal-streaming":        "universal-streaming-english",
}

// addOnFeatures name the capability each add-on row states of the model in
// every column that carries a rate for it. An add-on is a capability a model
// has for a price, so the model it is offered with records it.
var addOnFeatures = map[string]string{
	"keyterms prompting":  FeatureKeyterms,
	"prompting":           FeaturePrompting,
	"speaker diarization": FeatureDiarization,
	"medical mode":        FeatureMedicalVocabulary,
	"voice focus":         FeatureVoiceIsolation,
}

var (
	htmlTableRe = regexp.MustCompile(`(?is)<table[^>]*>(.*?)</table>`)
	htmlRowRe   = regexp.MustCompile(`(?is)<tr[^>]*>(.*?)</tr>`)
	htmlCellRe  = regexp.MustCompile(`(?is)<t[dh][^>]*>(.*?)</t[dh]\s*>`)
	// htmlSpanRe matches the first span of a cell, which holds the name ahead
	// of the paragraph describing it.
	htmlSpanRe = regexp.MustCompile(`(?is)<span[^>]*>(.*?)</span>`)
)

// applyPricing reads the add-on tables of the pricing page.
//
// An add-on is priced per model rather than once, in a column per model, so a
// column says two things: that the add-on can be had with the model it is
// headed by, which is a capability that model records, and what it costs
// there. It also says which mode the rate belongs to, since the models it
// names are pre-recorded in one table and streaming in another, so the mode
// each column's model was recorded under becomes the rate's dimension. A
// column naming a model this catalog does not have is skipped.
func (b *builder) applyPricing(doc catalog.Document) {
	for _, table := range htmlTableRe.FindAllStringSubmatch(
		string(doc.Body),
		-1,
	) {
		rows := htmlRowRe.FindAllStringSubmatch(table[1], -1)
		if len(rows) < 2 {
			continue
		}
		heads := rowCells(rows[0][1])
		if len(heads) == 0 ||
			!strings.EqualFold(clean(heads[0]), addOnHeading) {
			continue
		}
		for _, row := range rows[1:] {
			b.applyAddOnRow(rowCells(row[1]), heads, doc.URL)
		}
	}
}

// applyAddOnRow records what one add-on row states: the capability it gives
// every model whose column carries a rate for it, and, where the add-on is
// itself a model this catalog has, that rate under each of those models.
func (b *builder) applyAddOnRow(cells, heads []string, source string) {
	if len(cells) < 2 {
		return
	}
	name := nameOf(cells[0])
	addOn, priced := b.lookup(name)
	feature := addOnFeatures[strings.ToLower(name)]
	for i, cell := range cells[1:] {
		m, ok := b.lookup(nameOf(cellAt(heads, i+1)))
		if !ok {
			continue
		}
		mode, ok := billedMode(m)
		if !ok {
			continue
		}
		value := clean(cell)
		if value == "" || strings.EqualFold(value, notSupported) {
			continue
		}
		m.AddSource(source)
		m.AddList(ListFeatures, feature)
		if priced {
			addOn.AddSource(source)
			b.addOnRate(addOn, value, mode)
		}
	}
}

// addOnRate records what one column of an add-on row states.
func (b *builder) addOnRate(m *catalog.Model, cell, mode string) {
	price := catalog.Price{
		Metric:   sectionMetrics[mode],
		Unit:     UnitPerHour,
		Currency: currency,
		Dims:     catalog.Dims{DimMode: mode},
	}
	if strings.EqualFold(cell, "included") {
		price.Note = noteIncluded
		m.AddPrice(price)
		return
	}
	amount, ok := parseAmount(cell)
	if !ok {
		return
	}
	price.Amount = amount
	m.AddPrice(price)
}

// lookup finds a model by the name a document calls it, which for the pricing
// page is not always the name the documentation uses.
func (b *builder) lookup(name string) (*catalog.Model, bool) {
	id := slugID(name)
	if alias, ok := pricingNames[id]; ok {
		id = alias
	}
	m, ok := b.models[id]
	return m, ok
}

// billedMode reports which mode a model was recorded under, which is what says
// whether a rate quoted against it counts audio or session time.
func billedMode(m *catalog.Model) (string, bool) {
	mode := m.Attrs[AttrMode]
	if _, ok := sectionMetrics[mode]; !ok {
		return "", false
	}
	return mode, true
}

// nameOf reads the name out of a cell, which the page writes in the first span
// ahead of the paragraph describing it.
func nameOf(cell string) string {
	if match := htmlSpanRe.FindStringSubmatch(cell); match != nil {
		return clean(match[1])
	}
	return clean(cell)
}

// rowCells returns the raw markup of one row's cells.
func rowCells(row string) []string {
	matches := htmlCellRe.FindAllStringSubmatch(row, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m[1])
	}
	return out
}
