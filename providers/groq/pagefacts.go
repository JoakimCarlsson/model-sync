package groq

import (
	"html"
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Headings a model page states its bounds and its speed under, written in the
// same capitals as the headings naming what the model takes.
const (
	headTokenSpeed     = "TOKEN SPEED"
	headContextWindow  = "CONTEXT WINDOW"
	headMaxOutput      = "MAX OUTPUT TOKENS"
	headMaxFileSize    = "MAX FILE SIZE"
	headMaxInputImages = "MAX INPUT IMAGES"
)

// quantization is the one Groq applies to every model it serves, named in a
// section every model page carries.
const quantization = "TruePoint Numerics"

var (
	// pageSummaryRe matches the description in a page's front matter, which is
	// the one sentence Groq writes about the model rather than the paragraph
	// it also writes.
	pageSummaryRe = regexp.MustCompile(`(?m)^description:\s*(.+?)\s*$`)
	// pageTitleRe matches the name the page is headed with.
	pageTitleRe = regexp.MustCompile(`(?m)^#\s+(.+?)\s*$`)
	// pageAuthorRe matches the line naming who made the model, which is the
	// developer's logo and its name run together.
	pageAuthorRe = regexp.MustCompile(
		`(?m)^!\[[^\]]*logo\]\([^)]*\)([^\n\[\]()]+)$`,
	)
	// huggingFaceRe matches a model card published as a Hugging Face
	// repository, whose path is the identifier the weights are served under.
	huggingFaceRe = regexp.MustCompile(
		`^https://huggingface\.co/([^/]+/[^/?#]+)`,
	)
	// openWeightsRe matches the wording a page uses where it says the weights
	// are published.
	openWeightsRe = regexp.MustCompile(
		`(?i)open[- ]weight|openly available|open[- ]source`,
	)
	// detailRe matches one of the bullets a page closes its specifications
	// with, which are a label in bold and a value.
	detailRe = regexp.MustCompile(`(?m)^\*\s+\*\*([^*]+)\*\*:\s*(.+?)\s*$`)
)

// addPageFacts records everything a model page states beside its rates, its
// capabilities and the modalities it works in.
//
// The page names the developer, states the model card, describes the model in
// one sentence in its front matter, and repeats under headings of its own the
// two bounds the table also carries, together with two the table has no column
// for: how many images an input may hold and how fast the model answers.
func addPageFacts(m *catalog.Model, body string, sections map[string]string) {
	if m.Name == "" {
		m.Name = clean(firstOf(pageTitleRe, body))
	}
	m.SetAttr(AttrSummary, html.UnescapeString(firstOf(pageSummaryRe, body)))
	m.SetAttr(AttrAuthor, strings.TrimSpace(firstOf(pageAuthorRe, body)))
	card := firstOf(pageCardRe, body)
	m.SetAttr(AttrModelCard, card)
	m.SetAttr(AttrHuggingFaceID, firstOf(huggingFaceRe, card))
	if openWeightsRe.MatchString(body) || huggingFaceRe.MatchString(card) {
		m.SetAttr(AttrOpenWeights, "true")
	}
	if strings.Contains(body, quantization) {
		m.SetAttr(AttrQuantization, quantization)
	}
	m.SetAttr(AttrTokensPerSec, firstNumber(sections[headTokenSpeed]))
	m.SetAttr(AttrMaxFileSize, valueOrEmpty(sections[headMaxFileSize]))
	m.SetLimit(LimitContextWindow, parseCount(sections[headContextWindow]))
	m.SetLimit(LimitMaxOutputTokens, parseCount(sections[headMaxOutput]))
	m.SetLimit(LimitMaxInputImages, parseCount(sections[headMaxInputImages]))
	addPageDetails(m, body)
}

// addPageDetails reads the bullets a page states its size and the files it
// accepts in. Only the labels naming a fact about the model are read; the rest
// point at the guide that explains it.
func addPageDetails(m *catalog.Model, body string) {
	for _, match := range detailRe.FindAllStringSubmatch(body, -1) {
		value := clean(match[2])
		switch strings.ToLower(strings.TrimSpace(match[1])) {
		case "model size":
			m.SetAttr(AttrModelSize, value)
		case "language":
			m.SetAttr(AttrLanguages, value)
		case "supported audio":
			m.AddList(ListAudioFormats, splitFormats(value)...)
		}
	}
}

// splitFormats divides a list of file formats, which Groq writes as a comma
// separated list with the last item introduced by "or".
func splitFormats(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		item := strings.ToLower(strings.TrimSpace(part))
		item = strings.TrimSpace(strings.TrimPrefix(item, "or "))
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

// kindForModalities reports what a model is from what it takes and returns,
// which is the plainest thing a page states: a model that hears and writes is
// a transcriber, one that reads and speaks is a speech model, and one that
// does both converses.
func kindForModalities(m *catalog.Model) catalog.Kind {
	hears := hasModality(m, ListInputModalities, modalityAudio)
	speaks := hasModality(m, ListOutputModalities, modalityAudio)
	switch {
	case hears && speaks:
		return KindAudio
	case hears:
		return KindTranscription
	case speaks:
		return KindSpeech
	}
	return KindChat
}

// hasModality reports whether a model names one medium in one of its lists.
func hasModality(m *catalog.Model, key, medium string) bool {
	for _, value := range m.Lists[key] {
		if value == medium {
			return true
		}
	}
	return false
}
