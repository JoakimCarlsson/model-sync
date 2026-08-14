package azure

import (
	"slices"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Metrics Azure bills on.
const (
	MetricInputTokens       catalog.Metric = "input_tokens"
	MetricOutputTokens      catalog.Metric = "output_tokens"
	MetricCachedInputTokens catalog.Metric = "cached_input_tokens"
	MetricCacheWriteTokens  catalog.Metric = "cache_write_tokens"
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
	KindOCR           catalog.Kind = "ocr"
	KindRerank        catalog.Kind = "rerank"
)

// productKinds are the families whose every meter is one kind, whatever the
// SKU says. A text input to an image model is still an image model.
var productKinds = map[string]catalog.Kind{
	"Azure OpenAI Embedding": KindEmbedding,
	"Azure BFL Flux Models":  KindImage,
	"MAI Models":             KindImage,
}

// nonModelProducts are the families whose every meter charges for something
// that is not a model: an hour of the compute a model is deployed on, and the
// unit a reservation is sold in.
var nonModelProducts = map[string]bool{
	"Managed Compute":             true,
	"Foundry Local Azure Open AI": true,
	"Azure OpenAI Free Meter":     true,
}

// nonModelSKUs are the words marking one meter of a model family as charging
// for reserved capacity or for a hosted tool's calls rather than for a model.
var nonModelSKUs = []string{
	"throughput",
	"code-interpreter",
	"file search",
	"file-search",
	"assistants",
}

// nonModel reports whether a meter charges for something other than a model.
func nonModel(sku, product string) bool {
	if nonModelProducts[product] {
		return true
	}
	lower := strings.ToLower(sku)
	for _, term := range nonModelSKUs {
		if strings.Contains(lower, term) {
			return true
		}
	}
	return false
}

// kindWords are the SKU words that say what a model does, for the families
// that mix kinds in one product. They are checked in order, since a realtime
// meter names both what it is and what it carries and the more specific
// reading wins: "gpt rt img" is the image rate of a realtime model.
var kindWords = []struct {
	terms []string
	kind  catalog.Kind
}{
	{
		[]string{"tcrb", "trscb", "transcribe", "transcription", "whisper"},
		KindTranscription,
	},
	{[]string{"tts", "speech"}, KindSpeech},
	{[]string{"rt", "rtime", "realtime"}, KindRealtime},
	{[]string{"sora", "video", "vid"}, KindVideo},
	{[]string{"dall", "image", "img", "flux"}, KindImage},
	{[]string{"aud", "audio"}, KindAudio},
	{[]string{"embed", "embedding", "embeddings", "ada"}, KindEmbedding},
	{[]string{"ocr"}, KindOCR},
	{[]string{"rerank"}, KindRerank},
}

// kindFor reports what a meter's model does, read from the family it belongs
// to and failing that from the words left in its SKU once the rate's own
// dimensions are taken out of it. Reading the whole SKU would classify
// "gpt-realtime-2 Image inp Gl" as an image model, when what it says is the
// image rate of a realtime one.
func kindFor(name, product string) catalog.Kind {
	if kind, ok := productKinds[product]; ok {
		return kind
	}
	words := skuWords(name)
	for _, entry := range kindWords {
		for _, term := range entry.terms {
			if names(words, term) {
				return entry.kind
			}
		}
	}
	return KindChat
}

// glued is the length from which a term is also matched against the start of a
// word. Azure runs words together in some SKUs, so "realtimePrvwAudInp" is a
// realtime meter, while a term short enough to appear inside an unrelated word
// is matched whole.
const glued = 5

// names reports whether a SKU's words name a term.
func names(words map[string]bool, term string) bool {
	if words[term] {
		return true
	}
	if len(term) < glued {
		return false
	}
	for word := range words {
		if strings.HasPrefix(word, term) {
			return true
		}
	}
	return false
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
	// DimQuality, DimResolution, DimAspect and DimDuration are what an image
	// and a video rate vary along. Azure meters one clip length at one frame
	// size at one shape, so a model that generates them has a rate per
	// combination rather than a rate.
	DimQuality    = "quality"
	DimResolution = "resolution"
	DimAspect     = "aspect"
	DimDuration   = "duration"
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

// DimModelGrader marks a rate as covering a grader model's own tokens.
const DimModelGrader = "model_grader"

// AttrProduct records the family the meter belongs to.
const AttrProduct = "product"

// word is one abbreviation and what it means.
type word struct {
	term  string
	value string
}

// directions are the words naming which side of a request is charged. Cached
// input is matched before plain input, since Azure writes it as two words, and
// the abbreviations of input are matched longest first so that "inpt" is never
// read as the bare "in" with a letter left over.
var directions = []word{
	{"cchd inp", "cached"},
	{"cached inp", "cached"},
	{"cd inp", "cached"},
	{"cache inp", "cached"},
	{"cach inp", "cached"},
	{"ch inp", "cached"},
	{"cchd in", "cached"},
	{"cached in", "cached"},
	{"cd in", "cached"},
	{"in cd", "cached"},
	{"cd wr", "cache-write"},
	{"cd", "cached"},
	{"cached", "cached"},
	{"ccchd", "cached"},
	{"cched", "cached"},
	{"cchd", "cached"},
	{"inpt", "input"},
	{"input", "input"},
	{"inp", "input"},
	{"in", "input"},
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
	{"dzn", "data-zone"},
	{"dz", "data-zone"},
	{"regnl", "regional"},
	{"rgnl", "regional"},
	{"regional", "regional"},
	{"regn", "regional"},
	{"rg", "regional"},
	{"developer", "developer"},
	{"dev", "developer"},
	{"glbl", "global"},
	{"global", "global"},
	{"glb", "global"},
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
	{"std", TierStandard},
	{"standard", TierStandard},
}

// fineTunings are the words marking a rate as being for a fine tuned
// deployment. It is separate from the serving path because a meter can name
// both, as a provisioned fine tuned model does. "rft" is reinforcement fine
// tuning, which is a method of producing one rather than a different thing to
// charge for.
var fineTunings = []word{
	{"fine tuned", "true"},
	{"finetuned", "true"},
	{"rft", "true"},
	{"ft", "true"},
}

// graders are the words marking a rate as covering the tokens a grader model
// spends judging a fine tuning candidate rather than the tokens of a request.
var graders = []word{
	{"mdl grdr", "true"},
	{"mdel grdr", "true"},
	{"model grader", "true"},
	{"grdr", "true"},
	{"grader", "true"},
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
	{"text", "text"},
	{"aud", "audio"},
	{"audio", "audio"},
	{"img", "image"},
	{"image", "image"},
}

// qualities are the words naming the fidelity an image rate is quoted at.
// DALL-E 3 is metered once per combination of quality and resolution, and
// those are settings of one model rather than four models.
var qualities = []word{
	{"hd", "hd"},
}

// resolutions are the words naming the frame size a rate covers. Sora is
// priced per second at each of them, and DALL-E at two sizes it names in
// words rather than in pixels.
var resolutions = []word{
	{"1080p", "1080p"},
	{"720p", "720p"},
	{"480p", "480p"},
	{"high res", "high"},
	{"highres", "high"},
	{"lowres", "low"},
}

// aspects are the words naming the shape of the frame. Azure prices a square
// video apart from a landscape one of the same height.
var aspects = []word{
	{"sq", "square"},
}

// durations are the bands of clip length Sora prices apart. Azure writes them
// as a range with the unit on the end, which the separator has already split.
var durations = []word{
	{"1 5s", "1-5s"},
	{"6 10s", "6-10s"},
	{"11 15s", "11-15s"},
	{"16 20s", "16-20s"},
	{"1 20s", "1-20s"},
}

// charges are the words naming something other than inference.
var charges = []word{
	{"trng tkn", "training"},
	{"trng", "training"},
	{"training", "training"},
	{"hstng", "hosting"},
	{"hosting", "hosting"},
}

// noise are words that say nothing about what a rate is for. "mp" is the
// megapixel a FLUX rate is quoted against, which the unit of measure already
// states, and "model" is what Azure writes before a fine tuned model's own
// name in some SKUs.
var noise = []string{
	"managed",
	"spillover",
	"base",
	"tkn",
	"token",
	"tokens",
	"model",
	"mp",
	"ep",
	"d",
}

// unprefixed are the models a family's prefix does not belong to. Azure sells
// Sora under the same product as its GPT media models and it is not one.
var unprefixed = map[string]bool{
	"sora": true,
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
	grader     string
	quality    string
	resolution string
	aspect     string
	duration   string
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
	rest, out.modality = takeModality(rest)
	rest, out.charge = takeWord(rest, charges)
	rest, out.grader = takeWord(rest, graders)
	rest, out.direction = takeWord(rest, directions)
	rest = removeWords(rest, directions)
	rest, out.deployment = takeWord(rest, deployments)
	rest = removeWords(rest, deployments)
	rest, out.fineTuned = takeWord(rest, fineTunings)
	if remainder, tier := takeWord(rest, tiers); tier != "" {
		rest, out.tier = remainder, tier
	}
	rest, out.context = takeWord(rest, contexts)
	rest, out.quality = takeWord(rest, qualities)
	rest, out.resolution = takeWord(rest, resolutions)
	rest, out.aspect = takeWord(rest, aspects)
	rest, out.duration = takeWord(rest, durations)
	for _, term := range noise {
		rest = removeWord(rest, term)
	}
	out.model = prefixed(strings.Join(strings.Fields(rest), " "), product)
	return out
}

// prefixed supplies the family the SKU left out.
//
// A SKU states as much of the family as it feels like: the gpt-oss meters name
// it in full, in part and not at all, as "gpt oss 120b", "oss 20b" and "20b".
// Whatever of the prefix the name already carries is therefore dropped from
// its front rather than the prefix being skipped, so that all three read as
// one model.
func prefixed(name, product string) string {
	fields := strings.Fields(name)
	if len(fields) == 0 || unprefixed[fields[0]] {
		return name
	}
	prefix, ok := familyPrefixes[product]
	if !ok {
		return name
	}
	carried := strings.Split(strings.Trim(prefix, "-"), "-")
	for len(fields) > 1 && slices.Contains(carried, fields[0]) {
		fields = fields[1:]
	}
	rest := strings.Join(fields, " ")
	if strings.HasPrefix(rest, carried[0]) {
		return rest
	}
	return prefix + rest
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

// removeWords drops whatever a vocabulary still matches, so that a SKU naming
// one fact twice leaves nothing of it behind: "GPT 5 Batch Inpt cchd" states
// the direction as two separate words and only one of them is taken.
func removeWords(rest string, words []word) string {
	for _, w := range words {
		rest = removeWord(rest, w.term)
	}
	return rest
}

// takeModality removes the word naming what a rate covers on a multimodal
// meter.
//
// Telling that word from the model's own name is the hard part, because Azure
// names several models after a modality: gpt-audio is metered as "gpt aud",
// gpt-image-1 mini as "gpt img 1 mini" and DALL-E's family as "Image 2". The
// word is the rate's where it is the last of them, is neither of the first two
// words of the SKU, and either sits beside the word naming the direction or
// the deployment or is not the only one there. Every other modality word is
// part of the name: "gpt aud mini Inp" and "gpt aud mini txt Inp" are the
// audio and the text rate of one model, and this is the reading that gives
// them one identifier.
func takeModality(rest string) (string, string) {
	fields := strings.Fields(rest)
	count := 0
	for _, field := range fields {
		if modalityOf(field) != "" {
			count++
		}
	}
	for i := len(fields) - 1; i > 1; i-- {
		value := modalityOf(fields[i])
		if value == "" {
			continue
		}
		if count < 2 && !beside(fields, i) {
			continue
		}
		kept := append(fields[:i:i], fields[i+1:]...)
		return " " + strings.Join(kept, " ") + " ", value
	}
	return rest, ""
}

// modalityOf reports what a word names, or nothing where it names no modality.
func modalityOf(field string) string {
	for _, w := range modalities {
		if field == w.term {
			return w.value
		}
	}
	return ""
}

// beside reports whether the word at i neighbours one naming the direction
// charged or the deployment served, which is where a rate's modality is
// written.
func beside(fields []string, i int) bool {
	return i > 0 && neighbourWords[fields[i-1]] ||
		i+1 < len(fields) && neighbourWords[fields[i+1]]
}

// neighbourWords are the single words a direction or a deployment is written
// with, taken apart because Azure writes some of them as two.
var neighbourWords = func() map[string]bool {
	out := map[string]bool{}
	for _, list := range [][]word{directions, deployments} {
		for _, w := range list {
			for _, field := range strings.Fields(w.term) {
				out[field] = true
			}
		}
	}
	return out
}()

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
	case r.direction == "cache-write":
		return MetricCacheWriteTokens
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
