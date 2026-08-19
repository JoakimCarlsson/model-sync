package bedrock

import (
	"regexp"
	"strings"
)

// table is one of a document's markdown tables, with the bold line standing
// above it. AWS states most of a model's facts in a table, and says which
// table it is in the headings rather than in a heading of its own, so the
// headings are what a reader dispatches on. The caption matters only where
// one section holds several tables that differ in nothing else: a card's
// pricing section writes one table per context band and one per Region it
// prices apart, and names each in the bold line above it.
type table struct {
	caption  string
	headings []string
	rows     [][]string
}

// boldLineRe matches a line that is nothing but bold text, which is how AWS
// captions a table.
var boldLineRe = regexp.MustCompile(`^\*{2,3}([^*]+)\*{2,3}\s*$`)

// rowLineRe matches one row of a table.
var rowLineRe = regexp.MustCompile(`^\|(.*)\|\s*$`)

// dividerRe matches the row separating a table's headings from its body.
var dividerRe = regexp.MustCompile(`^[\s|:-]+$`)

// parseTables reads every table in a document.
//
// A table is recognized by the row of dashes markdown puts under its
// headings, rather than by the headings being written in bold, because AWS
// writes them in bold on a model card and in plain text on the pages listing
// many models at once.
func parseTables(body string) []table {
	var tables []table
	var current *table
	caption := ""
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		match := rowLineRe.FindStringSubmatch(trimmed)
		if match == nil {
			if current != nil {
				tables = append(tables, *current)
				current = nil
			}
			if strings.HasPrefix(trimmed, "#") {
				caption = ""
			}
			if bold := boldLineRe.FindStringSubmatch(trimmed); bold != nil {
				caption = strings.TrimSpace(linkText(bold[1]))
			}
			continue
		}
		if dividerRe.MatchString(match[1]) {
			continue
		}
		if heads(lines, i) {
			if current != nil {
				tables = append(tables, *current)
			}
			current = &table{
				caption:  caption,
				headings: tableHeadings(strings.Split(match[1], "|")),
			}
			caption = ""
			continue
		}
		if current == nil {
			continue
		}
		current.rows = append(current.rows, strings.Split(match[1], "|"))
	}
	if current != nil {
		tables = append(tables, *current)
	}
	return tables
}

// tableHeadings reduces a heading row to the words each column is headed
// with, since AWS writes a heading in bold on a model card and in plain text
// on the pages listing many models at once, and reads them lowercased because
// nothing downstream depends on how a heading is capitalized.
func tableHeadings(cells []string) []string {
	headings := make([]string, len(cells))
	for i, cell := range cells {
		text := strings.Trim(strings.TrimSpace(cell), "* ")
		headings[i] = strings.ToLower(linkText(text))
	}
	return headings
}

// heads reports whether the row on line i heads its table, which it does when
// the row of dashes stands under it.
func heads(lines []string, i int) bool {
	if i+1 >= len(lines) {
		return false
	}
	next := rowLineRe.FindStringSubmatch(strings.TrimSpace(lines[i+1]))
	return next != nil && dividerRe.MatchString(next[1])
}

// heading returns the heading of column i, or the empty string where the row
// has more cells than the table has headings.
func (t table) heading(i int) string {
	if i < 0 || i >= len(t.headings) {
		return ""
	}
	return t.headings[i]
}

// headed reports whether the table's first column is headed as given, which
// is how the tables of a model card are told apart.
func (t table) headed(name string) bool {
	return t.heading(0) == name
}

// hasHeading reports whether any column carries the heading.
func (t table) hasHeading(name string) bool {
	for _, h := range t.headings {
		if h == name {
			return true
		}
	}
	return false
}

// cell returns the text of a row's column, with its links reduced to what
// they read as.
func cell(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return linkText(row[i])
}

// entries splits a cell listing several values, which AWS writes either as
// one bulleted line broken by markup or as a comma-separated phrase.
func entries(value string) []string {
	value = strings.ReplaceAll(value, "<br />", "\n")
	value = strings.ReplaceAll(value, "<br/>", "\n")
	var out []string
	for _, part := range strings.FieldsFunc(value, func(r rune) bool {
		return r == '\n' || r == ',' || r == ';'
	}) {
		part = strings.TrimSpace(strings.TrimPrefix(
			strings.TrimSpace(part),
			"+",
		))
		part = strings.TrimPrefix(part, "and ")
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
