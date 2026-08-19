package vertexai

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Keys the index pages populate, which no model page states.
const (
	// AttrSummary is what Google says a model is for.
	AttrSummary = "summary"
	// ListLanguages holds the languages the index states a family answers in.
	ListLanguages = "languages"
)

var (
	// cardRe matches one entry of the model index, which names the model,
	// links its page and says in a sentence what it is for. The page carries
	// three such lists, of the models generally available, the models in
	// preview and the models of each other family Google publishes, and all of
	// them are read.
	cardRe = regexp.MustCompile(
		`(?is)<span class="model-name">\s*<a href="([^"]+)"[^>]*>(.*?)</a>` +
			`\s*</span>\s*<span class="model-description">(.*?)</span>`,
	)
	// tableRowRe matches one row of the tables indexing the models Vertex
	// serves for other labs, which name the model, the modalities it works in,
	// what it is for and the Model Garden card it is served from.
	tableRowRe = regexp.MustCompile(
		`(?is)<tr>\s*<td>(.*?)</td>\s*<td>(.*?)</td>\s*` +
			`<td>(.*?)</td>\s*<td>(.*?)</td>\s*</tr>`,
	)
	// cardLinkRe matches that card, whose address states the lab that
	// published the model and the identifier Vertex serves it under. Neither
	// is written anywhere else: a model page names the identifier and never
	// the lab, and the index names the lab only here.
	cardLinkRe = regexp.MustCompile(
		`publishers/([a-z0-9.-]+)/model-garden/([A-Za-z0-9._-]+)`,
	)
	// stageRe matches the launch stage the index hangs off a model's name,
	// which it writes as a link to Google's scale of them.
	stageRe = regexp.MustCompile(
		`(?is)\((<a[^>]*>)?\s*([A-Za-z ]+?)\s*(</a>)?\)`,
	)
	// languageSectionRe matches one family's language list, which the index
	// files under an expandable heading naming the family.
	languageSectionRe = regexp.MustCompile(
		`(?is)<h3[^>]*id="languages-gemini"[^>]*>(.*?)(?:<h3|\z)`,
	)
	// languageCodeRe matches one language of such a list, which the index sets
	// beside the language's name in code font.
	languageCodeRe = regexp.MustCompile(
		`(?is)<code[^>]*>([a-z]{2,3}(?:-[A-Za-z]+)?)</code>`,
	)
	// geminiPagePre prefixes the pages of the models the language list is
	// stated for. The list says it holds for all the Gemini models, and these
	// are the pages Google files under Gemini.
	geminiPagePre = modelPagePre + "gemini/"
)

// readModelCards records what the index says of the models Google made.
//
// The index is the only document saying what a model is for. A model page
// states what the model holds and what it can do and never why it exists, and
// the sentence the index gives each entry is the nearest thing Google
// publishes to a description. The entry is joined to the page it links, so
// nothing is matched on a name.
func readModelCards(byURL map[string]*documented, doc catalog.Document) {
	if doc.URL != ModelsURL {
		return
	}
	body := string(doc.Body)
	languages := readLanguages(body)
	for _, match := range cardRe.FindAllStringSubmatch(body, -1) {
		page, ok := byURL[docsBase+specAttr(match[1])]
		if !ok {
			continue
		}
		page.merge(documented{
			Title:   specText(match[2]),
			Summary: specText(match[3]),
		}, doc.URL)
	}
	for url, page := range byURL {
		if strings.HasPrefix(url, geminiPagePre) {
			page.merge(documented{Languages: languages}, doc.URL)
		}
	}
}

// readLanguages reads the languages the index states the Gemini models answer
// in, which it states once for the family rather than once per model: the
// sentence opening the list says it holds for all of them, so it is recorded
// against each. The other families' lists are not read, because a Gemma
// variant has no page to join one to and would have to be matched on its name.
func readLanguages(body string) []string {
	var out []string
	for _, match := range languageCodeRe.FindAllStringSubmatch(
		languageSectionRe.FindString(body),
		-1,
	) {
		out = appendNew(out, match[1])
	}
	return out
}

// readModelTable records what the two tables indexing the models Vertex serves
// for other labs state: the lab that made each, what it is for, the stage it
// is served at and the identifier it answers to.
//
// A row is joined on that identifier rather than on a page, because Google
// lists models here that it documents nowhere else, the two Jamba releases and
// the non-reasoning Grok variants among them. Such a row is all Vertex
// publishes about the model, and dropping it would leave the model out of the
// catalog rather than leave it thin.
func readModelTable(pages map[string]*documented, doc catalog.Document) {
	if doc.URL != openModelsURL && doc.URL != partnerModelsURL {
		return
	}
	rows := tableRowRe.FindAllStringSubmatch(string(doc.Body), -1)
	shared := sharedCards(rows)
	for _, row := range rows {
		link := cardLinkRe.FindStringSubmatch(row[4])
		if link == nil || shared[link[2]] {
			continue
		}
		documentedFor(pages, servedName(link[2])).merge(documented{
			Served:  true,
			ID:      link[2],
			Author:  link[1],
			Title:   cardTitle(row[1]),
			Summary: specText(row[3]),
			State:   stateOf(cardStage(row[1])),
		}, doc.URL)
	}
}

// sharedCards reports the Model Garden cards more than one row of a table
// points at.
//
// Google indexes Claude Opus 4 and Claude Opus 4.1 against the same card, so
// one of the two descriptions is written against the wrong model and the page
// does not say which. Neither row is read, because a description that may be
// either model's is not a fact about one. The models themselves are unaffected:
// each has a page of its own stating everything but what it is for.
func sharedCards(rows [][]string) map[string]bool {
	seen := map[string]int{}
	for _, row := range rows {
		if link := cardLinkRe.FindStringSubmatch(row[4]); link != nil {
			seen[link[2]]++
		}
	}
	shared := map[string]bool{}
	for id, count := range seen {
		if count > 1 {
			shared[id] = true
		}
	}
	return shared
}

// cardTitle returns the name the index writes for a model, without the launch
// stage it hangs off the end of it.
func cardTitle(cell string) string {
	return specText(stageRe.ReplaceAllString(cell, " "))
}

// cardStage returns the launch stage the index hangs off a model's name. A
// name may carry a bracketed word that is not one, the reasoning and
// non-reasoning Grok variants being told apart that way, and stateOf answers
// for nothing it does not recognise.
func cardStage(cell string) string {
	match := stageRe.FindStringSubmatch(cell)
	if match == nil {
		return ""
	}
	return match[2]
}

// specAttr unescapes the ampersands a link may carry.
func specAttr(value string) string {
	return strings.ReplaceAll(value, "&amp;", "&")
}
