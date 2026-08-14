package openai

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// FeatureTranslation is set where the model renders speech in another language
// as English text. OpenAI offers it on one transcription model and states it as
// a row of the guide's capability table rather than as a feature.
const FeatureTranslation = "translation"

// promptingSection heads the prose saying which models take a prompt.
const promptingSection = "## Prompting"

// quotedRe matches an identifier in the backticks OpenAI writes one in.
var quotedRe = regexp.MustCompile("`([^`]+)`")

// transcriptionCapabilities map a row of the transcription guide's capability
// table onto the capability the model named beside it therefore has. The table
// is keyed by what a caller wants rather than by what a model does, so the
// wording has to be translated; the model pages list at most "streaming", which
// is why this is the only place these capabilities can be read.
var transcriptionCapabilities = []struct {
	need    string
	feature string
}{
	{"speaker-labeled transcripts", catalog.CapabilityDiarization},
	{"word timestamps", catalog.CapabilityWordTimestamps},
	{"translation of a completed recording", FeatureTranslation},
	{"detected input languages", catalog.CapabilityLanguageDetection},
	{"committed-turn transcription", catalog.CapabilityRealtime},
}

// denials are the wordings the prompting section uses to say a model takes no
// prompt, so that the sentence saying so is not read as saying the opposite.
var denials = []string{"doesn't", "does not", "don't", "do not"}

// applyTranscriptionGuide reads the table saying which model to reach for when
// the recommended one does not do what is wanted.
func (b *builder) applyTranscriptionGuide(doc catalog.Document) {
	for _, t := range scanMarkdownTables(doc) {
		needCol := columnOf(t.Headers, []string{"if you need"})
		useCol := columnOf(t.Headers, []string{"use"})
		if needCol < 0 || useCol < 0 {
			continue
		}
		for _, row := range t.Rows {
			b.addTranscriptionCapability(
				t.Source,
				cellAt(row, needCol),
				cellAt(row, useCol),
			)
		}
	}
}

// addTranscriptionCapability records the capability one row states, against the
// model the Use cell names first.
func (b *builder) addTranscriptionCapability(source, need, use string) {
	feature := featureForNeed(need)
	match := quotedRe.FindStringSubmatch(use)
	if feature == "" || match == nil {
		return
	}
	m := b.model(match[1], KindTranscription)
	m.AddSource(source)
	m.AddList(ListFeatures, feature)
}

// featureForNeed reads the capability out of the need a row opens with.
func featureForNeed(need string) string {
	text := strings.ToLower(strings.TrimSpace(need))
	for _, entry := range transcriptionCapabilities {
		if strings.HasPrefix(text, entry.need) {
			return entry.feature
		}
	}
	return ""
}

// applySpeechToTextGuide reads which models accept a prompt biasing the
// transcript towards terms the caller supplies.
//
// OpenAI states this as prose and never as a feature on a model page, one
// sentence per group of models, so the section is read a sentence at a time and
// the sentence naming the model that takes no prompt is skipped rather than
// read as naming one that does. Only a model some earlier document already
// established is recorded against, because the same backticks hold parameter
// names as hold identifiers.
func (b *builder) applySpeechToTextGuide(doc catalog.Document) {
	section := sectionAfter(string(doc.Body), promptingSection)
	for _, sentence := range strings.Split(section, ".") {
		lower := strings.ToLower(sentence)
		if !strings.Contains(lower, "prompt") || denied(lower) {
			continue
		}
		for _, match := range quotedRe.FindAllStringSubmatch(sentence, -1) {
			m, ok := b.models[match[1]]
			if !ok {
				continue
			}
			m.AddSource(doc.URL)
			m.AddList(ListFeatures, catalog.CapabilityKeyterms)
		}
	}
}

// denied reports whether a sentence denies rather than states support.
func denied(sentence string) bool {
	for _, phrase := range denials {
		if strings.Contains(sentence, phrase) {
			return true
		}
	}
	return false
}

// sectionAfter returns the body of one section, from its heading to the next
// heading of any level.
func sectionAfter(body, heading string) string {
	lines := strings.Split(body, "\n")
	var out []string
	inside := false
	for _, line := range lines {
		if strings.TrimSpace(line) == heading {
			inside = true
			continue
		}
		if !inside {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			break
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
