package mistral

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// currency is the currency the catalog records. Mistral quotes every rate
// twice, in dollars and in euros, and only the dollar figure is kept because
// the euro one is the same price in another denomination rather than a second
// rate.
const currency = "USD"

// Metrics Mistral bills on.
const (
	MetricInputTokens       catalog.Metric = "input_tokens"
	MetricCachedInputTokens catalog.Metric = "cached_input_tokens"
	MetricOutputTokens      catalog.Metric = "output_tokens"
	MetricAudioInput        catalog.Metric = "audio_input"
	MetricSpeech            catalog.Metric = "speech"
	MetricOCR               catalog.Metric = "ocr"
)

// Units Mistral quotes amounts against.
const (
	UnitPer1MTokens catalog.Unit = "per_1m_tokens"
	UnitPer1MChars  catalog.Unit = "per_1m_characters"
	UnitPerMinute   catalog.Unit = "per_minute"
	UnitPer1KPages  catalog.Unit = "per_1k_pages"
)

// Kinds Mistral publishes.
const (
	KindChat          catalog.Kind = "chat"
	KindTranscription catalog.Kind = "transcription"
	KindSpeech        catalog.Kind = "speech"
	KindEmbedding     catalog.Kind = "embedding"
	KindModeration    catalog.Kind = "moderation"
	KindOCR           catalog.Kind = "ocr"
)

// DimPageKind separates the two page rates an OCR model carries, which differ
// by whether the page was annotated.
const DimPageKind = "page_kind"

// Standing a model can be in, read from the badge its page carries.
const (
	StateActive     = "active"
	StatePreview    = "preview"
	StateDeprecated = "deprecated"
	StateRetired    = "retired"
	StateShutdown   = "shutdown"
)

// withdrawnStates are the standings that mean Mistral has stopped serving a
// model, so the catalog does not carry it. Mistral's badge writes only the
// first of them today; the second is the other word for the same standing, and
// is matched so a change of wording drops the model rather than reviving it.
// Deprecated is deliberately not here: such a model still serves until its
// retirement date and Mistral still publishes it.
var withdrawnStates = []string{StateRetired, StateShutdown}

// Scalar keys the documents populate.
const (
	AttrState          = "state"
	AttrVersion        = "version"
	AttrSummary        = "summary"
	AttrReleased       = "released"
	AttrLicense        = "license"
	AttrOpenWeights    = "open_weights"
	AttrDeprecatedOn   = "deprecated_on"
	AttrRetirementDate = "retirement_date"
	AttrReplacement    = "recommended_replacement"
)

// Numeric keys the model pages populate.
const (
	LimitContextWindow  = "context_window"
	LimitMaxOutputToken = "max_output_tokens"
)

// Enumeration keys the model pages populate.
const (
	ListFeatures         = catalog.ListFeatures
	ListAliases          = "aliases"
	ListInputModalities  = "input_modalities"
	ListOutputModalities = "output_modalities"
)

// badgeStates map the lifecycle badge a model page carries onto the standing
// recorded for it.
var badgeStates = map[string]string{
	"GA":             StateActive,
	"Public Preview": StatePreview,
	"Deprecated":     StateDeprecated,
	"Retired":        StateRetired,
}

// featureNames map the identifier Mistral keys a capability tile by onto the
// catalog's vocabulary. Only the names that differ are listed; anything else
// keeps Mistral's own word with its separators normalized, because inventing a
// translation for a capability no other provider states would lose which
// capability it was.
var featureNames = map[string]string{
	"agents-conversations":       "agents",
	"annotations-structured-ocr": catalog.CapabilityStructuredOutputs,
	"chat-completions":           "streaming",
	"fim":                        "fill_in_the_middle",
	"predicted-outputs":          "predicted_outputs",
}

