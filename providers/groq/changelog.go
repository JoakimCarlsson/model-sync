package groq

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// ChangelogURL is the page Groq dates every change to the platform on.
const ChangelogURL = baseURL + "/docs/changelog.md"

var (
	// entryDateRe matches the date an entry is filed under, which Groq writes
	// on a line of its own. The year is written for every year but the current
	// one, and an entry without a year is passed over rather than dated by
	// guessing which year the page was read in.
	entryDateRe = regexp.MustCompile(
		`^([A-Z][a-z]{2}) (\d{1,2})(?:, (\d{4}))?$`,
	)
	// entryKindRe matches the heading saying what the entry did, which Groq
	// runs together with the title.
	entryKindRe = regexp.MustCompile(`^###\s+([A-Za-z]+)\[`)
	// entryModelRe matches a link to a model or a system, which is how an
	// entry names what it changed.
	entryModelRe = regexp.MustCompile(
		`/docs/(?:model|compound/systems)/([A-Za-z0-9./_-]+)`,
	)
)

// kindAdded is the word Groq heads the entry announcing a model with.
const kindAdded = "Added"

// applyChangelog reads the changelog.
//
// Groq dates every entry and links the models it concerns, so the earliest
// entry announcing a model is the day Groq began serving it and the latest
// entry naming it is the last time anything about it changed. Both are
// recorded, the first from the entries headed Added only, since an entry that
// merely mentions a model is not its announcement.
//
// Nothing is dated by inference: an entry from the year the page was read in
// carries no year, and those entries are passed over rather than dated from
// the clock, which would also make the output depend on when it ran.
func (b *builder) applyChangelog(doc catalog.Document) {
	var (
		released = map[string]string{}
		updated  = map[string]string{}
		date     string
		kind     string
	)
	for _, raw := range strings.Split(string(doc.Body), "\n") {
		line := strings.TrimSpace(raw)
		if match := entryDateRe.FindStringSubmatch(line); match != nil {
			date = ""
			if match[3] != "" {
				date = parseLongDate(
					match[1] + " " + match[2] + ", " + match[3],
				)
			}
			kind = ""
			continue
		}
		if match := entryKindRe.FindStringSubmatch(line); match != nil {
			kind = match[1]
		}
		if date == "" {
			continue
		}
		for _, id := range b.changelogIDs(line) {
			replaceLatest(updated, id, date)
			if kind == kindAdded {
				keepEarliest(released, id, date)
			}
		}
	}
	b.applyDates(released, updated, doc.URL)
}

// applyDates records what the changelog established.
func (b *builder) applyDates(released, updated map[string]string, src string) {
	for _, id := range b.order {
		m := b.models[id]
		release, hasRelease := released[id]
		update, hasUpdate := updated[id]
		if !hasRelease && !hasUpdate {
			continue
		}
		m.AddSource(src)
		m.SetAttr(AttrReleaseDate, release)
		m.SetAttr(AttrLastUpdated, update)
	}
}

// changelogIDs returns the models one line links to.
//
// An entry writes a model's page address, which is the identifier with a
// prefix, but not always the identifier the model is served under: Groq has
// filed the same model under a path with and without the name of the developer
// who published it. So an address that names no model is matched on its last
// segment, which is what differs between the two spellings.
func (b *builder) changelogIDs(line string) []string {
	var out []string
	for _, match := range entryModelRe.FindAllStringSubmatch(line, -1) {
		linked := strings.TrimSuffix(match[1], ".md")
		if _, ok := b.models[linked]; ok {
			out = append(out, linked)
			continue
		}
		if id := b.bySuffix(linked); id != "" {
			out = append(out, id)
		}
	}
	return out
}

// bySuffix returns the one model whose identifier ends in the same segment as
// the address, or nothing where more than one does.
func (b *builder) bySuffix(linked string) string {
	segment := linked[strings.LastIndex(linked, "/")+1:]
	var found string
	for _, id := range b.order {
		if id[strings.LastIndex(id, "/")+1:] != segment {
			continue
		}
		if found != "" {
			return ""
		}
		found = id
	}
	return found
}

// keepEarliest records the earlier of a date already held and a new one.
func keepEarliest(dates map[string]string, id, date string) {
	if held, ok := dates[id]; !ok || date < held {
		dates[id] = date
	}
}

// replaceLatest records the later of the two.
func replaceLatest(dates map[string]string, id, date string) {
	if held, ok := dates[id]; !ok || date > held {
		dates[id] = date
	}
}
