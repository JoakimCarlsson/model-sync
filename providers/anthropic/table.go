package anthropic

import "strings"

// mdTable is one markdown pipe table together with the heading in force above
// it, which is how Anthropic distinguishes its standard, batch and fast mode
// rate tables: the tables themselves look nearly identical.
type mdTable struct {
	Section string
	Headers []string
	Rows    [][]string
	Source  string
}

// scanTables walks a document and returns every pipe table, tracking the
// nearest preceding heading. The YAML frontmatter is skipped so a delimiter
// line in it is never mistaken for content.
func scanTables(body, source string) []mdTable {
	var (
		out     []mdTable
		section string
		current *mdTable
	)
	for _, raw := range strings.Split(skipFrontmatter(body), "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "|") {
			if current == nil {
				out = append(out, mdTable{Section: section, Source: source})
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
		if heading, ok := strings.CutPrefix(line, "#"); ok {
			section = strings.ToLower(
				strings.TrimSpace(strings.TrimLeft(heading, "# ")),
			)
		}
	}
	return out
}

// skipFrontmatter drops the YAML block Anthropic opens every page with.
func skipFrontmatter(body string) string {
	if !strings.HasPrefix(body, "---") {
		return body
	}
	if _, rest, ok := strings.Cut(body, "\n"); ok {
		if _, after, closed := strings.Cut(rest, "\n---"); closed {
			return after
		}
	}
	return body
}

// splitRow splits a pipe row into trimmed cells.
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
