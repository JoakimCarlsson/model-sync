package azure

import (
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Metrics Azure bills on.
const (
	MetricInputTokens       catalog.Metric = "input_tokens"
	MetricOutputTokens      catalog.Metric = "output_tokens"
	MetricCachedInputTokens catalog.Metric = "cached_input_tokens"
	MetricTraining          catalog.Metric = "training"
	MetricHosting           catalog.Metric = "hosting"
	MetricUsage             catalog.Metric = "usage"
)

// Units Azure quotes amounts against, taken from its unit of measure.
const (
	UnitPer1KTokens catalog.Unit = "per_1k_tokens"
	UnitPer1MTokens catalog.Unit = "per_1m_tokens"
	UnitPerHour     catalog.Unit = "per_hour"
	UnitPerDay      catalog.Unit = "per_day"
	UnitPerSecond   catalog.Unit = "per_second"
	UnitPerUnit     catalog.Unit = "per_unit"
	UnitPer100Units catalog.Unit = "per_100_units"
)

// Kinds of model Foundry serves. It is not only chat: the same price list
// carries embeddings, image and video generation, speech synthesis,
// transcription and realtime audio.
const (
	KindChat          catalog.Kind = "chat"
	KindEmbedding     catalog.Kind = "embedding"
	KindImage         catalog.Kind = "image"
	KindVideo         catalog.Kind = "video"
	KindSpeech        catalog.Kind = "speech"
	KindTranscription catalog.Kind = "transcription"
	KindRealtime      catalog.Kind = "realtime"
	KindAudio         catalog.Kind = "audio"
)

// productKinds are the families whose every meter is one kind, whatever the
// SKU says. A text input to an image model is still an image model.
var productKinds = map[string]catalog.Kind{
	"Azure OpenAI Embedding": KindEmbedding,
	"Azure BFL Flux Models":  KindImage,
	"MAI Models":             KindImage,
}

// kindWords are the SKU words that say what a model does, for the families
// that mix kinds in one product. They are checked in order, since a realtime
// audio meter names both and the more specific reading wins.
var kindWords = []struct {
	terms []string
	kind  catalog.Kind
}{
	{[]string{"sora", "video", "vid"}, KindVideo},
	{[]string{"dall", "image", "img", "flux"}, KindImage},
	{
		[]string{"tcrb", "transcribe", "transcription", "whisper"},
		KindTranscription,
	},
	{[]string{"tts", "speech"}, KindSpeech},
	{[]string{"rt", "realtime"}, KindRealtime},
	{[]string{"aud", "audio"}, KindAudio},
	{[]string{"embed", "embedding", "embeddings", "ada"}, KindEmbedding},
}

// kindFor reports what a meter's model does, read from the family it belongs
// to and failing that from the words in its SKU. The SKU is read before any
// word is stripped from it, since the words naming a modality are the same
// ones a rate's dimensions consume.
func kindFor(sku, product string) catalog.Kind {
	if kind, ok := productKinds[product]; ok {
		return kind
	}
	words := skuWords(sku)
	for _, entry := range kindWords {
		for _, term := range entry.terms {
			if words[term] {
				return entry.kind
			}
		}
	}
	return KindChat
}

// skuWords splits a SKU into the whole words it is made of, so that a term is
// never matched inside a longer one.
func skuWords(sku string) map[string]bool {
	out := map[string]bool{}
	replaced := strings.ReplaceAll(strings.ToLower(sku), "-", " ")
	for _, field := range strings.Fields(replaced) {
		out[field] = true
	}
	return out
}

// Dimension keys Azure's prices vary along.
const (
	// DimDeployment is where the model runs, which Azure charges differently
	// for: global, regional, or confined to a data zone.
	DimDeployment = "deployment"
	DimTier       = "tier"
	DimContext    = "context"
	DimModality   = "modality"
	// DimRegion is where the request is served. Azure charges differently by
	// region for the same meter: GPT-5.4 output on a data zone deployment is
	// 16.50 in the United States and Europe and 18.00 in Asia and Australia.
	DimRegion = "region"
)

// Serving paths Azure prices separately.
const (
	TierStandard    = "standard"
	TierBatch       = "batch"
	TierProvisioned = "provisioned"
)

// DimFineTuned marks a rate as applying to a fine tuned deployment.
const DimFineTuned = "fine_tuned"

// AttrProduct records the family the meter belongs to.
const AttrProduct = "product"

// word is one abbreviation and what it means.
type word struct {
	term  string
	value string
}

// directions are the words naming which side of a request is charged. Cached
// input is matched before plain input, since Azure writes it as two words.
var directions = []word{
	{"cchd inp", "cached"},
	{"cached inp", "cached"},
	{"cd inp", "cached"},
	{"cache inp", "cached"},
	{"cach inp", "cached"},
	{"ch inp", "cached"},
	{"cached", "cached"},
	{"cchd", "cached"},
	{"inpt", "input"},
	{"input", "input"},
	{"inp", "input"},
	{"outpt", "output"},
	{"output", "output"},
	{"outp", "output"},
	{"opt", "output"},
	{"out", "output"},
}

// deployments are the words naming where the model runs.
var deployments = []word{
	{"data zone", "data-zone"},
	{"datazone", "data-zone"},
	{"dzone", "data-zone"},
	{"dz", "data-zone"},
	{"regnl", "regional"},
	{"rgnl", "regional"},
	{"regional", "regional"},
	{"glbl", "global"},
	{"global", "global"},
	{"gl", "global"},
}

// tiers are the words naming the serving path. "pp" is one of them and not
// decoration: 4.1 pp cd inp Dz costs nearly twice what gpt 4.1 cached Inp Data
// Zone does in the same region, because one is provisioned and the other is
// not.
var tiers = []word{
	{"batch", TierBatch},
	{"provisioned", TierProvisioned},
	{"prov", TierProvisioned},
	{"pp", TierProvisioned},
}

// fineTunings are the words marking a rate as being for a fine tuned
// deployment. It is separate from the serving path because a meter can name
// both, as a provisioned fine tuned model does.
var fineTunings = []word{
	{"fine tuned", "true"},
	{"finetuned", "true"},
	{"ft", "true"},
}

// contexts are the words naming a prompt size band, which Azure prices apart
// on some models.
var contexts = []word{
	{"shortco", "short"},
	{"longco", "long"},
	{"l", "long"},
}

// modalities are the words naming what a rate covers on a multimodal meter.
var modalities = []word{
	{"txt", "text"},
	{"aud", "audio"},
	{"img", "image"},
}

// charges are the words naming something other than inference.
var charges = []word{
	{"trng tkn", "training"},
	{"trng", "training"},
	{"training", "training"},
	{"hstng", "hosting"},
	{"hosting", "hosting"},
}

// noise are words that say nothing about what a rate is for.
var noise = []string{
	"managed",
	"spillover",
	"base",
	"tkn",
	"token",
	"tokens",
	"d",
}

// familyPrefixes supply the part of a model's name the SKU leaves out, which
// is carried by the product instead.
var familyPrefixes = map[string]string{
	"Azure OpenAI GPT5":        "gpt-",
	"Azure OpenAI PP FT GPT4s": "gpt-",
	"Azure OpenAI Media":       "gpt-",
	"Azure OpenAI Embedding":   "text-",
	"Azure OpenAI OSS Models":  "gpt-oss-",
	"Azure Grok Models":        "grok-",
	"Azure Kimi":               "kimi-",
	"Azure Llama Models":       "llama-",
	"Qwen models":              "qwen3-",
	"MAI Models":               "mai-",
	"Azure Fireworks Models":   "fw-",
}

// unitsOfMeasure maps Azure's unit of measure onto a unit.
var unitsOfMeasure = map[string]catalog.Unit{
	"1k":       UnitPer1KTokens,
	"1m":       UnitPer1MTokens,
	"1/hour":   UnitPerHour,
	"1 hour":   UnitPerHour,
	"1/day":    UnitPerDay,
	"1 second": UnitPerSecond,
	"1":        UnitPerUnit,
	"100":      UnitPer100Units,
}

// reading is what one meter's SKU says once taken apart.
type reading struct {
	model      string
	direction  string
	deployment string
	tier       string
	context    string
	modality   string
	charge     string
	fineTuned  string
}

// readSKU takes a SKU apart, returning the model left over once every word
// naming a direction, deployment, serving path, context band, modality or
// non-inference charge is removed.
func readSKU(sku, product string) reading {
	rest := strings.ToLower(strings.TrimPrefix(sku, "Az-"))
	rest = " " + strings.Join(
		strings.Fields(strings.ReplaceAll(rest, "-", " ")),
		" ",
	) + " "
	out := reading{tier: TierStandard}
	rest, out.charge = takeWord(rest, charges)
	rest, out.direction = takeWord(rest, directions)
	rest, out.deployment = takeWord(rest, deployments)
	rest, out.fineTuned = takeWord(rest, fineTunings)
	if remainder, tier := takeWord(rest, tiers); tier != "" {
		rest, out.tier = remainder, tier
	}
	rest, out.context = takeWord(rest, contexts)
	rest, out.modality = takeWord(rest, modalities)
	for _, term := range noise {
		rest = removeWord(rest, term)
	}
	out.model = prefixed(strings.Join(strings.Fields(rest), " "), product)
	return out
}

// prefixed supplies the family the SKU left out.
func prefixed(name, product string) string {
	if name == "" {
		return ""
	}
	prefix, ok := familyPrefixes[product]
	if !ok {
		return name
	}
	if strings.HasPrefix(name, strings.TrimSuffix(prefix, "-")) {
		return name
	}
	return prefix + name
}

// takeWord removes the first matching abbreviation and reports what it meant.
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
	for {
		stripped, ok := cutWord(rest, term)
		if !ok {
			return rest
		}
		rest = stripped
	}
}

// cutWord removes one whole word from a padded phrase.
func cutWord(rest, term string) (string, bool) {
	padded := " " + strings.Join(strings.Fields(rest), " ") + " "
	at := strings.Index(padded, " "+term+" ")
	if at < 0 {
		return rest, false
	}
	return padded[:at+1] + padded[at+len(term)+2:], true
}

// metricFor reports what a rate counts. A meter naming neither a direction nor
// a charge is not a token rate, such as an hourly deployment fee, and is
// recorded as usage.
func metricFor(r reading) catalog.Metric {
	switch {
	case r.charge == "training":
		return MetricTraining
	case r.charge == "hosting":
		return MetricHosting
	case r.direction == "cached":
		return MetricCachedInputTokens
	case r.direction == "output":
		return MetricOutputTokens
	case r.direction == "input":
		return MetricInputTokens
	}
	return MetricUsage
}

// slugID turns the model left in a SKU into an identifier.
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
