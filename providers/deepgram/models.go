package deepgram

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// sectionKinds maps a page heading onto the kind of thing listed under it.
var sectionKinds = map[string]catalog.Kind{
	"speech to text":         KindTranscription,
	"speech-to-text add-ons": KindAddOn,
	"text to speech":         KindSpeech,
	"voice agent api":        KindAgent,
	"audio intelligence":     KindIntelligence,
}

var (
	headingRe = regexp.MustCompile(`(?is)<h[23][^>]*>(.*?)</h[23]>`)
	rowRe     = regexp.MustCompile(`(?is)<tr[^>]*>(.*?)</tr>`)
	cellRe    = regexp.MustCompile(`(?is)<(t[hd])[^>]*>(.*?)</t[hd]\s*>`)
	// nameRe matches the model name, which sits in the first span of a row
	// header ahead of the tooltip describing it.
	nameRe = regexp.MustCompile(`(?is)<span[^>]*>(.*?)</span>`)
)

// applyPricing reads the pricing page.
func (b *builder) applyPricing(doc catalog.Document) {
	for _, section := range sections(string(doc.Body)) {
		kind, ok := sectionKinds[section.heading]
		if !ok {
			continue
		}
		b.applySection(section, kind, doc.URL)
	}
}

// applySection reads the tables under one product heading.
func (b *builder) applySection(s section, kind catalog.Kind, source string) {
	var plans []string
	for _, match := range rowRe.FindAllStringSubmatch(s.body, -1) {
		cells := rowCells(match[1])
		if len(cells) < 2 {
			continue
		}
		if header, ok := planHeader(cells); ok {
			plans = header
			continue
		}
		b.applyRow(cells, plans, kind, s.heading, source)
	}
}

// headerFirstCells are the words Deepgram starts a header row with. The word
// differs per product, and a row whose first cell is not one of them is a
// model rather than a heading. The Voice Agent table says "Tier".
var headerFirstCells = map[string]bool{
	"model":   true,
	"feature": true,
	"tier":    true,
	"plan":    true,
}

// planHeader reports whether a row names the plans the columns price, and what
// they are.
func planHeader(cells []cell) ([]string, bool) {
	if !headerFirstCells[strings.ToLower(text(cells[0].html))] {
		return nil, false
	}
	plans := make([]string, 0, len(cells)-1)
	for _, c := range cells[1:] {
		plans = append(plans, slugID(text(c.html)))
	}
	return plans, true
}

// applyRow records one model and its rate under each plan.
func (b *builder) applyRow(
	cells []cell,
	plans []string,
	kind catalog.Kind,
	product, source string,
) {
	name, summary := splitNameCell(cells[0].html)
	if name == "" || headerFirstCells[strings.ToLower(name)] {
		return
	}
	m := b.model(slugID(name), kind)
	m.AddSource(source)
	m.AddList(ListInputModalities, kindFlows[kind].in...)
	m.AddList(ListOutputModalities, kindFlows[kind].out...)
	if m.Name == "" {
		m.Name = name
	}
	m.SetAttr(AttrSection, product)
	m.SetAttr(AttrSummary, summary)
	for i, c := range cells[1:] {
		plan := ""
		if i < len(plans) {
			plan = plans[i]
		}
		b.applyCell(m, c, plan, kind)
	}
}

// applyCell records the rate one plan charges, ignoring the struck-through
// amount beside it.
func (b *builder) applyCell(
	m *catalog.Model,
	c cell,
	plan string,
	kind catalog.Kind,
) {
	plain := text(c.html)
	if plain == "" {
		return
	}
	if strings.EqualFold(plain, "included") {
		m.SetAttr(AttrIncluded, "true")
		m.AddPrice(catalog.Price{
			Metric:   metricFor(kind, ""),
			Unit:     UnitPerMinute,
			Amount:   0,
			Currency: currency,
			Dims:     catalog.Dims{}.With(DimPlan, plan),
			Note:     noteIncluded,
		})
		return
	}
	if strings.Contains(strings.ToLower(plain), contactSales) {
		m.SetAttr(AttrAccess, contactSales)
		m.AddNote(noteContactSales)
		return
	}
	struck := struckAmounts(c.html)
	for _, r := range parseRates(plain) {
		if struck[r.Raw] {
			m.SetAttr(AttrPreviousRate, r.Raw)
			continue
		}
		if r.Unit == "" {
			continue
		}
		dims := catalog.Dims{}.With(DimPlan, plan)
		note := ""
		if isPromotional(r.Raw) {
			dims = dims.With(DimPromotion, "true")
			note = r.Raw
		}
		m.AddPrice(catalog.Price{
			Metric:   metricFor(kind, r.Raw),
			Unit:     r.Unit,
			Amount:   r.Amount,
			Currency: currency,
			Dims:     dims,
			Note:     note,
		})
	}
}

// splitNameCell separates a model's name from the tooltip describing it, which
// the page renders inside the same cell.
func splitNameCell(html string) (name, summary string) {
	spans := nameRe.FindAllStringSubmatch(html, -1)
	if len(spans) == 0 {
		return text(html), ""
	}
	name = text(spans[0][1])
	full := text(html)
	rest := strings.TrimSpace(strings.TrimPrefix(full, name))
	return name, strings.TrimSpace(strings.TrimPrefix(rest, "i"))
}

// cell is one table cell with its markup kept, since whether a rate is struck
// through is expressed in styling rather than in text.
type cell struct {
	html string
}

// rowCells returns the cells of one row.
func rowCells(row string) []cell {
	matches := cellRe.FindAllStringSubmatch(row, -1)
	out := make([]cell, 0, len(matches))
	for _, m := range matches {
		out = append(out, cell{html: m[2]})
	}
	return out
}

// section is one product heading and the markup under it.
type section struct {
	heading string
	body    string
}

// sections divides the page by heading so a table can be attributed to the
// product it prices.
func sections(body string) []section {
	locations := headingRe.FindAllStringSubmatchIndex(body, -1)
	out := make([]section, 0, len(locations))
	for i, at := range locations {
		end := len(body)
		if i+1 < len(locations) {
			end = locations[i+1][0]
		}
		out = append(out, section{
			heading: strings.ToLower(text(body[at[2]:at[3]])),
			body:    body[at[1]:end],
		})
	}
	return out
}
