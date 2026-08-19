package deepgram

import (
	"fmt"
	"regexp"
	"slices"
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

// DimDelivery records whether a rate is for transcribing a live connection or
// a finished recording. Deepgram charges differently for the two and prices
// them in separate tables, so without the dimension one model carries two
// rates for the same plan and nothing saying which is which.
const DimDelivery = "delivery"

// The two ways Deepgram takes audio, named as its pricing tables caption
// themselves.
const (
	DeliveryStreaming = "streaming"
	DeliveryBatch     = "pre-recorded"
)

// deliveryFeatures say what a model that is priced in a given table can do,
// which is the same statement the feature overviews make: a model with a
// streaming rate transcribes a live connection, and a model with a
// pre-recorded rate transcribes a recording.
var deliveryFeatures = map[string]string{
	DeliveryStreaming: FeatureRealtime,
	DeliveryBatch:     FeatureBatch,
}

// freeMarker is what Deepgram writes where an offer costs nothing at all.
const freeMarker = "free"

var (
	// offerRe matches the word introducing the rate an offer gives way to,
	// which Deepgram writes on a line of its own inside the same cell.
	offerRe = regexp.MustCompile(`(?i)\bthen\b`)
	// offerEndRe matches the date an offer runs to, written either way
	// Deepgram introduces it.
	offerEndRe = regexp.MustCompile(
		`(?i)\b(?:through|until)\s+(\d{1,2}/\d{1,2}(?:/\d{2,4})?)`,
	)
)

// splitOffer divides a cell into the introductory rate and the rate that
// replaces it. A cell naming no successor is all standard rate.
func splitOffer(plain string) (offer, standard string) {
	at := offerRe.FindStringIndex(plain)
	if at == nil {
		return "", plain
	}
	return strings.TrimSpace(plain[:at[0]]), strings.TrimSpace(plain[at[1]:])
}

// offerEnd reads the date an offer runs to, in full where the page states it
// in full. The rate cell writes the day and the month and no year, and the
// prose over the table it sits in writes the same day out with its year, so a
// prose date that agrees with the cell is what the cell means.
func offerEnd(offer, stated string) string {
	match := offerEndRe.FindStringSubmatch(offer)
	if match == nil {
		return ""
	}
	if agrees(stated, match[1]) {
		return stated
	}
	return match[1]
}

// months are the names Deepgram writes a date out with.
var months = []string{
	"january", "february", "march", "april", "may", "june",
	"july", "august", "september", "october", "november", "december",
}

// statedDateRe matches a date written out in full, which is how the prose over
// a table says when the promotion in it lapses.
var statedDateRe = regexp.MustCompile(
	`([A-Z][a-z]+)\s+(\d{1,2}),\s+(\d{4})`,
)

// statedDate returns the first date a stretch of the page writes out in full,
// as ISO.
func statedDate(body string) string {
	for _, match := range statedDateRe.FindAllStringSubmatch(text(body), -1) {
		month := slices.Index(months, strings.ToLower(match[1])) + 1
		if month == 0 {
			continue
		}
		return fmt.Sprintf(
			"%s-%02d-%02d",
			match[3],
			month,
			atoi(match[2]),
		)
	}
	return ""
}

// agrees reports whether a date written out in full is the same day as one
// written as a month and a day alone.
func agrees(iso, short string) bool {
	parts := strings.Split(short, "/")
	if iso == "" || len(parts) < 2 {
		return false
	}
	return iso[5:] == fmt.Sprintf("%02d-%02d", atoi(parts[0]), atoi(parts[1]))
}

// atoi reads a number written without a sign, and reports nothing where it is
// not one.
func atoi(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

// offerNote says what an introductory rate is, since a consumer reading the
// aggregate sees two amounts for one plan and nothing else to tell them apart.
func offerNote(ends string) string {
	if ends == "" {
		return "introductory rate"
	}
	return "introductory rate through " + ends
}

// tooltip returns the note a cell carries behind its information icon, which
// is where the Voice Agent tables say what an amount is metered against.
func tooltip(html string) string {
	match := titleRe.FindStringSubmatch(html)
	if match == nil {
		return ""
	}
	return text(match[1])
}

// noteIncluded marks a rate of zero that Deepgram writes as "Included",
// meaning the add-on costs nothing beyond the transcription it runs on.
const noteIncluded = "included in the base rate"

// contactSales is what Deepgram writes where another model has a rate, for the
// models it will only price in a conversation.
const contactSales = "contact sales"

// noteContactSales records that a model has no published rate. The access key
// already says so, but nothing reading the aggregate can see a package comment
// and a model with no amount otherwise reads as a free one.
const noteContactSales = "sold by arrangement; " +
	"the pricing page states no amount"

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
	// AttrOfferEnds is the date the introductory rate lapses on.
	AttrOfferEnds = "promotion_ends"
	// AttrMetered keeps what the Voice Agent tables say behind their
	// information icon, which is that a minute is a minute of connection
	// rather than a minute of audio.
	AttrMetered = "metered_on"
)

// ListPreviousRates keeps the struck-through amounts, which are what Deepgram
// charged before the rates beside them. There is one per plan and they differ,
// so they are a list: a model whose promotion covers two plans has two.
const ListPreviousRates = "previous_rates"

// planDescription is the column heading the add-on table puts between the
// feature and its rates. It describes the feature rather than pricing it, so
// what it holds is a summary and not an amount.
const planDescription = "description"

var (
	tagRe    = regexp.MustCompile(`(?s)<[^>]*>`)
	amountRe = regexp.MustCompile(
		`\$\s*([\d,]*\.?\d+)\s*(?:/\s*([A-Za-z0-9 ]{1,18}))?`,
	)
	// struckRe matches the wrapper Deepgram puts around a withdrawn rate. The
	// wide tables mark it as deleted text and the narrow ones only draw a line
	// through it, so both forms are matched: a rate read from either without
	// this is recorded as one Deepgram still charges.
	struckRe = regexp.MustCompile(
		`(?is)<del[^>]*>(.*?)</del>|` +
			`<span[^>]*text-gray-500[^>]*>(.*?)</span>\s*<span[^>]*aria-hidden`,
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
		denominator := trimDenominator(match[2])
		unit, _ := unitFor(denominator)
		out = append(out, rate{
			Amount: value,
			Unit:   unit,
			Raw:    raw(match[1], denominator),
		})
	}
	return out
}

// denominatorStops are the words that follow a denominator rather than belong
// to it. Deepgram writes the date an offer runs to straight after the unit, as
// "$0.110/min through 9/12", and a denominator read up to the next slash would
// otherwise swallow the first half of the date.
var denominatorStops = map[string]bool{
	"through": true,
	"until":   true,
	"then":    true,
	"free":    true,
	"i":       true,
}

// trimDenominator drops what follows the denominator in the same cell.
func trimDenominator(denominator string) string {
	fields := strings.Fields(denominator)
	for i, field := range fields {
		if denominatorStops[strings.ToLower(field)] {
			fields = fields[:i]
			break
		}
	}
	return strings.Join(fields, " ")
}

// raw rewrites an amount as Deepgram quotes it, which is what a struck-through
// rate is recognised by and what the previous rate is recorded as.
func raw(amount, denominator string) string {
	if denominator == "" {
		return "$" + amount
	}
	return "$" + amount + "/" + denominator
}

// struckAmounts returns the amounts a cell shows struck through, which are the
// rates Deepgram no longer charges.
func struckAmounts(cellHTML string) map[string]bool {
	out := map[string]bool{}
	for _, match := range struckRe.FindAllStringSubmatch(cellHTML, -1) {
		for _, group := range match[1:] {
			for _, r := range parseRates(text(group)) {
				out[r.Raw] = true
			}
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
