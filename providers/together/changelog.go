package together

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// ChangelogURL is the page Together announces each change on, dated entry by
// dated entry.
const ChangelogURL = "https://docs.together.ai/docs/changelog.md"

// changelogEntryRe matches the opening of one dated entry. The date is the
// label, written the same way the model library writes one.
var changelogEntryRe = regexp.MustCompile(`<Update label="([^"]+)"`)

// changelogAddedRe matches the heading an entry lists newly served models
// under. Several other headings list models too, for the fine-tuning service
// and for dedicated inference, and those are a different fact about a
// different product.
var changelogAddedRe = regexp.MustCompile(
	`(?mi)^\s*#+\s*New serverless models\s*$`,
)

// changelogRemovedRe matches the heading an entry lists withdrawn models
// under, which the page writes for models and for image models separately.
//
// The lifecycle page is meant to collect these, and does not collect them all:
// LiquidAI/LFM2-24B-A2B was announced as withdrawn from serverless inference
// here on July 9, 2026 and has no row in the removal table. So the entries are
// read too, and the day the entry is dated is the day the model went.
var changelogRemovedRe = regexp.MustCompile(
	`(?mi)^\s*#+\s*(?:\w+ )?model deprecations?\s*$`,
)

// changelogReplacementRe matches the model an entry names as the migration
// path, which it writes as a sentence after the identifier.
var changelogReplacementRe = regexp.MustCompile(
	"(?i)recommended replacement:" + `\s*` + "`([^`]+)`",
)

// changelogHeadingRe matches any heading, which is what ends the list a
// heading opened.
var changelogHeadingRe = regexp.MustCompile(`(?m)^\s*#+\s`)

// changelogModelRe matches one bullet of such a list. Together opens each with
// the identifier the API answers to and follows it with whatever it wants to
// say about the model, so only the identifier is read here.
var changelogModelRe = regexp.MustCompile("(?m)^\\s*\\*\\s+`([^`]+)`")

// applyChangelog reads the changelog for the day each model started being
// served.
//
// This is not the date the model was released, which its library page states
// and which is usually earlier: a model exists before Together carries it. It
// is the day Together began selling it per token, which nothing else states
// and which the entries under the new-serverless-models heading say exactly.
//
// The entries are walked oldest first so that a model listed twice keeps the
// first day it appeared. Everything else the changelog holds is deliberately
// not read: its price changes are superseded by the catalog page, which is
// current, and its deprecation notices are superseded by the lifecycle page,
// which collects them all in one table instead of scattering them across
// entries.
//
// An entry naming a model no other document names still creates it. Together
// announced MiniMax M2.5, M2.7 and LFM2-24B-A2B as newly served and then
// stopped listing them, without giving any of the three a removal date, so the
// announcement is the only record that Together ever sold them. Carried, they
// have a date and a note saying that the catalog page no longer lists them;
// left out, a consumer with a request naming one gets the same silence as for
// a model this catalog never read.
func (b *builder) applyChangelog(doc catalog.Document) {
	body := string(doc.Body)
	entries := changelogEntryRe.FindAllStringSubmatchIndex(body, -1)
	for i := len(entries) - 1; i >= 0; i-- {
		date := parseDate(body[entries[i][2]:entries[i][3]])
		if date == "" {
			continue
		}
		end := len(body)
		if i+1 < len(entries) {
			end = entries[i+1][0]
		}
		b.applyChangelogEntry(body[entries[i][1]:end], date, doc.URL)
	}
}

// noteUnlisted says what a model with an announcement and nothing else is: one
// Together served and no longer lists, and did not put in the removal table.
const noteUnlisted = "announced as a serverless model on the changelog; the " +
	"catalog page no longer lists it and the lifecycle page gives it no " +
	"removal date"

// applyUnlisted marks the models only the changelog names.
//
// It runs after every document, because what makes one of these models
// unlisted is the absence of everything else: no rate from the catalog page,
// and no removal date from the lifecycle page.
func (b *builder) applyUnlisted() {
	for _, id := range b.order {
		m := b.models[id]
		if m.Attrs[AttrServerlessSince] == "" || len(m.Prices) > 0 ||
			m.Attrs[AttrRetirementDate] != "" {
			continue
		}
		m.AddNote(noteUnlisted)
	}
}

// applyChangelogEntry reads the lists of newly served and newly withdrawn
// models one entry holds.
func (b *builder) applyChangelogEntry(entry, date, source string) {
	for _, bullet := range entryBullets(entry, changelogAddedRe) {
		m := b.model(bullet.id, "")
		m.SetAttr(AttrServerlessSince, date)
		m.AddSource(source)
	}
	for _, bullet := range entryBullets(entry, changelogRemovedRe) {
		m := b.model(bullet.id, "")
		m.SetAttr(AttrState, StateRetired)
		m.SetAttr(AttrRetirementDate, date)
		m.SetAttr(AttrReplacement, bullet.replacement)
		m.AddSource(source)
	}
}

// bullet is one model an entry lists, with whatever the sentence after it
// names as its replacement.
type bullet struct {
	id          string
	replacement string
}

// entryBullets returns the models listed under every heading of one kind in
// one entry.
func entryBullets(entry string, heading *regexp.Regexp) []bullet {
	var out []bullet
	for _, at := range heading.FindAllStringIndex(entry, -1) {
		rest := entry[at[1]:]
		if next := changelogHeadingRe.FindStringIndex(rest); next != nil {
			rest = rest[:next[0]]
		}
		for _, line := range strings.Split(rest, "\n") {
			match := changelogModelRe.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			id := strings.TrimSpace(match[1])
			if id == "" {
				continue
			}
			found := bullet{id: id}
			if to := changelogReplacementRe.FindStringSubmatch(
				line,
			); to != nil {
				found.replacement = strings.TrimSpace(to[1])
			}
			out = append(out, found)
		}
	}
	return out
}
