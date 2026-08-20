package googlecloud

import (
	"html"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// What Speech-to-Text bills for, and against what.
const (
	MetricAudio   catalog.Metric = "audio"
	UnitPerMinute catalog.Unit   = "per_minute"
)

// KindTranscription is what every model here does: it writes speech down.
const KindTranscription catalog.Kind = "transcription"

// Dimension keys the price list's rates vary along.
const (
	// DimAPIVersion separates the two APIs, which are priced apart: a minute
	// of the same audio costs $0.016 through v2 and $0.024 through v1 without
	// data logging.
	DimAPIVersion = "api_version"
	// DimMode is what the request asked for: recognition as it arrives, the
	// dynamic batch that trades urgency for a lower rate, or one of the two
	// medical transcriptions.
	DimMode = "mode"
	// DimDataLogging marks the v1 rates, which Google halves in exchange for
	// keeping the audio.
	DimDataLogging = "data_logging"
	// DimMonthlyMinutes is the volume band a rate applies in. Recognition is
	// cheaper per minute the more minutes an account sends in a month, and the
	// band is stated inside the price cell rather than in a column.
	DimMonthlyMinutes = "monthly_minutes"
	// DimSKU is the billing SKU the rate belongs to, which Google publishes
	// beside every category and which is what a bill cites.
	DimSKU = "sku"
)

// Scalar keys the model page populates.
const AttrSummary = "summary"

// AttrPriceClass is the category the price list puts a model in, which is
// stated in a footnote and is the only thing joining a rate to a model.
const AttrPriceClass = "price_class"

// The classes the footnotes name.
const (
	classStandard = "standard"
	classMedical  = "medical"
)

// noteUnpriced says why a model the model page lists carries no rate.
const noteUnpriced = "the pricing page states a rate per category and names " +
	"the models each category covers; it names none of the models the v2 " +
	"model page lists, so Google publishes no rate under this model's name"

var (
	tableRe = regexp.MustCompile(`(?is)<table.*?</table>`)
	rowRe   = regexp.MustCompile(`(?is)<tr[^>]*>(.*?)</tr>`)
	cellRe  = regexp.MustCompile(`(?is)<t[hd][^>]*>(.*?)</t[hd]\s*>`)
	tagRe   = regexp.MustCompile(`(?s)<[^>]*>`)
	// headingRe matches the heading that says which API the tables below it
	// price.
	headingRe = regexp.MustCompile(`(?is)<h[23][^>]*>(.*?)</h[23]\s*>`)
	// classRe matches a footnote naming the models one category covers, which
	// Google writes as "Standard¹ models include: default, …".
	//
	// The identifiers are matched by their own alphabet rather than up to a
	// terminator, because the page states the two footnotes one after the
	// other with nothing between them: a capture that ran to the end of the
	// line put the medical models in the standard class. An identifier here is
	// lower case, and the words that follow one are not, which is what ends
	// the list.
	classRe = regexp.MustCompile(
		`([Ss]tandard|[Mm]edical)[^ ]* models include:([a-z0-9_,\- ()]*)`,
	)
	// idRe matches an identifier inside such a footnote, which is written in
	// lower case with underscores and followed in one case by a parenthesis
	// qualifying it.
	idRe = regexp.MustCompile(`[a-z][a-z0-9_]*`)
	// skuRe matches the SKU Google states beside a category, with the
	// parentheses it sometimes wraps it in and sometimes does not, so that
	// removing it leaves the category's name and not a stray bracket.
	skuRe = regexp.MustCompile(
		`(?i)\(?\s*sku:\s*([0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4})\s*\)?`,
	)
	// bandRateRe matches one band and the amount charged in it, which a price
	// cell holds several of run together with no separator.
	bandRateRe = regexp.MustCompile(
		`([\d,]+ minutes? (?:to [\d,]+ minutes?|and above))\s*` +
			`\$([\d.]+)`,
	)
	// rateRe matches the single amount a cell holds where the category has no
	// bands.
	rateRe = regexp.MustCompile(`\$([\d.]+)`)
	// countRe matches a minute count inside a band.
	countRe = regexp.MustCompile(`[\d,]+`)
)

// modeNames map the category Google prices onto a word for what was asked
// for. A category absent from this map is not read: the rates are attached to
// models by category, and a category this does not name is one whose meaning
// has not been read.
var modeNames = map[string]string{
	"recognition":                               "recognition",
	"dynamic batch recognition":                 "dynamic_batch",
	"speech recognition (with data logging)":    "recognition",
	"speech recognition (without data logging)": "recognition",
	"medical dictation":                         "dictation",
	"medical conversation":                      "conversation",
}

// applyModels reads the model page, whose table is a name and a sentence.
func (b *builder) applyModels(doc catalog.Document) {
	for _, table := range tableRe.FindAllString(string(doc.Body), -1) {
		for _, row := range rowRe.FindAllStringSubmatch(table, -1) {
			cells := cellRe.FindAllStringSubmatch(row[1], -1)
			if len(cells) < 2 {
				continue
			}
			id := text(cells[0][1])
			if !isModelID(id) {
				continue
			}
			m := b.model(id)
			m.AddSource(doc.URL)
			m.SetAttr(AttrSummary, text(cells[1][1]))
		}
	}
}

// applyPricing reads the price list.
//
// The footnotes are read first, because they are what says which models a
// table's rates belong to: the tables state a category and never a model.
func (b *builder) applyPricing(doc catalog.Document) {
	body := string(doc.Body)
	classes := readClasses(body)
	for _, id := range classes[classStandard] {
		b.model(id).SetAttr(AttrPriceClass, classStandard)
	}
	for _, id := range classes[classMedical] {
		b.model(id).SetAttr(AttrPriceClass, classMedical)
	}
	version := ""
	for _, at := range marks(body) {
		if at.heading != "" {
			if named := apiVersion(at.heading); named != "" {
				version = named
			}
			continue
		}
		b.applyRateTable(at.table, version, classes, doc.URL)
	}
}

// applyRateTable records one table's rates against every model of the class
// its rows name.
func (b *builder) applyRateTable(
	table, version string,
	classes map[string][]string,
	source string,
) {
	for _, row := range rowRe.FindAllStringSubmatch(table, -1) {
		cells := cellRe.FindAllStringSubmatch(row[1], -1)
		if len(cells) < 3 {
			continue
		}
		category := text(cells[0][1])
		mode, ok := modeNames[strings.ToLower(categoryName(category))]
		if !ok {
			continue
		}
		ids := classes[classOf(text(cells[1][1]))]
		if len(ids) == 0 {
			continue
		}
		dims := catalog.Dims{}.
			With(DimAPIVersion, version).
			With(DimMode, mode).
			With(DimDataLogging, dataLogging(category)).
			With(DimSKU, first(skuRe, category))
		for _, rate := range readRates(text(cells[2][1])) {
			for _, id := range ids {
				m := b.model(id)
				m.AddSource(source)
				m.AddPrice(catalog.Price{
					Metric:   MetricAudio,
					Unit:     UnitPerMinute,
					Amount:   rate.amount,
					Currency: currency,
					Dims:     dims.With(DimMonthlyMinutes, rate.band),
				})
			}
		}
	}
}

// mark is one thing found in the price list: a heading, or a table.
type mark struct {
	at      int
	heading string
	table   string
}

// marks returns the headings and the tables in the order the page writes them,
// so that a table is read against the heading above it. The heading is what
// says which API a table prices, and the tables of the two APIs are otherwise
// identical in shape and in wording.
func marks(body string) []mark {
	var out []mark
	for _, at := range headingRe.FindAllStringSubmatchIndex(body, -1) {
		out = append(out, mark{
			at:      at[0],
			heading: text(body[at[2]:at[3]]),
		})
	}
	for _, at := range tableRe.FindAllStringIndex(body, -1) {
		out = append(out, mark{at: at[0], table: body[at[0]:at[1]]})
	}
	slices.SortFunc(out, func(a, b mark) int { return a.at - b.at })
	return out
}

// rate is one amount and the volume band it applies in.
type rate struct {
	amount float64
	band   string
}

// readRates reads a price cell, which holds either one amount or a band and an
// amount several times over, run together with no separator.
func readRates(cell string) []rate {
	var out []rate
	for _, match := range bandRateRe.FindAllStringSubmatch(cell, -1) {
		amount, err := strconv.ParseFloat(match[2], 64)
		if err != nil {
			continue
		}
		out = append(out, rate{amount: amount, band: bandName(match[1])})
	}
	if len(out) > 0 {
		return out
	}
	match := rateRe.FindStringSubmatch(cell)
	if match == nil {
		return nil
	}
	amount, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return nil
	}
	return []rate{{amount: amount}}
}

