package deepgram

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Metrics Deepgram bills on.
const (
	MetricAudio        catalog.Metric = "audio"
	MetricSpeech       catalog.Metric = "speech"
	MetricInputTokens  catalog.Metric = "input_tokens"
	MetricOutputTokens catalog.Metric = "output_tokens"
)

// Units Deepgram quotes amounts against.
const (
	UnitPerMinute   catalog.Unit = "per_minute"
	UnitPerHour     catalog.Unit = "per_hour"
	UnitPer1KChars  catalog.Unit = "per_1k_characters"
	UnitPer1KTokens catalog.Unit = "per_1k_tokens"
)

// Kinds of model Deepgram publishes.
const (
	KindTranscription catalog.Kind = "transcription"
	KindSpeech        catalog.Kind = "speech"
	KindAgent         catalog.Kind = "agent"
	KindAddOn         catalog.Kind = "add-on"
	KindIntelligence  catalog.Kind = "intelligence"
)

// DimPlan records which subscription a rate belongs to, which is the only
// thing separating Deepgram's two columns.
const DimPlan = "plan"

// DimPromotion marks an introductory rate that lapses on a stated date.
// Deepgram writes both it and the rate that follows it in one cell, as
// "$0.110/min through 9/12" and "Then $0.146/min", and without the dimension
// the two would be indistinguishable rates for the same thing.
const DimPromotion = "promotional"

// promotionalMarker is the word introducing the date an offer runs to.
const promotionalMarker = "through"

// isPromotional reports whether a matched rate is the introductory one.
func isPromotional(raw string) bool {
	return strings.Contains(strings.ToLower(raw), promotionalMarker)
}

// noteIncluded marks a rate of zero that Deepgram writes as "Included",
// meaning the add-on costs nothing beyond the transcription it runs on.
const noteIncluded = "included in the base rate"

// contactSales is what Deepgram writes where another model has a rate, for the
// models it will only price in a conversation.
const contactSales = "contact sales"

// Enumeration keys the pricing page populates.
const (
	ListInputModalities  = "input_modalities"
	ListOutputModalities = "output_modalities"
)

// Modalities Deepgram's models handle.
const (
	ModalityText  = "text"
	ModalityAudio = "audio"
)

// kindFlows say what each kind of model takes and returns. Deepgram states it
// by which product a model is sold under: speech to text hears and writes,
// text to speech reads and speaks, an agent does both, and an add-on or an
// intelligence feature runs on the transcription and answers in text.
var kindFlows = map[catalog.Kind]struct{ in, out []string }{
	KindTranscription: {
		[]string{ModalityAudio},
		[]string{ModalityText},
	},
	KindSpeech: {
		[]string{ModalityText},
		[]string{ModalityAudio},
	},
	KindAgent: {
		[]string{ModalityAudio, ModalityText},
		[]string{ModalityAudio, ModalityText},
	},
	KindAddOn: {
		[]string{ModalityAudio},
		[]string{ModalityText},
	},
	KindIntelligence: {
		[]string{ModalityAudio},
		[]string{ModalityText},
	},
}

// Scalar keys the pricing page populates.
const (
	AttrSummary  = "summary"
	AttrSection  = "product"
	AttrIncluded = "included"
	// AttrAccess records that a model is sold by arrangement rather than at a
	// published rate.
	AttrAccess = "access"
	// AttrPreviousRate keeps the struck-through amount, which is what
	// Deepgram charged before the current rate.
	AttrPreviousRate = "previous_rate"
)

var (
	tagRe    = regexp.MustCompile(`(?s)<[^>]*>`)
	amountRe = regexp.MustCompile(
		`\$\s*([\d,]*\.?\d+)\s*(?:/\s*([A-Za-z0-9 ]{1,18}))?`,
	)
	// struckRe matches the wrapper Deepgram puts around a withdrawn rate,
	// which is styled rather than labelled.
	struckRe = regexp.MustCompile(
		`(?is)<span[^>]*text-gray-500[^>]*>(.*?)</span>\s*<span[^>]*aria-hidden`,
	)
)

// text strips markup and collapses whitespace.
func text(html string) string {
	return strings.Join(strings.Fields(tagRe.ReplaceAllString(html, " ")), " ")
}

// unitFor maps Deepgram's denominator wording onto a unit.
func unitFor(denominator string) (catalog.Unit, bool) {
	field := strings.ToLower(strings.TrimSpace(denominator))
	switch {
	case strings.HasPrefix(field, "min"):
		return UnitPerMinute, true
	case strings.HasPrefix(field, "hour"):
		return UnitPerHour, true
	case strings.HasPrefix(field, "1k character"):
		return UnitPer1KChars, true
	case strings.HasPrefix(field, "1k input token"),
		strings.HasPrefix(field, "1k output token"):
		return UnitPer1KTokens, true
	}
	return "", false
}

// metricFor reports what a rate counts, which its denominator and wording
// settle: the agent's language model is billed per token in each direction,
// speech per character, and everything else per unit of audio time.
func metricFor(kind catalog.Kind, denominator string) catalog.Metric {
	field := strings.ToLower(denominator)
	switch {
	case strings.Contains(field, "input token"):
		return MetricInputTokens
	case strings.Contains(field, "output token"):
		return MetricOutputTokens
	case strings.Contains(field, "character"):
		return MetricSpeech
	case kind == KindSpeech:
		return MetricSpeech
	}
	return MetricAudio
}

// rate is one amount read from a cell.
type rate struct {
	Amount float64
	Unit   catalog.Unit
	Raw    string
}

// parseRates reads every amount in a cell, in the order written.
func parseRates(cell string) []rate {
	var out []rate
	for _, match := range amountRe.FindAllStringSubmatch(cell, -1) {
		value, err := strconv.ParseFloat(
			strings.ReplaceAll(match[1], ",", ""),
			64,
		)
		if err != nil {
			continue
		}
		unit, _ := unitFor(match[2])
		out = append(out, rate{
			Amount: value,
			Unit:   unit,
			Raw:    strings.TrimSpace(match[0]),
		})
	}
	return out
}

// struckAmounts returns the amounts a cell shows struck through, which are the
// rates Deepgram no longer charges.
func struckAmounts(cellHTML string) map[string]bool {
	out := map[string]bool{}
	for _, match := range struckRe.FindAllStringSubmatch(cellHTML, -1) {
		for _, r := range parseRates(text(match[1])) {
			out[r.Raw] = true
		}
	}
	return out
}

// slugID turns a display name such as "Nova-3 Multilingual" into an
// identifier. Deepgram names models this way on its pricing page and does not
// state an API identifier beside the rate.
func slugID(name string) string {
	s := strings.ToLower(text(name))
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return '-'
	}, s)
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}
