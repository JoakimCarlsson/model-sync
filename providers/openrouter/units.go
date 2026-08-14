package openrouter

import (
	"math/big"
	"strings"
	"time"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Metrics OpenRouter bills on.
const (
	MetricInputTokens       catalog.Metric = "input_tokens"
	MetricOutputTokens      catalog.Metric = "output_tokens"
	MetricCachedInputTokens catalog.Metric = "cached_input_tokens"
	MetricCacheWriteTokens  catalog.Metric = "cache_write_tokens"
	MetricReasoningTokens   catalog.Metric = "reasoning_tokens"
	MetricAudioInput        catalog.Metric = "audio_input"
	MetricAudioOutput       catalog.Metric = "audio_output"
	MetricCachedAudioInput  catalog.Metric = "cached_audio_input"
	MetricImageInput        catalog.Metric = "image_input"
	MetricImageOutput       catalog.Metric = "image_output"
	MetricToolCall          catalog.Metric = "tool_call"
)

// Units the published rates are scaled to.
const (
	UnitPer1MTokens catalog.Unit = "per_1m_tokens"
	UnitPer1KCalls  catalog.Unit = "per_1k_calls"
	UnitPerImage    catalog.Unit = "per_image"
)

// Kinds of model OpenRouter brokers.
const (
	KindChat  catalog.Kind = "chat"
	KindImage catalog.Kind = "image"
	KindAudio catalog.Kind = "audio"
)

// Dimension keys OpenRouter's prices vary along.
const (
	// DimMinPromptTokens carries the threshold of a conditional rate, which
	// OpenRouter states as the smallest prompt the override applies from.
	DimMinPromptTokens = "min_prompt_tokens"
	DimCacheTTL        = "cache_ttl"
)

// Scalar keys the API populates.
const (
	AttrSummary            = "summary"
	AttrAuthor             = "author"
	AttrCanonicalSlug      = "canonical_slug"
	AttrHuggingFaceID      = "hugging_face_id"
	AttrTokenizer          = "tokenizer"
	AttrInstructType       = "instruct_type"
	AttrKnowledgeCutoff    = "knowledge_cutoff"
	AttrExpirationDate     = "expiration_date"
	AttrReleased           = "released"
	AttrModerated          = "is_moderated"
	AttrReasoningMandatory = "reasoning_mandatory"
	AttrFree               = "is_free"
	AttrAliasTarget        = "alias_target"
	// AttrReasoningDefaultEffort and AttrReasoningDefaultEnabled state what a
	// reasoning model does when the caller asks for nothing in particular.
	AttrReasoningDefaultEffort  = "reasoning_default_effort"
	AttrReasoningDefaultEnabled = "reasoning_default_enabled"
)

// Capabilities OpenRouter states that catalog declares no canonical value for.
// The spellings are the ones the rest of the catalog already uses.
const (
	FeaturePromptCaching     = "prompt_caching"
	FeatureParallelToolCalls = "parallel_tool_calls"
	FeatureWebSearch         = "web_search"
)

// Numeric keys the API populates.
const (
	LimitContextWindow    = "context_window"
	LimitMaxOutputTokens  = "max_output_tokens"
	LimitProviderContext  = "top_provider_context_window"
	LimitMaxPromptTokens  = "max_prompt_tokens"
	LimitMaxRequestTokens = "max_request_tokens"
)

// Enumeration keys the API populates.
const (
	ListFeatures         = catalog.ListFeatures
	ListParameters       = catalog.ListParameters
	ListInputModalities  = "input_modalities"
	ListOutputModalities = "output_modalities"
	ListVoices           = "voices"
	ListReasoningEfforts = "reasoning_efforts"
)

// parameterFeatures map a request parameter OpenRouter states a model accepts
// onto the capability accepting it implies.
//
// OpenRouter publishes no capability list. What it publishes is the set of
// parameters its API will forward for a model, which is a different thing:
// "accepts a response_format parameter" is a fact about the request, and
// "supports structured output" is a fact about the model. The two are recorded
// separately, and this is the one bridge between them, so that a consumer
// asking either question of OpenRouter gets the same answer it gets of every
// other provider.
//
// response_format is OpenRouter's parameter for the weaker mode, which
// constrains the answer to JSON without constraining its shape, so it implies
// both the capability and the marker that says how far it goes.
var parameterFeatures = map[string][]string{
	"tools":              {catalog.CapabilityFunctionCalling},
	"tool_choice":        {catalog.CapabilityFunctionCalling},
	"structured_outputs": {catalog.CapabilityStructuredOutputs},
	"response_format": {
		catalog.CapabilityStructuredOutputs,
		catalog.CapabilityJSONMode,
	},
	"reasoning":           {catalog.CapabilityReasoning},
	"include_reasoning":   {catalog.CapabilityReasoning},
	"reasoning_effort":    {catalog.CapabilityReasoning},
	"parallel_tool_calls": {FeatureParallelToolCalls},
	"web_search_options":  {FeatureWebSearch},
}

// priceFeatures map a rate OpenRouter charges onto the capability it is charged
// for.
//
// It is the third way this source states a capability, and the only way it
// states these two: no model carries a parameter saying it can cache or search,
// and every model that can is billed for doing so. A zero rate says the charge
// does not apply, so it implies nothing, in keeping with how a zero is read
// everywhere else here.
var priceFeatures = map[string][]string{
	"input_cache_read":     {FeaturePromptCaching},
	"input_cache_write":    {FeaturePromptCaching},
	"input_cache_write_1h": {FeaturePromptCaching},
	"web_search":           {FeatureWebSearch},
}

// scale is how much a published per-unit rate is multiplied by to reach the
// denominator the catalog records it at.
type scale struct {
	metric catalog.Metric
	unit   catalog.Unit
	factor int64
	dims   catalog.Dims
}

// priceKeys maps each key of the pricing object onto what it bills for.
//
// A key absent from this map is not silently dropped: an unmapped key holding
// a non-zero rate is recorded as a note against the model, so a denominator
// OpenRouter adds later surfaces as something to handle rather than as missing
// money.
var priceKeys = map[string]scale{
	"prompt":     {MetricInputTokens, UnitPer1MTokens, 1_000_000, nil},
	"completion": {MetricOutputTokens, UnitPer1MTokens, 1_000_000, nil},
	"input_cache_read": {
		MetricCachedInputTokens, UnitPer1MTokens, 1_000_000, nil,
	},
	"input_cache_write": {
		MetricCacheWriteTokens, UnitPer1MTokens, 1_000_000, nil,
	},
	"input_cache_write_1h": {
		MetricCacheWriteTokens, UnitPer1MTokens, 1_000_000,
		catalog.Dims{DimCacheTTL: "1h"},
	},
	"internal_reasoning": {
		MetricReasoningTokens, UnitPer1MTokens, 1_000_000, nil,
	},
	"audio":        {MetricAudioInput, UnitPer1MTokens, 1_000_000, nil},
	"audio_output": {MetricAudioOutput, UnitPer1MTokens, 1_000_000, nil},
	"input_audio_cache": {
		MetricCachedAudioInput, UnitPer1MTokens, 1_000_000, nil,
	},
	"image":        {MetricImageInput, UnitPerImage, 1, nil},
	"image_output": {MetricImageOutput, UnitPerImage, 1, nil},
	"web_search":   {MetricToolCall, UnitPer1KCalls, 1_000, nil},
}

// scaleRate converts a published decimal string to the catalog's denominator.
//
// The arithmetic is rational rather than floating point because the published
// values are tiny decimals that binary floating point cannot hold exactly:
// multiplying the float nearest "0.000002" by a million yields
// 1.9999999999999998, which would then differ between a value and the same
// value written with another number of zeros.
func scaleRate(raw string, factor int64) (float64, bool) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return 0, false
	}
	rate, ok := new(big.Rat).SetString(text)
	if !ok {
		return 0, false
	}
	if rate.Sign() == 0 {
		return 0, false
	}
	rate.Mul(rate, new(big.Rat).SetInt64(factor))
	value, _ := rate.Float64()
	return value, true
}

