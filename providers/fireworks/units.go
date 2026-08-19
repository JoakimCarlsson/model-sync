package fireworks

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Metrics Fireworks bills on.
const (
	MetricInputTokens       catalog.Metric = "input_tokens"
	MetricCachedInputTokens catalog.Metric = "cached_input_tokens"
	MetricOutputTokens      catalog.Metric = "output_tokens"
	// MetricTrainingTokens is the token a training job consumes, which
	// Fireworks prices apart from inference and by a band of its own.
	MetricTrainingTokens catalog.Metric = "training_tokens"
	// MetricTrainingPrefillTokens and the two after it are what the shared
	// trainer bills separately: the prompt it reads, the answer it draws, and
	// the tokens it takes a step on.
	MetricTrainingPrefillTokens       catalog.Metric = "training_prefill_tokens"
	MetricTrainingCachedPrefillTokens catalog.Metric = "training_cached_prefill_tokens"
	MetricTrainingSampleTokens        catalog.Metric = "training_sample_tokens"
)

// UnitPer1MTokens is the only denominator Fireworks quotes.
const UnitPer1MTokens catalog.Unit = "per_1m_tokens"

// Kinds of model Fireworks serves.
const (
	KindChat      catalog.Kind = "chat"
	KindEmbedding catalog.Kind = "embedding"
	KindRerank    catalog.Kind = "rerank"
	KindImage     catalog.Kind = "image"
)

// Dimension keys Fireworks' prices vary along.
const (
	// DimTier is the serving path chosen per request.
	DimTier = "tier"
	// DimServing is the variant of the deployment, which Fireworks writes as
	// a suffix on the model's display name rather than as a parameter.
	DimServing = "serving"
	// DimBatch marks the rate a job billed asynchronously pays.
	DimBatch = "batch"
	// DimSizeBand is the row of a rate card that prices by parameter count
	// rather than by naming a model, quoted as Fireworks writes it.
	DimSizeBand = "size_band"
	// DimMethod is the training method a training rate applies to.
	DimMethod = "method"
	// DimSurface is where a training job runs, which Fireworks prices
	// differently for the managed jobs and for the shared trainer pool.
	DimSurface = "surface"
	// DimContextWindow is the window a shared-trainer rate is quoted against,
	// which differs per model there.
	DimContextWindow = "context_window"
)

// Serving paths Fireworks prices separately.
const (
	TierStandard = "standard"
	TierPriority = "priority"
)

// Training surfaces Fireworks prices apart.
const (
	SurfaceManaged           = "managed"
	SurfaceServerlessTrainer = "serverless_training_api"
)

// Scalar keys the model library's pages populate.
const (
	AttrSummary          = "summary"
	AttrHuggingFaceID    = "hugging_face_id"
	AttrModelURL         = "model_url"
	AttrModelPath        = "model_path"
	AttrAuthor           = "author"
	AttrLicense          = "license"
	AttrOpenWeights      = "open_weights"
	AttrState            = "state"
	AttrReleaseDate      = "release_date"
	AttrParameterCount   = "parameter_count"
	AttrMixtureOfExperts = "mixture_of_experts"
	AttrCalibrated       = "calibrated"
	// AttrModelKind is the word Fireworks files a model under in its library,
	// which separates a base model from a customized one and from the addons
	// that only run on top of another model.
	AttrModelKind = "model_kind"
)

// AttrDefaultDimension is the width of the vector an embedding model returns.
// Fireworks states a range rather than a set of widths, because the vector can
// be cut to any length the caller asks for, so what is recorded is the width
// it returns when the caller asks for nothing.
const AttrDefaultDimension = "default_embedding_dimension"

// Numeric bounds the documents state.
const (
	LimitContextWindow  = "context_window"
	LimitMaxOutput      = "max_output_tokens"
	LimitInputTPM       = "input_tokens_per_minute"
	LimitUncachedTPM    = "uncached_input_tokens_per_minute"
	LimitOutputTokenTPM = "output_tokens_per_minute"
)

// Enumeration keys.
const (
	ListFeatures         = catalog.ListFeatures
	ListInputModalities  = "input_modalities"
	ListOutputModalities = "output_modalities"
	ListAliases          = "aliases"
	// ListDeployment is how a model can be run: on the shared serverless
	// fleet, on GPUs of the caller's own, or as the base of a training job.
	ListDeployment = "deployment_types"
)

// Ways Fireworks says a model can be run.
const (
	DeploymentServerless = "serverless"
	DeploymentOnDemand   = "on_demand"
	DeploymentFineTuning = "fine_tuning"
)