// bandName names a volume band by the minutes bounding it: "0 minute to
// 500,000 minute" becomes 0-500000 and "2,000,000 minute and above" 2000000+.
func bandName(band string) string {
	counts := countRe.FindAllString(band, -1)
	for i, count := range counts {
		counts[i] = strings.ReplaceAll(count, ",", "")
	}
	switch len(counts) {
	case 0:
		return ""
	case 1:
		return counts[0] + "+"
	}
	return counts[0] + "-" + counts[1]
}

// readClasses reads the footnotes naming the models each category covers.
func readClasses(body string) map[string][]string {
	out := map[string][]string{}
	for _, match := range classRe.FindAllStringSubmatch(text(body), -1) {
		class := strings.ToLower(match[1])
		for _, id := range idRe.FindAllString(match[2], -1) {
			if !isModelID(id) || skipWords[id] {
				continue
			}
			if !slices.Contains(out[class], id) {
				out[class] = append(out[class], id)
			}
		}
	}
	return out
}

// skipWords are the words a footnote writes around the identifiers and which
// would read as identifiers themselves: the parenthesis qualifying chirp says
// it is available on one API only.
var skipWords = map[string]bool{
	"and":     true,
	"api":     true,
	"include": true,
	"models":  true,
	"only":    true,
	"speech":  true,
	"text":    true,
	"to":      true,
}

