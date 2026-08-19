package ollama

import (
	"html"
	"path"
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Attributes the page of a build states, which no other page of Ollama's does.
//
// A build is a model at one size and one quantization, and the page lists the
// layers it is assembled from: the weights, the licence they are published
// under, the default parameters and the prompt template. The weights layer is
// described by the metadata it was built with, which is where the architecture,
// the parameter count and the quantization come from.
const (
	AttrArchitecture  = "architecture"
	AttrParameterCnt  = "parameter_count"
	AttrQuantization  = "quantization"
	AttrLicense       = "license"
	AttrOpenWeights   = "open_weights"
	AttrDefaultSnapsh = "default_snapshot"
)

// detailRe matches one field of the weights layer's description. The label is
// set in a span the page hides on a narrow screen and the value in the span
// after it, and the three fields it states are the only ones written that way.
var detailRe = regexp.MustCompile(
	`(?is)<span class="hidden sm:block">([a-z ]+)</span>` +
		`<span[^>]*>([^<]*)</span>`,
)

// detailAttrs map the label of a weights layer field onto the key it is
// recorded under. Ollama writes the architecture as the weights name it, which
// is the family the build was converted from rather than the model's own
// identifier: a distillation of one model onto another's weights says the
// weights it runs on.
var detailAttrs = map[string]string{
	"arch":         AttrArchitecture,
	"parameters":   AttrParameterCnt,
	"quantization": AttrQuantization,
}

// licenseLayerRe matches the opening of the licence layer, which the page shows
// as the first few lines of the licence text itself rather than as a name.
var licenseLayerRe = regexp.MustCompile(
	`(?is)` + blobsPath + `[0-9a-f]+">\s*license\s*</a>\s*</div>\s*` +
		`<div[^>]*>\s*([^<]*?)\s*</div>`,
)

// licenseNames name a licence whose own text does not put its whole name on
// one line. Apache writes the version on the line below the name, the Creative
// Commons deed wraps its name mid-title, and a licence that opens with the
// copyright holder names itself further down, so the excerpt is searched for
// the title rather than read from the top.
var licenseNames = []struct {
	Contains string
	Name     string
}{
	{"Apache License\nVersion 2.0", "Apache License 2.0"},
	{
		"Creative Commons Attribution-NonCommercial 4.0 " +
			"International Public\nLicense",
		"Creative Commons Attribution-NonCommercial 4.0 " +
			"International Public License",
	},
	{"MIT License", "MIT License"},
}

// licenseWords are the words a line naming a licence is written with. A licence
// file opens with its own title, but not always on the first line: some open
// with the version or the copyright year instead, and a line stating those is
// not a name.
var licenseWords = []string{
	"license", "licence", "terms", "agreement", "policy", "commons",
}

// genericTitles are headings that say a licence follows without naming it, and
// so are read past to the line that does name it.
var genericTitles = map[string]bool{
	"license":       true,
	"licence":       true,
	"license text":  true,
	"licence text":  true,
	"the license":   true,
	"license terms": true,
}

// licenseTitleLines is how far into the excerpt a title is looked for.
const licenseTitleLines = 3

// decorationRe matches the markdown a licence written as markdown sets its
// title in, which is emphasis around the whole line or around a word of it.
var decorationRe = regexp.MustCompile(`[#*]+`)

// sizeCardRe matches the card a cloud model's page heads with, which states the
// parameter count of the build Ollama runs. A model distributed to run locally
// states the same figure on the page of its build instead, since it has one per
// size and the card would have to pick one.
var sizeCardRe = regexp.MustCompile(
	`(?is)>Size</div>.{0,400}?text-black">\s*([\d.]+[KMBT]?)\s*</span>\s*` +
		`<span[^>]*>\s*parameters\s*</span>`,
)

// applyBuildPage reads the page of the build Ollama serves by default.
//
// It is the one page stating what the weights are: the architecture they were
// converted from, how many parameters they hold, the quantization they are
// stored at and the licence they are published under. All four belong to the
// build rather than to the family, and the default build is the one recorded
// throughout, because that is what running the model plainly gives.
//
// That the page lists a weights layer at all is itself a fact, and the one that
// says whether the weights are published: a model Ollama only runs in its cloud
// has a page with no layers on it, because there is nothing to download.
func (b *builder) applyBuildPage(doc catalog.Document) {
	m, ok := b.models[buildModelID(doc.URL)]
	if !ok {
		return
	}
	body := string(doc.Body)
	read := false
	for _, detail := range detailRe.FindAllStringSubmatch(body, -1) {
		key, ok := detailAttrs[strings.ToLower(strings.TrimSpace(detail[1]))]
		if !ok {
			continue
		}
		if value := strings.TrimSpace(detail[2]); value != "" {
			m.SetAttr(key, value)
			read = true
		}
	}
	if modelBlobRe.Match(doc.Body) {
		m.SetAttr(AttrOpenWeights, "true")
		read = true
	}
	if name := licenseName(body); name != "" {
		m.SetAttr(AttrLicense, name)
		read = true
	}
	if read {
		m.AddSource(doc.URL)
	}
}

// licenseName reads the licence the build is published under from the excerpt
// the page shows of the licence layer.
//
// The excerpt is the opening of the licence text itself, so the name is its
// title. Three of the common ones spread the title over two lines and are
// matched whole; otherwise the title is the first of the opening lines that
// names a licence, since a file opening with its version, its copyright line or
// a bare heading states no name there. A layer holding a path rather than a
// licence names nothing and yields nothing.
func licenseName(body string) string {
	match := licenseLayerRe.FindStringSubmatch(body)
	if match == nil {
		return ""
	}
	excerpt := html.UnescapeString(match[1])
	for _, known := range licenseNames {
		if strings.Contains(excerpt, known.Contains) {
			return known.Name
		}
	}
	lines := strings.SplitN(excerpt, "\n", licenseTitleLines+1)
	for i, line := range lines {
		if i == licenseTitleLines {
			break
		}
		if title := licenseTitle(line); title != "" {
			return title
		}
	}
	return ""
}

// licenseTitle returns the licence one line names, or the empty string where
// the line names none.
func licenseTitle(line string) string {
	title := strings.TrimSpace(decorationRe.ReplaceAllString(line, ""))
	if title == "" || strings.Contains(title, "/") {
		return ""
	}
	lower := strings.ToLower(title)
	if genericTitles[lower] {
		return ""
	}
	for _, word := range licenseWords {
		if strings.Contains(lower, word) {
			return title
		}
	}
	return ""
}

// applySizeCard records the parameter count a cloud model's page states,
// reporting whether the page stated one.
func applySizeCard(m *catalog.Model, body string) bool {
	match := sizeCardRe.FindStringSubmatch(body)
	if match == nil {
		return false
	}
	m.SetAttr(AttrParameterCnt, match[1])
	return true
}

// buildModelID names the model a build belongs to, which is its reference with
// the tag cut off.
func buildModelID(url string) string {
	id, _, _ := strings.Cut(path.Base(url), ":")
	return id
}

// isBuildURL reports whether a URL names one build rather than a model family.
// A build carries a tag and a family does not, which is the whole difference
// between the two pages.
func isBuildURL(url string) bool {
	return strings.Contains(path.Base(url), ":")
}
