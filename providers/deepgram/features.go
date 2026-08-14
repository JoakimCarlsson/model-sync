package deepgram

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Capabilities Deepgram's feature overviews name, in the catalog's words where
// it has one. The rest keep Deepgram's own name for something no other
// provider states: a formatter, a redactor and the four things its audio
// intelligence reads out of a transcript.
const (
	FeatureDiarization       = catalog.CapabilityDiarization
	FeatureKeyterms          = catalog.CapabilityKeyterms
	FeatureWordTimestamps    = catalog.CapabilityWordTimestamps
	FeatureLanguageDetection = catalog.CapabilityLanguageDetection
	FeatureCodeSwitching     = catalog.CapabilityCodeSwitching
	FeatureEntityDetection   = catalog.CapabilityEntityDetection
	FeatureFunctionCalling   = catalog.CapabilityFunctionCalling
	FeatureRealtime          = catalog.CapabilityRealtime
	FeatureSmartFormatting   = "smart_formatting"
	FeatureRedaction         = "redaction"
	FeatureSummarization     = "summarization"
	FeatureTopicDetection    = "topic_detection"
	FeatureSentiment         = "sentiment_analysis"
	FeatureIntent            = "intent_recognition"
	// FeatureLanguagePrompting is biasing a multilingual model towards the
	// languages a caller expects to hear.
	FeatureLanguagePrompting = "language_prompting"
	// FeatureStreamingOutput is returning audio as it is generated rather than
	// as a finished file.
	FeatureStreamingOutput = "streaming_audio_output"
	// FeatureBatch is transcribing a finished recording, which Deepgram
	// documents separately from transcribing a live connection and calls
	// pre-recorded.
	FeatureBatch = "batch"
)

// docFeatures map a row of a feature overview onto what the model can do. The
// tables hold more than capabilities: encodings, sample rates, callbacks and
// container formats are how a request is shaped rather than what a model can
// do, and a row naming one of those is not a feature and is dropped.
var docFeatures = map[string]string{
	"speaker-diarization":        FeatureDiarization,
	"keyterm-prompting":          FeatureKeyterms,
	"word-level-timestamps":      FeatureWordTimestamps,
	"language-detection":         FeatureLanguageDetection,
	"language-prompting":         FeatureLanguagePrompting,
	"multilingual-codeswitching": FeatureCodeSwitching,
	"entity-detection":           FeatureEntityDetection,
	"function-call-request":      FeatureFunctionCalling,
	"smart-formatting":           FeatureSmartFormatting,
	"redaction":                  FeatureRedaction,
	"summarization":              FeatureSummarization,
	"topic-detection":            FeatureTopicDetection,
	"sentiment-analysis":         FeatureSentiment,
	"intent-recognition":         FeatureIntent,
	"audio-output-streaming":     FeatureStreamingOutput,
}

// Numeric bounds Deepgram states in prose rather than in a field.
const (
	// LimitInputCharacters is the most text one speech request may carry.
	LimitInputCharacters = "max_input_characters"
	// LimitSessionSeconds is how long one agent connection may stay open.
	LimitSessionSeconds = "max_session_seconds"
)

// products say which models a feature overview answers for. Deepgram states a
// capability once per product and not once per model, so the page a model's
// product is documented on is what says what it can do.
var products = map[string]product{
	STTStreamingURL: {kind: KindTranscription, delivery: FeatureRealtime},
	STTBatchURL:     {kind: KindTranscription, delivery: FeatureBatch},
	FluxURL: {
		kind:     KindTranscription,
		flux:     true,
		delivery: FeatureRealtime,
	},
	SpeechURL: {kind: KindSpeech},
	AgentURL:  {kind: KindAgent, delivery: FeatureRealtime},
}

// product is one feature overview and the models it describes.
type product struct {
	// kind is the pricing-page product the page documents.
	kind catalog.Kind
	// flux distinguishes the two speech-to-text pages, since the Flux models
	// support a different set from the models beside them and are documented
	// apart from them.
	flux bool
	// delivery is what the page's models do with audio, which is the page
	// itself rather than a row in it: Deepgram documents transcribing a live
	// connection and transcribing a recording as separate products.
	delivery string
}

// applyDocs reads one documentation page onto the models the pricing page
// named.
func (b *builder) applyDocs(doc catalog.Document) {
	if p, ok := products[doc.URL]; ok {
		b.applyFeatures(doc, p)
		return
	}
	switch doc.URL {
	case SpeechLimitsURL:
		b.applySpeechLimits(doc)
	}
}

// applyFeatures records what one product's models can do.
//
// A row applies to the models of the product the page documents. It also
// applies to the add-on and audio intelligence models, which are the same
// capabilities sold separately: the pricing page lists Speaker Diarization as
// a thing with a rate, the documentation lists it as a thing a transcription
// can do, and the two are joined on the name, which is all either states.
func (b *builder) applyFeatures(doc catalog.Document, p product) {
	rows := featureRows(string(doc.Body))
	b.each(func(m *catalog.Model) {
		if p.describes(m) {
			for _, row := range rows {
				if row.multilingual && !multilingual(m.ID) {
					continue
				}
				m.AddList(catalog.ListFeatures, row.feature)
			}
			m.AddList(catalog.ListFeatures, p.delivery)
			m.AddSource(doc.URL)
			return
		}
		if !sold(m.Kind) {
			return
		}
		for _, row := range rows {
			if row.id != m.ID {
				continue
			}
			m.AddList(catalog.ListFeatures, row.feature, p.delivery)
			m.AddSource(doc.URL)
		}
	})
	if p.kind == KindAgent {
		b.applySessionLimit(doc)
	}
}

