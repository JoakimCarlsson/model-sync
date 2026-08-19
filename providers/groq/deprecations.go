package groq

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// DeprecationsURL is the page listing every model Groq has announced a
// shutdown date for.
const DeprecationsURL = baseURL + "/docs/deprecations.md"

var (
	// announcedRe matches the sentence saying when Groq told its users, which
	// is the date the deprecation was announced rather than the date the model
	// stops answering.
	announcedRe = regexp.MustCompile(
		`(?i)on ([A-Z][a-z]+ \d{1,2}, \d{4}), we emailed`,
	)
	// appliesRe matches the sentence qualifying who a deprecation reaches,
	// which some announcements carry and some do not.
	appliesRe = regexp.MustCompile(`This deprecation applies to[^.]*\.`)
)

// Headings the deprecation tables are written under.
const (
	colDeprecated  = "deprecated model"
	colModelID     = "model id"
	colShutdown    = "shutdown date"
	colReplacement = "recommended replacement model id"
)

// applyDeprecations reads the deprecation page.
//
// The page is a section per announcement: a heading naming the day, a
// paragraph saying when Groq emailed its users and whom the deprecation
// reaches, and a table of the models, their shutdown dates and what to move
// to. The models are read from the table and the two dates apart, because the
// paragraph states the announcement and only the table states the shutdown.
//
// A model here is generally gone from the supported models page, so this is
// the only document naming it, and the section is read whether or not the
// table listed it.
func (b *builder) applyDeprecations(doc catalog.Document) {
	for _, s := range deprecationSections(string(doc.Body)) {
		announced := parseLongDate(firstOf(announcedRe, s.prose))
		applies := strings.TrimSpace(appliesRe.FindString(s.prose))
		for _, t := range s.tables {
			b.applyDeprecationTable(t, doc.URL, announced, applies)
		}
	}
}

// applyDeprecationTable reads one announcement's table.
func (b *builder) applyDeprecationTable(
	t table,
	source, announced, applies string,
) {
	idCol := columnOf(t.Headers, colDeprecated)
	if idCol < 0 {
		idCol = columnOf(t.Headers, colModelID)
	}
	dateCol := columnOf(t.Headers, colShutdown)
	if idCol < 0 || dateCol < 0 {
		return
	}
	replaceCol := columnOf(t.Headers, colReplacement)
	for _, row := range t.Rows {
		id := clean(cellAt(row, idCol))
		if id == "" {
			continue
		}
		m := b.model(id, "")
		m.AddSource(source)
		m.SetAttr(AttrState, StateDeprecated)
		m.SetAttr(AttrRetirementDate, parseShortDate(cellAt(row, dateCol)))
		m.SetAttr(AttrDeprecatedOn, announced)
		m.SetAttr(AttrReplacement, clean(cellAt(row, replaceCol)))
		m.AddNote(applies)
	}
}

// deprecationSection is one announcement: the prose under its heading and the
// tables that follow it.
type deprecationSection struct {
	prose  string
	tables []table
}

// deprecationSections divides the page. A heading opens a section and the
// prose runs to the first table, which is why the two are gathered together
// rather than by scanning the whole document for tables: the announcement date
// and the qualification belong to the models in the table below them and to no
// others.
func deprecationSections(body string) []deprecationSection {
	var (
		out     []deprecationSection
		current *deprecationSection
		prose   []string
		rows    *table
	)
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "#") {
			out = append(out, deprecationSection{})
			current = &out[len(out)-1]
			prose = nil
			rows = nil
			continue
		}
		if current == nil {
			continue
		}
		if !strings.HasPrefix(line, "|") {
			rows = nil
			if line != "" {
				prose = append(prose, line)
				current.prose = strings.Join(prose, " ")
			}
			continue
		}
		if rows == nil {
			current.tables = append(current.tables, table{})
			rows = &current.tables[len(current.tables)-1]
		}
		cells := splitRow(line)
		switch {
		case rows.Headers == nil:
			rows.Headers = cells
		case isSeparator(cells):
		default:
			rows.Rows = append(rows.Rows, cells)
		}
	}
	return out
}
