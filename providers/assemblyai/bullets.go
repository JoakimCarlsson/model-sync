package assemblyai

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Numeric keys a card's bullets state.
const (
	// LimitKeyterms is how many terms may be given to bias a transcription.
	// AssemblyAI counts them in words, which is what its cards say.
	LimitKeyterms = "max_keyterms"
	// LimitLanguageCount is how many languages a model covers where the card
	// counts them instead of naming them. Where it names them they go in
	// ListLanguages, which is a list of languages and not a count of them.
	LimitLanguageCount = "language_count"
)

// Capabilities a card's bullets name, in the catalog's words. AssemblyAI names
// none of them: each is a sentence with the capability inside it.
const (
	FeatureKeyterms          = catalog.CapabilityKeyterms
	FeatureRealtime          = catalog.CapabilityRealtime
	FeatureCodeSwitching     = catalog.CapabilityCodeSwitching
	FeatureDiarization       = catalog.CapabilityDiarization
	FeatureLanguageDetection = catalog.CapabilityLanguageDetection
	// FeatureEndpointing is deciding when a speaker has finished, which only
	// a model transcribing a live connection has to do.
	FeatureEndpointing = "endpointing"
	// FeaturePrompting is describing the audio in plain language to steer the
	// transcription. AssemblyAI documents it beside keyterms prompting and as a
	// different thing: one supplies the words to expect, the other the
	// situation to expect them in.
	FeaturePrompting = "prompting"
	// FeatureMedicalVocabulary is transcribing medical terminology accurately,
	// which is the whole of what the medical add-on states it does and what
	// every model it can be turned on for gains.
	FeatureMedicalVocabulary = "medical_vocabulary"
	// FeatureVoiceIsolation is transcribing the speaker rather than the room.
	FeatureVoiceIsolation = "voice_isolation"
	// FeaturePartialTranscripts is emitting a transcript before the speaker has
	// finished, which the streaming selection page answers per model.
	FeaturePartialTranscripts = "partial_transcripts"
)

var (
	// keytermsRe matches the bullet stating how many terms may be supplied.
	keytermsRe = regexp.MustCompile(
		`(?i)keyterms? prompting,?\s*(?:up to\s*)?([\d,]+)`,
	)
	// namedLanguagesRe matches a bullet that both counts the languages and
	// names them, "4 languages: en, es, de, fr".
	namedLanguagesRe = regexp.MustCompile(`(?i)(\d+)\s*languages:\s*(.+)$`)
	// languageCountRe matches a bullet that only counts them, however the
	// sentence is arranged: "Supports 18 languages", "Support across 99
	// languages", "18 languages with native code switching".
	languageCountRe = regexp.MustCompile(`(?i)\b(\d+)\+?\s*languages\b`)
	// englishOnlyRe matches the bullet naming the one language a model covers.
	englishOnlyRe = regexp.MustCompile(`(?i)^english transcription$`)
	// codeRe matches one language code of a named list.
	codeRe = regexp.MustCompile(`^[a-z]{2}(-[A-Za-z]{2,4})?$`)
)

// namedFeatures map a bullet naming a capability onto the catalog's word for
// it. The match is on the phrase appearing anywhere in the sentence, because
// AssemblyAI writes the same capability into several shapes of sentence.
var namedFeatures = []struct {
	re      *regexp.Regexp
	feature string
}{
	{regexp.MustCompile(`(?i)code switching`), FeatureCodeSwitching},
	{regexp.MustCompile(`(?i)real-?time transcription`), FeatureRealtime},
	{regexp.MustCompile(`(?i)intelligent endpointing`), FeatureEndpointing},
	{regexp.MustCompile(`(?i)prompting capabilities`), FeaturePrompting},
	{regexp.MustCompile(`(?i)medical terminology`), FeatureMedicalVocabulary},
}

// applyBullet records what one card bullet states.
//
// AssemblyAI's cards are sales copy with specification mixed into it. Some
// bullets carry a fact worth keeping and most do not, and none of them is a
// capability name, so nothing goes in as written: a bullet is read for the
// capability it names, the ceiling it states and the languages it lists, and a
// bullet holding none of those is dropped.
func applyBullet(m *catalog.Model, bullet string) {
	text := strings.TrimSpace(bullet)
	if match := keytermsRe.FindStringSubmatch(text); match != nil {
		m.AddList(ListFeatures, FeatureKeyterms)
		m.SetLimit(LimitKeyterms, count(match[1]))
	}
	switch match := namedLanguagesRe.FindStringSubmatch(text); {
	case match != nil:
		m.SetLimit(LimitLanguageCount, count(match[1]))
		m.AddList(ListLanguages, languageCodes(match[2])...)
	case englishOnlyRe.MatchString(text):
		m.AddList(ListLanguages, "en")
	default:
		if counted := languageCountRe.FindStringSubmatch(text); counted != nil {
			m.SetLimit(LimitLanguageCount, count(counted[1]))
		}
	}
	for _, named := range namedFeatures {
		if named.re.MatchString(text) {
			m.AddList(ListFeatures, named.feature)
		}
	}
}

// languageCodes reads the codes a bullet lists after its count, keeping only
// what is shaped like a code so that a sentence continuing past the list does
// not enter it as a language.
func languageCodes(list string) []string {
	var out []string
	for _, part := range strings.Split(list, ",") {
		code := strings.ToLower(strings.Trim(strings.TrimSpace(part), "."))
		if codeRe.MatchString(code) {
			out = append(out, code)
		}
	}
	return out
}

// count reads a figure a card writes with thousands separators in it.
func count(text string) int64 {
	n, err := strconv.ParseInt(strings.ReplaceAll(text, ",", ""), 10, 64)
	if err != nil {
		return 0
	}
	return n
}
