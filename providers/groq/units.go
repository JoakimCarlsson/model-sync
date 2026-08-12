package groq

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Metrics Groq bills on.
const (
	MetricInputTokens  catalog.Metric = "input_tokens"
	MetricOutputTokens catalog.Metric = "output_tokens"
	MetricAudio        catalog.Metric = "audio"
	MetricSpeech       catalog.Metric = "speech"
)

// Units Groq quotes amounts against.
const (
	UnitPer1MTokens catalog.Unit = "per_1m_tokens"
	UnitPer1MChars  catalog.Unit = "per_1m_characters"
	UnitPerHour     catalog.Unit = "per_hour"
)

// Kinds of model Groq serves.
const (
	KindChat  catalog.Kind = "chat"
	KindAudio catalog.Kind = "audio"
)

// States Groq distinguishes by which table a model appears in.
const (
	StateProduction = "production"
	StatePreview    = "preview"
)

// Scalar keys the models page populates.
const (
	AttrState        = "state"
	AttrTokensPerSec = "tokens_per_second"
	AttrMaxFileSize  = "max_file_size"
	AttrSystem       = "is_system"
	// AttrAccess records the badge marking a model as restricted to a plan.
	AttrAccess = "access"
)

// Numeric keys the models page populates. The rate limit codes are Groq's
// own: TPM and RPM are tokens and requests per minute, ASH is audio seconds
// per hour.
const (
	LimitContextWindow   = "context_window"
	LimitMaxOutputTokens = "max_output_tokens"
)

// rateLimitKeys maps Groq's rate limit codes onto numeric keys.
var rateLimitKeys = map[string]string{
	"tpm": "tokens_per_minute",
	"rpm": "requests_per_minute",
	"tpd": "tokens_per_day",
	"rpd": "requests_per_day",
	"ash": "audio_seconds_per_hour",
	"asd": "audio_seconds_per_day",
}

var (
	imageRe = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	linkRe  = regexp.MustCompile(`\[([^\]]*)\]\(([^)]*)\)`)
	// rateRe matches an amount and the label following it within one fragment
	// of a price cell.
	rateRe = regexp.MustCompile(`^\s*([\d,]*\.?\d+)\s*(.*)$`)
	// limitRe matches one rate limit and its code, likewise run together.
	limitRe = regexp.MustCompile(`([\d,]*\.?\d+)\s*([KMB]?)\s*([A-Za-z]{3})`)
	countRe = regexp.MustCompile(`(?i)^([\d,]*\.?\d+)\s*([kmb])?\b`)
)

// clean strips markdown decoration from a cell value.
func clean(cell string) string {
	s := imageRe.ReplaceAllString(cell, "")
	s = linkRe.ReplaceAllString(s, "$1")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, `\-`, "-")
	return strings.Join(strings.Fields(s), " ")
}

// modelPagePrefix is the path a model's own page sits under, which is the
// identifier with a prefix.
const modelPagePrefix = "/docs/model/"

// modelRef is what one model-id cell holds: a link to the model page, then any
// access badge, then the bare identifier, all run together.
type modelRef struct {
	ID    string
	Name  string
	Badge string
}

// splitModelCell separates them.
//
// The identifier is taken from the link target rather than from the trailing
// text, because a badge sits between the two with no separator: the cell for
// MiniMax M2.7 ends "Enterpriseminimaxai/minimax-m2.7", and reading the
// trailing text would name the model after the badge.
func splitModelCell(cell string) modelRef {
	text := imageRe.ReplaceAllString(cell, "")
	match := linkRe.FindStringSubmatchIndex(text)
	if match == nil {
		return modelRef{ID: strings.TrimSpace(clean(text))}
	}
	ref := modelRef{Name: strings.TrimSpace(text[match[2]:match[3]])}
	trailing := strings.TrimSpace(text[match[1]:])
	target := strings.TrimSpace(text[match[4]:match[5]])
	if id, ok := strings.CutPrefix(target, modelPagePrefix); ok && id != "" {
		ref.ID = id
		ref.Badge = strings.TrimSpace(strings.TrimSuffix(trailing, id))
		return ref
	}
	ref.ID = trailing
	if ref.ID == "" {
		ref.ID = ref.Name
	}
	return ref
}

// rate is one amount and what it is charged for.
type rate struct {
	Metric catalog.Metric
	Unit   catalog.Unit
	Amount float64
}

// parseRates reads every amount in a price cell. Groq states two token rates
// in one cell for a text model, and a single rate with its own denominator for
// a speech one.
//
// The cell is split on the currency sign rather than matched as a whole,
// because the amounts are run together with no separator and each label ends
// only where the next amount begins.
func parseRates(cell string) []rate {
	var out []rate
	for _, fragment := range strings.Split(clean(cell), "$") {
		match := rateRe.FindStringSubmatch(fragment)
		if match == nil {
			continue
		}
		value, err := strconv.ParseFloat(
			strings.ReplaceAll(match[1], ",", ""),
			64,
		)
		if err != nil {
			continue
		}
		metric, unit, ok := billingFor(match[2])
		if !ok {
			continue
		}
		out = append(out, rate{Metric: metric, Unit: unit, Amount: value})
	}
	return out
}

// billingFor maps the label following an amount.
func billingFor(label string) (catalog.Metric, catalog.Unit, bool) {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "input":
		return MetricInputTokens, UnitPer1MTokens, true
	case "output":
		return MetricOutputTokens, UnitPer1MTokens, true
	case "per 1m characters", "per 1m character":
		return MetricSpeech, UnitPer1MChars, true
	case "per hour":
		return MetricAudio, UnitPerHour, true
	}
	return "", "", false
}

// parseLimits reads every rate limit in a cell.
func parseLimits(cell string) map[string]int64 {
	out := map[string]int64{}
	for _, match := range limitRe.FindAllStringSubmatch(clean(cell), -1) {
		key, ok := rateLimitKeys[strings.ToLower(match[3])]
		if !ok {
			continue
		}
		out[key] = scaled(match[1], match[2])
	}
	return out
}

// parseCount reads a quantity such as "131,072" or "250K".
func parseCount(value string) int64 {
	match := countRe.FindStringSubmatch(clean(value))
	if match == nil {
		return 0
	}
	return scaled(match[1], match[2])
}

// scaled applies a magnitude suffix to a number.
func scaled(number, suffix string) int64 {
	n, err := strconv.ParseFloat(strings.ReplaceAll(number, ",", ""), 64)
	if err != nil {
		return 0
	}
	switch strings.ToLower(suffix) {
	case "k":
		n *= 1_000
	case "m":
		n *= 1_000_000
	case "b":
		n *= 1_000_000_000
	}
	return int64(n)
}

// kindForRates reports what a model is from how it is billed, which is the
// only signal Groq gives: a model charged per hour or per character works on
// speech, and everything else on text.
func kindForRates(rates []rate) catalog.Kind {
	for _, r := range rates {
		if r.Unit == UnitPerHour || r.Unit == UnitPer1MChars {
			return KindAudio
		}
	}
	return KindChat
}
