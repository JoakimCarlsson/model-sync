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

// applyChangelogEntry reads the lists of newly served models one entry holds.
func (b *builder) applyChangelogEntry(entry, date, source string) {
	for _, heading := range changelogAddedRe.FindAllStringIndex(entry, -1) {
		rest := entry[heading[1]:]
		if next := changelogHeadingRe.FindStringIndex(rest); next != nil {
			rest = rest[:next[0]]
		}
		for _, match := range changelogModelRe.FindAllStringSubmatch(rest, -1) {
			m, ok := b.models[strings.TrimSpace(match[1])]
			if !ok {
				continue
			}
			m.SetAttr(AttrServerlessSince, date)
			m.AddSource(source)
		}
	}
}
