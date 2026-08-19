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
	MusicURL = "https://elevenlabs.io/docs/api-reference/" +
		"music/compose.md"
	TextToSpeechURL = "https://elevenlabs.io/docs/api-reference/" +
		"text-to-speech/convert.md"
	SpeechToSpeechURL = "https://elevenlabs.io/docs/api-reference/" +
		"speech-to-speech/convert.md"
)

// endpointURLs are the references read for what they say about one model
// rather than about the endpoint.
var endpointURLs = []string{
	VoiceDesignURL,
	SoundEffectsURL,
	SpeechToTextURL,
	MusicURL,
	TextToSpeechURL,
	SpeechToSpeechURL,
}

// endpointKinds name the models of an endpoint whose model_id is a free string
// rather than an enumeration.
//
// Two endpoints identify their models by what a model can do instead of by
// listing them: the text to speech endpoint takes any model with support for
// text to speech and the voice changer endpoint any model with support for
// speech to speech, which is the same statement the identifiers make and the
// kind is read from. Both are therefore served by every model of one kind, and
// what the page states of the endpoint is stated of all of them.
//
// The speech to text endpoint is deliberately absent. It is the batch endpoint
// and the realtime model is served by a websocket instead, so its kind holds
// models it does not serve.
var endpointKinds = map[string]catalog.Kind{
	TextToSpeechURL:   KindSpeech,
	SpeechToSpeechURL: KindVoiceChanger,
}

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

// Enumerations an endpoint reference states for the models it names.
const (
	// ListOutputFormats holds the encodings a request may ask the audio back
	// in, which ElevenLabs writes as codec, sample rate and bitrate joined by
	// underscores.
	ListOutputFormats = "output_formats"
	// ListParameters holds the request parameters the endpoint accepts. It is
	// the catalog's name for them rather than one invented here.
	ListParameters = catalog.ListParameters
	// ListEndpoints holds the path a model is called at.
	ListEndpoints = "endpoints"
)

// outputFormatParam is the parameter whose enumeration is the list of
// encodings.
const outputFormatParam = "output_format"

var (
	paramRe    = regexp.MustCompile("^- `([a-z0-9_]+)`")
	restrictRe = regexp.MustCompile(`(?i)only (?:supported|available|applies)`)
	allowedRe  = regexp.MustCompile(`^- Allowed values:(.*)$`)
	textLenRe  = regexp.MustCompile(
		`(?i)text length has to be between [\d,]+ and ([\d,]+)`,
	)
	routeRe = regexp.MustCompile(
		`^(?:GET|POST|PUT|PATCH|DELETE) https?://[^/]+(/\S+)$`,
	)
	requestRe  = regexp.MustCompile(`(?i)^##\s+request\s*$`)
	responseRe = regexp.MustCompile(`(?i)^##\s+response\s*$`)
)

// applyEndpoint reads one API reference page.
//
// What a page says about the endpoint becomes a fact about a model only where
// the page enumerates the models its model_id accepts. Where it does, the route
// it is called at, the parameters it takes, the encodings it returns and the
// bound on the text a request may carry all belong to exactly those models. The
// bound is the same per-request character limit the models page states for
// speech synthesis. Where model_id is a free string instead, as it is for the
// text to speech and speech to text endpoints, the page names no model and
// nothing on it is recorded against one.
//
// A page whose model_id is a free string names its models by what they do
// instead, and where that is the same statement the identifiers make, the page
// is read onto every model of that kind.
//
// A parameter the page restricts to a named model is read whether or not
// model_id is enumerated, because the restriction names the model itself.
func (b *builder) applyEndpoint(doc catalog.Document) {
	e := readEndpoint(doc, b)
	apply := func(m *catalog.Model) {
		m.AddSource(doc.URL)
		m.SetLimit(LimitCharacterLimit, e.limit)
		m.AddList(ListEndpoints, e.route)
		m.AddList(ListParameters, e.params...)
		m.AddList(ListOutputFormats, e.formats...)
	}
	if len(e.models) == 0 {
		b.eachOfKind(doc, endpointKinds[doc.URL], apply)
		return
	}
	for _, id := range e.models {
		if m, ok := b.models[id]; ok {
			apply(m)
		}
	}
}

// endpoint is what one API reference page says about the models it names.
type endpoint struct {
	route   string
	models  []string
	params  []string
	formats []string
	limit   int64
}

// readEndpoint walks one API reference page.
//
// Only the request half is read. The response half enumerates the fields of a
// result, which describe what comes back rather than what may be asked for, and
// a parameter list holding both would be neither.
func readEndpoint(doc catalog.Document, b *builder) endpoint {
	var (
		e       endpoint
		param   string
		request bool
	)
	for _, raw := range strings.Split(string(doc.Body), "\n") {
		line := strings.TrimSpace(raw)
		if match := routeRe.FindStringSubmatch(line); match != nil &&
			e.route == "" {
			e.route = match[1]
		}
		switch {
		case requestRe.MatchString(line):
			request = true
			continue
		case responseRe.MatchString(line):
			request = false
			continue
		}
		if !request {
			continue
		}
		if match := paramRe.FindStringSubmatch(line); match != nil {
			param = match[1]
			e.params = append(e.params, param)
			if bound := textLenRe.FindStringSubmatch(line); bound != nil {
				e.limit = count(bound[1])
			}
			b.applyRestriction(doc, param, line)
			continue
		}
		match := allowedRe.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		switch param {
		case "model_id":
			e.models = append(e.models, codeValues(match[1])...)
		case outputFormatParam:
			e.formats = append(e.formats, codeValues(match[1])...)
		}
	}
	return e
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
