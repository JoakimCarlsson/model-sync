package openai

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Lifecycle keys the deprecations page populates.
const (
	AttrState          = "state"
	AttrRetirementDate = "retirement_date"
	AttrDeprecatedOn   = "deprecated_on"
	AttrReplacement    = "recommended_replacement"
)

// States OpenAI's lifecycle pages describe. A model is deprecated once its
// removal is announced and shut down once the date has passed, which the page
// records by listing it under upcoming or past rather than by naming a state.
const (
	StateActive     = "active"
	StateDeprecated = "deprecated"
	StateShutdown   = "shutdown"
)

// Sections dividing announcements that have taken effect from those that have
// not.
const (
	sectionUpcoming = "upcoming deprecations"
	sectionPast     = "past deprecations"
)

// idShapeRe matches the shape of an OpenAI model identifier. The deprecations
// tables mix models with products under one heading, listing "Videos API" and
// "OpenAI-Beta: realtime=v1" in the same column as gpt-4o-realtime-preview,
// and the identifiers are not reliably marked as code. Requiring the shape of
// an identifier, which is lowercase and unspaced, separates them.
var idShapeRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._:-]*$`)

// Column headings the deprecations tables use for the same three things.
var (
	dateHeaders  = []string{"shutdown date"}
	modelHeaders = []string{
		"model / system",
		"deprecated model",
		"model snapshot",
		"model family / snapshot",
		"legacy model",
	}
	replacementHeaders = []string{
		"recommended replacement",
		"substitute model",
		"recommended replacement base model",
	}
)

// applyDeprecations reads the deprecation tables.
//
// A model OpenAI has shut down is not carried at all: the past deprecations
// table is read to learn which models those are and they are then left out,
// while the upcoming table marks the models it has merely deprecated, which
// OpenAI still serves and still prices. That is why the table is still read
// rather than skipped; it is also the only place a retirement date is stated
// for a model still on sale.
//
// Deprecated snapshots become entries of their own rather than marking the
// alias they belong to. OpenAI deprecates gpt-5-2025-08-07 while gpt-5 remains
// current, so writing the snapshot's state onto gpt-5 would report a live
// model as going away.
//
// The price column some of these tables carry is not read. It states what a
// model cost when it was withdrawn, and mixing that into current rates would
// be indistinguishable from a rate still on offer.
func (b *builder) applyDeprecations(doc catalog.Document) {
	for _, t := range scanDeprecationTables(doc) {
		state := ""
		switch t.Section {
		case sectionUpcoming:
			state = StateDeprecated
		case sectionPast:
			state = StateShutdown
		default:
			continue
		}
		dateCol := columnOf(t.Headers, dateHeaders)
		modelCol := columnOf(t.Headers, modelHeaders)
		if dateCol < 0 || modelCol < 0 {
			continue
		}
		replCol := columnOf(t.Headers, replacementHeaders)
		for _, row := range t.Rows {
			b.applyDeprecationRow(t, row, state, dateCol, modelCol, replCol)
		}
	}
}

// applyDeprecationRow records one withdrawal. A row from the past deprecations
// table names a model OpenAI no longer serves, which is withdrawn rather than
// recorded: nothing it states about such a model describes something still on
// sale.
func (b *builder) applyDeprecationRow(
	t deprecationTable,
	row []string,
	state string,
	dateCol, modelCol, replCol int,
) {
	id := unquote(cellAt(row, modelCol))
	if !idShapeRe.MatchString(id) || b.statedDeprecation(id) {
		return
	}
	if state == StateShutdown {
		b.withdraw(id)
		return
	}
	m := b.model(id, "")
	m.AddSource(t.Source)
	m.SetAttr(AttrState, state)
	m.SetAttr(AttrRetirementDate, isoDate(cellAt(row, dateCol)))
	m.SetAttr(AttrDeprecatedOn, t.Announced)
	if replCol >= 0 {
		m.SetAttr(AttrReplacement, unquote(cellAt(row, replCol)))
	}
}

// statedDeprecation reports whether a deprecation table has already said what
// became of a model. The two sections overlap: gpt-4-1106-preview is listed as
// an upcoming deprecation and again among the past ones, with different dates
// and different replacements. The row read first is the one that stands, which
// is the rule the scalar attributes already follow.
func (b *builder) statedDeprecation(id string) bool {
	if b.withdrawn[id] {
		return true
	}
	m, ok := b.models[id]
	return ok && m.Attrs[AttrState] != ""
}

// deprecationTable is one table together with the top level section it sits
// under, which is what says whether the withdrawal has happened yet, and the
// date of the announcement it appears under.
type deprecationTable struct {
	Section   string
	Announced string
	Headers   []string
	Rows      [][]string
	Source    string
}

// announcementRe matches the heading each announcement sits under, which opens
// with the date OpenAI gave the notice: "### 2026-07-20: Legacy audio,
// realtime, and transcription models". The prose under it repeats the same
// date in words, saying that on that day developers using the models were
// notified of their deprecation. A few headings carry no date, and a table
// under one of those records none.
var announcementRe = regexp.MustCompile(`^### (\d{4}-\d{2}-\d{2}):`)

// scanDeprecationTables walks the page, tracking the section in force.
func scanDeprecationTables(doc catalog.Document) []deprecationTable {
	var (
		out       []deprecationTable
		section   string
		announced string
		current   *deprecationTable
	)
	for _, raw := range strings.Split(string(doc.Body), "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "|") {
			if current == nil {
				out = append(out, deprecationTable{
					Section:   section,
					Announced: announced,
					Source:    doc.URL,
				})
				current = &out[len(out)-1]
			}
			cells := splitRow(line)
			switch {
			case current.Headers == nil:
				current.Headers = cells
			case isSeparator(cells):
			default:
				current.Rows = append(current.Rows, cells)
			}
			continue
		}
		current = nil
		if after, ok := strings.CutPrefix(line, "## "); ok {
			section, announced = strings.ToLower(strings.TrimSpace(after)), ""
		}
		if match := announcementRe.FindStringSubmatch(line); match != nil {
			announced = match[1]
			continue
		}
		if strings.HasPrefix(line, "### ") {
			announced = ""
		}
	}
	return out
}

// columnOf returns the index of the first column matching any of the headers a
// table might use for one thing, or -1.
func columnOf(headers, wanted []string) int {
	for i, h := range headers {
		normalized := strings.ToLower(strings.Join(strings.Fields(h), " "))
		for _, w := range wanted {
			if normalized == w {
				return i
			}
		}
	}
	return -1
}

// unquote strips the code markers and any trailing parenthetical from a cell
// naming a model, so that a replacement written as "`gpt-5.6-sol`
// (`reasoning.mode: pro`)" yields the identifier alone. A cell holding nothing
// but a rule yields nothing: OpenAI writes one where it withdraws a model
// without naming anything to move to, as it does for the Sora models, and
// reading it as a name would report a replacement called "---".
func unquote(cell string) string {
	text := strings.TrimSpace(strings.ReplaceAll(cell, "`", ""))
	if strings.Trim(hyphens.Replace(text), "-") == "" {
		return ""
	}
	if open := strings.Index(text, " ("); open >= 0 {
		text = text[:open]
	}
	return strings.TrimSpace(text)
}
