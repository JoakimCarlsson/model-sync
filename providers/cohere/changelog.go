package cohere

import (
	"regexp"
	"sort"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// ChangelogURL indexes every release note Cohere has published. The overview
// states no date at all, and this is the only document saying when a model
// arrived: the index carries the date of each entry and the entry carries the
// identifier it announces.
const ChangelogURL = "https://docs.cohere.com/changelog"

// changelogBase is where an entry named by the index is read from. The index
// gives a slug and the documentation serves the entry as markdown under it.
const changelogBase = "https://docs.cohere.com/"

var (
	// entryRe matches one entry of the index, which the page carries as data
	// rather than as prose. The slug and the date sit in the same object and
	// nothing else separates one entry from the next, so they are matched
	// together and the match is stopped at the object's edge.
	entryRe = regexp.MustCompile(
		`"slug":"(changelog/[^"]+)"[^{}]*?"date":"(\d{4}-\d{2}-\d{2})`,
	)
	// releaseRe matches the sentence an entry announces a release in. An entry
	// that says something else about a model, a platform it has reached or a
	// date it is being withdrawn on, names the identifier too, and dating a
	// model from the first entry to mention it would put a model's release on
	// the day it was deprecated.
	releaseRe = regexp.MustCompile(
		`(?i)announc\w*\s+(?:the\s+)?(?:release of|updates to)`,
	)
	// entryIDRe matches an identifier an entry names in code style.
	entryIDRe = regexp.MustCompile("`([a-z0-9][a-z0-9.-]{4,40})`")
)

// changelogDates reads the index and returns the day each entry was published,
// keyed by the address the entry is served under.
//
// A slug the index lists twice keeps the earlier of the two dates, so that the
// output does not depend on which of them the page happens to state first.
func changelogDates(body []byte) map[string]string {
	out := map[string]string{}
	for _, match := range entryRe.FindAllStringSubmatch(unescaped(body), -1) {
		url := changelogBase + match[1] + ".md"
		if date, ok := out[url]; !ok || match[2] < date {
			out[url] = match[2]
		}
	}
	return out
}

// changelogEntries returns the addresses of every entry the index names, in
// the order they are read in.
func changelogEntries(body []byte) []string {
	dates := changelogDates(body)
	out := make([]string, 0, len(dates))
	for url := range dates {
		out = append(out, url)
	}
	sort.Strings(out)
	return out
}

// unescaped resolves the quoting a rendered page wraps its data in, so that
// the data can be matched with the same pattern whether the page carries it
// escaped or plain.
func unescaped(body []byte) string {
	return strings.ReplaceAll(string(body), `\"`, `"`)
}

// applyChangelog reads one release note.
//
// An entry reaches a model two ways, and both name the model rather than
// implying it. Most entries carry a technical details list headed by the
// identifier, which is the entry saying what it is about. The rest announce a
// release in prose and name the identifiers in it, and only those are read: an
// entry that mentions a model without announcing it is announcing something
// else, and the deprecation entries mention every model they recommend as a
// replacement.
//
// The date comes from the index rather than from the entry, which states none.
func (b *builder) applyChangelog(doc catalog.Document, date string) {
	if date == "" {
		return
	}
	body := string(doc.Body)
	details := modelDetails(body)
	ids := map[string]bool{}
	if named := details[detailID]; named != "" {
		ids[named] = true
	}
	if releaseRe.MatchString(body) {
		for _, match := range entryIDRe.FindAllStringSubmatch(body, -1) {
			ids[match[1]] = true
		}
	}
	named := make([]string, 0, len(ids))
	for id := range ids {
		named = append(named, id)
	}
	sort.Strings(named)
	for _, id := range named {
		m, ok := b.models[id]
		if !ok {
			continue
		}
		m.AddSource(doc.URL)
		m.SetAttr(AttrReleaseDate, date)
		m.SetAttr(AttrLicense, details[detailLicense])
		m.SetAttr(AttrModelSize, details[detailSize])
		m.AddList(ListLanguages, languageList(details[detailLanguages])...)
	}
}

// detailSize is how an entry labels how large the model it announces is.
const detailSize = "size"

// languageList divides the list of languages a details bullet states. The
// bullet is written as a sentence and ends in a full stop, which belongs to
// the sentence rather than to the last language.
func languageList(value string) []string {
	var out []string
	for _, item := range splitList(value) {
		if name := strings.TrimRight(item, "."); name != "" {
			out = append(out, name)
		}
	}
	return out
}
