package perplexity

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Guides stating what the Sonar API can do. Neither names a model: they
// describe the API, which serves exactly the models the Sonar index lists, so
// what they state is recorded against each of those.
const (
	FeaturesURL = baseURL + "/docs/sonar/features.md"
	MediaURL    = baseURL + "/docs/sonar/media.md"
)

// Enumeration keys the Sonar documentation populates.
const (
	ListFeatures         = catalog.ListFeatures
	ListInputModalities  = "input_modalities"
	ListOutputModalities = "output_modalities"
)

// ModalityText is what every Sonar model takes and returns whatever else it
// handles, since the API is a chat API and the media guide states only what may
// accompany the text.
const ModalityText = "text"

// guideFeatures map a heading of the core features guide onto the capability it
// documents.
var guideFeatures = map[string]string{
	"streaming responses": "streaming",
	"structured outputs":  "structured_outputs",
}

// guideModalities map a heading of the media guide onto the modality it
// documents. Perplexity heads each section with the verb, which is what tells
// the two directions apart.
//
// Only what a model accepts is read. The guide's other two sections, Receiving
// Images and Receiving Videos, are about media the search found and the
// response links to, which the model did not produce: recording them as output
// modalities would say Sonar generates images, and it does not.
var guideModalities = map[string]struct{ in, out string }{
	"sending images": {in: "image"},
	"sending files":  {in: "file"},
}

// featureReasoning is the one capability a Sonar model page states for itself
// rather than for the API.
const featureReasoning = "reasoning"

var (
	// headingRe matches a section heading of a guide.
	headingRe = regexp.MustCompile(`(?m)^##\s+(.+?)\s*$`)
	// cardRe matches a heading of one card on a model page, which is where the
	// page states what the model is and what it holds.
	cardRe = regexp.MustCompile(`(?is)<h3[^>]*>(.*?)</h3>`)
	// mdxRe matches the markup a heading carries.
	mdxRe = regexp.MustCompile(`(?s)<[^>]*>`)
)

// applyGuide reads one of the Sonar guides onto every model the Sonar index
// listed. The guides state a capability of the API and never of a model, and
// the API serves Perplexity's own models only: the models it brokers answer on
// the Agent API, whose own documentation says outright that not all of them
// support all of its features.
func (b *builder) applyGuide(doc catalog.Document) {
	for _, match := range headingRe.FindAllStringSubmatch(
		string(doc.Body),
		-1,
	) {
		heading := strings.ToLower(strings.TrimSpace(match[1]))
		feature, isFeature := guideFeatures[heading]
		media, isMedia := guideModalities[heading]
		if !isFeature && !isMedia {
			continue
		}
		for _, id := range b.sonar {
			m := b.models[id]
			m.AddSource(doc.URL)
			m.AddList(ListFeatures, feature)
			m.AddList(ListInputModalities, media.in)
			m.AddList(ListOutputModalities, media.out)
		}
	}
}

// applyBaseModalities records the modality every Sonar model handles, which the
// media guide leaves implicit by documenting only what may accompany the text.
func (b *builder) applyBaseModalities(source string) {
	for _, id := range b.sonar {
		m := b.models[id]
		m.AddSource(source)
		m.AddList(ListInputModalities, ModalityText)
		m.AddList(ListOutputModalities, ModalityText)
	}
}

// readsReasoning reports whether a model page's cards say the model reasons.
// The pages state it either way, as "Advanced reasoning model" or as
// "Non-reasoning model", so the negative has to be told from the positive.
func readsReasoning(body string) bool {
	for _, match := range cardRe.FindAllStringSubmatch(body, -1) {
		card := strings.ToLower(mdxRe.ReplaceAllString(match[1], " "))
		if !strings.Contains(card, featureReasoning) {
			continue
		}
		if strings.Contains(card, "non-"+featureReasoning) {
			continue
		}
		return true
	}
	return false
}
