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

// The two words the pricing page heads a first column with, which is what
// separates a table of one model per column from a table of one per row.
const (
	addOnHeading   = "add-on features"
	productHeading = "models"
)

// featureHeadings are the product headings whose rows are themselves the
// roster, with the kind each sells. Under every other heading a row names
// something this catalog either already holds or does not hold as a model.
var featureHeadings = map[string]catalog.Kind{
	"Speech Understanding": KindSpeechUnderstanding,
	"Guardrails":           KindGuardrail,
}

// redactionHalves join the two rows AssemblyAI sells redaction as to the one
// page it documents redaction on, and say which half each row is the rate for.
var redactionHalves = map[string]string{
	"pii-audio-redaction": "audio",
	"pii-text-redaction":  "text",
}

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
	"pii-audio-redaction":        "pii-redaction",
	"pii-text-redaction":         "pii-redaction",
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
	// htmlParaRe matches that paragraph.
	htmlParaRe = regexp.MustCompile(`(?is)<p[^>]*>(.*?)</p>`)
	// htmlHeadingRe matches a product heading.
	htmlHeadingRe = regexp.MustCompile(`(?is)<h[1-4][^>]*>(.*?)</h[1-4]\s*>`)
)

// applyPricing reads the tables of the pricing page, each of which belongs to
// the product heading above it.
//
// An add-on is priced per model rather than once, in a column per model, so a
// column says two things: that the add-on can be had with the model it is
// headed by, which is a capability that model records, and what it costs
// there. It also says which mode the rate belongs to, since the models it
// names are pre-recorded in one table and streaming in another, so the mode
// each column's model was recorded under becomes the rate's dimension. A
// column naming a model this catalog does not have is skipped.
//
// The other shape is a product table, one row per thing sold. Under the two
// transcription headings and the two headings this catalog holds no models
// for, it is read only for the sentence describing each model. Under the
// Speech Understanding and Guardrails headings it is the rate card for the
// feature pages, which state everything about those features except what they
// cost.
func (b *builder) applyPricing(doc catalog.Document) {
	for _, section := range pricingTables(string(doc.Body)) {
		rows := htmlRowRe.FindAllStringSubmatch(section.table, -1)
		if len(rows) < 2 {
			continue
		}
		heads := rowCells(rows[0][1])
		if len(heads) == 0 {
			continue
		}
		switch strings.ToLower(clean(heads[0])) {
		case addOnHeading:
			for _, row := range rows[1:] {
				b.applyAddOnRow(rowCells(row[1]), heads, doc.URL)
			}
		case productHeading:
			for _, row := range rows[1:] {
				b.applyProductRow(rowCells(row[1]), section.heading, doc.URL)
			}
		}
	}
}

// applyProductRow records one row of a product table: the sentence AssemblyAI
// describes the thing with, and, where the row is a feature sold by the hour,
// its rate.
//
// A row naming something this catalog does not hold as a model is skipped
// under every heading but the two feature ones. Those two are where the row
// itself is the roster: the Sync API and the Voice Agent API are ways of
// reaching a model this catalog already has, while Entity Detection is a thing
// sold on its own with a page and a rate of its own.
func (b *builder) applyProductRow(cells []string, heading, source string) {
	if len(cells) < 2 {
		return
	}
	name := nameOf(cells[0])
	kind, priced := featureHeadings[heading]
	summary := descriptionOf(cells[0])
	m, ok := b.lookup(name)
	switch {
	case ok && !priced && summary == "":
		return
	case !ok && !priced:
		return
	case !ok:
		m = b.model(slugID(name), kind)
		if m.Name == "" {
			m.Name = name
		}
	}
	m.AddSource(source)
	m.SetAttr(AttrSummary, summary)
	if !priced {
		return
	}
	m.SetAttr(AttrProduct, heading)
	amount, ok := parseAmount(cells[1])
	if !ok {
		return
	}
	m.AddPrice(catalog.Price{
		Metric:   MetricAudio,
		Unit:     UnitPerHour,
		Amount:   amount,
		Currency: currency,
		Dims:     featureDims(slugID(name)),
	})
}

// featureDims says which half of a feature a rate is for, where AssemblyAI
// documents a feature once and sells it twice.
func featureDims(id string) catalog.Dims {
	if half, ok := redactionHalves[id]; ok {
		return catalog.Dims{DimRedaction: half}
	}
	return nil
}

// section is one pricing table and the product heading it sits under.
type section struct {
	heading string
	table   string
}

// pricingTables pairs every table of the pricing page with the heading above
// it. The page carries the same two table shapes under every product it sells
// and nothing inside a table says which product that is, so the heading is
// what separates a transcription rate card from a feature one.
func pricingTables(body string) []section {
	headings := htmlHeadingRe.FindAllStringSubmatchIndex(body, -1)
	var out []section
	for _, table := range htmlTableRe.FindAllStringSubmatchIndex(body, -1) {
		heading := ""
		for _, h := range headings {
			if h[0] > table[0] {
				break
			}
			heading = clean(body[h[2]:h[3]])
		}
		out = append(out, section{heading, body[table[2]:table[3]]})
	}
	return out
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

// descriptionOf reads the sentence a product cell describes its row with,
// which sits in the paragraph under the name.
func descriptionOf(cell string) string {
	if match := htmlParaRe.FindStringSubmatch(cell); match != nil {
		return clean(match[1])
	}
	return ""
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
