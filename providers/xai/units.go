package xai

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Metrics xAI bills on.
const (
	MetricInputTokens       catalog.Metric = "input_tokens"
	MetricCachedInputTokens catalog.Metric = "cached_input_tokens"
	MetricOutputTokens      catalog.Metric = "output_tokens"
	MetricImageOutput       catalog.Metric = "image_output"
	MetricVideoOutput       catalog.Metric = "video_output"
	MetricAudio             catalog.Metric = "audio"
	MetricToolCall          catalog.Metric = "tool_call"
)

// Units xAI quotes amounts against.
const (
	UnitPer1MTokens catalog.Unit = "per_1m_tokens"
	UnitPer1MChars  catalog.Unit = "per_1m_characters"
	UnitPerImage    catalog.Unit = "per_image"
	UnitPerSecond   catalog.Unit = "per_second"
	UnitPerMinute   catalog.Unit = "per_minute"
	UnitPerHour     catalog.Unit = "per_hour"
	UnitPer1KCalls  catalog.Unit = "per_1k_calls"
)

// Kinds of model xAI publishes.
const (
	KindChat  catalog.Kind = "chat"
	KindImage catalog.Kind = "image"
	KindVideo catalog.Kind = "video"
	KindVoice catalog.Kind = "voice"
	KindTool  catalog.Kind = "tool"
)

// Dimension keys xAI's prices vary along. The prompt band is xAI's alternative
// to a service tier: one model has two input rates, chosen by how many tokens
// the request carries rather than by which endpoint served it.
const (
	DimPromptBand = "prompt_band"
	DimMode       = "mode"
	DimModality   = "modality"
	// DimTransport separates rates that differ only by how the request is
	// delivered, which xAI prices differently for REST and streaming.
	DimTransport = "transport"
)

// Scalar keys the model pages populate.
const (
	AttrSummary         = "summary"
	AttrKnowledgeCutoff = "knowledge_cutoff"
	AttrBatchDiscount   = "batch_discount"
)

// Numeric keys the documents populate.
const (
	LimitContextWindow    = "context_window"
	LimitRequestsPerSec   = "requests_per_second"
	LimitTokensPerMinute  = "tokens_per_minute"
	LimitRequestsPerMin   = "requests_per_minute"
	LimitImagesPerMinute  = "images_per_minute"
	LimitRequestsPerHour  = "requests_per_hour"
	LimitConcurrentJobs   = "concurrent_jobs"
	LimitVideosPerMinute  = "videos_per_minute"
	LimitSessionsPerHour  = "sessions_per_hour"
	LimitMinutesPerMinute = "minutes_per_minute"
)

// Enumeration keys the documents populate.
const (
	ListFeatures         = catalog.ListFeatures
	ListAliases          = "aliases"
	ListRegions          = "regions"
	ListInputModalities  = "input_modalities"
	ListOutputModalities = "output_modalities"
)

var (
	linkRe    = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	tagRe     = regexp.MustCompile(`(?i)<br\s*/?>`)
	htmlTagRe = regexp.MustCompile(`(?s)<[^>]*>`)
	// amountRe matches a rate. The denominator is optional because the text
	// table states it in the column heading and leaves the cell a bare amount,
	// and it may begin with a digit because xAI writes "1M chars".
	amountRe = regexp.MustCompile(
		`\$\s*([\d,]+(?:\.\d+)?)\s*(?:(?:/|per\b)\s*([A-Za-z0-9][\w ]*)?)?`,
	)
	countRe  = regexp.MustCompile(`^([\d,.]+)\s*([kKmM])?`)
	cutoffRe = regexp.MustCompile(
		`(?i)knowledge cut-?off date of (.+?) is ([^.]+)\.`,
	)
	discountRe = regexp.MustCompile(`(?m)^\*\*(\d+)% off standard rates\*\*`)
)

