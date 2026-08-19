package anthropic

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// ReleaseNotesURL is the platform changelog. It is the only document dating a
// model: the comparison table states cutoffs, the deprecations page states
// when a model goes away, and neither states when it arrived.
const ReleaseNotesURL = baseURL + "/release-notes/overview.md"

var (
	// releaseSectionRe splits the changelog into dated sections. Every entry
	// lives under a third level heading that is nothing but the date, so the
	// heading is the date of everything below it until the next one.
	releaseSectionRe = regexp.MustCompile(`(?m)^###[ \t]+(.+?)[ \t]*$`)
	// launchModelRe matches the two phrases a launch entry names a model in.
	// A model is announced as launched, and a model released the same day as
	// another is named after it rather than in a sentence of its own, which is
	// how Claude Mythos 5 is announced. Nothing else in an entry is anchored
	// this way, so a name reached through either phrase is the entry's
	// subject and never a model merely mentioned in passing.
	launchModelRe = regexp.MustCompile(
		`(?:We've launched|alongside) (Claude [A-Z][A-Za-z]* \d+(?:\.\d+)?)\b`,
	)
	// launchToolRe matches the same announcement made of a tool. Anthropic
	// writes a tool with the definite article and a model without one.
	launchToolRe = regexp.MustCompile(
		`(?i)We've launched (?:the |a |an )?([A-Za-z][A-Za-z ]*tool)\b`,
	)
	// summaryRe matches the appositive a launch entry describes the model
	// with, which runs from the model's name to the first comma or full stop
	// after it. It is Anthropic's own one line description of the model, and
	// for every model but the four current ones it is the only one published.
	summaryRe = regexp.MustCompile(
		`^(?:\s*\([^)]*\))?,\s*([^,]+?)(?:,|\.(?:\s|$))`,
	)
)

// applyReleaseNotes dates every model and tool the changelog announces, and
// describes the models the comparison table has no Description row for.
//
// The changelog is written newest first and an entry names models other than
// the one it announces, so neither the position of a name nor its presence is
// enough: an entry announcing Claude Opus 5 also names Claude Opus 4.8, and
// taking every name would date the older model to the newer one's launch. Only
// a name reached through one of the two announcing phrases is read, and the
// earliest date announcing a given model wins, so that a later entry revising
// a tool does not overwrite the day it shipped.
func (b *builder) applyReleaseNotes(doc catalog.Document) {
	dates := map[string]string{}
	summaries := map[string]string{}
	for _, section := range releaseSections(string(doc.Body)) {
		if section.date == "" {
			continue
		}
		for _, entry := range strings.Split(section.body, "\n*") {
			b.readLaunch(clean(entry), section.date, dates, summaries)
		}
	}
	for id, date := range dates {
		m := b.models[id]
		m.SetAttr(AttrReleaseDate, date)
		m.SetAttr(AttrSummary, summaries[id])
		m.AddSource(doc.URL)
	}
}

// readLaunch records what one changelog entry announces.
func (b *builder) readLaunch(
	entry, date string,
	dates, summaries map[string]string,
) {
	for _, match := range launchModelRe.FindAllStringSubmatchIndex(entry, -1) {
		name := entry[match[2]:match[3]]
		id := b.resolve(name)
		if _, ok := b.models[id]; !ok {
			continue
		}
		if earlier(dates[id], date) {
			dates[id] = date
			summaries[id] = summaryOf(entry[match[3]:])
		}
	}
	for _, match := range launchToolRe.FindAllStringSubmatch(entry, -1) {
		id, ok := b.toolByName(match[1])
		if !ok {
			continue
		}
		if earlier(dates[id], date) {
			dates[id] = date
		}
	}
}

// toolByName returns the tool the changelog's wording names, matching on the
// title the tool directory gives it. The changelog writes a tool's name in
// running prose and the directory writes it as a page title, so the two agree
// only up to case, and a phrase matching no title is not a tool at all: the
// same sentence shape announces an API and a parameter.
func (b *builder) toolByName(name string) (string, bool) {
	for _, id := range b.order {
		m := b.models[id]
		if m.Kind == KindTool && strings.EqualFold(m.Name, name) {
			return id, true
		}
	}
	return "", false
}

// earlier reports whether a date should replace what has been recorded, which
// is when nothing has been or when the new date is older. A tool announced
// once and revised later carries the day it shipped rather than the day it
// changed.
func earlier(recorded, date string) bool {
	return recorded == "" || date < recorded
}

// summaryOf reads the description a launch entry follows a model's name with,
// which Anthropic writes as an appositive and closes at the first comma or
// full stop. The identifier in parentheses after the name is stepped over,
// since it is the same fact the comparison table states as a column.
func summaryOf(rest string) string {
	match := summaryRe.FindStringSubmatch(rest)
	if match == nil {
		return ""
	}
	return strings.TrimSpace(match[1])
}

// releaseSection is one dated block of the changelog.
type releaseSection struct {
	date string
	body string
}

// releaseSections splits the changelog on its date headings.
func releaseSections(body string) []releaseSection {
	matches := releaseSectionRe.FindAllStringSubmatchIndex(body, -1)
	out := make([]releaseSection, 0, len(matches))
	for i, match := range matches {
		end := len(body)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		out = append(out, releaseSection{
			date: releaseDate(body[match[2]:match[3]]),
			body: body[match[1]:end],
		})
	}
	return out
}

// releaseDate reads a changelog heading, which is a date and nothing else. A
// heading in any other shape yields nothing rather than a guess, so a section
// that is not dated dates nothing.
func releaseDate(heading string) string {
	date := isoDate(stripOrdinal(clean(heading)))
	if !isoDateRe.MatchString(date) {
		return ""
	}
	return date
}

// isoDateRe matches a fully resolved day.
var isoDateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
