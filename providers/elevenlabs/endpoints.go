package elevenlabs

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// The endpoint references. They are the only documents saying which model an
// ability belongs to: the models page describes a family and the pricing page
// prices one, and neither distinguishes the two models an endpoint accepts.
const (
	VoiceDesignURL = "https://elevenlabs.io/docs/api-reference/" +
		"text-to-voice/design.md"
	SoundEffectsURL = "https://elevenlabs.io/docs/api-reference/" +
		"text-to-sound-effects/convert.md"
	SpeechToTextURL = "https://elevenlabs.io/docs/api-reference/" +
		"speech-to-text/convert.md"
)

// endpointURLs are the references read for what they say about one model
// rather than about the endpoint.
var endpointURLs = []string{VoiceDesignURL, SoundEffectsURL, SpeechToTextURL}

// Capabilities an endpoint reference states.
const (
	// FeatureAudioReference is set where a voice may be designed from a
	// recording as well as from a description of one.
	FeatureAudioReference = "audio_reference"
	// FeatureLooping is set where the generated audio is made to repeat with
	// no audible seam.
	FeatureLooping = "seamless_looping"
	// FeatureDisfluencyRemoval is set where the transcript can be asked to
	// leave out filler words, false starts and non-speech sounds.
	FeatureDisfluencyRemoval = "disfluency_removal"
)

// paramFeatures names the capability a request parameter states.
//
// Only a parameter ElevenLabs restricts to one model is here, because that
// restriction is the whole of what makes it a fact about a model: a parameter
// every model of an endpoint accepts describes the endpoint, and a capability
// list is not a list of what an API takes.
var paramFeatures = map[string]string{
	"reference_audio_base64": FeatureAudioReference,
	"prompt_strength":        FeatureAudioReference,
	"loop":                   FeatureLooping,
	"no_verbatim":            FeatureDisfluencyRemoval,
}

var (
	paramRe    = regexp.MustCompile("^- `([a-z0-9_]+)`")
	restrictRe = regexp.MustCompile(`(?i)only (?:supported|available|applies)`)
	allowedRe  = regexp.MustCompile(`^- Allowed values:(.*)$`)
	textLenRe  = regexp.MustCompile(
		`(?i)text length has to be between [\d,]+ and ([\d,]+)`,
	)
)

// applyEndpoint reads one API reference page.
//
// Two things on it are facts about a model. A parameter the page restricts to a
// named model states what that model can do and the others cannot, and is
// recorded as the capability it names. The bound on the text a request may
// carry belongs to every model the endpoint's model_id accepts, which the page
// enumerates, and is the same per-request character limit the models page
// states for speech synthesis.
func (b *builder) applyEndpoint(doc catalog.Document) {
	var (
		param  string
		models []string
		limit  int64
	)
	for _, raw := range strings.Split(string(doc.Body), "\n") {
		line := strings.TrimSpace(raw)
		if match := paramRe.FindStringSubmatch(line); match != nil {
			param = match[1]
			if bound := textLenRe.FindStringSubmatch(line); bound != nil {
				limit = count(bound[1])
			}
			b.applyRestriction(doc, param, line)
			continue
		}
		if param != "model_id" {
			continue
		}
		if match := allowedRe.FindStringSubmatch(line); match != nil {
			models = append(models, codeValues(match[1])...)
		}
	}
	if limit == 0 {
		return
	}
	for _, id := range models {
		m, ok := b.models[id]
		if !ok {
			continue
		}
		m.AddSource(doc.URL)
		m.SetLimit(LimitCharacterLimit, limit)
	}
}

// applyRestriction records a parameter one model of an endpoint accepts and the
// others do not. A restriction naming an identifier no document listed is
// dropped, since there is nothing to record it against.
func (b *builder) applyRestriction(doc catalog.Document, param, line string) {
	feature := paramFeatures[param]
	if feature == "" || !restrictRe.MatchString(line) {
		return
	}
	id, ok := b.longestID(line)
	if !ok {
		return
	}
	m := b.models[id]
	m.AddSource(doc.URL)
	m.AddList(ListFeatures, feature)
}

// longestID returns the model a sentence names, which is the one whose
// identifier is the longest of those the sentence contains. The longest wins
// because ElevenLabs' identifiers nest: a sentence naming eleven_flash_v2_5
// contains eleven_flash_v2 as well.
func (b *builder) longestID(line string) (string, bool) {
	var (
		best  string
		found bool
	)
	for id := range b.models {
		if !strings.Contains(line, id) || len(id) <= len(best) {
			continue
		}
		best, found = id, true
	}
	return best, found
}

// codeValues returns the backticked tokens of an enumeration.
func codeValues(list string) []string {
	var out []string
	for _, match := range codeRe.FindAllStringSubmatch(list, -1) {
		out = append(out, strings.TrimSpace(match[1]))
	}
	return out
}
