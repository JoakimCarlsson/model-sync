package deepseek

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// entryRe matches the three elements the change log is built from, in the
// order they appear: the dated heading of an entry, the heading naming what
// the entry released, and the paragraph describing it.
var entryRe = regexp.MustCompile(
	`(?is)<h2[^>]*>(.*?)</h2\s*>|<h3[^>]*>(.*?)</h3\s*>|<p>(.*?)</p\s*>`,
)

// anchorRe matches the permalink the site puts inside every heading, which is
// not part of the heading's text.
var anchorRe = regexp.MustCompile(`(?is)<a\b.*?</a\s*>`)

// dateRe matches the day a change log entry is headed with.
var dateRe = regexp.MustCompile(`(\d{4}-\d{2}-\d{2})`)

// updateSuffix is what the change log appends to a heading naming a model.
const updateSuffix = " update"

// releaseStates map a phrase DeepSeek describes a release with onto the
// catalog's vocabulary for how available a model is. They are tried in order,
// so a paragraph carrying both would be read as the narrower of the two.
var releaseStates = []struct {
	phrase string
	state  string
}{
	{"public beta", "beta"},
	{"preview", "preview"},
	{"ga release", "active"},
}

// applyChangeLog reads each model's name, the day it was released, what
// DeepSeek says it is, and how available DeepSeek says it is.
//
// The pricing table heads its columns with the identifier, so the name is not
// on it. The change log heads the entry for a release with the model's name,
// written as DeepSeek writes it rather than as the identifier is spelled, and
// a heading is taken only where it is the identifier of a model the pricing
// page already stated. That is what keeps "DeepSeek-V4", the heading of the
// release that introduced both models, and the headings of the models
// withdrawn before them, from naming anything.
//
// The dated heading above that one gives the release date, and the paragraph
// below it gives the summary. Both are read in document order, which is why
// the three elements are matched by one expression rather than three.
func (b *builder) applyChangeLog(doc catalog.Document) {
	var date string
	var pending *catalog.Model
	for _, match := range entryRe.FindAllStringSubmatch(string(doc.Body), -1) {
		switch {
		case match[1] != "":
			date = changeLogDate(match[1])
			pending = nil
		case match[2] != "":
			pending = b.applyRelease(doc.URL, match[2], date)
		case match[3] != "" && pending != nil:
			applySummary(pending, text(match[3]))
			pending = nil
		}
	}
}

// changeLogDate reads the day out of a dated heading.
func changeLogDate(heading string) string {
	match := dateRe.FindStringSubmatch(
		text(anchorRe.ReplaceAllString(heading, "")),
	)
	if match == nil {
		return ""
	}
	return match[1]
}

// applyRelease names the model an entry heading announces and dates it,
// returning that model so the paragraph following the heading can describe it.
func (b *builder) applyRelease(source, heading, date string) *catalog.Model {
	name := text(anchorRe.ReplaceAllString(heading, ""))
	id := strings.TrimSuffix(strings.ToLower(name), updateSuffix)
	m, ok := b.models[id]
	if !ok || m.Name != "" {
		return nil
	}
	m.Name = name[:len(id)]
	m.SetAttr(AttrReleaseDate, date)
	m.AddSource(source)
	return m
}

// applySummary records what the release paragraph says the model is.
//
// Only the leading sentence is kept, because the sentences after it are about
// how to call the model rather than about the model. That sentence also
// carries the one word DeepSeek states about availability, so the state is
// read from it and not from the entry's later prose, which describes the
// release rather than the model.
func applySummary(m *catalog.Model, prose string) {
	summary := firstSentence(prose)
	m.SetAttr(AttrSummary, summary)
	lowered := strings.ToLower(summary)
	for _, candidate := range releaseStates {
		if strings.Contains(lowered, candidate.phrase) {
			m.SetAttr(AttrState, candidate.state)
			return
		}
	}
}
