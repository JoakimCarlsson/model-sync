package elevenlabs

import (
	"regexp"
	"slices"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// MetricFinetune counts a training run rather than a generation. It is a
// separate metric because a finetune is bought once and the audio it then
// produces is billed again at the model's own rate.
const MetricFinetune catalog.Metric = "finetune"

// UnitPerFinetune is what a finetune is quoted against.
const UnitPerFinetune catalog.Unit = "per_finetune"

// DimFeature says which optional part of a request a rate belongs to, for the
// riders ElevenLabs prices separately from the transcription itself.
const DimFeature = "feature"

// The pieces of the model pricing table further down the API pricing page. It
// is a grid: a heading names the family, a column of labels names what is
// quoted, and one column per plan holds the figures in label order.
var (
	rateSectionRe = regexp.MustCompile(
		`<h3 class="tw-type-2xl[^"]*">([^<]{1,60})</h3>`,
	)
	rateFamilyRe = regexp.MustCompile(`tw-type-2xs">([^<]{1,60})<`)
	rateLabelRe  = regexp.MustCompile(
		`<p class="tw-text-gray-700 tw-line-clamp-3 f-paragraph-03">` +
			`([^<]{0,60})</p>`,
	)
	rateValueRe = regexp.MustCompile(
		`(?s)<p class="f-paragraph-03">(.{0,300}?)</p>`,
	)
)

// The row labels. A row is quoted either as the price of the thing itself, as
// "Price per hour", or as the price of one optional part of a request, as
// "Entity detection (per
// hour)". The second is the same metric under a dimension and not a metric of
// its own: it is the same hour of audio, billed again for having asked for
// more work on it.
var (
	ratePlainRe  = regexp.MustCompile(`(?i)^price per (.+)$`)
	rateRiderRe  = regexp.MustCompile(`(?i)^(.+?) \(per (.+)\)$`)
	rateAmountRe = regexp.MustCompile(`\$\s*([\d,]*\.?\d+)`)
	rateMarkupRe = regexp.MustCompile(`<[^>]*>`)
)

// rateFinetune is the one row quoting something other than a rate on the audio
// itself.
const rateFinetune = "cost per finetune"

// ratePlanCount is how many plans the table quotes each row against.
const ratePlanCount = 6

// rateBlock is one family's rows of the model pricing table.
type rateBlock struct {
	Family string
	Labels []string
	Values []string
}

// scanRates reads the model pricing table.
//
// The table is the one place ElevenLabs quotes a rate against a plan, and it
// quotes every model's rate against all six of them. A row is read only where
// all six agree, so that a figure recorded without a plan on it is one the page
// states without a plan on it too.
func scanRates(body string) []rateBlock {
	type token struct {
		at   int
		kind int
		text string
	}
	const (
		heading = iota
		label
		value
	)
	var tokens []token
	for _, re := range []*regexp.Regexp{rateSectionRe, rateFamilyRe} {
		for _, m := range re.FindAllStringSubmatchIndex(body, -1) {
			tokens = append(
				tokens,
				token{m[0], heading, body[m[2]:m[3]]},
			)
		}
	}
	for _, m := range rateLabelRe.FindAllStringSubmatchIndex(body, -1) {
		tokens = append(tokens, token{m[0], label, body[m[2]:m[3]]})
	}
	for _, m := range rateValueRe.FindAllStringSubmatchIndex(body, -1) {
		tokens = append(tokens, token{m[0], value, body[m[2]:m[3]]})
	}
	slices.SortStableFunc(tokens, func(a, b token) int {
		return a.at - b.at
	})
	var (
		out     []rateBlock
		current rateBlock
	)
	flush := func() {
		if len(current.Labels) > 0 && len(current.Values) > 0 {
			out = append(out, current)
		}
		current.Labels, current.Values = nil, nil
	}
	for _, t := range tokens {
		switch t.kind {
		case heading:
			flush()
			current.Family = normalize(t.text)
		case label:
			if len(current.Values) > 0 {
				flush()
			}
			current.Labels = append(current.Labels, normalize(t.text))
		case value:
			current.Values = append(current.Values, t.text)
		}
	}
	flush()
	return out
}

// rateOf returns the amount a row quotes, and reports whether every plan quotes
// it. A row whose cells hold something other than an amount, as an allowance of
// included hours does, yields nothing.
func (r rateBlock) rateOf(row int) (float64, bool) {
	if len(r.Values) != len(r.Labels)*ratePlanCount {
		return 0, false
	}
	var first float64
	for plan := range ratePlanCount {
		cell := r.Values[plan*len(r.Labels)+row]
		match := rateAmountRe.FindStringSubmatch(
			rateMarkupRe.ReplaceAllString(cell, " "),
		)
		if match == nil {
			return 0, false
		}
		amount, ok := parseFloat(match[1])
		if !ok || (plan > 0 && amount != first) {
			return 0, false
		}
		first = amount
	}
	return first, true
}

// applyRates reads the model pricing table onto the models each family covers.
//
// The rows repeat the rate the cards at the top of the page quote, which costs
// nothing: a rate already recorded is recorded again identically and dropped.
// What the table adds is the rest of a family's row, which the cards leave out
// altogether: what ElevenLabs charges for the parts of a transcription request
// that are asked for separately, and what a music finetune costs.
func (b *builder) applyRates(doc catalog.Document) {
	for _, block := range scanRates(string(doc.Body)) {
		for row, label := range block.Labels {
			price, ok := priceOf(label)
			if !ok {
				continue
			}
			amount, ok := block.rateOf(row)
			if !ok {
				continue
			}
			price.Amount = amount
			b.priceFamily(doc, block.Family, price)
		}
	}
}

// priceFamily records one rate against every model of a family.
func (b *builder) priceFamily(
	doc catalog.Document,
	card string,
	price catalog.Price,
) {
	for _, id := range b.order {
		f, ok := familyOf(id)
		if !ok || f.Card != card {
			continue
		}
		m := b.models[id]
		m.AddSource(doc.URL)
		if price.Metric == "" {
			price.Metric = f.Metric
		}
		m.AddPrice(price)
	}
}

// priceOf reads a row label into the shape of the price it quotes, leaving the
// amount and, for a rate of the family's own metric, the metric to the caller.
func priceOf(label string) (catalog.Price, bool) {
	if label == rateFinetune {
		return catalog.Price{
			Metric:   MetricFinetune,
			Unit:     UnitPerFinetune,
			Currency: currency,
		}, true
	}
	if match := ratePlainRe.FindStringSubmatch(label); match != nil {
		unit, ok := unitFor(match[1])
		return catalog.Price{Unit: unit, Currency: currency}, ok
	}
	match := rateRiderRe.FindStringSubmatch(label)
	if match == nil {
		return catalog.Price{}, false
	}
	unit, ok := unitFor(match[2])
	if !ok {
		return catalog.Price{}, false
	}
	return catalog.Price{
		Unit:     unit,
		Currency: currency,
		Dims:     catalog.Dims{DimFeature: featureOf(match[1])},
	}, true
}

// featureOf names the rider a row prices as the capability it is, so that a
// dimension reads the same as the capability list it qualifies.
func featureOf(label string) string {
	switch {
	case strings.Contains(label, "entity detection"):
		return FeatureEntities
	case strings.Contains(label, "keyterm prompting"):
		return FeatureKeyterms
	}
	return strings.ReplaceAll(label, " ", "_")
}
