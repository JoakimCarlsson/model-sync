package deepgram

import (
	"regexp"
	"slices"
	"strconv"
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
	cellRe    = regexp.MustCompile(`(?is)<(t[hd])([^>]*)>(.*?)</t[hd]\s*>`)
	// nameRe matches the model name, which sits in the first span of a row
	// header ahead of the tooltip describing it.
	nameRe = regexp.MustCompile(`(?is)<span[^>]*>(.*?)</span>`)
	// spanRe matches how far down a cell reaches, which is how Deepgram writes
	// one rate against several models.
	spanRe = regexp.MustCompile(`(?i)rowspan\s*=\s*"?(\d+)`)
	// titleRe matches the tooltip a rate cell carries, which says what the
	// amount is metered against.
	titleRe = regexp.MustCompile(`(?is)title="([^"]+)"`)
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
	var held []carried
	for _, match := range rowRe.FindAllStringSubmatch(s.body, -1) {
		var cells []cell
		cells, held = fill(rowCells(match[1]), held)
		if len(cells) < 2 {
			continue
		}
		if header, ok := planHeader(cells); ok {
			plans = header
			held = nil
			continue
		}
		b.applyRow(cells, plans, kind, s.heading, source)
	}
}

// carried is a cell still covering rows below the one it was written in.
type carried struct {
	index int
	cell  cell
	left  int
}

// fill inserts the cells an earlier row spans into this one and returns what
// still reaches further down. Deepgram prices four audio intelligence models
// with a single cell reaching across all four, so a row holding a name and
// nothing else is a model priced above rather than a model with no rate.
func fill(cells []cell, held []carried) ([]cell, []carried) {
	for _, h := range held {
		h.cell.span = 1
		cells = slices.Insert(cells, min(h.index, len(cells)), h.cell)
	}
	var next []carried
	for i, c := range cells {
		if c.span > 1 {
			next = append(next, carried{index: i, cell: c, left: c.span - 1})
		}
	}
	for _, h := range held {
		if h.left > 1 {
			h.left--
			next = append(next, h)
		}
	}
	slices.SortStableFunc(next, func(a, b carried) int {
		return a.index - b.index
	})
	return cells, next
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

// applyCell records what one column says about a model: the description where
// the column is the description, and otherwise the rate that plan charges.
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
	if plan == planDescription {
		m.SetAttr(AttrSummary, plain)
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
	m.SetAttr(AttrMetered, tooltip(c.html))
	offer, standard := splitOffer(plain)
	dims := catalog.Dims{}.With(DimPlan, plan)
	unit := b.applyRates(m, standard, c.html, dims, kind)
	if offer == "" {
		return
	}
	b.applyOffer(m, offer, dims, unit, kind)
}

// applyRates records every amount a cell states, ignoring the struck-through
// one beside it, and reports the unit they were quoted against.
func (b *builder) applyRates(
	m *catalog.Model,
	plain, html string,
	dims catalog.Dims,
	kind catalog.Kind,
) catalog.Unit {
	struck := struckAmounts(html)
	var unit catalog.Unit
	for _, r := range parseRates(plain) {
		if struck[r.Raw] {
			m.SetAttr(AttrPreviousRate, r.Raw)
			continue
		}
		if r.Unit == "" {
			continue
		}
		unit = r.Unit
		m.AddPrice(catalog.Price{
			Metric:   metricFor(kind, r.Raw),
			Unit:     r.Unit,
			Amount:   r.Amount,
			Currency: currency,
			Dims:     dims,
		})
	}
	return unit
}

// applyOffer records the introductory rate a cell states above the one that
// replaces it. Where the offer is a word rather than an amount, "Free until
// 9/12", the rate is nothing until the stated date and is recorded as zero
// against the unit the standard rate is quoted in.
func (b *builder) applyOffer(
	m *catalog.Model,
	offer string,
	dims catalog.Dims,
	unit catalog.Unit,
	kind catalog.Kind,
) {
	ends := offerEnd(offer)
	m.SetAttr(AttrOfferEnds, ends)
	dims = dims.With(DimPromotion, "true")
	rates := parseRates(offer)
	if len(rates) == 0 {
		if !strings.Contains(strings.ToLower(offer), freeMarker) || unit == "" {
			return
		}
		rates = []rate{{Amount: 0, Unit: unit}}
	}
	for _, r := range rates {
		if r.Unit == "" {
			continue
		}
		m.AddPrice(catalog.Price{
			Metric:   metricFor(kind, r.Raw),
			Unit:     r.Unit,
			Amount:   r.Amount,
			Currency: currency,
			Dims:     dims,
			Note:     offerNote(ends),
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
	// span is how many rows the cell covers, which is one unless Deepgram
	// wrote one rate against several models.
	span int
}

// rowCells returns the cells of one row.
func rowCells(row string) []cell {
	matches := cellRe.FindAllStringSubmatch(row, -1)
	out := make([]cell, 0, len(matches))
	for _, m := range matches {
		out = append(out, cell{html: m[3], span: rowSpan(m[2])})
	}
	return out
}

// rowSpan reads how far down a cell reaches from its attributes.
func rowSpan(attrs string) int {
	match := spanRe.FindStringSubmatch(attrs)
	if match == nil {
		return 1
	}
	n, err := strconv.Atoi(match[1])
	if err != nil || n < 1 {
		return 1
	}
	return n
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
