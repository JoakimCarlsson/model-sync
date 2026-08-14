package together

import (
	"regexp"
	"slices"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// GuideIndexURL is the index of every page Together's documentation holds. The
// per-model guides are named there and nowhere else: the catalog page links to
// none of them.
const GuideIndexURL = "https://docs.together.ai/llms.txt"

// guideMark is what the index writes into the path of a page introducing one
// model.
const guideMark = "quickstart"

// LimitMaxOutputTokens is the bound a guide states, which no table does.
const LimitMaxOutputTokens = "max_output_tokens"

// guidePageRe matches one documentation page in the index.
var guidePageRe = regexp.MustCompile(
	`https://docs\.together\.ai/[A-Za-z0-9._/-]+\.md`,
)

// guideModelRe matches the sentence a guide names its model in. Every guide
// opens by stating the string the API answers to, which is what ties the page
// to a catalog row.
var guideModelRe = regexp.MustCompile("(?i)model ID is `([^`]+)`")

// guideOutputRe matches the clause a guide states an output ceiling in.
var guideOutputRe = regexp.MustCompile(
	`(?i)up to ([\d.,]+\s*[kmb]?) output tokens`,
)

// guideURLs derives the per-model guides the index names.
func guideURLs(index catalog.Document) []string {
	var urls []string
	for _, url := range guidePageRe.FindAllString(string(index.Body), -1) {
		if !strings.Contains(url, guideMark) || slices.Contains(urls, url) {
			continue
		}
		urls = append(urls, url)
	}
	return urls
}

// applyGuide reads one model's guide onto the model the catalog page
// established for it.
//
// A guide is prose rather than a table, and the one fact it holds that no
// table does is a ceiling on how much the model may generate. Together states
// that ceiling for a model whose ceiling is lower than its context window and
// says nothing where the two are the same, so a guide yielding nothing is the
// usual case rather than a failure to read it.
func (b *builder) applyGuide(doc catalog.Document) {
	body := string(doc.Body)
	name := guideModelRe.FindStringSubmatch(body)
	if name == nil {
		return
	}
	m, ok := b.models[clean(name[1])]
	if !ok {
		return
	}
	bound := guideOutputRe.FindStringSubmatch(body)
	if bound == nil {
		return
	}
	m.SetLimit(LimitMaxOutputTokens, parseCount(bound[1]))
	m.AddSource(doc.URL)
}
