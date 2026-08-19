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
	// FeaturePunctuation is placing punctuation in a transcript that was
	// spoken without any.
	FeaturePunctuation = "punctuation"
	// FeatureNumerals is writing spoken numbers as digits.
	FeatureNumerals = "numerals"
	// FeatureProfanityFilter is masking profanity in the transcript.
	FeatureProfanityFilter = "profanity_filter"
	// FeatureParagraphs is dividing a transcript into paragraphs.
	FeatureParagraphs = "paragraphs"
	// FeatureUtterances is dividing a transcript into the stretches of speech
	// between pauses.
	FeatureUtterances = "utterances"
	// FeatureMultichannel is transcribing each channel of the audio
	// separately.
	FeatureMultichannel = "multichannel"
	// FeatureSearch is finding a spoken phrase in the audio.
	FeatureSearch = "search"
	// FeatureFindAndReplace is rewriting terms in the transcript as it is
	// produced.
	FeatureFindAndReplace = "find_and_replace"
	// FeatureFillerWords is keeping the ums and uhs a transcript would
	// otherwise drop.
	FeatureFillerWords = "filler_words"
	// FeatureEndpointing is deciding when a speaker has stopped talking.
	FeatureEndpointing = "endpointing"
	// FeatureKeywords is the older way of biasing towards a term, which
	// Deepgram keeps beside keyterm prompting and calls legacy.
	FeatureKeywords = "keyword_boosting"
	// FeatureAlternatives is returning more than one candidate transcript.
	FeatureAlternatives = "alternatives"
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
	"punctuation":                FeaturePunctuation,
	"numerals":                   FeatureNumerals,
	"profanity-filter":           FeatureProfanityFilter,
	"paragraphs":                 FeatureParagraphs,
	"utterances":                 FeatureUtterances,
	"multichannel":               FeatureMultichannel,
	"search":                     FeatureSearch,
	"find-and-replace":           FeatureFindAndReplace,
	"filler-words":               FeatureFillerWords,
	"endpointing":                FeatureEndpointing,
	"keywords":                   FeatureKeywords,
	"alternatives":               FeatureAlternatives,
	"smart-format":               FeatureSmartFormatting,
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
			if !b.serves(m, p) {
				return
			}
			for _, row := range rows {
				if !row.applies(m) {
					continue
				}
				b.addFeature(m, row.feature)
			}
			b.addFeature(m, p.delivery)
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
			b.addFeature(m, row.feature)
			b.addFeature(m, p.delivery)
			m.AddSource(doc.URL)
		}
	})
	if p.kind == KindAgent {
		b.applySessionLimit(doc)
	}
}

// serves reports whether a page describes how a model is reached. Deepgram
// documents transcribing a live connection and transcribing a recording as
// separate products, and its concurrency reference and its pricing page say
// which of the two each model answers on: Whisper only takes a recording and
// Flux only takes a connection. A model neither document places is described
// by both pages, which is what Deepgram's own wording leaves open.
func (b *builder) serves(m *catalog.Model, p product) bool {
	if p.delivery == "" {
		return true
	}
	known := false
	for _, f := range m.Lists[catalog.ListFeatures] {
		if f == p.delivery {
			return true
		}
		if f == FeatureRealtime || f == FeatureBatch {
			known = true
		}
	}
	return !known
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
	// models are the models a row is restricted to, where it names them. The
	// streaming overview writes a column of them beside entity detection, and
	// the Flux overview writes one model against two of its rows.
	models []string
}

// applies reports whether a row describes one model. A row naming the models
// it belongs to answers for itself; a row that only says it is for the
// multilingual model of a pair is left to the naming of the pair, which is how
// the speech-to-text overviews restrict a row without naming anything.
func (r featureRow) applies(m *catalog.Model) bool {
	if len(r.models) > 0 {
		for _, name := range r.models {
			if name == m.ID || name == m.Attrs[AttrFamily] {
				return true
			}
		}
		return false
	}
	return !r.multilingual || multilingual(m.ID)
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
	// restrictionSplitRe matches how a cell separates the models it names.
	restrictionSplitRe = regexp.MustCompile(`(?i),|\sand\s`)
	// modelNameRe matches a name that can only be a Deepgram speech-to-text
	// model, which is what tells a column of models from a column of prose.
	modelNameRe = regexp.MustCompile(
		`^(?:flux|nova|nova-2|nova-3|enhanced|base|whisper)(?:-[a-z0-9-]+)?$`,
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
			models:       restrictionModels(cells),
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

// restrictionModels returns the models a row names beside itself, where the
// cell holds nothing but model names. Deepgram writes the models a feature is
// available on as a column of its own, and writes a single model followed by
// the word only where a feature belongs to one of a pair.
func restrictionModels(cells []string) []string {
	for _, c := range cells[1:] {
		names := modelNames(c)
		if len(names) > 0 {
			return names
		}
	}
	return nil
}

// modelNames reads a cell as a list of models, and reports none unless every
// word in it is one.
func modelNames(cell string) []string {
	var out []string
	stripped := strings.ReplaceAll(plain(cell), "`", " ")
	for _, field := range restrictionSplitRe.Split(stripped, -1) {
		field = strings.TrimSpace(strings.TrimSuffix(
			strings.TrimSpace(field),
			"only",
		))
		if field == "" {
			continue
		}
		name := slugID(field)
		if !modelNameRe.MatchString(name) {
			return nil
		}
		out = append(out, name)
	}
	return out
}

// plain reduces a markdown cell to the words it shows, dropping the marks
// that make a link a link and a value code.
func plain(cell string) string {
	return text(strings.ReplaceAll(
		linkRe.ReplaceAllString(cell, "$1"),
		"`",
		" ",
	))
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

// AttrLanguageSupport is what a page says about the languages a capability
// covers where it names them in words instead of in codes. The intelligence
// overview writes "English (all available regions)" and links to the model
// overview rather than listing the codes, which is a statement about coverage
// that no list of codes could carry without inventing them.
const AttrLanguageSupport = "language_support"

// applyIntelligence reads the intelligence overview, which is the only
// document saying whether each of the four things Deepgram reads out of a
// transcript runs on a live connection or only on a finished recording. The
// pricing page sells them as models, and this page describes them by the same
// names, so the two are joined on the name.
func (b *builder) applyIntelligence(doc catalog.Document) {
	for _, match := range tableRowRe.FindAllStringSubmatch(
		string(doc.Body),
		-1,
	) {
		cells := strings.Split(match[1], "|")
		if len(cells) < 4 {
			continue
		}
		m, ok := b.models[slugID(label(cells[0]))]
		if !ok {
			continue
		}
		if affirms(cells[1]) {
			b.addFeature(m, FeatureBatch)
		}
		if affirms(cells[2]) {
			b.addFeature(m, FeatureRealtime)
		}
		m.SetAttr(AttrLanguageSupport, plain(cells[3]))
		m.AddSource(doc.URL)
	}
}

// affirms reports whether a cell says a feature is available. Deepgram writes
// yes with a footnote mark beside it where the answer is qualified, and the
// qualification is the column of models the streaming overview states.
func affirms(cell string) bool {
	return strings.HasPrefix(strings.ToLower(plain(cell)), "yes")
}