// modalityNames map the wording of a modality tooltip onto the catalog's
// vocabulary. Mistral marks a reasoning model by giving it a reasoning output,
// which is a capability rather than a modality and is recorded as one.
var modalityNames = map[string]string{
	"text":       "text",
	"image":      "image",
	"audio":      "audio",
	"document":   "file",
	"embeddings": "",
	"scores":     "",
	"reasoning":  "",
}

// denominators map the denominator Mistral writes after an amount onto a unit
// and, where the denominator alone says what is being counted, onto a metric.
var denominators = map[string]struct {
	Unit   catalog.Unit
	Metric catalog.Metric
}{
	"/m tokens":             {UnitPer1MTokens, ""},
	"/m chars":              {UnitPer1MChars, MetricSpeech},
	"/min":                  {UnitPerMinute, MetricAudioInput},
	"/1000 pages":           {UnitPer1KPages, MetricOCR},
	"/1000 annotated pages": {UnitPer1KPages, MetricOCR},
}

// pageKinds name the two OCR page rates apart.
var pageKinds = map[string]string{
	"/1000 pages":           "plain",
	"/1000 annotated pages": "annotated",
}

// featureKinds map a capability onto what a model carrying it does. A model
// stating several is classified by the first match in this order, so that a
// chat model that also transcribes is still a chat model.
var featureKinds = []struct {
	feature string
	kind    catalog.Kind
}{
	{"chat-completions", KindChat},
	{"fim", KindChat},
	{"ocr", KindOCR},
	{"tts", KindSpeech},
	{"transcriptions", KindTranscription},
	{"embeddings", KindEmbedding},
	{"moderations", KindModeration},
}

// kindFor reports what a model does, read from the capabilities its page
// lists. Mistral states no modality for a model whose page lists none, so the
// name is the fallback: Voxtral hears, and the rest say what they are.
func kindFor(id string, features []string) catalog.Kind {
	for _, entry := range featureKinds {
		if slices.Contains(features, entry.feature) {
			return entry.kind
		}
	}
	lower := strings.ToLower(id)
	for _, entry := range nameKinds {
		if strings.Contains(lower, entry.fragment) {
			return entry.kind
		}
	}
	return KindChat
}

// nameKinds map a fragment of a model's name onto what it does, used only for
// models whose page lists no capabilities at all.
var nameKinds = []struct {
	fragment string
	kind     catalog.Kind
}{
	{"voxtral-mini-tts", KindSpeech},
	{"voxtral", KindTranscription},
	{"ocr", KindOCR},
	{"moderation", KindModeration},
	{"embed", KindEmbedding},
}

// featureName rewrites a capability identifier into the catalog's vocabulary.
func featureName(id string) string {
	if name, ok := featureNames[id]; ok {
		return name
	}
	return strings.ReplaceAll(id, "-", "_")
}

// parseCount reads a quantity such as "256k", "1M" or "8k". Mistral writes
// "--" where a model has no such bound, which is not a zero.
func parseCount(value string) int64 {
	text := strings.TrimSpace(value)
	if text == "" || text == "--" {
		return 0
	}
	scale := 1.0
	switch {
	case strings.HasSuffix(strings.ToLower(text), "k"):
		scale, text = 1_000, text[:len(text)-1]
	case strings.HasSuffix(strings.ToLower(text), "m"):
		scale, text = 1_000_000, text[:len(text)-1]
	}
	n, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil {
		return 0
	}
	return int64(n * scale)
}

// dateLayouts are the date formats Mistral writes.
var dateLayouts = []struct{ in, out string }{
	{"January 2, 2006", "2006-01-02"},
	{"1/2/2006", "2006-01-02"},
	{"01/02/2006", "2006-01-02"},
}

// isoDate rewrites a date into calendar order. Mistral writes release dates in
// prose and lifecycle dates month-first.
func isoDate(value string) string {
	text := strings.TrimSpace(value)
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout.in, text); err == nil {
			return t.Format(layout.out)
		}
	}
	return text
}
