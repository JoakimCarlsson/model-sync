package vertexai

import (
	"math/big"
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Metrics Vertex bills on.
const (
	MetricInputTokens       catalog.Metric = "input_tokens"
	MetricOutputTokens      catalog.Metric = "output_tokens"
	MetricCachedInputTokens catalog.Metric = "cached_input_tokens"
	MetricTraining          catalog.Metric = "training"
)

// UnitPer1MTokens is the denominator every rate read here is quoted at.
const UnitPer1MTokens catalog.Unit = "per_1m_tokens"

// Kinds these SKUs price. Vertex bills embeddings, image models and document
// readers by the token exactly as it bills chat, so the rate cannot say which
// a model is and the name has to.
const (
	KindChat      catalog.Kind = "chat"
	KindEmbedding catalog.Kind = "embedding"
	KindImage     catalog.Kind = "image"
	KindVideo     catalog.Kind = "video"
	KindOCR       catalog.Kind = "ocr"
)

// nameKinds map a fragment of a model's name onto what it does.
var nameKinds = []struct {
	fragment string
	kind     catalog.Kind
}{
	{"ocr", KindOCR},
	{"embed", KindEmbedding},
	{"e5", KindEmbedding},
	{"imagen", KindImage},
	{"image", KindImage},
	{"veo", KindVideo},
	{"video", KindVideo},
}

// kindFor reports what a model does, read from its name.
func kindFor(model string) catalog.Kind {
	lower := strings.ToLower(model)
	for _, entry := range nameKinds {
		if strings.Contains(lower, entry.fragment) {
			return entry.kind
		}
	}
	return KindChat
}

// Dimension keys Vertex's prices vary along.
const (
	DimDeployment = "deployment"
	DimTier       = "tier"
	DimModality   = "modality"
	DimContext    = "context"
	DimStage      = "stage"
)

// Serving paths Vertex prices separately.
const (
	TierStandard = "standard"
	TierPriority = "priority"
	TierFlex     = "flex"
	TierBatch    = "batch"
)

// AttrAuthor records the lab that made a resold open model.
const AttrAuthor = "author"

// tokenQuantity is the quantity a token SKU is quoted for. A SKU quoted for
// anything else is not a token rate and is not read.
const tokenQuantity = 1_000_000

// Suffixes marking which of the two description forms a SKU uses.
const (
	predictionSuffix = " - predictions"
	tokenSuffix      = " token"
)

// Prefixes the Model Garden puts in front of a model name.
var gardenPrefixes = []string{
	"cloud vertex ai model garden model as a service ",
	"cloud vertex ai model garden managed oss fine tuning for ",
}

// fineTuningPrefix is the Model Garden form that prices training rather than
// inference.
const fineTuningPrefix = "cloud vertex ai model garden managed oss fine tuning for "

// word is a term that may appear in a description and what it means.
type word struct {
	term  string
	value string
}

// deployments are where a model runs, which Vertex prices differently.
var deployments = []word{
	{"global", "global"},
	{"regional", "regional"},
}

// tierWords are the serving paths named in a description.
var tierWords = []word{
	{"priority", TierPriority},
	{"flex", TierFlex},
	{"batch", TierBatch},
}

// modalities are the kinds of input priced separately on one model.
var modalities = []word{
	{"text", "text"},
	{"image", "image"},
	{"audio", "audio"},
	{"video", "video"},
}

// directions are which side of a request a rate covers.
var directions = []word{
	{"input", "input"},
	{"output", "output"},
}

// stages are release stages, which Vertex prices separately: the generally
// available rate for Gemini 2.5 Flash is half its preview rate, so treating
// the word as decoration would leave two rates that differ only in amount.
var stages = []word{
	{"ga", "ga"},
	{"preview", "preview"},
}

// noise are words that say nothing about what a rate is for.
var noise = []string{"prompts", "prompt", "tuned models"}

// longContextRe matches the suffix marking the long context rate, which is the
// same model above a prompt threshold rather than a model of its own.
var longContextRe = regexp.MustCompile(`(?i)\s*\(long\)\s*`)

// cachingRe matches the wording marking a rate as being for cached input.
var cachingRe = regexp.MustCompile(`(?i)\s*(cache storage|caching|cached)\s*`)

// reading is what one description says once taken apart.
type reading struct {
	model      string
	deployment string
	tier       string
	modality   string
	direction  string
	stage      string
	longCtx    bool
	cached     bool
	training   bool
}

// readDescription takes a SKU description apart, returning the model left once
// every word naming a deployment, tier, modality or direction is removed.
func readDescription(description string) (reading, bool) {
	lower := strings.ToLower(strings.TrimSpace(description))
	out := reading{tier: TierStandard}
	switch {
	case strings.HasSuffix(lower, predictionSuffix):
		lower = strings.TrimSuffix(lower, predictionSuffix)
	case strings.HasPrefix(lower, fineTuningPrefix):
		out.training = true
		lower = strings.TrimPrefix(lower, fineTuningPrefix)
	case strings.HasSuffix(lower, tokenSuffix):
		lower = strings.TrimSuffix(lower, tokenSuffix)
	default:
		return reading{}, false
	}
	for _, prefix := range gardenPrefixes {
		lower = strings.TrimPrefix(lower, prefix)
	}
	if cachingRe.MatchString(lower) {
		out.cached = true
		lower = cachingRe.ReplaceAllString(lower, " ")
	}
	if longContextRe.MatchString(lower) {
		out.longCtx = true
		lower = longContextRe.ReplaceAllString(lower, " ")
	}
	lower, out.stage = takeWord(lower, stages)
	lower, out.deployment = takeWord(lower, deployments)
	if stripped, tier := takeWord(lower, tierWords); tier != "" {
		lower, out.tier = stripped, tier
	}
	lower, out.modality = takeWord(lower, modalities)
	lower, out.direction = takeWord(lower, directions)
	for _, term := range noise {
		lower = removeWord(lower, term)
	}
	out.model = strings.Join(strings.Fields(lower), " ")
	return out, out.model != ""
}

// takeWord removes the first matching term and reports what it meant.
func takeWord(rest string, words []word) (string, string) {
	for _, w := range words {
		if stripped, ok := cutWord(rest, w.term); ok {
			return stripped, w.value
		}
	}
	return rest, ""
}

// removeWord drops a term wherever it appears as a whole word.
func removeWord(rest, term string) string {
	if stripped, ok := cutWord(rest, term); ok {
		return stripped
	}
	return rest
}

// cutWord removes one whole word from a phrase.
func cutWord(rest, term string) (string, bool) {
	padded := " " + strings.Join(strings.Fields(rest), " ") + " "
	at := strings.Index(padded, " "+term+" ")
	if at < 0 {
		return rest, false
	}
	return padded[:at+1] + padded[at+len(term)+2:], true
}

// metricFor reports what a rate counts.
func metricFor(r reading) (catalog.Metric, bool) {
	switch {
	case r.training:
		return MetricTraining, true
	case r.cached:
		return MetricCachedInputTokens, true
	case r.direction == "output":
		return MetricOutputTokens, true
	case r.direction == "input":
		return MetricInputTokens, true
	}
	return "", false
}

// amountOf combines the whole units and the nanos remainder a rate is stated
// as, then scales to the quantity the SKU is quoted for. The arithmetic is
// rational so that a rate of 250 nanos over a million records as 0.25 exactly.
func amountOf(units string, nanos int64, quantity int64) (float64, bool) {
	whole, ok := new(big.Int).SetString(strings.TrimSpace(units), 10)
	if !ok {
		return 0, false
	}
	total := new(big.Rat).SetInt(whole)
	total.Add(total, big.NewRat(nanos, 1_000_000_000))
	if total.Sign() == 0 {
		return 0, false
	}
	total.Mul(total, new(big.Rat).SetInt64(quantity))
	value, _ := total.Float64()
	return value, true
}

// slugID turns a model name into an identifier.
func slugID(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
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