// isZeroRate reports whether a published rate is exactly zero, which is how
// OpenRouter marks both a free model and a charge that does not apply.
func isZeroRate(raw string) bool {
	rate, ok := new(big.Rat).SetString(strings.TrimSpace(raw))
	return ok && rate.Sign() == 0
}

// kindFor reports what a model is from what it emits.
func kindFor(outputs []string) catalog.Kind {
	for _, out := range outputs {
		switch strings.ToLower(out) {
		case "image":
			return KindImage
		case "audio":
			return KindAudio
		}
	}
	return KindChat
}

// authorOf returns the lab an identifier is namespaced under, which is the
// only place OpenRouter records who made a model.
func authorOf(id string) string {
	author, _, ok := strings.Cut(id, "/")
	if !ok {
		return ""
	}
	return author
}

// summaryOf reduces a description to its first sentence. OpenRouter writes
// several paragraphs per model, and storing them whole would multiply the size
// of the catalog for prose that says little the structured fields do not.
func summaryOf(description string) string {
	text := strings.Join(strings.Fields(description), " ")
	if sentence, _, ok := strings.Cut(text, ". "); ok {
		return sentence + "."
	}
	return text
}

// isoFromUnix renders a release timestamp as a calendar date.
func isoFromUnix(seconds int64) string {
	if seconds <= 0 {
		return ""
	}
	return time.Unix(seconds, 0).UTC().Format("2006-01-02")
}

// isoDate normalizes a date the API states as text.
func isoDate(value string) string {
	text := strings.TrimSpace(value)
	for _, layout := range []string{
		"2006-01-02",
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"Jan 2, 2006",
		"January 2, 2006",
	} {
		if t, err := time.Parse(layout, text); err == nil {
			return t.UTC().Format("2006-01-02")
		}
	}
	return text
}
