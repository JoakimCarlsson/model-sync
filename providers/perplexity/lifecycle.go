package perplexity

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// ChangelogURL is Perplexity's changelog, which is the only document dating
// anything. It is written as a run of month-labelled entries, and a model's
// dates are read from the entries naming it.
const ChangelogURL = baseURL + "/docs/resources/changelog.md"

// MigrateURL is the Sonar migration guide. It is the only document saying what
// to use instead of a Sonar model, which it states as a table mapping each of
// them onto an Agent API preset rather than onto another model.
const MigrateURL = baseURL + "/docs/agent-api/migrate-from-sonar/overview.md"

var (
	// updateRe matches one changelog entry, capturing the month it is labelled
	// with, the subjects it is tagged with and its body.
	updateRe = regexp.MustCompile(
		`(?s)<Update\s+label="([^"]*)"\s+tags=\{\[([^\]]*)\]\}>(.*?)</Update>`,
	)
	// idRe matches a backticked identifier. The changelog writes model
	// identifiers this way and writes request parameters the same way, so the
	// shape of the identifier is what tells them apart.
	idRe = regexp.MustCompile("`([a-z0-9][a-z0-9./-]*)`")
	// launchRe matches a sentence stating that a model has been added, which
	// is the only sentence a release date may be read from: the changelog also
	// names a model when recommending it as a replacement for another, and
	// that says nothing about when it arrived.
	launchRe = regexp.MustCompile(
		`(?i)now supports?\b|now available\b|added support for\b`,
	)
	// bulletRe matches the other form a month of new models is written in, a
	// list whose every item names one model in bold and then in full.
	bulletRe = regexp.MustCompile(`^\*\s+\*\*.+\*\*.*—`)
	// retireRe matches a sentence withdrawing a model.
	retireRe = regexp.MustCompile(
		`(?i)\bdeprecated\b|\bretired\b|\bremoved from the api\b`,
	)
	// listMarkerRe matches the line the older deprecation notices head their
	// list of withdrawn identifiers with. Those notices state the identifiers
	// as bare lines rather than in a sentence, and the same entry lists the
	// replacements the same way, so only the lines under this marker are read.
	listMarkerRe = regexp.MustCompile(
		`(?i)following model names will no longer be`,
	)
	// replacementRe matches the recommendation an entry closes a withdrawal
	// with, which names the successor in full.
	replacementRe = regexp.MustCompile(
		"(?i)(?:use|migrating to|switching to) `([a-z0-9][a-z0-9./-]*)`",
	)
	// noticeRe matches the sentence every Sonar page carries, which is the
	// only statement of when the Sonar API stops being served.
	noticeRe = regexp.MustCompile(
		`(?i)Sonar will be supported until\s+([A-Z][a-z]+\s+\d{1,2},\s*\d{4})`,
	)
	// dayRe matches a date written the way the changelog writes one.
	dayRe = regexp.MustCompile(`([A-Z][a-z]+)\s+(\d{1,2}),\s*(\d{4})`)
	// monthRe matches an entry's label, which is a month and a year.
	monthRe = regexp.MustCompile(`^([A-Z][a-z]+)\s+(\d{4})$`)
	// sentenceRe splits prose into sentences.
	sentenceRe = regexp.MustCompile(`(?m)[.!?]\s|\n`)
)

// months maps a month name onto its number.
var months = map[string]int{
	"january": 1, "february": 2, "march": 3, "april": 4,
	"may": 5, "june": 6, "july": 7, "august": 8,
	"september": 9, "october": 10, "november": 11, "december": 12,
}

// update is one changelog entry.
type update struct {
	month string
	tags  string
	body  string
}

// applyChangelog reads the changelog.
//
// Withdrawals are read first, because they decide which identifiers a release
// date may be read for: an entry withdrawing a model names it in the month it
// went, not the month it arrived, and the same entry often names its successor
// too. A withdrawn model is created here if the current rate cards no longer
// carry it, since a model that has been removed from the API is exactly the
// thing those cards stop listing.
func (b *builder) applyChangelog(doc catalog.Document) {
	updates := changelogUpdates(string(doc.Body))
	retired := b.applyRetirements(doc.URL, updates)
	b.applyReleases(doc.URL, updates, retired)
}

