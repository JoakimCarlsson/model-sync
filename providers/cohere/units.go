package cohere

import (
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// currency is the only currency Cohere quotes.
const currency = "USD"

// Metrics Cohere bills on.
const (
	MetricInputTokens   catalog.Metric = "input_tokens"
	MetricOutputTokens  catalog.Metric = "output_tokens"
	MetricImageInput    catalog.Metric = "image_input"
	MetricSearchQueries catalog.Metric = "search_queries"
	// MetricHosting counts the time an instance is held rather than anything
	// a request carries, which is how Cohere bills a dedicated deployment.
	MetricHosting catalog.Metric = "hosting"
)

// Units Cohere quotes amounts against.
const (
	UnitPer1MTokens catalog.Unit = "per_1m_tokens"
	UnitPer1KSearch catalog.Unit = "per_1k_searches"
	UnitPerHour     catalog.Unit = "per_hour"
	UnitPerMonth    catalog.Unit = "per_month"
	UnitPerYear     catalog.Unit = "per_year"
)

// FeatureStreaming is set where a model returns what it generates a piece at a
// time. The catalog declares no value for it, so the spelling here is the one
// the other providers already use.
const FeatureStreaming = "streaming"

// instanceUnits map the denominator a dedicated instance is quoted against
// onto a unit.
var instanceUnits = map[string]catalog.Unit{
	"hour":  UnitPerHour,
	"month": UnitPerMonth,
}

// Dimensions separating a dedicated deployment's rate from the rate of a call
// to the shared API.
const (
	// DimDeployment records that a rate buys an instance rather than a call.
	DimDeployment = "deployment"
	// DeploymentVault is the platform Cohere sells those instances on.
	DeploymentVault = "model-vault"
	// DimTier records the performance tier an instance is sized at.
	DimTier = "tier"
	// TierXL is the tier the generative table names in a column heading rather
	// than in a column of its own.
	TierXL = "xl"
)

// noteStartingRate marks an amount Cohere quotes as a floor rather than as the
// rate itself, which it writes as "From".
const noteStartingRate = "quoted as a starting rate"

// modalityNames map the wording of the overview's modality column onto the
// catalog's vocabulary. Cohere names a mixed document by the formats it may
// contain, which is the two modalities it already states rather than a third.
var modalityNames = map[string]string{
	"text":                           "text",
	"images":                         "image",
	"image":                          "image",
	"audio":                          "audio",
	"audio waveform":                 "audio",
	"mixed texts/images (i.e. pdfs)": "file",
}

// modalityName rewrites one modality into the catalog's vocabulary, keeping
// Cohere's own word for anything the table names that this does not cover.
func modalityName(value string) string {
	if name, ok := modalityNames[strings.ToLower(strings.TrimSpace(value))]; ok {
		return name
	}
	return strings.ToLower(strings.TrimSpace(value))
}

// productModels map the product name Cohere writes outside the overview onto
// the identifier that product is called by.
//
// Everything but the overview names products, not models: a card headed
// "Embed 4" states the rate of the model the API calls embed-v4.0, and the
// structured outputs guide declares itself compatible with "Command A+", which
// is command-a-plus-05-2026. Where a document names a model precisely, as the
// legacy rates and most of that guide's list do, no entry is needed and the
// name is reduced to an identifier instead.
var productModels = map[string][]string{
	"command a":           {"command-a-03-2025"},
	"command a+":          {"command-a-plus-05-2026"},
	"command a reasoning": {"command-a-reasoning-08-2025"},
	"command a translate": {"command-a-translate-08-2025"},
	"command a vision":    {"command-a-vision-07-2025"},
	"command r":           {"command-r-08-2024"},
	"command r7b":         {"command-r7b-12-2024"},
	"embed 4":             {"embed-v4.0"},
	"rerank 3.5":          {"rerank-v3.5"},
	"rerank 4 fast":       {"rerank-v4.0-fast"},
	"rerank 4 pro":        {"rerank-v4.0-pro"},
	"transcribe":          {"cohere-transcribe-03-2026"},
	"aya expanse":         {"c4ai-aya-expanse-8b", "c4ai-aya-expanse-32b"},
}

// cardLabels map the wording of a rate's label onto what it counts. Cohere
// labels the ordinary rates "Input" and "Output" on one card and "Cost" on
// another, and names only the exception within a card.
var cardLabels = map[string]catalog.Metric{
	"input":      MetricInputTokens,
	"cost":       MetricInputTokens,
	"output":     MetricOutputTokens,
	"image cost": MetricImageInput,
}

// cardUnits map the denominator a card quotes against onto a unit and, where
// the denominator says what is being counted, onto a metric.
var cardUnits = map[string]struct {
	Unit   catalog.Unit
	Metric catalog.Metric
}{
	"1m tokens":   {UnitPer1MTokens, ""},
	"1k searches": {UnitPer1KSearch, MetricSearchQueries},
}
