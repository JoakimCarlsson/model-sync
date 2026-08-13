package cerebras

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// KindChat is the only kind Cerebras serves.
const KindChat catalog.Kind = "chat"

// States Cerebras distinguishes by which table a model appears in.
const (
	StateProduction = "production"
	StatePreview    = "preview"
)

// Scalar keys the catalog populates.
const (
	AttrState        = "state"
	AttrParameters   = "parameters"
	AttrTokensPerSec = "tokens_per_second"
	AttrModelURL     = "model_url"
)

// Numeric keys the catalog populates. Cerebras allows a longer context on a
// paid plan than a free one, so both ceilings are recorded.
const (
	LimitContextWindow     = "context_window"
	LimitContextWindowFree = "context_window_free"
)

// sectionStates maps a heading onto the standing of the models under it.
var sectionStates = map[string]string{
	"production models": StateProduction,
	"preview models":    StatePreview,
}

var (
	linkRe  = regexp.MustCompile(`\[([^\]]*)\]\(([^)]*)\)`)
	tagRe   = regexp.MustCompile(`(?s)<[^>]*>`)
	countRe = regexp.MustCompile(
		`(?i)([\d,]*\.?\d+)\s*(k|m|b|billion|million)?`,
	)
)

// clean strips markdown decoration and the footnote markers Cerebras puts in
// model names.
func clean(cell string) string {
	s := linkRe.ReplaceAllString(cell, "$1")
	s = tagRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, `\~`, "~")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "`", "")
	return strings.Join(strings.Fields(s), " ")
}

// parseCount reads a quantity such as "131k" or "120 billion".
func parseCount(value string) int64 {
	match := countRe.FindStringSubmatch(strings.TrimSpace(clean(value)))
	if match == nil {
		return 0
	}
	n, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", ""), 64)
	if err != nil {
		return 0
	}
	switch strings.ToLower(match[2]) {
	case "k":
		n *= 1_000
	case "m", "million":
		n *= 1_000_000
	case "b", "billion":
		n *= 1_000_000_000
	}
	return int64(n)
}

// applyCatalog reads the model catalog page.
func (b *builder) applyCatalog(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		state, ok := sectionStates[t.Section]
		if !ok {
			continue
		}
		idCol := columnOf(t.Headers, "model id")
		if idCol < 0 {
			continue
		}
		var (
			nameCol    = columnOf(t.Headers, "model name")
			paramCol   = columnOf(t.Headers, "parameters")
			contextCol = columnOf(t.Headers, "context (free / paid)")
			speedCol   = columnOf(t.Headers, "speed (tokens/s)")
		)
		for _, row := range t.Rows {
			id := clean(cellAt(row, idCol))
			if id == "" {
				continue
			}
			m := b.model(id, KindChat)
			m.AddSource(t.Source)
			if m.Name == "" {
				m.Name = clean(cellAt(row, nameCol))
			}
			m.SetAttr(AttrState, state)
			m.SetAttr(AttrParameters, clean(cellAt(row, paramCol)))
			m.SetAttr(AttrTokensPerSec, clean(cellAt(row, speedCol)))
			m.SetAttr(AttrModelURL, linkTarget(cellAt(row, nameCol)))
			applyContext(m, cellAt(row, contextCol))
		}
	}
}

// applyContext reads the "65k / 131k" cell stating the free and paid ceilings.
func applyContext(m *catalog.Model, cell string) {
	free, paid, ok := strings.Cut(clean(cell), "/")
	if !ok {
		m.SetLimit(LimitContextWindow, parseCount(cell))
		return
	}
	m.SetLimit(LimitContextWindowFree, parseCount(free))
	m.SetLimit(LimitContextWindow, parseCount(paid))
}

// linkTarget returns the page a cell links to.
func linkTarget(cell string) string {
	match := linkRe.FindStringSubmatch(cell)
	if match == nil {
		return ""
	}
	return strings.TrimSpace(match[2])
}

// table is one markdown table with the heading above it.
type table struct {
	Section string
	Headers []string
	Rows    [][]string
	Source  string
}

// scanTables walks a document and returns every pipe table, tracking the
// nearest preceding heading.
func scanTables(body, source string) []table {
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
			section = strings.ToLower(
				clean(strings.TrimSpace(strings.TrimLeft(after, "# "))),
			)
		}
	}
	return out
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

// cellAt returns a row's cell, tolerating short rows.
func cellAt(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

// columnOf returns the index of the column with the given heading, or -1.
func columnOf(headers []string, name string) int {
	for i, h := range headers {
		if strings.EqualFold(clean(h), name) {
			return i
		}
	}
	return -1
}
