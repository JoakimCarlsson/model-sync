package cerebras

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// ChangeLogURL is where Cerebras dates what it changed, including the day a
// model became callable. It is the only place a release date is written for a
// model the public list gives no creation date for.
const ChangeLogURL = baseURL + "/support/change-log.md"

// DeprecationsURL is where Cerebras dates what it withdrew, most recent
// first, naming the model by identifier and often naming what to move to.
const DeprecationsURL = baseURL + "/support/deprecation.md"

// AttrReplacement names what Cerebras says to move to when it withdraws a
// model. The page names it as a title and not as an identifier, so that is
// what is recorded.
const AttrReplacement = "recommended_replacement"

// Patterns over the dated entries both pages are written as.
var (
	// updateRe matches one dated entry and the prose under it.
	updateRe = regexp.MustCompile(
		`(?s)<Update\s+label="([^"]*)">(.*?)</Update>`,
	)
	// availableRe matches an announcement that a model can now be called,
	// naming it by the identifier in backticks. The wording is narrow on
	// purpose: a later entry saying a model is now selectable in some editor
	// is not the day it was released.
	availableRe = regexp.MustCompile(
		"`([^`]+)`\\)?\\s+is now available",
	)
	// withdrawnRe matches the identifiers of an entry announcing that a model
	// is withdrawn, which names one model or two and nothing else. An entry
	// announcing that a parameter of a model is withdrawn is worded almost the
	// same and says nothing about the model's own standing, so the heading is
	// required to be identifiers and the word joining them.
	withdrawnRe = regexp.MustCompile(
		"(?i)\\*\\*Deprecated\\s+(`[^`]+`(?: and `[^`]+`)*)\\*\\*",
	)
	// backtickRe matches one identifier of such an announcement.
	backtickRe = regexp.MustCompile("`([^`]+)`")
	// migrateRe matches the model an entry recommends instead.
	migrateRe = regexp.MustCompile(
		`(?i)recommend (?:migrating|transitioning) to\s+(.+?)(?:[.,]|$)`,
	)
)

// applyChangeLog reads the change log for the day a model became callable.
//
// The entries are read oldest first, so that a model announced more than once
// keeps the day it first became callable rather than the day it last changed.
func (b *builder) applyChangeLog(doc catalog.Document) {
	entries := updateRe.FindAllStringSubmatch(string(doc.Body), -1)
	for i := len(entries) - 1; i >= 0; i-- {
		date, body := entries[i][1], entries[i][2]
		for _, match := range availableRe.FindAllStringSubmatch(body, -1) {
			m, ok := b.models[strings.TrimSpace(match[1])]
			if !ok {
				continue
			}
			m.SetAttr(AttrReleaseDate, isoDate(date))
			m.AddSource(doc.URL)
		}
	}
}

// applyDeprecationPage reads the deprecation page.
//
// It names a withdrawn model by identifier where the notice on the catalog
// names it by title, and it is the page that says what to move to. Only a
// model some other document has named is touched: the page is a record of what
// Cerebras has withdrawn over the years, and reading it as a list of models
// would fill the catalog with models nobody can call.
func (b *builder) applyDeprecationPage(doc catalog.Document) {
	for _, entry := range updateRe.FindAllStringSubmatch(string(doc.Body), -1) {
		date, body := entry[1], entry[2]
		heading := withdrawnRe.FindStringSubmatch(body)
		if heading == nil {
			continue
		}
		replacement := ""
		if match := migrateRe.FindStringSubmatch(body); match != nil {
			replacement = clean(match[1])
		}
		for _, id := range backtickRe.FindAllStringSubmatch(heading[1], -1) {
			m, ok := b.models[strings.TrimSpace(id[1])]
			if !ok {
				continue
			}
			m.SetAttr(AttrDeprecatedOn, isoDate(date))
			m.SetAttr(AttrReplacement, replacement)
			m.AddSource(doc.URL)
		}
	}
}
