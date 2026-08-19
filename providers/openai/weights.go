package openai

import (
	"regexp"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// What an open weight model's page states that a hosted model's does not.
const (
	AttrOpenWeights      = "open_weights"
	AttrLicense          = "license"
	AttrHuggingFaceID    = "hugging_face_id"
	AttrParameters       = "parameters"
	AttrActiveParameters = "active_parameters"
	AttrModelCardURL     = "model_card_url"
)

var (
	// huggingFaceRe matches the download link the two gpt-oss pages carry,
	// which is the only place OpenAI publishes a weight repository for
	// anything it makes.
	huggingFaceRe = regexp.MustCompile(
		`https://huggingface\.co/([\w.-]+/[\w.-]+)`,
	)
	// licenseRe matches the licence bullet those pages open their key
	// features with, written as "**Permissive Apache 2.0 license:**".
	licenseRe = regexp.MustCompile(
		`\*\*(?:Permissive )?([\w.]+(?: [\w.]+)*) license[:,]`,
	)
	// parameterRe matches the parenthesis stating how large the model is and
	// how much of it runs per token, written as "(117B parameters with 5.1B
	// active parameters)".
	parameterRe = regexp.MustCompile(
		`\(([\d.]+[BM]) parameters with ([\d.]+[BM]) active parameters\)`,
	)
)

// applyWeights records what a page says about weights it publishes.
//
// OpenAI serves two models whose weights anyone may download, and their pages
// are the only ones carrying a repository link, a licence and a parameter
// count. Nothing here fires on the other ninety-four pages, which state none
// of the three, and the absence is the fact that those models are not open.
func applyWeights(m *catalog.Model, body string) {
	match := huggingFaceRe.FindStringSubmatch(body)
	if match == nil {
		return
	}
	m.SetAttr(AttrHuggingFaceID, match[1])
	m.SetAttr(AttrModelCardURL, match[0])
	m.SetAttr(AttrOpenWeights, "true")
	if license := licenseRe.FindStringSubmatch(body); license != nil {
		m.SetAttr(AttrLicense, license[1])
	}
	if params := parameterRe.FindStringSubmatch(body); params != nil {
		m.SetAttr(AttrParameters, params[1])
		m.SetAttr(AttrActiveParameters, params[2])
	}
}