// clean strips markdown and HTML decoration from a cell value.
func clean(cell string) string {
	s := linkRe.ReplaceAllString(cell, "$1")
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, `\`, "")
	return strings.Join(strings.Fields(s), " ")
}

// splitFragments divides a cell that states more than one rate. xAI separates
// them with a line break element inside the cell.
func splitFragments(cell string) []string {
	var out []string
	for _, part := range tagRe.Split(cell, -1) {
		if text := clean(part); text != "" {
			out = append(out, text)
		}
	}
	return out
}

// splitRates divides a fragment stating more than one rate for one mode, which
// xAI separates with a comma. A comma inside a grouped number is put back,
// since only a clause naming its own amount is a rate.
func splitRates(fragment string) []string {
	var out []string
	for _, part := range strings.Split(fragment, ",") {
		switch {
		case strings.Contains(part, "$"):
			out = append(out, strings.TrimSpace(part))
		case len(out) > 0:
			out[len(out)-1] += "," + part
		}
	}
	if len(out) == 0 {
		return []string{fragment}
	}
	return out
}

// rateQualifier separates a trailing parenthesis naming how a rate is
// delivered from one merely restating the same rate in another unit. xAI
// writes both, as "(Streaming)" and as "($3.00 / hr)", and only the first
// distinguishes one price from another.
func rateQualifier(clause string) (rest, qualifier string) {
	text := strings.TrimSpace(clause)
	open := strings.LastIndex(text, "(")
	if open < 0 || !strings.HasSuffix(text, ")") {
		return text, ""
	}
	inner := strings.TrimSpace(text[open+1 : len(text)-1])
	if inner == "" || strings.Contains(inner, "$") {
		return text, ""
	}
	return strings.TrimSpace(text[:open]), slugID(inner)
}

// unitFor maps xAI's denominator wording onto a unit.
func unitFor(text string) (catalog.Unit, bool) {
	field := strings.ToLower(strings.TrimSpace(text))
	field = strings.TrimSuffix(field, "s")
	switch field {
	case "1m token", "1m prompt token", "million token":
		return UnitPer1MTokens, true
	case "1m char", "1m character":
		return UnitPer1MChars, true
	case "image":
		return UnitPerImage, true
	case "sec", "second":
		return UnitPerSecond, true
	case "min", "minute":
		return UnitPerMinute, true
	case "hr", "hour":
		return UnitPerHour, true
	case "1k call":
		return UnitPer1KCalls, true
	}
	return "", false
}

// amount is one parsed price.
type amount struct {
	Value float64
	Unit  catalog.Unit
	Note  string
	Found bool
}

// parseAmount reads the first rate in a fragment, such as "$0.05 / min" or
// "$0.080 per second". Whatever follows the rate is kept as a note, which is
// where xAI puts the equivalent hourly rate and the modality a voice rate
// applies to.
func parseAmount(fragment string) amount {
	text := clean(fragment)
	at := amountRe.FindStringSubmatchIndex(text)
	if at == nil {
		return amount{Note: text}
	}
	raw := text[at[2]:at[3]]
	value, err := strconv.ParseFloat(strings.ReplaceAll(raw, ",", ""), 64)
	if err != nil {
		return amount{Note: text}
	}
	out := amount{Value: value, Found: true}
	rest := text[at[1]:]
	if at[4] >= 0 {
		if unit, ok := unitFor(text[at[4]:at[5]]); ok {
			out.Unit = unit
		} else {
			rest = text[at[4]:]
		}
	}
	out.Note = strings.TrimSpace(strings.Trim(strings.TrimSpace(rest), ","))
	return out
}

// parseCount reads a quantity such as "500k", "1M", "50,000,000" or
// "500,000 tokens".
func parseCount(value string) int64 {
	match := countRe.FindStringSubmatch(strings.TrimSpace(clean(value)))
	if match == nil {
		return 0
	}
	n, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", ""), 64)
	if err != nil {
		return 0
	}
	switch strings.ToLower(match[2]) {
	case "k":
		n *= 1_000
	case "m":
		n *= 1_000_000
	}
	return int64(n)
}

// promptBands are the thresholds xAI appends to a model name to say which of
// its two rates a row states.
var promptBands = strings.NewReplacer(
	"≥", ">=",
	"<", "<",
	" prompt tokens", "",
	" prompt token", "",
)

// modelRef is a model named by a pricing row together with the prompt band the
// row's rates apply to.
type modelRef struct {
	ID   string
	Band string
}

// splitModelCell separates the identifier from the prompt band xAI writes in
// parentheses after it.
func splitModelCell(cell string) modelRef {
	text := clean(cell)
	open := strings.Index(text, "(")
	if open < 0 || !strings.HasSuffix(text, ")") {
		return modelRef{ID: text}
	}
	band := strings.TrimSpace(text[open+1 : len(text)-1])
	return modelRef{
		ID:   strings.TrimSpace(text[:open]),
		Band: strings.Join(strings.Fields(promptBands.Replace(band)), ""),
	}
}

// dateLayouts are the date formats xAI writes.
var dateLayouts = []struct{ in, out string }{
	{"January 2, 2006", "2006-01-02"},
	{"Jan 2, 2006", "2006-01-02"},
	{"2006-01-02", "2006-01-02"},
	{"January 2006", "2006-01"},
	{"Jan 2006", "2006-01"},
}

// isoDate rewrites a date into its machine readable form, keeping the
// precision it was written at.
func isoDate(value string) string {
	text := strings.TrimSpace(clean(value))
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout.in, text); err == nil {
			return t.Format(layout.out)
		}
	}
	return text
}

// slugID turns a display name such as "Speech to Text" into an identifier.
func slugID(name string) string {
	s := strings.ToLower(clean(name))
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' {
			return r
		}
		return '-'
	}, s)
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}
