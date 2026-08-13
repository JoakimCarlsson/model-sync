package together

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Metrics Together bills on.
const (
	MetricInputTokens       catalog.Metric = "input_tokens"
	MetricCachedInputTokens catalog.Metric = "cached_input_tokens"
	MetricOutputTokens      catalog.Metric = "output_tokens"
	MetricImageOutput       catalog.Metric = "image_output"
	MetricVideoOutput       catalog.Metric = "video_output"
	MetricAudio             catalog.Metric = "audio"
)

// Units Together quotes amounts against.
const (
	UnitPer1MTokens  catalog.Unit = "per_1m_tokens"
	UnitPer1MChars   catalog.Unit = "per_1m_characters"
	UnitPerMinute    catalog.Unit = "per_minute"
	UnitPerMegapixel catalog.Unit = "per_megapixel"
	UnitPerVideo     catalog.Unit = "per_video"
)

// Kinds of model Together serves.
const (
	KindChat       catalog.Kind = "chat"
	KindImage      catalog.Kind = "image"
	KindVideo      catalog.Kind = "video"
	KindAudio      catalog.Kind = "audio"
	KindEmbedding  catalog.Kind = "embedding"
	KindRerank     catalog.Kind = "rerank"
	KindModeration catalog.Kind = "moderation"
)

// Dimension keys Together's prices vary along.
const (
	DimResolution = "resolution"
	DimDuration   = "duration"
)

// Scalar keys the catalog page populates.
const (
	AttrAuthor           = "author"
	AttrModality         = "modality"
	AttrModelSize        = "model_size"
	AttrDefaultSteps     = "default_steps"
	AttrDefaultDimension = "default_embedding_dimension"
	AttrQuantization     = "quantization"
)

// Numeric keys the catalog page populates.
const LimitContextWindow = "context_window"

// Enumeration keys the catalog page populates.
const ListDimensions = "embedding_dimensions"

// ListFeatures holds the capabilities the catalog reports on per model, which
// it states as a yes-or-no column each.
const ListFeatures = "features"

// Capabilities the catalog's columns report on.
const (
	FeatureFunctionCalling   = "function_calling"
	FeatureStructuredOutputs = "structured_outputs"
)

var (
	linkRe   = regexp.MustCompile(`\[!?\[?([^\]]*)\]?\]\([^)]*\)`)
	imageRe  = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	amountRe = regexp.MustCompile(`\$\s*([\d,]*\.?\d+)`)
	countRe  = regexp.MustCompile(`(?i)^([\d,]*\.?\d+)\s*([kmb])?\b`)
)

// clean strips the decoration Together wraps around cell values. Its model
// cells carry an organization logo and a link to the model page, and every
// dollar sign in the document is escaped.
func clean(cell string) string {
	s := imageRe.ReplaceAllString(cell, "")
	s = linkRe.ReplaceAllString(s, "$1")
	s = strings.ReplaceAll(s, `\$`, "$")
	s = strings.ReplaceAll(s, `\[`, "[")
	s = strings.ReplaceAll(s, `\]`, "]")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "`", "")
	return strings.Join(strings.Fields(s), " ")
}

// amount is one parsed price cell.
type amount struct {
	Value float64
	Unit  catalog.Unit
	Found bool
}

// parseAmount reads a price cell. Together writes a bare amount where the
// column heading states the denominator, an amount followed by its own
// denominator where the column does not, and the word Free for a model it does
// not charge for.
func parseAmount(cell string) amount {
	text := clean(cell)
	if text == "" || text == "-" {
		return amount{}
	}
	if strings.EqualFold(text, "free") {
		return amount{Found: true}
	}
	match := amountRe.FindStringSubmatchIndex(text)
	if match == nil {
		return amount{}
	}
	value, err := strconv.ParseFloat(
		strings.ReplaceAll(text[match[2]:match[3]], ",", ""),
		64,
	)
	if err != nil {
		return amount{}
	}
	out := amount{Value: value, Found: true}
	if unit, ok := unitFor(text[match[1]:]); ok {
		out.Unit = unit
	}
	return out
}

// unitFor maps the denominator Together writes beside an amount.
func unitFor(text string) (catalog.Unit, bool) {
	switch {
	case text == "":
		return "", false
	case strings.Contains(text, "1M chars"), strings.Contains(text, "1M char"):
		return UnitPer1MChars, true
	case strings.Contains(text, "audio min"), strings.Contains(text, "min"):
		return UnitPerMinute, true
	case strings.Contains(text, "1M token"):
		return UnitPer1MTokens, true
	}
	return "", false
}

// parseCount reads a quantity such as "524288", "560M" or "1024".
func parseCount(value string) int64 {
	match := countRe.FindStringSubmatch(clean(value))
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
	case "b":
		n *= 1_000_000_000
	}
	return int64(n)
}

// modalityKind maps the Modality column of the audio table, which is the only
// thing distinguishing a model that reads speech from one that writes it.
func modalityKind(cell string) (catalog.Kind, bool) {
	switch strings.ToLower(clean(cell)) {
	case "text-to-speech", "speech-to-text":
		return KindAudio, true
	}
	return "", false
}