// describes reports whether a page's product is the one a model is sold under.
func (p product) describes(m *catalog.Model) bool {
	if m.Kind != p.kind {
		return false
	}
	return p.kind != KindTranscription || p.flux == flux(m.ID)
}

// flux reports whether a model is one of the Flux models, which are the ones
// with a feature overview of their own.
func flux(id string) bool {
	return strings.HasPrefix(id, "flux-")
}

// multilingual reports whether a model is the variant of its pair that follows
// more than one language, which is what a row restricted to a multilingual
// model applies to.
func multilingual(id string) bool {
	return strings.Contains(id, "multilingual")
}

// sold reports whether a model is a capability the pricing page sells in its
// own right, which is what an add-on and an audio intelligence feature are.
func sold(kind catalog.Kind) bool {
	return kind == KindAddOn || kind == KindIntelligence
}

// featureRow is one row of a feature overview that names a capability.
type featureRow struct {
	// id is the row's name slugged, which is how a capability sold as a model
	// is recognised: the pricing page and the documentation call it the same
	// thing.
	id string
	// feature is the catalog's word for what the row names.
	feature string
	// multilingual records that the row applies only to the model of its pair
	// that follows more than one language.
	multilingual bool
}

var (
	// tableRowRe matches one row of a markdown table.
	tableRowRe = regexp.MustCompile(`(?m)^\|(.*)\|[ \t]*$`)
	// linkRe matches a markdown link, whose text is what the table means to
	// say and whose target is the page documenting it.
	linkRe = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	// multiOnlyRe matches a cell restricting a row to the multilingual model
	// of a pair, "`flux-general-multi` only".
	multiOnlyRe = regexp.MustCompile(`(?i)-multi\b.*\bonly\b`)
	// charactersRe matches the row of the text-to-speech limits table, which
	// names the models an amount of text applies to and then the amount.
	charactersRe = regexp.MustCompile(
		`(?m)^\|([^|]*(?:Aura|aura)[^|]*)\|\s*([\d,]+)\s*\|`,
	)
	// sessionRe matches how long the Voice Agent API leaves a connection open,
	// which its feature overview states in a sentence rather than a table.
	sessionRe = regexp.MustCompile(
		`(?i)sessions close automatically after (\d+) hours?`,
	)
)

// featureRows reads every row of a page that names a capability.
func featureRows(body string) []featureRow {
	var out []featureRow
	for _, match := range tableRowRe.FindAllStringSubmatch(body, -1) {
		cells := strings.Split(match[1], "|")
		id := slugID(label(cells[0]))
		feature, ok := docFeatures[id]
		if !ok {
			continue
		}
		out = append(out, featureRow{
			id:           id,
			feature:      feature,
			multilingual: restricted(cells),
		})
	}
	return out
}

// restricted reports whether a row applies only to the multilingual model of
// its pair, which the row says either in its name or in the cell naming the
// model option it needs.
func restricted(cells []string) bool {
	if strings.HasPrefix(strings.ToLower(label(cells[0])), "multilingual") {
		return true
	}
	for _, c := range cells[1:] {
		if multiOnlyRe.MatchString(c) {
			return true
		}
	}
	return false
}

// plain reduces a markdown cell to the words it shows.
func plain(cell string) string {
	return text(linkRe.ReplaceAllString(cell, "$1"))
}

// label reads what a row names. Where the cell carries a link the link's text
// is the name, since Deepgram appends an aside to some of them and the link is
// the name without it.
func label(cell string) string {
	if match := linkRe.FindStringSubmatch(cell); match != nil {
		return text(match[1])
	}
	return plain(cell)
}

// applySpeechLimits records the ceiling on the text one speech request may
// carry, which the text-to-speech guide states in a table of models against an
// amount rather than per model.
func (b *builder) applySpeechLimits(doc catalog.Document) {
	for _, match := range charactersRe.FindAllStringSubmatch(
		string(doc.Body),
		-1,
	) {
		limit, err := strconv.ParseInt(
			strings.ReplaceAll(strings.TrimSpace(match[2]), ",", ""),
			10,
			64,
		)
		if err != nil {
			continue
		}
		for _, name := range strings.Split(plain(match[1]), ",") {
			m, ok := b.models[slugID(name)]
			if !ok {
				continue
			}
			m.SetLimit(LimitInputCharacters, limit)
			m.AddSource(doc.URL)
		}
	}
}

// applySessionLimit records how long an agent connection may stay open, which
// is the only bound the Voice Agent API states.
func (b *builder) applySessionLimit(doc catalog.Document) {
	match := sessionRe.FindStringSubmatch(string(doc.Body))
	if match == nil {
		return
	}
	hours, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil {
		return
	}
	b.each(func(m *catalog.Model) {
		if m.Kind == KindAgent {
			m.SetLimit(LimitSessionSeconds, hours*3600)
		}
	})
}
