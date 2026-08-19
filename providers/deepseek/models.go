package deepseek

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Metrics DeepSeek bills on.
const (
	MetricInputTokens       catalog.Metric = "input_tokens"
	MetricCachedInputTokens catalog.Metric = "cached_input_tokens"
	MetricOutputTokens      catalog.Metric = "output_tokens"
)

// UnitPer1MTokens is the only denominator DeepSeek quotes.
const UnitPer1MTokens catalog.Unit = "per_1m_tokens"

// KindChat is the only kind DeepSeek publishes.
const KindChat catalog.Kind = "chat"

// DimPeriod is the axis DeepSeek varies every rate along: the hour of the day
// a request is made in. It has no analogue at any other provider, so it is a
// dimension rather than a metric of its own.
const DimPeriod = "period"

// The two values of DimPeriod, as the pricing table heads them.
const (
	PeriodPeak    = "peak"
	PeriodOffPeak = "off_peak"
)

// Scalar keys the pricing page populates.
const (
	AttrModelVersion     = "model_version"
	AttrDefaultSnapshot  = "default_snapshot"
	AttrThinkingMode     = "thinking_mode"
	AttrBaseURL          = "base_url"
	AttrAnthropicBaseURL = "anthropic_base_url"
	AttrFIMModes         = "fim_completion_modes"
)

// Scalar keys the change log populates.
const (
	AttrSummary     = "summary"
	AttrState       = "state"
	AttrReleaseDate = "release_date"
)

// Scalar keys the model card populates.
const (
	AttrLicense             = "license"
	AttrOpenWeights         = "open_weights"
	AttrHuggingFaceID       = "hugging_face_id"
	AttrModelCardURL        = "model_card_url"
	AttrQuantization        = "quantization"
	AttrTotalParameters     = "total_parameters"
	AttrActivatedParameters = "activated_parameters"
	AttrTechnicalReportURL  = "technical_report_url"
)

// Scalar keys the guides populate.
const (
	AttrBetaBaseURL            = "beta_base_url"
	AttrThinkingModeDefault    = "thinking_mode_default"
	AttrDefaultReasoningEffort = "default_reasoning_effort"
	AttrConcurrencyScope       = "concurrency_limit_scope"
	AttrContextCacheLifetime   = "context_cache_lifetime"
)

// Numeric keys the pricing page populates.
const (
	LimitContextWindow   = "context_window"
	LimitMaxOutputTokens = "max_output_tokens"
	LimitConcurrency     = "concurrency_limit"
)

// Numeric keys the guides populate.
const (
	// LimitConcurrencyPerUserID is the second concurrency ceiling, applied to
	// each user_id an expanded account passes rather than to the account.
	LimitConcurrencyPerUserID = "concurrency_limit_per_user_id"
	// LimitFIMMaxOutputTokens is the output ceiling of the beta FIM endpoint,
	// which is far below the ceiling of the chat endpoints.
	LimitFIMMaxOutputTokens = "fim_max_output_tokens"
	// LimitInferenceStartTimeout is how long a queued request is held open
	// before the server closes the connection.
	LimitInferenceStartTimeout = "inference_start_timeout_seconds"
)

// Enumeration keys the pricing table populates.
const (
	// ListFeatures holds the capabilities marked as supported.
	ListFeatures = catalog.ListFeatures
	// ListEndpoints holds the APIs a model answers on, which DeepSeek marks
	// as supported in the same column as its capabilities.
	ListEndpoints = "endpoints"
)

// Enumeration keys the guides populate.
const (
	ListInputModalities  = "input_modalities"
	ListOutputModalities = "output_modalities"
	// ListParameters holds the Responses API request parameters DeepSeek
	// states it honours.
	ListParameters = catalog.ListParameters
	// ListReasoningEfforts holds the effort levels the thinking mode accepts.
	ListReasoningEfforts = "reasoning_efforts"
	// ListThinkingUnsupported holds the sampling parameters the thinking mode
	// accepts without acting on.
	ListThinkingUnsupported = "thinking_mode_unsupported_parameters"
)

// Capabilities DeepSeek names that the catalog has no constant for.
const (
	FeatureChatPrefixCompletion = "chat_prefix_completion"
	FeatureFIMCompletion        = "fim_completion"
	FeatureWebSearch            = "web_search"
	FeatureContextCaching       = "context_caching"
)

// Modalities DeepSeek names a content part for.
const (
	ModalityText  = "text"
	ModalityImage = "image"
)

// The APIs DeepSeek states a base URL or a support tick for.
const (
	EndpointOpenAI    = "OpenAI"
	EndpointAnthropic = "Anthropic"
	EndpointResponses = "Responses"
)

// featureNames map a row label onto the catalog's vocabulary. DeepSeek heads a
// row with prose, so the label is not an identifier and is translated into
// one; anything not listed keeps DeepSeek's own words with its punctuation and
// spacing reduced.
var featureNames = map[string]string{
	"tool calls":                   catalog.CapabilityFunctionCalling,
	"chat prefix completion（beta）": FeatureChatPrefixCompletion,
	"fim completion（beta）":         FeatureFIMCompletion,
}

// endpointLabels are the rows naming an API a model answers on rather than
// something the model can do.
var endpointLabels = map[string]string{
	"anthropic api": EndpointAnthropic,
	"responses api": EndpointResponses,
}

// labelWordRe matches whatever in a row label is not part of an identifier,
// including the full-width brackets DeepSeek writes a qualifier in.
var labelWordRe = regexp.MustCompile(`[^a-z0-9]+`)

// featureName rewrites a row label into the catalog's vocabulary.
func featureName(label string) string {
	if name, ok := featureNames[label]; ok {
		return name
	}
	return strings.Trim(labelWordRe.ReplaceAllString(label, "_"), "_")
}

// supported is the mark DeepSeek uses for a capability a model has.
const supported = "✓"

var (
	rowRe    = regexp.MustCompile(`(?is)<tr[^>]*>(.*?)</tr>`)
	cellRe   = regexp.MustCompile(`(?is)<t[hd][^>]*>(.*?)</t[hd]\s*>`)
	tagRe    = regexp.MustCompile(`(?s)<[^>]*>`)
	amountRe = regexp.MustCompile(`\$\s*([\d,]*\.?\d+)`)
	countRe  = regexp.MustCompile(`(?i)([\d,]*\.?\d+)\s*([km])?`)
)

// text strips markup and collapses whitespace.
func text(html string) string {
	return strings.Join(strings.Fields(tagRe.ReplaceAllString(html, " ")), " ")
}

// parseAmount reads a rate cell.
func parseAmount(cell string) (float64, bool) {
	match := amountRe.FindStringSubmatch(cell)
	if match == nil {
		return 0, false
	}
	value, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", ""), 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

// parseCount reads a quantity such as "1M" or "384K".
func parseCount(cell string) int64 {
	match := countRe.FindStringSubmatch(cell)
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

// rowCells returns the text of one row's cells.
func rowCells(row string) []string {
	matches := cellRe.FindAllStringSubmatch(row, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, text(m[1]))
	}
	return out
}

// firstSentence returns the leading sentence of a paragraph, which is what
// DeepSeek's prose puts the fact in and what follows it enlarges on.
func firstSentence(prose string) string {
	if i := strings.Index(prose, ". "); i >= 0 {
		return prose[:i+1]
	}
	return prose
}
