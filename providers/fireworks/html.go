package fireworks

import (
	"encoding/json"
	"html"
	"regexp"
	"slices"
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

// flightChunkRe matches one piece of the data a rendered page was built from,
// which the page streams to the browser as string literals in script tags.
var flightChunkRe = regexp.MustCompile(
	`(?s)self\.__next_f\.push\(\[1,"(.*?)"\]\)</script>`,
)

// flightRecords returns the records of the data a rendered page was built
// from, keyed by the identifier each is referred to by elsewhere in it. The
// pieces are joined before they are split, since a record can span two of
// them, and each line of the joined stream is one record.
func flightRecords(body string) map[string]string {
	var stream strings.Builder
	for _, m := range flightChunkRe.FindAllStringSubmatch(body, -1) {
		var piece string
		if json.Unmarshal([]byte(`"`+m[1]+`"`), &piece) == nil {
			stream.WriteString(piece)
		}
	}
	out := map[string]string{}
	for _, line := range strings.Split(stream.String(), "\n") {
		if id, value, ok := strings.Cut(line, ":"); ok && id != "" {
			out[id] = value
		}
	}
	return out
}

// flightValues decodes every record of a page's data that is JSON, in a
// stable order. A record of another shape, such as a run of raw text, is
// skipped.
func flightValues(body string) []any {
	records := flightRecords(body)
	ids := make([]string, 0, len(records))
	for id := range records {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	var out []any
	for _, id := range ids {
		var v any
		if json.Unmarshal([]byte(records[id]), &v) == nil {
			out = append(out, v)
		}
	}
	return out
}

// flightObjects returns every object anywhere in a page's data that keep
// accepts, in a stable order. An accepted object is not
// searched further.
func flightObjects(body string, keep func(map[string]any) bool) []map[string]any {
	var out []map[string]any
	var walk func(v any)
	walk = func(v any) {
		switch v := v.(type) {
		case map[string]any:
			if keep(v) {
				out = append(out, v)
				return
			}
			keys := make([]string, 0, len(v))
			for k := range v {
				keys = append(keys, k)
			}
			slices.Sort(keys)
			for _, k := range keys {
				walk(v[k])
			}
		case []any:
			for _, item := range v {
				walk(item)
			}
		}
	}
	for _, v := range flightValues(body) {
		walk(v)
	}
	return out
}

// flightText returns the words a node of a page's data renders. An element is
// written as an array opening with "$" and carrying its properties fourth, and
// what it renders is its children; a string opening with "$" refers to
// another record rather than being text.
func flightText(v any) string {
	switch v := v.(type) {
	case string:
		if strings.HasPrefix(v, "$") {
			return ""
		}
		return v
	case map[string]any:
		return flightText(v["children"])
	case []any:
		if len(v) >= 4 && v[0] == "$" {
			return flightText(v[3])
		}
		var sb strings.Builder
		for _, item := range v {
			sb.WriteString(flightText(item))
		}
		return sb.String()
	}
	return ""
}
