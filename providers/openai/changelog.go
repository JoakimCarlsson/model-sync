package openai

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Dates the changelog states.
const (
	AttrReleaseDate = "release_date"
	AttrLastUpdated = "last_updated"
)

// midDot separates the fields of a changelog entry's tag line, which reads
// "Feature · Model: gpt-transcribe · API: v1/audio/transcriptions".
const midDot = " · "

// entryTypes are the four words a changelog entry opens with. The word itself
// is not recorded; it is read only to recognize the line that carries an
// entry's model tags.
var entryTypes = []string{"Feature", "Update", "Fix", "Announcement"}

// modelTag prefixes the tag naming a model an entry is about.
const modelTag = "Model: "

var (
	changelogMonthRe = regexp.MustCompile(`^## ([A-Za-z]+), (\d{4})$`)
	changelogDayRe   = regexp.MustCompile(`^### ([A-Za-z]{3}) (\d{1,2})$`)
	// changelogLinkRe matches a markdown link to a model's page. The
	// changelog writes these absolute and without the .md suffix the model
	// index uses, and older entries point at the platform host rather than
	// this one, so only the path is matched.
	changelogLinkRe = regexp.MustCompile(
		`\]\([^)]*?/docs/models/([A-Za-z0-9._-]+)\)`,
	)
	// releaseVerbRe matches the verb a release is announced with, either
	// opening the sentence or after the "and" joining a second announcement to
	// the first.
	releaseVerbRe = regexp.MustCompile(
		`(?:^|\band )(Released|Launched|Introduced)\b`,
	)
	// releaseHeadRe matches the same verb only where it opens the sentence,
	// which is what marks an entry as announcing something rather than merely
	// mentioning a release in passing.
	releaseHeadRe = regexp.MustCompile(`^(Released|Launched|Introduced)\b`)
)

// appositions are the wordings that turn what follows into a description of
// the model just announced rather than a second model being announced. OpenAI
// writes "Released o1-pro, a version of the o1 reasoning model", and reading
// the second link as a release would date o1 to the day o1-pro arrived.
var appositions = []string{", a ", ", an ", ", which"}

// changelogEntry is one dated announcement together with the models OpenAI
// tags it with and the prose under it.
type changelogEntry struct {
	Date   string
	Models []string
	Text   []string
}

// applyChangelog reads the release notes for the two dates no other OpenAI
// document states.
//
// last_updated is the plainest of the two: OpenAI tags each entry with the
// models it concerns, so the newest entry tagged with a model is the last time
// OpenAI published anything about it. The entries run newest first, so the
// first tag seen wins, which is the rule the scalar attributes already follow.
//
// release_date is read two ways, because OpenAI announces a release two ways.
// Where a sentence opens with Released, Launched or Introduced and links the
// model's page, that link is the model being announced and the entry's date is
// its release. Where the sentence names a family instead and links a guide, as
// it does for the GPT-5.6 models, there is no link to read and the entry's own
// model tags are what say which models arrived; those are used only for an
// entry that announces a release in the first place, so that a deprecation
// notice mentioning a released API does not date the models it withdraws.
// Entries are walked oldest first so the earliest announcement stands.
//
// Nothing here creates a model. The changelog names models OpenAI has since
// removed and writes some identifiers as the old platform URLs spelled them,
// with the dots replaced by hyphens, and a document read for dates should not
// be able to add a model to the catalog on the strength of a URL slug.
func (b *builder) applyChangelog(doc catalog.Document) {
	entries := scanChangelog(doc)
	for i := len(entries) - 1; i >= 0; i-- {
		b.applyReleaseEntry(doc.URL, entries[i])
	}
	for _, e := range entries {
		for _, id := range e.Models {
			b.setChangelogDate(doc.URL, id, AttrLastUpdated, e.Date)
		}
	}
}

