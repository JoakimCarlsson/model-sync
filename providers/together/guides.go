package together

import (
	"regexp"
	"slices"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// GuideIndexURL is the index of every page Together's documentation holds. The
// per-model guides are named there and nowhere else: the catalog page links to
// none of them.
const GuideIndexURL = "https://docs.together.ai/llms.txt"

// guideMark is what the index writes into the path of a page introducing one
// model.
const guideMark = "quickstart"

// LimitMaxOutputTokens is the bound a guide states, which no table does.
const LimitMaxOutputTokens = "max_output_tokens"

// Column and row headings the guides state a model's geometry under. A video
// family's guide opens with a table of its members, and a single model's guide
// opens with a table of the bounds it generates within.
const (
	colFamilyAPIString = "api string"
	colDuration        = "duration"
	colFeature         = "feature"
	colLimit           = "limit"
	colParameter       = "parameter"
	colMode            = "mode"
	rowDuration        = "duration"
	rowResolutions     = "resolutions"
)

// sectionPricing is the heading a guide states a rate card under, which is
// where a model priced by the second rather than by the video says so.
const sectionPricing = "pricing"

// guidePageRe matches one documentation page in the index.
var guidePageRe = regexp.MustCompile(
	`https://docs\.together\.ai/[A-Za-z0-9._/-]+\.md`,
)

// guideModelRe matches the sentence a guide names its model in. A guide to a
// single model opens by stating the string the API answers to.
var guideModelRe = regexp.MustCompile("(?i)model ID is `([^`]+)`")

// guideOutputRe matches the clause a guide states an output ceiling in.
var guideOutputRe = regexp.MustCompile(
	`(?i)up to ([\d.,]+\s*[kmb]?) output tokens`,
)

// guideTitleRe matches the heading a guide opens with, which is what says
// whether the page is about a model at all.
var guideTitleRe = regexp.MustCompile(`(?m)^#\s+(.+)$`)

// guideNameRe matches one identifier written as code, which is how a guide
// names the parameters it documents.
var guideNameRe = regexp.MustCompile("`([^`]+)`")

// guideRateRe matches a per-second rate, the one denominator a guide quotes
// that the catalog page does not.
var guideRateRe = regexp.MustCompile(`(?i)/\s*second`)

// guideURLs derives the per-model guides the index names.
func guideURLs(index catalog.Document) []string {
	var urls []string
	for _, url := range guidePageRe.FindAllString(string(index.Body), -1) {
		if !strings.Contains(url, guideMark) || slices.Contains(urls, url) {
			continue
		}
		urls = append(urls, url)
	}
	return urls
}

// applyGuide reads one model's guide onto the models the catalog page
// established.
//
// A guide is written for one model or for one family of them, and it is the
// only document that says anything beyond a rate about the models it covers.
// The models it covers are read off the guide itself: every catalog
// identifier the page states is one of them, which is exact because a guide
// spells an identifier out wherever it means it and Together's identifiers do
// not occur by accident.
//
// Three things are taken. A ceiling on how much the model may generate, which
// Together states in prose for a model whose ceiling is lower than its context
// window and omits where the two are the same, so a guide yielding none is the
// usual case. The request parameters the guide documents, from the first of
// its parameter tables only: a guide listing two is listing the parameters
// every model in the family takes and then the ones only some of them do, and
// the second table names the models it applies to in a sentence above itself
// rather than in a column. And the geometry a generation runs within, which
// only the two video families state, one as a column of its model table and
// one as a table of its own.
func (b *builder) applyGuide(doc catalog.Document) {
	body := string(doc.Body)
	var models []*catalog.Model
	for _, id := range b.order {
		if strings.Contains(body, id) {
			models = append(models, b.models[id])
		}
	}
	if len(models) == 0 || !b.guideIsAboutAModel(body) {
		return
	}
	if name := guideModelRe.FindStringSubmatch(body); name != nil {
		if m, ok := b.models[clean(name[1])]; ok {
			if bound := guideOutputRe.FindStringSubmatch(body); bound != nil {
				m.SetLimit(LimitMaxOutputTokens, parseCount(bound[1]))
				m.AddSource(doc.URL)
			}
		}
	}
	tables := scanTables(body, doc.URL)
	if params := guideParameters(tables); len(params) > 0 {
		for _, m := range models {
			m.AddList(ListParameters, params...)
			m.AddSource(doc.URL)
		}
	}
	for _, t := range tables {
		b.applyGuideTable(t, models)
	}
}

// guideIsAboutAModel reports whether a guide documents a model rather than a
// part of the platform.
//
// The index marks both the same way, and a guide to the fine-tuning service or
// to a cluster names a model in its worked example exactly as a guide to that
// model would. What separates them is the title: a guide to a model is titled
// after it, and one to a product is titled after the product. A guide opening
// by stating the identifier the API answers to counts too, since that sentence
// is written by a guide to one model and by nothing else.
func (b *builder) guideIsAboutAModel(body string) bool {
	if name := guideModelRe.FindStringSubmatch(body); name != nil {
		if _, ok := b.models[clean(name[1])]; ok {
			return true
		}
	}
	title := guideTitleRe.FindStringSubmatch(body)
	if title == nil {
		return false
	}
	subject := strings.TrimSpace(strings.ReplaceAll(
		strings.ToLower(clean(title[1])),
		guideMark,
		"",
	))
	if len(subject) < guideSubjectMin {
		return false
	}
	for _, m := range b.models {
		if strings.Contains(strings.ToLower(m.Name), subject) {
			return true
		}
	}
	return false
}

// guideSubjectMin is the shortest a title may reduce to and still be read as
// naming a model, which keeps a one-word or empty title from matching by
// accident.
const guideSubjectMin = 4

// guideParameters reads the request parameters the first parameter table of a
// guide documents. Together writes each name as code in the leading column,
// and writes more than one there where two parameters share a description.
func guideParameters(tables []table) []string {
	for _, t := range tables {
		if columnOf(t.Headers, colParameter) != 0 {
			continue
		}
		var names []string
		for _, row := range t.Rows {
			for _, m := range guideNameRe.FindAllStringSubmatch(cellAt(row, 0), -1) {
				names = append(names, m[1])
			}
		}
		return names
	}
	return nil
}

// applyGuideTable reads one table of a guide.
func (b *builder) applyGuideTable(t table, models []*catalog.Model) {
	switch {
	case columnOf(t.Headers, colFamilyAPIString) >= 0:
		b.applyGuideModels(t)
	case columnOf(t.Headers, colFeature) == 0 &&
		columnOf(t.Headers, colLimit) == 1:
		applyGuideLimits(t, models)
	case t.Section == sectionPricing && columnOf(t.Headers, colMode) == 0:
		applyGuideRates(t, models)
	}
}

// applyGuideModels reads the table a family's guide opens with, which names
// each member and how long a clip it generates.
func (b *builder) applyGuideModels(t table) {
	idCol := columnOf(t.Headers, colFamilyAPIString)
	durationCol := columnOf(t.Headers, colDuration)
	if durationCol < 0 {
		return
	}
	for _, row := range t.Rows {
		m, ok := b.models[clean(cellAt(row, idCol))]
		if !ok {
			continue
		}
		m.SetAttr(AttrDuration, clean(cellAt(row, durationCol)))
		m.AddSource(t.Source)
	}
}

// applyGuideLimits reads the table a single model's guide opens with, whose
// rows are a bound each rather than a model each.
func applyGuideLimits(t table, models []*catalog.Model) {
	for _, row := range t.Rows {
		value := clean(cellAt(row, 1))
		for _, m := range models {
			switch strings.ToLower(clean(cellAt(row, 0))) {
			case rowDuration:
				m.SetAttr(AttrDuration, value)
			case rowResolutions:
				m.AddList(ListResolutions, splitList(value)...)
			default:
				continue
			}
			m.AddSource(t.Source)
		}
	}
}

// applyGuideRates reads a rate card whose rows are a mode of generation and
// whose columns are a resolution. It is the only place Together prices a video
// by the second rather than by the video, and the two rates are both kept: the
// catalog page's per-video figure and this one answer different questions.
//
// A cell whose wording makes the amount a floor rather than the price keeps
// that wording as the price's note, since the amount alone does not say it.
func applyGuideRates(t table, models []*catalog.Model) {
	for _, row := range t.Rows {
		mode := strings.ToLower(clean(cellAt(row, 0)))
		for i := 1; i < len(t.Headers); i++ {
			cell := clean(cellAt(row, i))
			a := parseAmount(cell)
			if !a.Found || !guideRateRe.MatchString(cell) {
				continue
			}
			price := catalog.Price{
				Metric:   MetricVideoOutput,
				Unit:     UnitPerSecond,
				Amount:   a.Value,
				Currency: currency,
				Dims: catalog.Dims{
					DimMode:       mode,
					DimResolution: strings.ToLower(clean(t.Headers[i])),
				},
			}
			if !strings.HasPrefix(strings.ToLower(cell), "$") {
				price.Note = cell
			}
			for _, m := range models {
				m.AddPrice(price)
				m.AddSource(t.Source)
			}
		}
	}
}

// splitList splits a comma-separated cell into its values.
func splitList(cell string) []string {
	var out []string
	for _, part := range strings.Split(cell, ",") {
		if value := strings.TrimSpace(part); value != "" {
			out = append(out, value)
		}
	}
	return out
}
