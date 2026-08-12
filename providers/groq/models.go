package groq

import (
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// table is one markdown table with the heading above it, which is what says
// whether the models under it are in production or preview.
type table struct {
	Section string
	Headers []string
	Rows    [][]string
	Source  string
}

// sectionStates maps a heading onto the standing of the models under it. A
// heading absent from this map introduces prose rather than a table of models.
var sectionStates = map[string]string{
	"production models":  StateProduction,
	"production systems": StateProduction,
	"preview models":     StatePreview,
}

// sectionSystems is the heading under which Groq lists compound systems rather
// than single models.
const sectionSystems = "production systems"

// applyModels reads the supported models page.
func (b *builder) applyModels(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		state, ok := sectionStates[t.Section]
		if !ok {
			continue
		}
		b.applyTable(t, state)
	}
}

// applyTable reads one standing's table.
func (b *builder) applyTable(t table, state string) {
	idCol := columnOf(t.Headers, "model id")
	priceCol := columnOf(t.Headers, "price per 1m tokens")
	if idCol < 0 {
		return
	}
	var (
		speedCol   = columnOf(t.Headers, "speed (t/sec)")
		limitCol   = columnOf(t.Headers, "rate limits (developer plan)")
		contextCol = columnOf(t.Headers, "context window (tokens)")
		outputCol  = columnOf(t.Headers, "max completion tokens")
		fileCol    = columnOf(t.Headers, "max file size")
	)
	for _, row := range t.Rows {
		ref := splitModelCell(cellAt(row, idCol))
		if ref.ID == "" {
			continue
		}
		rates := parseRates(cellAt(row, priceCol))
		m := b.model(ref.ID, kindForRates(rates))
		m.AddSource(t.Source)
		if m.Name == "" {
			m.Name = ref.Name
		}
		m.SetAttr(AttrState, state)
		m.SetAttr(AttrAccess, ref.Badge)
		if t.Section == sectionSystems {
			m.SetAttr(AttrSystem, "true")
		}
		m.SetAttr(AttrTokensPerSec, valueOrEmpty(cellAt(row, speedCol)))
		m.SetAttr(AttrMaxFileSize, valueOrEmpty(cellAt(row, fileCol)))
		m.SetLimit(LimitContextWindow, parseCount(cellAt(row, contextCol)))
		m.SetLimit(LimitMaxOutputTokens, parseCount(cellAt(row, outputCol)))
		for key, value := range parseLimits(cellAt(row, limitCol)) {
			m.SetLimit(key, value)
		}
		for _, r := range rates {
			m.AddPrice(catalog.Price{
				Metric:   r.Metric,
				Unit:     r.Unit,
				Amount:   r.Amount,
				Currency: currency,
			})
		}
	}
}

// valueOrEmpty returns a cell's value, treating the dash Groq writes for "not
// applicable" as absent.
func valueOrEmpty(cell string) string {
	if text := clean(cell); text != "-" {
		return text
	}
	return ""
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

// cellAt returns a row's cell, tolerating rows shorter than the header.
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
