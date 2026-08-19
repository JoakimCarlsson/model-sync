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

// predictionTailRe matches the first of the description forms a SKU uses,
// which ends in the word Google meters its own predictions by.
//
// The serving path may be written into that ending rather than into the body
// of the description, "Gemini 3.5 Flash Global Audio Input - Batch
// Predictions", and reading only the plain ending dropped every batch and flex
// rate Vertex charges for a Gemini model. The word is also mistyped in the
// catalog, thirteen rates ending in "Predictionss", and a rate is not a
// different rate for having a letter too many.
var predictionTailRe = regexp.MustCompile(
	`(?i)\s+-\s+(?:(batch|flex|priority)\s+)?predictionss?$`,
)

// directionTailRe matches a second form of Google's own meters, which ends at
// the modality and the direction with no word for what it counts: "Gemini MM
// Embedding - Batch Text Input". The separator is what makes it Google's own,
// the Model Garden never writing one, so the ending is read there and only
// there.
var directionTailRe = regexp.MustCompile(
	`(?i)\s+-\s+((?:batch\s+)?(?:text|image|audio|video)\s+(?:input|output))$`,
)

// tokenSuffixes mark another. The Model Garden counts a rate in tokens
// either way it spells the word, and reading only the singular dropped the
// standard rate of every model whose meter is named for its tokens: Llama 3.3
// 70B was left priced for tuning alone.
var tokenSuffixes = []string{" tokens", " token"}

// maasPrefix is the Model Garden form that prices inference.
const maasPrefix = "cloud vertex ai model garden model as a service "

// fineTuningPrefix is the Model Garden form that prices training rather than
// inference.
const fineTuningPrefix = "cloud vertex ai model garden managed oss fine tuning for "

// Prefixes the Model Garden puts in front of a model name.
var gardenPrefixes = []string{maasPrefix, fineTuningPrefix}

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

// spacedVersionRe matches a lab written against its version with no space, so
// that both spellings of a model reach one entry. The catalog meters Llama 3.3
// 70B as "Llama 3.3 70B" for tuning and as "Llama3.3 70B" for inference, and
// reading them apart left one identifier priced for tuning alone and another
// for inference alone. Only Llama is written both ways; Qwen is written
// "Qwen3" throughout, which is also how its page names it.
var spacedVersionRe = regexp.MustCompile(`(?i)\b(llama)(\d)`)

// cutSuffix removes the first of the suffixes the value ends with.
func cutSuffix(value string, suffixes []string) (string, bool) {
	for _, suffix := range suffixes {
		if trimmed, ok := strings.CutSuffix(value, suffix); ok {
			return trimmed, true
		}
	}
	return value, false
}

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
	// bare reports a description that ended at none of the words Google
	// closes a meter with. Such a description names a model only if some
	// other document already named it; see acceptBare.
	bare bool
}

// readDescription takes a SKU description apart, returning the model left once
// every word naming a deployment, tier, modality or direction is removed.
func readDescription(description string) (reading, bool) {
	lower := spacedVersionRe.ReplaceAllString(
		strings.ToLower(strings.TrimSpace(description)),
		"$1 $2",
	)
	out := reading{tier: TierStandard}
	trimmed, isTokens := cutSuffix(lower, tokenSuffixes)
	switch tail := predictionTailRe.FindStringSubmatch(lower); {
	case tail != nil:
		lower = predictionTailRe.ReplaceAllString(lower, " "+tail[1])
	case strings.HasPrefix(lower, fineTuningPrefix):
		out.training = true
		lower = strings.TrimPrefix(lower, fineTuningPrefix)
	case isTokens:
		lower = trimmed
	case maasDirection(lower):
		lower = strings.TrimPrefix(lower, maasPrefix)
	case directionTailRe.MatchString(lower):
		lower = directionTailRe.ReplaceAllString(lower, " $1")
	default:
		out.bare = true
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
	lower, out.modality, out.direction = takePair(lower)
	for _, term := range noise {
		lower = removeWord(lower, term)
	}
	out.model = strings.Join(strings.Fields(lower), " ")
	return out, out.model != ""
}

// takePair removes the modality a rate covers and the side of the request it
// falls on, which a description writes next to each other, "Text Input" or
// "Input Text".
//
// The modality has to be taken from beside the direction rather than from
// wherever it first appears, because a model can be named for a modality
// itself: "Gemini 3.1 Flash Image Global Video Input" prices video input on
// the image model, and taking the first modality left the model reading as a
// Gemini 3.1 Flash Video, which Vertex does not serve.
func takePair(rest string) (string, string, string) {
	words := strings.Fields(rest)
	for at, w := range words {
		direction, ok := wordValue(w, directions)
		if !ok {
			continue
		}
		modality, from := neighbourModality(words, at)
		kept := make([]string, 0, len(words))
		for i, word := range words {
			if i != at && i != from {
				kept = append(kept, word)
			}
		}
		return strings.Join(kept, " "), modality, direction
	}
	return rest, "", ""
}

// neighbourModality reports the modality written beside the direction, and
// where it was written, so that only that occurrence of the word is removed.
func neighbourModality(words []string, at int) (string, int) {
	for _, beside := range []int{at - 1, at + 1} {
		if beside < 0 || beside >= len(words) {
			continue
		}
		if value, ok := wordValue(words[beside], modalities); ok {
			return value, beside
		}
	}
	return "", -1
}

// wordValue reports what a whole word means, where it is one of the terms.
func wordValue(word string, words []word) (string, bool) {
	for _, w := range words {
		if word == w.term {
			return w.value, true
		}
	}
	return "", false
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

// maasDirection reports a Model Garden inference rate whose description stops
// at the direction, "... Llama3.1 405B Input", where the rest of that form
// carries on to the word it is counted in, "... Llama 4 Scout Input Tokens".
//
// The direction alone is only read under this prefix, and there it is
// unambiguous: the Model Garden never writes the " - Predictions" suffix, which
// is how Gemini's own meters end, so nothing under the prefix can be a
// prediction rate whose suffix has been left off. Read without the prefix the
// two forms cannot be told apart, and the same rule would take Google's
// per-product meters, "CodeMender Gemini 3 Flash Global Text Input", for models
// of their own. Llama 3.1 405B is metered this way and no other, so without
// this it is billed for and absent from the catalog.
func maasDirection(lower string) bool {
	if !strings.HasPrefix(lower, maasPrefix) {
		return false
	}
	fields := strings.Fields(lower)
	_, ok := wordValue(fields[len(fields)-1], directions)
	return ok
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
