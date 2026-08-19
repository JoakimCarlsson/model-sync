package google

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Keys the deprecation schedule populates.
const (
	AttrReleaseDate     = "release_date"
	AttrRetirementDate  = "retirement_date"
	AttrReplacement     = "recommended_replacement"
	AttrKnowledgeCutoff = "knowledge_cutoff"
)

// States the deprecation schedule distinguishes. It divides each of its tables
// in two under a heading of its own, and its opening paragraph says which two:
// "stable (GA) and preview models". Nothing above the divider is marked, so a
// row above it is the GA half of that sentence.
const (
	StateActive  = "active"
	StatePreview = "preview"
)

// previewDivider is the one-cell row dividing a table's GA half from its
// preview half.
const previewDivider = "preview models"

// grayRow is the class the schedule marks a model it has already shut down
// with, which its opening paragraph states in as many words. Such a row is
// read for its dates and not for its state: the model index goes on listing
// two of them without the aside it hangs off a withdrawn model, and the index
// is the document that says what the API still answers to.
const grayRow = "row-gray"

var (
	// scheduleTableRe matches one family's schedule.
	scheduleTableRe = regexp.MustCompile(
		`(?is)<table class="pricing-table">(.*?)</table>`,
	)
	// scheduleRowRe keeps a row's attributes, which is where the schedule says
	// a model has already been shut down.
	scheduleRowRe = regexp.MustCompile(`(?is)<tr([^>]*)>(.*?)</tr>`)
	// dayDateRe and monthDateRe match the two precisions the schedule writes a
	// date in. Google states a day for almost every model and a bare month for
	// the newest, and neither is rounded to the other.
	dayDateRe = regexp.MustCompile(
		`^([A-Za-z]+)\s+(\d{1,2}),\s*(\d{4})$`,
	)
	monthDateRe = regexp.MustCompile(`^([A-Za-z]+)\s+(\d{4})$`)
)

// months maps the name Google writes a month under onto its number.
var months = map[string]string{
	"january":   "01",
	"february":  "02",
	"march":     "03",
	"april":     "04",
	"may":       "05",
	"june":      "06",
	"july":      "07",
	"august":    "08",
	"september": "09",
	"october":   "10",
	"november":  "11",
	"december":  "12",
}

// applyDeprecations reads the deprecation schedule, which is the only document
// stating when a model was released, when it may be shut down and what Google
// would have a caller move to. It is also the only one drawing the line
// between a model that is generally available and one still in preview, which
// every other document leaves to be inferred from an identifier.
//
// Each table is one family and is read in order, because the two halves are
// divided by a row rather than by a column.
func (b *builder) applyDeprecations(doc catalog.Document) {
	for _, table := range scheduleTableRe.FindAllStringSubmatch(
		string(doc.Body),
		-1,
	) {
		state := StateActive
		for _, row := range scheduleRowRe.FindAllStringSubmatch(table[1], -1) {
			cells := pageCellRe.FindAllStringSubmatch(row[2], -1)
			if len(cells) == 1 {
				if strings.EqualFold(text(cells[0][1]), previewDivider) {
					state = StatePreview
				}
				continue
			}
			if len(cells) < 3 {
				continue
			}
			b.applySchedule(
				cells,
				state,
				strings.Contains(row[1], grayRow),
				doc.URL,
			)
		}
	}
}

// applySchedule records one row of the schedule onto the model it names.
func (b *builder) applySchedule(
	cells [][]string,
	state string,
	shutdown bool,
	src string,
) {
	codes := codesIn(cells[0][1])
	if len(codes) == 0 {
		return
	}
	m := b.models[codes[0]]
	if m == nil {
		return
	}
	m.AddSource(src)
	m.SetAttr(AttrReleaseDate, isoDate(text(cells[1][1])))
	m.SetAttr(AttrRetirementDate, isoDate(text(cells[2][1])))
	if len(cells) > 3 {
		if replacements := codesIn(cells[3][1]); len(replacements) > 0 {
			m.SetAttr(AttrReplacement, replacements[0])
		}
	}
	if !shutdown {
		m.SetAttr(AttrState, state)
	}
}

// isoDate rewrites a date the way the schedule states it, keeping the
// precision Google published rather than filling a day in. A cell saying no
// date has been announced states no date, and yields nothing.
func isoDate(value string) string {
	trimmed := strings.TrimSpace(value)
	if match := dayDateRe.FindStringSubmatch(trimmed); match != nil {
		month, ok := months[strings.ToLower(match[1])]
		if !ok {
			return ""
		}
		day := match[2]
		if len(day) == 1 {
			day = "0" + day
		}
		return match[3] + "-" + month + "-" + day
	}
	if match := monthDateRe.FindStringSubmatch(trimmed); match != nil {
		month, ok := months[strings.ToLower(match[1])]
		if !ok {
			return ""
		}
		return match[2] + "-" + month
	}
	return ""
}
