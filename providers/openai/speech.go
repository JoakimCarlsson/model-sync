package openai

import (
	"regexp"
	"slices"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Enumerations and bounds the speech guides state. A speech model's page lists
// two modalities and a snapshot and nothing else, so the voice it can use, the
// file it will return and the languages it will read are all read here.
const (
	ListVoices            = "voices"
	ListLanguages         = "languages"
	ListInputFormats      = "input_formats"
	LimitMaxFileMegabytes = "max_file_megabytes"
)

// TextToSpeechGuideURL is where the voices, the output formats and the
// languages of the speech models are written.
const TextToSpeechGuideURL = baseURL + "/api/docs/guides/text-to-speech.md"

// Headings the text to speech guide writes its facts under.
const (
	speechModelsHeading  = "### text-to-speech models"
	voiceOptionsHeading  = "### voice options"
	speechFormatsHeading = "## supported output formats"
	speechLanguageHead   = "## supported languages"
)

// languageListCommas is the number of commas a line must carry to be the list
// of languages rather than a sentence about it. The list runs to more than
// fifty names and every other line in the section is prose.
const languageListCommas = 20

var (
	// voiceBulletRe matches one voice of the built-in set, which the guide
	// lists a bullet at a time.
	voiceBulletRe = regexp.MustCompile("^- `([a-z]+)`$")
	// smallerVoiceSetRe matches the sentence saying which models take fewer
	// voices than the rest, written as "The `tts-1` and `tts-1-hd` models
	// support a smaller set: `alloy`, ... and `shimmer`."
	smallerVoiceSetRe = regexp.MustCompile(
		`The ([^\n]+?) models support a smaller set: ([^.\n]+)\.`,
	)
	// speechFormatRe matches one bullet of the output format list, which names
	// the format in bold and describes it after a colon.
	speechFormatRe = regexp.MustCompile(`^- \*\*([A-Za-z0-9]+)\*\*:`)
	// audioFileSizeRe and audioFormatsRe match the two bounds the speech to
	// text guide opens with, which are stated for the transcription endpoint
	// and never on a model page.
	audioFileSizeRe = regexp.MustCompile(
		`(?i)files can be up to (\d+)\s*MB`,
	)
	audioFormatsRe = regexp.MustCompile(
		`(?i)supported input formats are ([^.]+)\.`,
	)
)

// transcriptionEndpoint is the route the speech to text guide's file bounds
// apply to. The guide states them for the Transcriptions API rather than for a
// model, so the models they hold for are the ones whose pages list that route.
const transcriptionEndpoint = "v1/audio/transcriptions"

// audioFileBounds are the transcription bounds read from the guide, held until
// the model pages have said which models serve the endpoint they apply to.
type audioFileBounds struct {
	Source    string
	Megabytes int64
	Formats   []string
}

// applyTextToSpeechGuide reads what a speech model will say and in what.
//
// The guide names the three speech models in a section of its own and then
// gives the voices twice: thirteen as a bulleted list for the endpoint, and a
// smaller set of nine in a sentence naming the two older models it holds for.
// Both are read, and a model named in that sentence takes the smaller set
// instead of the full one rather than as well as it. The output formats and
// the languages are stated once each, for the endpoint, and are recorded
// against every model the guide named.
func (b *builder) applyTextToSpeechGuide(doc catalog.Document) {
	body := string(doc.Body)
	models := quotedTokens(sectionAfterPrefix(body, speechModelsHeading))
	if len(models) == 0 {
		return
	}
	voices := speechVoices(body)
	smaller, smallerVoices := smallerVoiceSet(body)
	formats := speechFormats(body)
	languages := languageList(body)
	for _, id := range models {
		m := b.models[id]
		if m == nil {
			continue
		}
		m.AddSource(doc.URL)
		if slices.Contains(smaller, id) {
			m.AddList(ListVoices, smallerVoices...)
		} else {
			m.AddList(ListVoices, voices...)
		}
		m.AddList(ListOutputFormats, formats...)
		m.AddList(ListLanguages, languages...)
	}
}

// speechVoices returns the full built-in voice set.
func speechVoices(body string) []string {
	var out []string
	for _, line := range strings.Split(
		sectionAfterPrefix(body, voiceOptionsHeading),
		"\n",
	) {
		if match := voiceBulletRe.FindStringSubmatch(
			strings.TrimSpace(line),
		); match != nil {
			out = append(out, match[1])
		}
	}
	return out
}

// smallerVoiceSet returns the models restricted to fewer voices and the voices
// they take.
func smallerVoiceSet(body string) ([]string, []string) {
	match := smallerVoiceSetRe.FindStringSubmatch(body)
	if match == nil {
		return nil, nil
	}
	return quotedTokens(match[1]), quotedTokens(match[2])
}

// speechFormats returns the file formats the speech endpoint returns.
func speechFormats(body string) []string {
	var out []string
	for _, line := range strings.Split(
		sectionAfterPrefix(body, speechFormatsHeading),
		"\n",
	) {
		if match := speechFormatRe.FindStringSubmatch(
			strings.TrimSpace(line),
		); match != nil {
			out = append(out, strings.ToLower(match[1]))
		}
	}
	return out
}

// languageList returns the languages the guide says speech can be generated
// in, which it writes as one long comma separated line of prose.
func languageList(body string) []string {
	for _, line := range strings.Split(
		sectionAfterPrefix(body, speechLanguageHead),
		"\n",
	) {
		if strings.Count(line, ",") < languageListCommas ||
			strings.Contains(line, "](") {
			continue
		}
		var out []string
		for _, part := range strings.Split(strings.TrimSuffix(line, "."), ",") {
			name := strings.TrimSpace(strings.TrimPrefix(
				strings.TrimSpace(part),
				"and ",
			))
			if name != "" {
				out = append(out, name)
			}
		}
		return out
	}
	return nil
}

// readAudioFileBounds records the file bounds the speech to text guide states,
// which cannot be applied yet: they hold for the models serving the
// transcription route, and which models those are is on the pages read after
// the guides.
func (b *builder) readAudioFileBounds(doc catalog.Document) {
	size := audioFileSizeRe.FindStringSubmatch(string(doc.Body))
	formats := audioFormatsRe.FindStringSubmatch(string(doc.Body))
	if size == nil || formats == nil {
		return
	}
	b.audio = &audioFileBounds{
		Source:    doc.URL,
		Megabytes: parseCount(size[1]),
		Formats:   quotedTokens(formats[1]),
	}
}

// applyAudioFileBounds records the guide's file bounds against every model
// whose page lists the transcription route they apply to.
func (b *builder) applyAudioFileBounds() {
	if b.audio == nil {
		return
	}
	for _, id := range b.order {
		m := b.models[id]
		if !slices.Contains(m.Lists[ListEndpoints], transcriptionEndpoint) {
			continue
		}
		m.AddSource(b.audio.Source)
		m.SetLimit(LimitMaxFileMegabytes, b.audio.Megabytes)
		m.AddList(ListInputFormats, b.audio.Formats...)
	}
}
