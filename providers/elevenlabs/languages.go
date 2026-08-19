package elevenlabs

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// docsHost prefixes every documentation URL. A link inside a page is written
// as a site-absolute path, so the host is taken off a document's URL to compare
// the two.
const docsHost = "https://elevenlabs.io"

// langCodeRe matches a language a prose list names, which ElevenLabs writes as
// the language followed by its three letter code in brackets, as "Afrikaans
// (afr)".
var langCodeRe = regexp.MustCompile(`\(([a-z]{3})\)`)

// sectionHeadingRe matches a heading of any depth.
var sectionHeadingRe = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)

// linkTargetRe matches the target of a markdown link.
var linkTargetRe = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)

// indexLanguages records the language list of every section of a document that
// has one.
//
// A languages cell in the models table is often a link rather than a list:
// ElevenLabs writes "70+ languages" against Eleven v3 and points at the section
// further down the page that names them, and the same for the transcription
// models, whose cells point at the speech to text page. The link is the vendor
// saying which list belongs to which model, so the sections are indexed by the
// anchor a link can reach them by and the cell is resolved through them.
//
// The first section under an anchor wins, which is how a browser resolves one:
// the models page carries the same list twice, once under Eleven v3 and once
// under Eleven v3 Conversational, and both spell the anchor the same way.
func (b *builder) indexLanguages(doc catalog.Document) {
	path := docPath(doc.URL)
	var (
		anchor string
		codes  []string
	)
	store := func() {
		if anchor == "" || len(codes) == 0 {
			return
		}
		key := path + "#" + anchor
		if _, ok := b.sections[key]; !ok {
			b.sections[key] = codes
		}
	}
	for _, raw := range strings.Split(string(doc.Body), "\n") {
		line := strings.TrimSpace(raw)
		if match := sectionHeadingRe.FindStringSubmatch(line); match != nil {
			store()
			anchor, codes = slugify(clean(match[2])), nil
			continue
		}
		for _, code := range langCodeRe.FindAllStringSubmatch(line, -1) {
			codes = append(codes, code[1])
		}
	}
	store()
}

// docPath returns the site-absolute path a document is served under, which is
// what a link inside another document names it by.
func docPath(url string) string {
	return strings.TrimSuffix(strings.TrimPrefix(url, docsHost), ".md")
}

// slugify turns a heading into the anchor a link reaches it by.
func slugify(heading string) string {
	var out strings.Builder
	for _, r := range strings.ToLower(heading) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out.WriteRune(r)
		case r == ' ' || r == '-':
			out.WriteRune('-')
		}
	}
	return out.String()
}

// linkedLanguages returns the languages of the sections a cell links to.
func (b *builder) linkedLanguages(path, cell string) []string {
	var out []string
	for _, match := range linkTargetRe.FindAllStringSubmatch(cell, -1) {
		target := strings.TrimSpace(match[1])
		page, anchor, ok := strings.Cut(target, "#")
		if !ok {
			continue
		}
		if page == "" {
			page = path
		}
		out = append(out, b.sections[page+"#"+anchor]...)
	}
	return out
}