// changelogUpdates splits the changelog into its entries.
func changelogUpdates(body string) []update {
	matches := updateRe.FindAllStringSubmatch(body, -1)
	updates := make([]update, 0, len(matches))
	for _, match := range matches {
		updates = append(updates, update{
			month: isoMonth(match[1]),
			tags:  strings.ToLower(match[2]),
			body:  match[3],
		})
	}
	return updates
}

// applyRetirements records every model the changelog withdraws, and returns
// the identifiers it withdrew.
func (b *builder) applyRetirements(source string, updates []update) []string {
	var retired []string
	for _, u := range updates {
		replacement := entryReplacement(u.body)
		for _, gone := range withdrawnIDs(u.body) {
			m := b.model(gone.id, KindChat)
			m.AddSource(source)
			if m.Name == "" {
				m.Name = gone.id
			}
			m.SetAttr(AttrState, StateRetired)
			m.SetAttr(AttrRetirementDate, gone.date)
			m.SetAttr(AttrReplacement, replacement)
			m.AddNote(gone.sentence)
			retired = append(retired, gone.id)
		}
	}
	return retired
}

// withdrawal is one model the changelog says is gone, with the sentence
// saying so and the day it names where it names one.
type withdrawal struct {
	id       string
	date     string
	sentence string
}

// withdrawnIDs returns the models one changelog entry withdraws. Two forms
// occur: a sentence naming the model and what became of it, and a list of bare
// identifiers under a marker line.
func withdrawnIDs(body string) []withdrawal {
	out := listWithdrawals(body)
	for _, sentence := range sentenceRe.Split(body, -1) {
		text := clean(sentence)
		if !retireRe.MatchString(text) {
			continue
		}
		for _, id := range modelIDs(sentence) {
			out = append(out, withdrawal{
				id:       id,
				date:     isoDay(text),
				sentence: text,
			})
		}
	}
	return out
}

// listWithdrawals reads the identifiers listed under a deprecation notice's
// marker line, which run until the first line that is not one.
func listWithdrawals(body string) []withdrawal {
	lines := strings.Split(body, "\n")
	marker := -1
	for i, line := range lines {
		if listMarkerRe.MatchString(line) {
			marker = i
			break
		}
	}
	if marker < 0 {
		return nil
	}
	notice := withdrawalNotice(body)
	var out []withdrawal
	for _, line := range lines[marker+1:] {
		text := strings.TrimSpace(line)
		if text == "" {
			continue
		}
		ids := modelIDs(text)
		if len(ids) != 1 || clean(text) != ids[0] {
			break
		}
		out = append(out, withdrawal{
			id:       ids[0],
			date:     isoDay(notice),
			sentence: notice,
		})
	}
	return out
}

// withdrawalNotice returns the sentence a deprecation notice states its
// withdrawal in, which is the one the date is in too and is not the line
// heading the list of identifiers.
func withdrawalNotice(body string) string {
	for _, sentence := range sentenceRe.Split(body, -1) {
		text := clean(sentence)
		if noLongerRe.MatchString(text) && !listMarkerRe.MatchString(text) {
			return text
		}
	}
	return ""
}

// noLongerRe matches the phrase a deprecation notice withdraws a model with.
var noLongerRe = regexp.MustCompile(`(?i)no longer be (?:accessible|available)`)

// entryReplacement returns the successor a changelog entry recommends, and
// nothing where it recommends more than one, since an entry withdrawing
// several models at once may recommend a different successor for each.
func entryReplacement(body string) string {
	matches := replacementRe.FindAllStringSubmatch(clean(body), -1)
	if len(matches) != 1 {
		return ""
	}
	return matches[0][1]
}

// applyReleases dates the models the changelog announces. The earliest entry
// announcing a model is when it arrived and the latest is when it was last
// written about, and only entries tagged as being about models are read.
func (b *builder) applyReleases(
	source string,
	updates []update,
	retired []string,
) {
	first := map[string]string{}
	last := map[string]string{}
	for _, u := range updates {
		if u.month == "" || !strings.Contains(u.tags, "models") {
			continue
		}
		for _, id := range announcedIDs(u.body) {
			if slices.Contains(retired, id) {
				continue
			}
			if _, ok := b.models[id]; !ok {
				continue
			}
			if prior, ok := first[id]; !ok || u.month < prior {
				first[id] = u.month
			}
			if prior, ok := last[id]; !ok || u.month > prior {
				last[id] = u.month
			}
		}
	}
	for _, id := range slices.Sorted(maps.Keys(first)) {
		m := b.models[id]
		m.AddSource(source)
		m.SetAttr(AttrReleaseDate, first[id])
		m.SetAttr(AttrLastUpdated, last[id])
	}
}