// Capabilities read off the documents.
const (
	FeatureFunctionCalling = catalog.CapabilityFunctionCalling
	FeatureReasoning       = catalog.CapabilityReasoning
	FeatureStructured      = catalog.CapabilityStructuredOutputs
	// FeatureGrammarMode is Fireworks' own word, kept alongside the canonical
	// value because it says something that one does not: the shape a model is
	// held to may be a formal grammar and not only a JSON schema.
	FeatureGrammarMode = "grammar_mode"
	// FeaturePromptCaching is set where Fireworks says a prompt prefix is
	// cached and billed at the cached rate without being asked for.
	FeaturePromptCaching = "prompt_caching"
)

// tripleOrder is what the three amounts in a cell mean, in the order the page
// writes them.
var tripleOrder = []catalog.Metric{
	MetricInputTokens,
	MetricCachedInputTokens,
	MetricOutputTokens,
}

// servingSuffixes are the words Fireworks appends to a display name to mark a
// deployment variant rather than a different model.
var servingSuffixes = []string{"fast us", "fast", "us"}

var (
	linkRe   = regexp.MustCompile(`\[([^\]]*)\]\(([^)]*)\)`)
	amountRe = regexp.MustCompile(`\$\s*([\d,]*\.?\d+)`)
)

// clean strips markdown decoration from a cell value.
func clean(cell string) string {
	s := strings.ReplaceAll(cell, `\$`, "$")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "`", "")
	return strings.Join(strings.Fields(s), " ")
}

// nameWords splits a display name into the words two names are compared on,
// so that "GPT-OSS 120B" and "GPT OSS 120B" are the same three words while
// "M2" and "M2.7" stay two different ones.
func nameWords(name string) []string {
	return strings.FieldsFunc(strings.ToLower(clean(name)), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.')
	})
}

// modelRef is what one model cell holds.
type modelRef struct {
	ID      string
	Name    string
	Serving string
	URL     string
}

// modelPathPrefix precedes the identifier in a model's console URL.
const modelPathPrefix = "/models/"

// splitModelCell reads a model cell, separating the deployment variant from
// the model it is a variant of.
func splitModelCell(cell string) (modelRef, bool) {
	match := linkRe.FindStringSubmatch(cell)
	if match == nil {
		return modelRef{}, false
	}
	name := clean(match[1])
	url := strings.TrimSpace(match[2])
	at := strings.Index(url, modelPathPrefix)
	if at < 0 {
		return modelRef{}, false
	}
	ref := modelRef{
		ID:   strings.Trim(url[at+len(modelPathPrefix):], "/"),
		Name: name,
		URL:  url,
	}
	lower := strings.ToLower(name)
	for _, suffix := range servingSuffixes {
		if base, ok := strings.CutSuffix(lower, " "+suffix); ok {
			ref.Serving = strings.ReplaceAll(suffix, " ", "-")
			ref.Name = strings.TrimSpace(name[:len(base)])
			break
		}
	}
	return ref, ref.ID != ""
}

// parseTriple reads the amounts in a rate cell, in the order the page states
// them. A cell holding a dash offers that serving path no rate at all.
func parseTriple(cell string) []float64 {
	var out []float64
	for _, match := range amountRe.FindAllStringSubmatch(clean(cell), -1) {
		value, err := strconv.ParseFloat(
			strings.ReplaceAll(match[1], ",", ""),
			64,
		)
		if err != nil {
			continue
		}
		out = append(out, value)
	}
	return out
}

// namesSameModel reports whether one name is the other spelled out. The
// pricing table writes "Qwen3 8B" where the library writes "Qwen3 Embedding
// 8B", so a match is every word of the shorter name appearing in the longer
// one, in order.
func namesSameModel(priced, listed string) bool {
	want, have := nameWords(priced), nameWords(listed)
	if len(want) == 0 {
		return false
	}
	for _, word := range want {
		at := slices.Index(have, word)
		if at < 0 {
			return false
		}
		have = have[at+1:]
	}
	return true
}

// tokenCount reads a count a page abbreviated, which it writes as "262k
// tokens" or "1.05m tokens".
func tokenCount(amount, scale string) int64 {
	value, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		return 0
	}
	if strings.EqualFold(scale, "m") {
		return int64(value * 1_000_000)
	}
	return int64(value * 1_000)
}

// parseAmount reads one dollar amount, tolerating the thousands separators the
// pages write.
func parseAmount(cell string) (float64, bool) {
	match := amountRe.FindStringSubmatch(clean(cell))
	if match == nil {
		return 0, false
	}
	value, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", ""), 64)
	return value, err == nil
}
