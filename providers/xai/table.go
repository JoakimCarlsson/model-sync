package xai

import "strings"

// mdTable is one markdown pipe table together with the heading above it, which
// is what distinguishes xAI's four rate tables from each other.
type mdTable struct {
	Section string
	Headers []string
	Rows    [][]string
	Source  string
}

// scanTables walks a document and returns every pipe table, tracking the
// nearest preceding heading.
func scanTables(body, source string) []mdTable {
	var (
		out     []mdTable
		section string
		current *mdTable
	)
	for _, raw := range strings.Split(body, "\n") {
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
		if after, ok := strings.CutPrefix(line, "#"); ok {
			section = strings.ToLower(
				strings.TrimSpace(strings.TrimLeft(after, "# ")),
			)
		}
	}
	return out
}

// splitRow splits a pipe row into trimmed cells, keeping the raw text so that
// a line break element separating two rates in one cell survives.
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

// columnOf returns the index of the first column whose cleaned heading matches
// any of the given names, or -1.
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
