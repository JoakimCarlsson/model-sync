package cohere

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Pages describing one transcription model each. The overview's audio table
// carries no modality column and lists only one of the two models, so what a
// transcription model takes and returns, and that the second one exists at
// all, is stated here and nowhere else.
const (
	TranscribeURL       = "https://docs.cohere.com/docs/transcribe.md"
	TranscribeArabicURL = "https://docs.cohere.com/docs/transcribe-arabic.md"
)

// familyAudio is the family the overview files the transcription models under.
const familyAudio = "audio"

var (
	// detailRe matches one fact of the model details list, which is where the
	// page states the identifier, the two modalities and the file ceiling.
	detailRe = regexp.MustCompile(`(?m)^\*\s*\*\*([^*]+)\*\*:\s*(.+?)\s*$`)
	// pageTitleRe matches the page's heading, which names the model.
	pageTitleRe = regexp.MustCompile(`(?m)^#\s+(.+?)\s*$`)
	// aboutRe matches the paragraph opening the page, which describes the
	// model rather than the page.
	aboutRe = regexp.MustCompile(`(?s)## About[^\n]*\n+(.+?)\n\n`)
	// endpointRe matches the link a transcription page closes with, which is
	// the only place the endpoint that calls the model is named. The overview's
	// audio table has an endpoint column and lists one of the two models, so
	// without this the Arabic model would answer nowhere.
	endpointRe = regexp.MustCompile(
		`\[(Audio Transcriptions) API reference documentation\]`,
	)
)

// Facts a model details list states, as the page labels them.
const (
	detailID        = "model name"
	detailInput     = "input"
	detailOutput    = "output"
	detailFileSize  = "maximum file size"
	detailSwitching = "code-switching support"
)

// detailYes is how a details list answers a question of capability.
const detailYes = "yes"

// applyTranscribe reads one transcription model's page.
//
// The page states the modalities outright, "Input: Audio waveform" and
// "Output: Text", which is the one place Cohere writes what a model returns
// rather than leaving it to be inferred from the family it belongs to.
//
// It also states a capability the way a capability has to be stated to be
// worth recording, as a question answered against the model: the Arabic model
// says it follows a speaker who changes language mid-utterance, and the model
// it is finetuned from does not claim to.
func (b *builder) applyTranscribe(doc catalog.Document) {
	body := string(doc.Body)
	details := modelDetails(body)
	id := details[detailID]
	if id == "" {
		return
	}
	m := b.model(id, KindTranscription)
	m.AddSource(doc.URL)
	m.SetAttr(AttrFamily, familyAudio)
	if title := pageTitleRe.FindStringSubmatch(body); title != nil &&
		m.Name == "" {
		m.Name = clean(title[1])
	}
	if about := aboutRe.FindStringSubmatch(body); about != nil {
		m.SetAttr(AttrSummary, firstSentence(clean(about[1])))
	}
	for _, item := range splitList(details[detailInput]) {
		m.AddList(ListInputModalities, modalityName(item))
	}
	for _, item := range splitList(details[detailOutput]) {
		m.AddList(ListOutputModalities, modalityName(item))
	}
	if named := endpointRe.FindStringSubmatch(body); named != nil {
		m.AddList(ListEndpoints, named[1])
	}
	m.SetAttr(AttrMaxFileSize, details[detailFileSize])
	m.SetAttr(AttrLicense, details[detailLicense])
	m.AddList(ListLanguages, languageList(details[detailLanguages])...)
	if strings.EqualFold(details[detailSwitching], detailYes) {
		m.AddList(ListFeatures, catalog.CapabilityCodeSwitching)
	}
}

// modelDetails reads the labelled facts a page states about the model it is
// about, as a run of bullets each opening with the name of the fact.
//
// A bullet is read to its end rather than to the end of its first line. Cohere
// wraps a long one, and the longest of them is the list of languages a model
// covers, so a bullet read a line at a time would state a model covers the
// first eight of its fourteen languages.
func modelDetails(body string) map[string]string {
	out := map[string]string{}
	label, value := "", ""
	keep := func() {
		if label == "" {
			return
		}
		if _, ok := out[label]; !ok {
			out[label] = clean(value)
		}
		label, value = "", ""
	}
	for _, line := range strings.Split(body, "\n") {
		if match := detailRe.FindStringSubmatch(line); match != nil {
			keep()
			label, value = strings.ToLower(clean(match[1])), match[2]
			continue
		}
		if label != "" && continues(line) {
			value += " " + line
			continue
		}
		keep()
	}
	keep()
	return out
}

// continues reports whether a line carries on the bullet above it, which
// Cohere writes indented and never empty.
func continues(line string) bool {
	return strings.HasPrefix(line, " ") && strings.TrimSpace(line) != ""
}