// isModelID reports whether a cell holds an identifier rather than prose. A
// Speech-to-Text model is named in lower case with underscores and no spaces.
func isModelID(value string) bool {
	if value == "" || strings.ContainsAny(value, " .") {
		return false
	}
	return idRe.FindString(value) == value
}

// classOf reads the class a table's Model column names, which Google marks
// with a footnote number the class itself does not have.
func classOf(cell string) string {
	lower := strings.ToLower(cell)
	switch {
	case strings.HasPrefix(lower, classMedical):
		return classMedical
	case strings.HasPrefix(lower, classStandard):
		return classStandard
	}
	return ""
}

// categoryName is the category a row prices, without the SKU written into the
// same cell.
func categoryName(cell string) string {
	return strings.TrimSpace(skuRe.ReplaceAllString(cell, ""))
}

// dataLogging reports whether a category says the audio is kept, which only
// the v1 categories state and which is the whole difference between the two.
func dataLogging(category string) string {
	lower := strings.ToLower(category)
	switch {
	case strings.Contains(lower, "without data logging"):
		return "false"
	case strings.Contains(lower, "with data logging"):
		return "true"
	}
	return ""
}

// apiVersion reads which API a heading introduces.
func apiVersion(heading string) string {
	lower := strings.ToLower(heading)
	switch {
	case strings.Contains(lower, "v2 api"):
		return "v2"
	case strings.Contains(lower, "v1 api"):
		return "v1"
	}
	return ""
}

// first returns the first capture of a pattern, or the empty string.
func first(re *regexp.Regexp, value string) string {
	match := re.FindStringSubmatch(value)
	if match == nil {
		return ""
	}
	return match[1]
}

// text strips markup, resolves entities and collapses whitespace.
func text(markup string) string {
	stripped := tagRe.ReplaceAllString(markup, " ")
	return html.UnescapeString(strings.Join(strings.Fields(stripped), " "))
}
