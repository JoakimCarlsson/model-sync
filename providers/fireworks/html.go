package fireworks

import (
	"html"
	"regexp"
	"strings"
)

// The model library and the marketing pricing page are rendered rather than
// served as data, so what they state has to be read off the markup. These are
// the pieces every reader of those two pages shares.
var (
	tagRe     = regexp.MustCompile(`<[^>]*>`)
	commentRe = regexp.MustCompile(`<!--.*?-->`)
	rowRe     = regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
	cellRe    = regexp.MustCompile(`(?s)<t[dh][^>]*>(.*?)</t[dh]>`)
)

// text reduces a run of markup to the words it renders, which is what a reader
// of the page sees and therefore what Fireworks can be said to have stated.
func text(markup string) string {
	s := commentRe.ReplaceAllString(markup, "")
	s = tagRe.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(html.UnescapeString(s)), " ")
}

// htmlTable is one rendered table, read as rows of rendered cells.
type htmlTable struct {
	Headers []string
	Rows    [][]string
}

// scanHTMLTables returns every table in a rendered page. The first row of a
// table is taken as its header when it is the one the page marked up with
// header cells, which is how the pricing page distinguishes the column titles
// from the rates under them.
func scanHTMLTables(body string) []htmlTable {
	var out []htmlTable
	for _, table := range splitTables(body) {
		var t htmlTable
		for i, row := range rowRe.FindAllStringSubmatch(table, -1) {
			cells := make([]string, 0, 8)
			for _, cell := range cellRe.FindAllStringSubmatch(row[1], -1) {
				cells = append(cells, text(cell[1]))
			}
			if len(cells) == 0 {
				continue
			}
			if i == 0 && strings.Contains(row[1], "<th") {
				t.Headers = cells
				continue
			}
			t.Rows = append(t.Rows, cells)
		}
		if len(t.Rows) > 0 {
			out = append(out, t)
		}
	}
	return out
}

// tableRe matches one rendered table.
var tableRe = regexp.MustCompile(`(?s)<table[^>]*>(.*?)</table>`)

// splitTables returns the body of every table on a page.
func splitTables(body string) []string {
	matches := tableRe.FindAllStringSubmatch(body, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m[1])
	}
	return out
}
