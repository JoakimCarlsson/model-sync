package openai

import (
	"strconv"
	"strings"
	"time"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Metrics OpenAI bills on.
const (
	MetricInputTokens       catalog.Metric = "input_tokens"
	MetricCachedInputTokens catalog.Metric = "cached_input_tokens"
	MetricCacheWriteTokens  catalog.Metric = "cache_write_tokens"
	MetricOutputTokens      catalog.Metric = "output_tokens"
	MetricImageOutput       catalog.Metric = "image_output"
	MetricVideoOutput       catalog.Metric = "video_output"
	MetricTraining          catalog.Metric = "training"
	MetricToolCall          catalog.Metric = "tool_call"
	MetricStorage           catalog.Metric = "storage"
	MetricUsage             catalog.Metric = "usage"
)

// Units OpenAI quotes amounts against.
const (
	UnitPer1MTokens catalog.Unit = "per_1m_tokens"
	UnitPer1MChars  catalog.Unit = "per_1m_characters"
	UnitPerImage    catalog.Unit = "per_image"
	UnitPerSecond   catalog.Unit = "per_second"
	UnitPerMinute   catalog.Unit = "per_minute"
	UnitPerHour     catalog.Unit = "per_hour"
	UnitPer1KCalls  catalog.Unit = "per_1k_calls"
	UnitPerGBDay    catalog.Unit = "per_gb_day"
	// UnitPerSession prices a container for the length of one session, which
	// OpenAI defines as twenty minutes.
	UnitPerSession catalog.Unit = "per_20_minute_session"
)

// Kinds of model OpenAI publishes.
const (
	KindChat          catalog.Kind = "chat"
	KindImage         catalog.Kind = "image"
	KindVideo         catalog.Kind = "video"
	KindAudio         catalog.Kind = "audio"
	KindRealtime      catalog.Kind = "realtime"
	KindTranscription catalog.Kind = "transcription"
	KindEmbedding     catalog.Kind = "embedding"
	KindModeration    catalog.Kind = "moderation"
	KindTool          catalog.Kind = "tool"
	KindFinetune      catalog.Kind = "finetune"
)

// Dimension keys OpenAI's prices vary along.
const (
	DimTier        = "tier"
	DimContext     = "context"
	DimModality    = "modality"
	DimQuality     = "quality"
	DimSize        = "size"
	DimOrientation = "orientation"
	DimUseCase     = "use_case"
	DimDetail      = "detail"
	DimMemory      = "memory"
	DimDataSharing = "data_sharing"
	DimLegacy      = "legacy"
	DimMaxContext  = "max_context_length"
	// dimPortrait and dimLandscape are the two columns the video rate table
	// prices a resolution under, each holding the pixel size of that
	// orientation.
	dimPortrait  = "portrait"
	dimLandscape = "landscape"
)

// Service tiers OpenAI prices separately.
const (
	TierStandard = "standard"
	TierBatch    = "batch"
	TierFlex     = "flex"
	TierFast     = "fast"
)

// tierFor recognizes the bare line that introduces a tier's tables.
func tierFor(line string) (string, bool) {
	switch strings.ToLower(line) {
	case "standard":
		return TierStandard, true
	case "batch":
		return TierBatch, true
	case "flex":
		return TierFlex, true
	case "fast mode", "fast":
		return TierFast, true
	}
	return "", false
}

// sectionKind recognizes the bare line that introduces a family of tables and
// reports the kind of model those tables describe.
func sectionKind(line string) (catalog.Kind, bool) {
	switch strings.ToLower(line) {
	case "flagship models", "cyber models", "specialized models":
		return KindChat, true
	case "realtime and audio generation models":
		return KindRealtime, true
	case "image generation models":
		return KindImage, true
	case "video generation models":
		return KindVideo, true
	case "transcription models":
		return KindTranscription, true
	case "tools":
		return KindTool, true
	case "finetuning":
		return KindFinetune, true
	}
	return "", false
}

// categoryKind maps the Category column of the specialized-models table.
func categoryKind(cell string) (catalog.Kind, bool) {
	switch strings.ToLower(cell) {
	case "embedding":
		return KindEmbedding, true
	case "moderation":
		return KindModeration, true
	case "chatgpt", "codex", "search":
		return KindChat, true
	}
	return "", false
}

// unitHint reads the "Prices per 1M tokens." line that states the denominator
// for a section's tables when the cells themselves omit it.
func unitHint(line string) (catalog.Unit, bool) {
	l := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(line), "."))
	if !strings.HasPrefix(l, "prices per ") {
		return "", false
	}
	l = strings.TrimSuffix(
		strings.TrimPrefix(l, "prices per "),
		" unless noted",
	)
	return unitFor(l)
}

// unitPhrases is every wording OpenAI uses for a denominator.
var unitPhrases = map[string]catalog.Unit{
	"1m tokens":     UnitPer1MTokens,
	"1m token":      UnitPer1MTokens,
	"1m characters": UnitPer1MChars,
	"1m character":  UnitPer1MChars,
	"image":         UnitPerImage,
	"second":        UnitPerSecond,
	"minute":        UnitPerMinute,
	"hour":          UnitPerHour,
	"1k calls":      UnitPer1KCalls,
	"1k call":       UnitPer1KCalls,
	"gb per day":    UnitPerGBDay,
	"gb-day":        UnitPerGBDay,
	"gb day":        UnitPerGBDay,
}

// unitFor maps a denominator that stands alone onto a unit.
func unitFor(text string) (catalog.Unit, bool) {
	u, ok := unitPhrases[strings.ToLower(strings.TrimSpace(text))]
	return u, ok
}

