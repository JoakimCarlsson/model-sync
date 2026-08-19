package elevenlabs

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// The capability guides. Each documents one thing the platform does, and each
// states bounds and abilities the models page leaves out: how long a generation
// may run, what a request may carry in, and which version of a model an ability
// arrived with.
const (
	SpeechToTextGuideURL = "https://elevenlabs.io/docs/overview/" +
		"capabilities/speech-to-text.md"
	MusicGuideURL = "https://elevenlabs.io/docs/overview/" +
		"capabilities/music.md"
	SoundEffectsGuideURL = "https://elevenlabs.io/docs/overview/" +
		"capabilities/sound-effects.md"
	VoiceChangerGuideURL = "https://elevenlabs.io/docs/overview/" +
		"capabilities/voice-changer.md"
)

// guideURLs are the capability guides, in the order they are read.
var guideURLs = []string{
	SpeechToTextGuideURL,
	MusicGuideURL,
	SoundEffectsGuideURL,
	VoiceChangerGuideURL,
}

// guideKinds say which models a guide speaks for where a sentence on it names
// none. A guide documents one capability, and every model of that kind is an
// implementation of it, so a bound the guide states without qualification binds
// them all.
var guideKinds = map[string]catalog.Kind{
	SpeechToTextGuideURL: KindTranscription,
	MusicGuideURL:        KindMusic,
	SoundEffectsGuideURL: KindSoundEffects,
	VoiceChangerGuideURL: KindVoiceChanger,
}

// Bounds on a generation that the guides state.
const (
	// LimitMinDuration is the shortest generation a model produces.
	LimitMinDuration = "min_duration_seconds"
	// LimitMaxDuration is the longest, which for the voice changer is the
	// longest recording one request may carry rather than the longest output.
	LimitMaxDuration = "max_duration_seconds"
)

// ModalityVideo is what the transcription guide accepts alongside audio.
const ModalityVideo = "video"

// FeatureNoiseRemoval is set where the guide says the environment a recording
// was made in can be taken out of the result.
const FeatureNoiseRemoval = "background_noise_removal"

// availabilityRules read the sentences a guide states an ability with.
//
// ElevenLabs writes an ability as "X is available with Y", where Y names the
// models rather than the whole family, which is the only place the platform
// says which version an ability arrived with. The models a sentence names are
// read from the sentence itself, so a rule states the ability and never the
// models it belongs to.
var availabilityRules = []struct {
	re      *regexp.Regexp
	feature string
	input   string
}{
	{
		regexp.MustCompile(
			`(?i)keyterm prompting is available with ([^.]+)\.`,
		),
		FeatureKeyterms,
		"",
	},
	{
		regexp.MustCompile(
			`(?i)no verbatim mode is available with ([^.]+)\.`,
		),
		FeatureDisfluencyRemoval,
		"",
	},
	{
		regexp.MustCompile(
			`(?i)entity detection is available with ([^.]+)\.`,
		),
		FeatureEntities,
		"",
	},
	{
		regexp.MustCompile(
			`(?i)audio reference is available with ([^.]+)\.`,
		),
		FeatureAudioReference,
		ModalityAudio,
	},
}

var (
	// noiseRemovalRe matches the voice changer guide stating that the room a
	// recording was made in can be taken out of the result. The guide states
	// it of the capability rather than of a model, so it belongs to every
	// model that implements the capability.
	noiseRemovalRe = regexp.MustCompile(
		`(?i)background noise\**:\s*use .remove_background_noise`,
	)
	// videoInputRe matches the transcription guide stating that a request may
	// carry a recording of either kind.
	videoInputRe = regexp.MustCompile(
		`(?i)supported input\**:\s*both audio and video files are accepted`,
	)
	// musicDurationRe matches the pair of bounds the music guide states in one
	// sentence, the shorter in seconds and the longer in minutes.
	musicDurationRe = regexp.MustCompile(
		`(?i)minimum duration of (\d+) seconds and a maximum duration ` +
			`of (\d+) minutes`,
	)
	// effectDurationRe matches the longest sound effect one request produces.
	effectDurationRe = regexp.MustCompile(
		`(?i)maximum duration\**:\s*(\d+) seconds per generation`,
	)
	// segmentLengthRe matches the longest recording the voice changer accepts
	// in one request.
	segmentLengthRe = regexp.MustCompile(
		`(?i)maximum segment length\**:\s*(\d+) minutes`,
	)
)

// secondsPerMinute converts the bounds a guide states in minutes.
const secondsPerMinute = 60

