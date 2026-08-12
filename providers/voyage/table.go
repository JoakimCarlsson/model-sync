package voyage

import (
	"regexp"
	"strings"
)

// table is one table together with the heading in force above it, which is
// what tells a text embedding rate table from a reranker one.
type table struct {
	Section string
	Headers []string
	Rows    [][]string
	Source  string
}

var (
	htmlTableRe = regexp.MustCompile(`(?is)<table\b.*?</table>`)
	htmlRowRe   = regexp.MustCompile(`(?is)<tr\b[^>]*>(.*?)</tr>`)
	htmlCellRe  = regexp.MustCompile(`(?is)<(t[dh])\b[^>]*>(.*?)</t[dh]\s*>`)
)

// scanTables returns every table in a document, in both notations Voyage
// writes them in. Its reranker page states the model table as HTML while every
// other page uses markdown, so a reader of one notation would silently miss
// every reranker.
func scanTables(body, source string) []table {
	out := scanMarkdownTables(body, source)
	return append(out, scanHTMLTables(body, source)...)
}

// scanMarkdownTables walks a document and returns every pipe table, tracking
// the nearest preceding heading.
func scanMarkdownTables(body, source string) []table {
	var (
		out     []table
		section string
		current *table
	)
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "|") {
			if current == nil {
				out = append(out, table{Section: section, Source: source})
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
		if after, ok := strings.CutPrefix(line, "#"); ok {
			section = headingName(after)
		}
	}
	return out
}

// scanHTMLTables returns every HTML table, with the heading above it.
func scanHTMLTables(body, source string) []table {
	var out []table
	for _, at := range htmlTableRe.FindAllStringIndex(body, -1) {
		t := parseHTMLTable(body[at[0]:at[1]])
		if len(t.Headers) == 0 || len(t.Rows) == 0 {
			continue
		}
		t.Section = headingBefore(body[:at[0]])
		t.Source = source
		out = append(out, t)
	}
	return out
}

// parseHTMLTable reads one HTML table.
func parseHTMLTable(block string) table {
	var t table
	for _, row := range htmlRowRe.FindAllStringSubmatch(block, -1) {
		var cells []string
		header := false
		for _, cell := range htmlCellRe.FindAllStringSubmatch(row[1], -1) {
			header = header || strings.EqualFold(cell[1], "th")
			cells = append(cells, clean(cell[2]))
		}
		switch {
		case len(cells) == 0:
		case t.Headers == nil && header:
			t.Headers = cells
		default:
			t.Rows = append(t.Rows, cells)
		}
	}
	return t
}

// headingBefore returns the last heading appearing in a passage.
func headingBefore(body string) string {
	section := ""
	for _, raw := range strings.Split(body, "\n") {
		if after, ok := strings.CutPrefix(strings.TrimSpace(raw), "#"); ok {
			section = headingName(after)
		}
	}
	return section
}

// headingName normalizes a heading to compare against.
func headingName(heading string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimLeft(heading, "# ")))
}

// splitRow splits a pipe row into cells, keeping the raw text so a line break
// element separating two model names survives.
func splitRow(line string) []string {
	parts := strings.Split(line, "|")
	if len(parts) > 0 && strings.TrimSpace(parts[0]) == "" {
		parts = parts[1:]
	}
	if len(parts) > 0 && strings.TrimSpace(parts[len(parts)-1]) == "" {
		parts = parts[:len(parts)-1]
	}
	cells := make([]string, len(parts))
	for i, p := range parts {
		cells[i] = strings.TrimSpace(p)
	}
	return cells
}

// isSeparator reports whether a row is the dashed rule under a header.
func isSeparator(cells []string) bool {
	for _, c := range cells {
		if strings.Trim(c, ":- ") != "" {
			return false
		}
	}
	return len(cells) > 0
}

// cellAt returns a row's cell, tolerating rows shorter than the header.
func cellAt(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

// columnOf returns the index of the first column whose heading matches any of
// the given names, or -1.
func columnOf(headers []string, names ...string) int {
	for i, h := range headers {
		normalized := strings.ToLower(clean(h))
		for _, n := range names {
			if normalized == n {
				return i
			}
		}
	}
	return -1
}
