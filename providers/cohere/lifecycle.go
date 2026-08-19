package cohere

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// DeprecationsURL announces the models Cohere has withdrawn and when. The
// overview states a standing only in the tables that have a status column, and
// the tables of platform identifiers have none, so this is the only place the
// standing of a model listed nowhere else is written.
const DeprecationsURL = "https://docs.cohere.com/docs/deprecations.md"

var (
	// announcementRe matches the heading of one dated announcement. Cohere
	// heads each with the date it takes effect, which is the date the
	// sentences below repeat in prose.
	//
	// Only the heading is matched, and the body is taken as the text up to the
	// next one. A pattern that matched the body as well would have to consume
	// the heading that ends it, which would leave every second announcement
	// unread.
	announcementRe = regexp.MustCompile(
		`(?m)^###\s+(\d{4}-\d{2}-\d{2})[^\n]*$`,
	)
	// identifierRe matches one identifier inside a bullet, which Cohere writes
	// in code style. A bullet names more than one where a version and the
	// alias pointing at it go out together.
	identifierRe = regexp.MustCompile("`([^`]+)`")
)

// withdrawal is one phrase an announcement introduces a list of models with,
// and the standing it puts them in.
type withdrawal struct {
	List  *regexp.Regexp
	State string
	Date  string
}

// withdrawals are the two ways an announcement lists what it withdraws. Both
// are matched rather than assumed, so an announcement worded some third way
// yields nothing instead of a standing read off the wrong list.
var withdrawals = []withdrawal{
	{bulletsAfter("will be retired:"), StateRetired, AttrRetirementDate},
	{bulletsAfter("Deprecated Models:"), StateDeprecated, AttrDeprecatedOn},
}

// bulletsAfter matches the run of bullets an introducing phrase is followed
// by.
func bulletsAfter(intro string) *regexp.Regexp {
	return regexp.MustCompile(
		`(?s)` + regexp.QuoteMeta(intro) + `\s*\n\n((?:\*[^\n]*\n)+)`,
	)
}

// served reports whether Cohere still answers to a model. A model it does not
// is left out of the catalog entirely, since there is no rate to find for it
// and nothing left to call.
//
// Deprecated is not one of those standings. Cohere still serves a deprecated
// model to the customers already using it and still states its rate, so it is a
// model with a date on it rather than a model that is gone.
//
// The standing is matched as a prefix, because the overview's status column
// writes it as a word followed by the date it takes effect.
func served(m *catalog.Model) bool {
	state := strings.ToLower(m.Attrs[AttrState])
	for _, gone := range []string{StateRetired, StateShutdown} {
		if strings.HasPrefix(state, gone) {
			return false
		}
	}
	return true
}

// applyLifecycle records the standing of every model an announcement names.
//
// Only a model the overview established is reached, and only a standing it did
// not already state is recorded: where both documents speak they agree, and
// the overview is the authority on the models it lists. What this adds is the
// models the overview lists only in a table of platform identifiers, which has
// no status column: the three second generation embedders were retired in
// April 2026 and their standing is stated nowhere else, so without this they
// would read as live and be published as models anyone can still call.
func (b *builder) applyLifecycle(doc catalog.Document) {
	body := string(doc.Body)
	found := announcementRe.FindAllStringSubmatchIndex(body, -1)
	for i, at := range found {
		end := len(body)
		if i+1 < len(found) {
			end = found[i+1][0]
		}
		date := body[at[2]:at[3]]
		announcement := body[at[1]:end]
		for _, w := range withdrawals {
			list := w.List.FindStringSubmatch(announcement)
			if list == nil {
				continue
			}
			b.withdraw(doc, list[1], w, date)
			b.applyReplacements(doc, announcement, list[1])
		}
	}
}

// ListReplacements enumerates what Cohere recommends instead of a model it is
// withdrawing.
const ListReplacements = "recommended_replacements"

// replacementRe matches the sentence an announcement recommends a replacement
// in. Cohere names more than one and ranks them loosely, "or command-a-03-2025
// (which is the strongest-performing model across domains)", so all of them are
// recorded and none is promoted to the recommendation.
var replacementRe = regexp.MustCompile(
	"(?i)we recommend you use ((?:[^.]*?`[^`]+`)+[^.]*)\\.",
)

// applyReplacements records what an announcement recommends instead of the
// models it withdraws.
func (b *builder) applyReplacements(
	doc catalog.Document,
	announcement, bullets string,
) {
	match := replacementRe.FindStringSubmatch(announcement)
	if match == nil {
		return
	}
	var named []string
	for _, id := range identifierRe.FindAllStringSubmatch(match[1], -1) {
		if _, ok := b.models[id[1]]; ok {
			named = append(named, id[1])
		}
	}
	for _, withdrawn := range identifierRe.FindAllStringSubmatch(bullets, -1) {
		m, ok := b.models[withdrawn[1]]
		if !ok {
			continue
		}
		m.AddList(ListReplacements, named...)
		m.AddSource(doc.URL)
	}
}

// withdraw applies one announcement's standing to the models its bullets name.
func (b *builder) withdraw(
	doc catalog.Document,
	bullets string,
	w withdrawal,
	date string,
) {
	for _, match := range identifierRe.FindAllStringSubmatch(bullets, -1) {
		m, ok := b.models[match[1]]
		if !ok {
			continue
		}
		m.SetAttr(AttrState, w.State)
		m.SetAttr(w.Date, date)
		m.AddSource(doc.URL)
	}
}