// applyReleaseEntry records the release dates one entry states.
func (b *builder) applyReleaseEntry(source string, e changelogEntry) {
	announces := false
	for _, para := range e.Text {
		for _, sentence := range splitSentences(para) {
			if releaseHeadRe.MatchString(sentence) {
				announces = true
			}
			for _, id := range releasedIn(sentence) {
				b.setChangelogDate(source, id, AttrReleaseDate, e.Date)
			}
		}
	}
	if !announces {
		return
	}
	for _, id := range e.Models {
		b.setChangelogDate(source, id, AttrReleaseDate, e.Date)
	}
}

// releasedIn returns the models one sentence announces, which are the model
// pages it links after the release verb. A sentence that qualifies what it
// announced with an apposition names a second model only to describe the
// first, and states no release of its own.
func releasedIn(sentence string) []string {
	at := releaseVerbRe.FindStringSubmatchIndex(sentence)
	if at == nil {
		return nil
	}
	lower := strings.ToLower(sentence)
	for _, phrase := range appositions {
		if strings.Contains(lower, phrase) {
			return nil
		}
	}
	var out []string
	for _, link := range changelogLinkRe.FindAllStringSubmatchIndex(
		sentence,
		-1,
	) {
		if link[0] < at[3] {
			continue
		}
		out = append(out, sentence[link[2]:link[3]])
	}
	return out
}

// setChangelogDate records a date against a model some other document already
// established.
func (b *builder) setChangelogDate(source, id, key, date string) {
	m, ok := b.models[id]
	if !ok || date == "" {
		return
	}
	m.SetAttr(key, date)
	m.AddSource(source)
}

// scanChangelog splits the page into its dated entries. A date is spread over
// two headings, the year on the month heading and the day on the entry's own,
// so both are tracked and joined into the form isoDate reads.
func scanChangelog(doc catalog.Document) []changelogEntry {
	var (
		out  []changelogEntry
		year string
		date string
		cur  *changelogEntry
	)
	for _, raw := range strings.Split(string(doc.Body), "\n") {
		line := strings.TrimSpace(raw)
		if match := changelogMonthRe.FindStringSubmatch(line); match != nil {
			year, date, cur = match[2], "", nil
			continue
		}
		if match := changelogDayRe.FindStringSubmatch(line); match != nil {
			date = isoDate(match[1] + " " + match[2] + ", " + year)
			cur = nil
			continue
		}
		if models, ok := entryTags(line); ok && date != "" {
			out = append(out, changelogEntry{Date: date, Models: models})
			cur = &out[len(out)-1]
			continue
		}
		if cur != nil && line != "" {
			cur.Text = append(cur.Text, line)
		}
	}
	return out
}

// entryTags reads the line opening an entry, returning the models it is tagged
// with. A line that is not such a line is rejected rather than read as an
// untagged entry, so prose beginning with the word Update does not open one.
func entryTags(line string) ([]string, bool) {
	fields := strings.Split(line, midDot)
	opener := strings.TrimSpace(fields[0])
	found := false
	for _, t := range entryTypes {
		if opener == t {
			found = true
		}
	}
	if !found {
		return nil, false
	}
	var models []string
	for _, field := range fields[1:] {
		if id, ok := strings.CutPrefix(strings.TrimSpace(field), modelTag); ok {
			models = append(models, cleanToken(id))
		}
	}
	return models, true
}

// splitSentences breaks a paragraph at the sentence ends OpenAI writes, which
// are a full stop or a colon followed by a space. The colon counts because the
// changelog introduces a list of releases with one.
func splitSentences(para string) []string {
	var (
		out   []string
		start int
	)
	for i := 0; i < len(para)-1; i++ {
		if para[i] != '.' && para[i] != ':' {
			continue
		}
		if para[i+1] != ' ' {
			continue
		}
		out = append(out, strings.TrimSpace(para[start:i+1]))
		start = i + 1
	}
	return append(out, strings.TrimSpace(para[start:]))
}