// announcedIDs returns the models one changelog entry announces, read from the
// sentences that state an addition and from the list items that are one.
func announcedIDs(body string) []string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		if !bulletRe.MatchString(strings.TrimSpace(line)) {
			continue
		}
		out = append(out, modelIDs(line)...)
	}
	for _, sentence := range sentenceRe.Split(body, -1) {
		if !launchRe.MatchString(clean(sentence)) {
			continue
		}
		out = append(out, modelIDs(sentence)...)
	}
	return out
}

// modelIDs returns the backticked model identifiers in a fragment of prose.
// The changelog backticks request parameters, response fields and preset names
// the same way it backticks a model, so an identifier counts only where it is
// namespaced under a lab or begins the way one of the families Perplexity
// serves without a namespace begins.
func modelIDs(text string) []string {
	var out []string
	for _, match := range idRe.FindAllStringSubmatch(text, -1) {
		id := match[1]
		if !isModelID(id) || slices.Contains(out, id) {
			continue
		}
		out = append(out, id)
	}
	return out
}

// bareFamilies are the prefixes Perplexity has served a model family under
// without a namespace, which is what makes an unnamespaced identifier one.
var bareFamilies = []string{
	"sonar",
	"pplx-",
	"llama-",
	"mistral-",
	"mixtral-",
	"codellama-",
}

// isModelID reports whether a backticked identifier is a model's.
func isModelID(id string) bool {
	if strings.Contains(id, "/") {
		return strings.Count(id, "/") == 1 && authorOf(id) != ""
	}
	for _, prefix := range bareFamilies {
		if strings.HasPrefix(id, prefix) {
			return true
		}
	}
	return false
}

// applySonarNotice reads the notice every Sonar page carries, which says the
// API is superseded and names the day it stops being served. The model is
// still served until then and the page says so, so it is recorded as legacy
// rather than as deprecated.
func (b *builder) applySonarNotice(m *catalog.Model, doc catalog.Document) {
	match := noticeRe.FindStringSubmatch(string(doc.Body))
	if match == nil {
		return
	}
	m.SetAttr(AttrState, StateLegacy)
	m.SetAttr(AttrRetirementDate, isoDay(match[1]))
}

// applyMigration reads the migration guide's mapping table, which pairs each
// Sonar model with the Agent API preset that replaces it. The successor is a
// preset rather than a model, because the API that replaces Sonar is
// configured rather than chosen.
func (b *builder) applyMigration(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		at := columnOf(t.Headers, "agent api preset")
		if at < 0 {
			continue
		}
		for _, row := range t.Rows {
			m, ok := b.namedSonar(cellAt(row, 0))
			if !ok {
				continue
			}
			preset := clean(cellAt(row, at))
			if preset == "" {
				continue
			}
			m.AddSource(doc.URL)
			m.SetAttr(AttrReplacement, preset+" preset")
		}
	}
}

// namedSonar returns the Sonar model a fragment of text names, which is the
// one whose identifier is the longest the text contains: a heading naming
// Sonar Pro contains the identifier of Sonar too.
func (b *builder) namedSonar(text string) (*catalog.Model, bool) {
	slug := slugID(text)
	var found string
	for _, id := range b.sonar {
		if strings.Contains(slug, id) && len(id) > len(found) {
			found = id
		}
	}
	if found == "" {
		return nil, false
	}
	return b.models[found], true
}

// isoMonth turns a changelog label into a date.
func isoMonth(label string) string {
	match := monthRe.FindStringSubmatch(strings.TrimSpace(label))
	if match == nil {
		return ""
	}
	month, ok := months[strings.ToLower(match[1])]
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s-%02d", match[2], month)
}

// isoDay turns a date written in prose into one. A fragment stating no day, or
// stating one without a year, yields nothing rather than a guess.
func isoDay(text string) string {
	match := dayRe.FindStringSubmatch(text)
	if match == nil {
		return ""
	}
	month, ok := months[strings.ToLower(match[1])]
	if !ok {
		return ""
	}
	day, err := strconv.Atoi(match[2])
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s-%02d-%02d", match[3], month, day)
}
