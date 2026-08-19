package assemblyai

import (
	"regexp"
	"slices"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// featureURLs are the pages for what AssemblyAI does to a transcript once it
// has one. Each is one thing sold by the hour and each page answers the same
// three questions in the same shape, so they are read by one parser: which
// speech models it may be turned on for, which languages it covers, and which
// regions serve it.
var featureURLs = []string{
	"https://www.assemblyai.com/docs/speech-understanding/action-items.md",
	"https://www.assemblyai.com/docs/speech-understanding/auto-chapters.md",
	"https://www.assemblyai.com/docs/speech-understanding/" +
		"custom-formatting.md",
	"https://www.assemblyai.com/docs/speech-understanding/" +
		"entity-detection.md",
	"https://www.assemblyai.com/docs/speech-understanding/key-phrases.md",
	"https://www.assemblyai.com/docs/speech-understanding/" +
		"sentiment-analysis.md",
	"https://www.assemblyai.com/docs/speech-understanding/" +
		"speaker-identification.md",
	"https://www.assemblyai.com/docs/speech-understanding/summarization.md",
	"https://www.assemblyai.com/docs/speech-understanding/topic-detection.md",
	"https://www.assemblyai.com/docs/speech-understanding/translation.md",
	"https://www.assemblyai.com/docs/guardrails/" +
		"redact-pii-from-transcripts.md",
	"https://www.assemblyai.com/docs/guardrails/detect-sensitive-content.md",
	"https://www.assemblyai.com/docs/guardrails/" +
		"filter-profanity-from-transcripts.md",
	"https://www.assemblyai.com/docs/guardrails/" +
		"set-minimum-speech-threshold.md",
}

// The accordions a feature page answers in. Their titles are the questions,
// and the answer is a table of a name and the code a request uses, except for
// regions, which is a sentence.
const (
	accordionModels    = "supported models"
	accordionLanguages = "supported languages"
	accordionRegions   = "supported regions"
)

// ListSupportedModels holds the speech models a feature may be turned on for,
// by the identifier a request names them with. It is the feature's side of the
// same statement the model records as a capability.
const ListSupportedModels = "supported_models"

// AttrRegions is where a feature is served, which only these pages state.
const AttrRegions = "regions"

// featureCapabilities translate the name of a page into the catalog's word for
// what that page describes. Only one of these has a canonical value to
// translate to; the rest keep AssemblyAI's own name, lowered and joined,
// because no other provider in this catalog states them.
var featureCapabilities = map[string]string{
	"entity-detection": catalog.CapabilityEntityDetection,
}

var (
	// accordionRe matches one accordion of a feature page.
	accordionRe = regexp.MustCompile(
		`(?is)<Accordion\s+title="([^"]*)"[^>]*>(.*?)</Accordion>`,
	)
	// titleRe matches the heading a feature page opens with, which is the
	// only place the feature's own name is stated.
	titleRe = regexp.MustCompile(`(?m)^#\s+(.+?)\s*$`)
)

// applyFeature reads one page describing something done to a transcript.
//
// AssemblyAI heads these on its pricing page with the word "Models" and sells
// each by the hour, and its documentation calls them models too, so each is an
// entry rather than a footnote on the transcription models. What it is not is
// a model you can ask for on its own: it runs on a transcript some speech
// model produced, and the page names which ones. That statement is recorded
// twice, once as the feature's list of the models it runs on and once as a
// capability on each of those models, because a consumer asking "what can this
// model do" and a consumer asking "where can I use this" are both asking about
// the same sentence.
func (b *builder) applyFeature(doc catalog.Document) {
	body := string(doc.Body)
	title := titleRe.FindStringSubmatch(body)
	if title == nil {
		return
	}
	name := clean(title[1])
	m := b.model(slugID(name), featureKind(doc.URL))
	m.AddSource(doc.URL)
	if m.Name == "" {
		m.Name = name
	}
	m.AddList(ListFeatures, featureOf(m.ID))
	for _, accordion := range accordionRe.FindAllStringSubmatch(body, -1) {
		b.applyAccordion(m, accordion[1], accordion[2], doc.URL)
	}
}

// applyAccordion records one answer of a feature page.
func (b *builder) applyAccordion(
	m *catalog.Model,
	title, body, source string,
) {
	codes := codesOf(body)
	switch strings.ToLower(clean(title)) {
	case accordionModels:
		m.AddList(ListSupportedModels, codes...)
		b.applyFeatureModels(m, codes, source)
	case accordionLanguages:
		m.AddList(ListLanguages, codes...)
	case accordionRegions:
		m.SetAttr(AttrRegions, clean(body))
	}
}

// applyFeatureModels records the feature as a capability of every speech model
// the page names.
//
// The match is on the identifier and narrowed to the pre-recorded models,
// because these pages work on a finished transcript and name the identifiers
// the pre-recorded API takes. Two models answer to "universal-3-5-pro", one
// pre-recorded and one streaming, and matching the identifier alone would
// credit a streaming model with a capability its page does not mention.
func (b *builder) applyFeatureModels(
	m *catalog.Model,
	codes []string,
	source string,
) {
	feature := featureOf(m.ID)
	for _, id := range b.order {
		target := b.models[id]
		if target.Attrs[AttrMode] != ModePrerecorded {
			continue
		}
		if !slices.Contains(codes, target.Attrs[AttrAPIIdentifier]) {
			continue
		}
		target.AddSource(source)
		target.AddList(ListFeatures, feature)
	}
}

// featureKind classifies a feature by the section of the documentation it is
// filed under, which is where AssemblyAI separates understanding a transcript
// from policing one.
func featureKind(url string) catalog.Kind {
	if strings.Contains(url, "/guardrails/") {
		return KindGuardrail
	}
	return KindSpeechUnderstanding
}

// featureOf is the catalog's word for what a feature page describes, which is
// the page's own name unless the catalog already has a word for it.
func featureOf(id string) string {
	if canonical, ok := featureCapabilities[id]; ok {
		return canonical
	}
	return strings.ReplaceAll(id, "-", "_")
}

// codesOf reads the codes out of an accordion's table, which pairs a display
// name with the value a request carries.
func codesOf(body string) []string {
	var out []string
	for _, match := range languageCodeRe.FindAllStringSubmatch(body, -1) {
		out = append(out, strings.ToLower(match[1]))
	}
	return out
}
