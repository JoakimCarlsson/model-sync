package assemblyai

import (
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Metrics AssemblyAI bills on. Both are quoted per hour but they count
// different things, so they are not one metric with two rates.
const (
	// MetricAudio counts hours of audio submitted.
	MetricAudio catalog.Metric = "audio"
	// MetricSession counts hours a streaming connection stays open,
	// regardless of how much audio crosses it.
	MetricSession catalog.Metric = "session"
)

// UnitPerHour is the denominator everything AssemblyAI meters in time is
// quoted against, which is everything but the models it resells.
const UnitPerHour catalog.Unit = "per_hour"

// KindTranscription is what AssemblyAI trains and serves itself.
const KindTranscription catalog.Kind = "transcription"

// Modes AssemblyAI separates its models into.
const (
	ModePrerecorded = "pre-recorded"
	ModeStreaming   = "streaming"
	ModeAddOn       = "add-on"
)

// DimMode records which of those a rate belongs to.
const DimMode = "mode"

// Scalar keys the models page populates.
const (
	AttrMode             = "mode"
	AttrVolumeDiscounts  = "volume_discounts"
	AttrDocumentationURL = "documentation_url"
)

// AttrAPIIdentifier is the string a request selects the model with. The models
// page names models for a reader and never states it; the two model-selection
// pages give it a column of its own, and the add-on states it as the value of
// the domain parameter that turns it on.
const AttrAPIIdentifier = "api_identifier"

// ListFeatures holds the capabilities a model card's bullets name. The bullets
// themselves are sentences and are not kept: each is read for the capability
// in it, the ceiling it states and the languages it lists.
const ListFeatures = catalog.ListFeatures

// ListLanguages holds the languages a card names. A card that counts them
// without naming them states LimitLanguageCount instead.
const ListLanguages = "languages"

// Enumeration keys holding what a model takes and returns.
const (
	ListInputModalities  = "input_modalities"
	ListOutputModalities = "output_modalities"
)

// Modalities AssemblyAI's models handle. A transcription model hears audio and
// writes text, whether it is given a recording or a connection; a model reached
// through the gateway is given text and answers in text.
const (
	ModalityText  = "text"
	ModalityAudio = "audio"
)

var (
	cardRe = regexp.MustCompile(
		`(?is)<Card\s+title="([^"]*)"[^>]*?(?:href="([^"]*)")?[^>]*>(.*?)</Card>`,
	)
	listItemRe = regexp.MustCompile(`(?is)<li[^>]*>(.*?)</li>`)
	tagRe      = regexp.MustCompile(`(?s)<[^>]*>`)
	linkRe     = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	amountRe   = regexp.MustCompile(`\$\s*([\d,]*\.?\d+)`)
)

// clean strips markdown and MDX decoration from a value, and resolves the
// character references the pricing page writes its prose with.
func clean(text string) string {
	s := linkRe.ReplaceAllString(text, "$1")
	s = tagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, `\$`, "$")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "`", "")
	return strings.Join(strings.Fields(s), " ")
}

// parseAmount reads a rate cell such as "\$0.21/hr".
func parseAmount(cell string) (float64, bool) {
	match := amountRe.FindStringSubmatch(clean(cell))
	if match == nil {
		return 0, false
	}
	value, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", ""), 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

// slugID turns a display name such as "Universal-3.5 Pro" into an identifier.
// AssemblyAI names models one way in its cards and rate tables and does not
// publish an API identifier for them, so the display name is what there is.
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

// Metrics the LLM Gateway bills on. It is the one thing AssemblyAI sells that
// is not metered in time, so none of these is comparable to the two above.
const (
	MetricInputTokens       catalog.Metric = "input_tokens"
	MetricCachedInputTokens catalog.Metric = "cached_input_tokens"
	MetricCacheWriteTokens  catalog.Metric = "cache_write_tokens"
	MetricOutputTokens      catalog.Metric = "output_tokens"
)

// UnitPer1MTokens is the denominator the gateway quotes every rate against.
const UnitPer1MTokens catalog.Unit = "per_1m_tokens"

// The kinds AssemblyAI sells besides transcription: the models it resells
// through its gateway, and the two families of thing it does to a transcript
// once one exists.
const (
	KindChat                catalog.Kind = "chat"
	KindSpeechUnderstanding catalog.Kind = "speech_understanding"
	KindGuardrail           catalog.Kind = "guardrail"
)

// DimCacheTTL says how long a cache write buys, which the gateway prices in
// two columns and states only for the longer of them.
const DimCacheTTL = "cache_ttl"

// DimRedaction says which half of a redaction a rate is for. AssemblyAI
// documents PII redaction once and sells it twice, once for the transcript and
// once for the audio, at different rates.
const DimRedaction = "redaction"

// LimitContextWindow is how much a gateway model may be given at once, which
// is the only bound AssemblyAI states for one.
const LimitContextWindow = "context_window"

// Scalar keys the gateway and pricing pages populate.
const (
	// AttrSummary is the sentence the pricing page describes a model with,
	// which is the only prose AssemblyAI writes per model in a fixed place.
	AttrSummary = "summary"
	// AttrRetirementDate is the day a gateway model stops being served, which
	// the roster states in a column of its own and leaves empty for a model
	// with no date set.
	AttrRetirementDate = "retirement_date"
	// AttrRegionalSurcharge is what a gateway model costs above its listed
	// rate when the request is pinned to a region rather than routed globally.
	// It is kept as published, a percentage, because AssemblyAI states it as
	// one and computing the second rate here would invent a figure.
	AttrRegionalSurcharge = "regional_surcharge"
	// AttrProduct is the heading the pricing page sells a model under.
	AttrProduct = "product"
)

// ListEndpoints holds the endpoints a model is reached at.
const ListEndpoints = "endpoints"

// ListParameters holds the request parameters a model accepts, which is
// answered per model for the gateway models and for no others.
const ListParameters = catalog.ListParameters

// Capabilities the gateway roster states by naming the parameter that carries
// them, in the catalog's words.
const (
	FeatureFunctionCalling   = catalog.CapabilityFunctionCalling
	FeatureStructuredOutputs = catalog.CapabilityStructuredOutputs
	FeatureWordTimestamps    = catalog.CapabilityWordTimestamps
	// FeatureStreaming is returning an answer as it is generated. It is not
	// CapabilityRealtime, which is transcribing a live connection: one is
	// about how an answer arrives, the other about what is being listened to.
	FeatureStreaming = "streaming"
)

// hoursToSeconds converts a bound AssemblyAI states in hours, which is the
// unit its prose uses and not one a consumer can compare against a duration.
func hoursToSeconds(text string) int64 {
	hours, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0
	}
	return int64(hours * 3600)
}