// unitPrefix reads a denominator off the front of a phrase and returns what
// follows it. OpenAI runs prose straight on from the unit, as in "GB-day after
// 1 GB free per account per month", so an exact match is not enough. The
// longest matching phrase wins, so "gb per day" is preferred over "gb day".
func unitPrefix(text string) (catalog.Unit, string, bool) {
	lower := strings.ToLower(text)
	longest := ""
	var unit catalog.Unit
	for phrase, u := range unitPhrases {
		if strings.HasPrefix(lower, phrase) && len(phrase) > len(longest) {
			longest, unit = phrase, u
		}
	}
	if longest == "" {
		return "", text, false
	}
	return unit, strings.TrimSpace(text[len(longest):]), true
}

// amount is one parsed price cell.
type amount struct {
	Value float64
	Unit  catalog.Unit
	Note  string
	Found bool
}

// parseAmount reads a price cell. OpenAI writes plain amounts ("$5.00"),
// amounts carrying their own denominator ("$15.00 / 1M characters"), amounts
// with a rider ("$10.00 / 1k calls + Search content tokens billed at model
// rates."), the word Free, an em dash for "not offered", and occasionally a
// sentence listing several amounts at once. Everything that is not an amount
// is returned as a note so it survives into the output.
func parseAmount(cell string) amount {
	c := strings.TrimSpace(cell)
	if c == "" || c == "-" || c == "—" || c == "–" {
		return amount{}
	}
	if strings.EqualFold(c, "free") {
		return amount{Found: true}
	}
	if !strings.HasPrefix(c, "$") {
		return amount{Note: c}
	}
	rest := strings.TrimSpace(c[1:])
	end := 0
	for end < len(rest) && (rest[end] >= '0' && rest[end] <= '9' || rest[end] == '.' || rest[end] == ',') {
		end++
	}
	value, err := strconv.ParseFloat(
		strings.ReplaceAll(strings.TrimSuffix(rest[:end], "."), ",", ""),
		64,
	)
	if err != nil {
		return amount{Note: c}
	}
	out := amount{Value: value, Found: true}
	rest = strings.TrimSpace(rest[end:])
	if after, ok := strings.CutPrefix(rest, "/"); ok {
		if unit, tail, found := unitPrefix(strings.TrimSpace(after)); found {
			out.Unit, rest = unit, tail
		} else {
			rest = strings.TrimSpace(after)
		}
	}
	out.Note = rest
	return out
}

// dateLayouts are the date formats OpenAI writes, paired with the precision to
// render each at. Deprecation tables use abbreviated and full month names as
// well as calendar dates, and a knowledge cutoff is sometimes only a month.
var dateLayouts = []struct{ in, out string }{
	{"2006-01-02", "2006-01-02"},
	{"Jan 2, 2006", "2006-01-02"},
	{"January 2, 2006", "2006-01-02"},
	{"Jan 2006", "2006-01"},
	{"January 2006", "2006-01"},
}

// isoDate rewrites a date into its machine readable form, keeping the
// precision it was written at, so "Feb 16, 2026" becomes 2026-02-16 and
// "Oct 2023" becomes 2023-10. A value in no recognized format is returned
// unchanged rather than dropped.
func isoDate(value string) string {
	text := strings.TrimSpace(hyphens.Replace(value))
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout.in, text); err == nil {
			return t.Format(layout.out)
		}
	}
	return strings.TrimSpace(value)
}

// hyphens normalizes the typographic dashes OpenAI's tables use in dates,
// which are not the hyphen time.Parse expects.
var hyphens = strings.NewReplacer(
	"‐", "-",
	"‑", "-",
	"‒", "-",
	"–", "-",
	"—", "-",
)

// idAliases maps a display name OpenAI uses in a pricing row onto the
// identifier its API and model pages use, so one model does not become two.
var idAliases = map[string]string{
	"whisper": "whisper-1",
}

// slugID turns a display name such as "GPT Image 1 Mini" into the identifier
// OpenAI uses in its API.
func slugID(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	if alias, ok := idAliases[strings.Join(strings.Fields(s), "-")]; ok {
		return alias
	}
	s = strings.ReplaceAll(s, " ", " ")
	s = strings.Join(strings.Fields(s), "-")
	return s
}

// qualifier is a parenthesized suffix OpenAI appends to a model name in a
// pricing row, carrying a dimension rather than part of the identifier. Name is
// the cell as written, which is the identifier itself in a model row and a
// display name in the tool table.
type qualifier struct {
	ID   string
	Name string
	Dims catalog.Dims
	Note string
}

// splitQualifier separates a model cell into its identifier and whatever the
// trailing parenthesis says about the row.
func splitQualifier(cell string) qualifier {
	raw := strings.TrimSpace(strings.ReplaceAll(cell, "`", ""))
	open := strings.Index(raw, "(")
	if open < 0 || !strings.HasSuffix(raw, ")") {
		return qualifier{ID: slugID(raw), Name: raw}
	}
	inner := strings.TrimSpace(raw[open+1 : len(raw)-1])
	name := strings.TrimSpace(raw[:open])
	q := qualifier{
		ID:   slugID(name),
		Name: name,
		Dims: catalog.Dims{},
	}
	switch {
	case strings.EqualFold(inner, "data sharing"):
		q.Dims[DimDataSharing] = "true"
	case strings.EqualFold(inner, "legacy"):
		q.Dims[DimLegacy] = "true"
	case strings.HasSuffix(strings.ToLower(inner), "context length"):
		q.Dims[DimMaxContext] = strings.TrimSpace(
			strings.TrimSuffix(strings.ToLower(inner), "context length"),
		)
	default:
		q.Note = inner
	}
	return q
}
