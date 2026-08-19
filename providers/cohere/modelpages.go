package cohere

import (
	"regexp"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// modelPages are the pages Cohere gives one Command model each, keyed by the
// product each page is about.
//
// A page is attributed by the product it is headed by rather than by its
// address, because an address is a slug and a slug outlives the model behind
// it the same way a product name does. The two older pages, Command R and
// Command R+, are deliberately absent: each describes a family whose members
// the overview lists separately, one of them an alias for a version the page
// does not claim to describe, and there is no reading of "Command R+" those
// two pages settle.
var modelPages = map[string]string{
	"https://docs.cohere.com/docs/command-a-plus.md":      "Command A+",
	"https://docs.cohere.com/docs/command-a.md":           "Command A",
	"https://docs.cohere.com/docs/command-a-reasoning.md": "Command A Reasoning",
	"https://docs.cohere.com/docs/command-a-vision.md":    "Command A Vision",
	"https://docs.cohere.com/docs/command-a-translate.md": "Command A Translate",
	"https://docs.cohere.com/docs/command-r7b.md":         "Command R7B",
}

// Keys a model page populates that no other document states.
const (
	AttrReleaseDate   = "release_date"
	AttrAuthor        = "author"
	AttrLicense       = "license"
	AttrOpenWeights   = "open_weights"
	AttrHuggingFaceID = "hugging_face_id"
	AttrModelSize     = "model_size"
)

// openWeights is what an identifier published under a licence anyone may run
// under is marked with.
const openWeights = "true"

// Facts a model page's properties list states, as the page labels them.
const (
	detailReleaseDate = "release date"
	detailProvider    = "name of the model provider"
	detailModelSize   = "model size"
	detailLicense     = "license"
	detailLanguages   = "languages covered"
)

var (
	// thinkingRe matches the sentence a page states the reasoning feature in.
	// Cohere documents reasoning in a guide that enumerates nothing, and this
	// is the one place it is stated against a model: the page points at that
	// guide for what turning the model's thinking off costs, which only a
	// model that thinks has.
	thinkingRe = regexp.MustCompile(
		"(?i)enabling and disabling the `thinking` operation",
	)
	// noToolsRe matches the sentence a page withholds tool use in. The tool
	// use guide claims the whole Command family, and this is the one document
	// saying a member of it is excepted.
	noToolsRe = regexp.MustCompile(
		`(?i)\[tool use\]\([^)]*\) isn't supported with this model`,
	)
	// huggingFaceRe matches the repository a page publishes the weights in.
	huggingFaceRe = regexp.MustCompile(
		`\[Hugging ?Face\]\(https://huggingface\.co/` +
			`([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)\)`,
	)
)

// applyModelPage reads one Command model's own page.
//
// A page states two things no table does. It answers a capability against the
// model, which is the only form a capability is worth reading in: the reasoning
// models point at the guide for how to turn their thinking off, and Command A
// Vision says outright that tool use is not supported with it. It also carries,
// for the newest model, a properties list giving the day it was released, who
// published it and what licence the weights are under.
func (b *builder) applyModelPage(doc catalog.Document) {
	product, ok := modelPages[doc.URL]
	if !ok {
		return
	}
	body := string(doc.Body)
	details := modelDetails(body)
	weights := huggingFaceRe.FindStringSubmatch(body)
	for _, id := range b.identify(product) {
		m := b.models[id]
		m.AddSource(doc.URL)
		if thinkingRe.MatchString(body) {
			m.AddList(ListFeatures, catalog.CapabilityReasoning)
		}
		if noToolsRe.MatchString(body) {
			b.noTools[id] = true
		}
		if weights != nil {
			m.SetAttr(AttrHuggingFaceID, weights[1])
			m.SetAttr(AttrOpenWeights, openWeights)
		}
		applyProperties(m, details)
	}
}

// applyProperties records the labelled facts a model page's properties list
// states.
func applyProperties(m *catalog.Model, details map[string]string) {
	m.SetAttr(AttrReleaseDate, isoDate(details[detailReleaseDate]))
	m.SetAttr(AttrAuthor, details[detailProvider])
	m.SetAttr(AttrModelSize, details[detailModelSize])
	m.SetAttr(AttrLicense, details[detailLicense])
	m.AddList(ListLanguages, splitList(details[detailLanguages])...)
}
