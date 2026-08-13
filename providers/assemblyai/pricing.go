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
// An add-on is priced per model rather than once, in a column per model, and
// the column heads name the models by the same display name the documentation
// does. What the column says is which mode the rate belongs to — the models it
// names are pre-recorded in one table and streaming in another — so the mode
// each column's model was recorded under becomes the rate's dimension, and a
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

// applyAddOnRow records one add-on's rate under each model it is offered with.
func (b *builder) applyAddOnRow(cells, heads []string, source string) {
	if len(cells) < 2 {
		return
	}
	m, ok := b.models[slugID(nameOf(cells[0]))]
	if !ok {
		return
	}
	m.AddSource(source)
	for i, cell := range cells[1:] {
		mode, ok := b.modeOf(cellAt(heads, i+1))
		if !ok {
			continue
		}
		b.addOnRate(m, clean(cell), mode)
	}
}

// addOnRate records what one column of an add-on row states.
func (b *builder) addOnRate(m *catalog.Model, cell, mode string) {
	if cell == "" || strings.EqualFold(cell, notSupported) {
		return
	}
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

// modeOf reports which mode a column's model was recorded under, which is what
// says whether the rate beneath it counts audio or session time.
func (b *builder) modeOf(head string) (string, bool) {
	m, ok := b.models[slugID(nameOf(head))]
	if !ok {
		return "", false
	}
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