// applyGuide reads one capability guide.
//
// Two kinds of sentence on it are facts about a model. One names the models an
// ability is available with, and is recorded against those. The other states a
// bound on the capability the whole guide documents and names no model, and is
// recorded against every model of that kind, because the guide is that kind's
// documentation and the bound is stated of the capability itself.
func (b *builder) applyGuide(doc catalog.Document) {
	body := string(doc.Body)
	for _, rule := range availabilityRules {
		match := rule.re.FindStringSubmatch(body)
		if match == nil {
			continue
		}
		for _, id := range b.mentioned(match[1]) {
			m := b.models[id]
			m.AddSource(doc.URL)
			m.AddList(ListFeatures, rule.feature)
			if rule.input != "" {
				m.AddList(ListInputModalities, rule.input)
			}
		}
	}
	b.applyGuideBounds(doc, body)
	b.applyGuideBullets(doc, body)
}

// applyGuideBullets reads the list a guide opens with as the cards on the
// models page are read.
//
// Only the music guide has one. It opens by naming what the model does in the
// same sentences the flagship card uses, which is the only place ElevenLabs
// states them of Eleven Music rather than of Eleven Music v2, and so the only
// place the first version of the model is described at all.
func (b *builder) applyGuideBullets(doc catalog.Document, body string) {
	if doc.URL != MusicGuideURL {
		return
	}
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "* ") {
			continue
		}
		bullet := clean(strings.TrimPrefix(line, "* "))
		b.eachOfKind(doc, guideKinds[doc.URL], func(m *catalog.Model) {
			applyBullet(m, bullet)
		})
	}
}

// applyGuideBounds records the bounds a guide states of its capability rather
// than of a named model.
func (b *builder) applyGuideBounds(doc catalog.Document, body string) {
	kind := guideKinds[doc.URL]
	if noiseRemovalRe.MatchString(body) {
		b.eachOfKind(doc, kind, func(m *catalog.Model) {
			m.AddList(ListFeatures, FeatureNoiseRemoval)
		})
	}
	if videoInputRe.MatchString(body) {
		b.eachOfKind(doc, kind, func(m *catalog.Model) {
			m.AddList(ListInputModalities, ModalityVideo)
		})
	}
	if match := musicDurationRe.FindStringSubmatch(body); match != nil {
		b.eachOfKind(doc, kind, func(m *catalog.Model) {
			m.SetLimit(LimitMinDuration, count(match[1]))
			m.SetLimit(LimitMaxDuration, count(match[2])*secondsPerMinute)
		})
	}
	if match := effectDurationRe.FindStringSubmatch(body); match != nil {
		b.eachOfKind(doc, kind, func(m *catalog.Model) {
			m.SetLimit(LimitMaxDuration, count(match[1]))
		})
	}
	if match := segmentLengthRe.FindStringSubmatch(body); match != nil {
		b.eachOfKind(doc, kind, func(m *catalog.Model) {
			m.SetLimit(LimitMaxDuration, count(match[1])*secondsPerMinute)
		})
	}
}

// eachOfKind applies a fact to every model of one kind, recording the document
// it came from on each.
func (b *builder) eachOfKind(
	doc catalog.Document,
	kind catalog.Kind,
	apply func(*catalog.Model),
) {
	if kind == "" {
		return
	}
	for _, id := range b.order {
		m := b.models[id]
		if m.Kind != kind {
			continue
		}
		m.AddSource(doc.URL)
		apply(m)
	}
}

// mentioned returns the models a sentence names.
//
// The guides never write an identifier. They write the model the way a reader
// says it, and ElevenLabs builds its identifiers out of exactly those words, so
// a sentence names a model when it contains that model's identifier with the
// underscores spelled as spaces. The vendor's own prefix is dropped from some
// identifiers and not others, so "Multilingual v2" is looked for as well as
// "Eleven Multilingual v2", but only where dropping it leaves more than one
// word, since a bare "v3" would match any sentence mentioning any version of
// anything.
func (b *builder) mentioned(sentence string) []string {
	lower := strings.ToLower(sentence)
	var out []string
	for _, id := range b.order {
		phrase := strings.ReplaceAll(id, "_", " ")
		short := strings.TrimPrefix(phrase, "eleven ")
		if strings.Contains(lower, phrase) {
			out = append(out, id)
			continue
		}
		if strings.Contains(short, " ") && strings.Contains(lower, short) {
			out = append(out, id)
		}
	}
	return out
}
