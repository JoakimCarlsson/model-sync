package openai

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// additionalSizes is the phrase OpenAI puts inside the model cell of the
// per-image table to say the listed resolutions are not the only ones.
const additionalSizes = "Additional sizes available"

var (
	tableRe      = regexp.MustCompile(`(?is)<table\b.*?</table>`)
	rowRe        = regexp.MustCompile(`(?is)<tr\b[^>]*>(.*?)</tr>`)
	cellRe       = regexp.MustCompile(`(?is)<(t[dh])\b([^>]*)>(.*?)</t[dh]\s*>`)
	rowSpanRe    = regexp.MustCompile(`(?i)rowspan\s*=\s*"?\{?(\d+)\}?"?`)
	tagRe        = regexp.MustCompile(`(?s)<[^>]*>`)
	sizeHeaderRe = regexp.MustCompile(`^(\d+)\s*[x×]\s*(\d+)$`)
)

// jsxTable is one raw HTML table lifted out of a guide. The guides state
// per-image dollar prices this way rather than as markdown, with the model
// name spanning its quality rows, so a markdown reader cannot see them.
type jsxTable struct {
	Headers []string
	Rows    [][]string
	Source  string
}

// jsxCell is one parsed cell together with the rows it spans.
type jsxCell struct {
	text    string
	header  bool
	rowSpan int
}

// span is a cell still occupying a column in the rows below its own.
type span struct {
	text string
	left int
}

// scanJSXTables returns every HTML table in a document, with rowspans already
// expanded so each row carries a value in every column.
func scanJSXTables(doc catalog.Document) []jsxTable {
	var out []jsxTable
	for _, block := range tableRe.FindAllString(string(doc.Body), -1) {
		t := parseJSXTable(block)
		if len(t.Headers) == 0 || len(t.Rows) == 0 {
			continue
		}
		t.Source = doc.URL
		out = append(out, t)
	}
	return out
}

// parseJSXTable reads one table block.
func parseJSXTable(block string) jsxTable {
	var t jsxTable
	carry := map[int]*span{}
	for _, match := range rowRe.FindAllStringSubmatch(block, -1) {
		cells := parseJSXRow(match[1])
		if len(cells) == 0 {
			continue
		}
		if t.Headers == nil && cells[0].header {
			t.Headers = cellTexts(cells)
			continue
		}
		t.Rows = append(t.Rows, expandRow(cells, carry))
	}
	return t
}

// parseJSXRow reads the cells of one row.
func parseJSXRow(row string) []jsxCell {
	matches := cellRe.FindAllStringSubmatch(row, -1)
	cells := make([]jsxCell, 0, len(matches))
	for _, m := range matches {
		cell := jsxCell{text: cellText(m[3]), header: strings.EqualFold(m[1], "th"), rowSpan: 1}
		if s := rowSpanRe.FindStringSubmatch(m[2]); s != nil {
			if n, err := strconv.Atoi(s[1]); err == nil && n > 1 {
				cell.rowSpan = n
			}
		}
		cells = append(cells, cell)
	}
	return cells
}

// expandRow places this row's cells around the columns still held by a
// rowspan from an earlier row, and records the spans this row opens.
func expandRow(cells []jsxCell, carry map[int]*span) []string {
	var row []string
	next, col := 0, 0
	for {
		if s, ok := carry[col]; ok {
			row = append(row, s.text)
			if s.left--; s.left <= 0 {
				delete(carry, col)
			}
			col++
			continue
		}
		if next >= len(cells) {
			return row
		}
		cell := cells[next]
		next++
		row = append(row, cell.text)
		if cell.rowSpan > 1 {
			carry[col] = &span{text: cell.text, left: cell.rowSpan - 1}
		}
		col++
	}
}

// cellTexts returns the text of each cell.
func cellTexts(cells []jsxCell) []string {
	out := make([]string, len(cells))
	for i, c := range cells {
		out[i] = c.text
	}
	return out
}

// cellText strips markup and collapses the whitespace JSX indentation leaves
// behind.
func cellText(s string) string {
	s = tagRe.ReplaceAllString(s, " ")
	r := strings.NewReplacer("&nbsp;", " ", "&amp;", "&", "&times;", "x", "&lt;", "<", "&gt;", ">")
	return strings.Join(strings.Fields(r.Replace(s)), " ")
}

// applyImageTable turns a per-image price table into prices dimensioned by
// quality and size. Tables without a size column are not price tables and are
// ignored.
func (b *builder) applyImageTable(t jsxTable) {
	sizes := map[int]string{}
	idCol, qualityCol := -1, -1
	for i, h := range t.Headers {
		switch {
		case strings.EqualFold(h, "model"):
			idCol = i
		case strings.EqualFold(h, "quality"):
			qualityCol = i
		default:
			if m := sizeHeaderRe.FindStringSubmatch(strings.TrimSpace(h)); m != nil {
				sizes[i] = m[1] + "x" + m[2]
			}
		}
	}
	if idCol < 0 || len(sizes) == 0 {
		return
	}
	for _, row := range t.Rows {
		b.applyImageRow(t, row, idCol, qualityCol, sizes)
	}
}

// applyImageRow emits one per-image price per size column.
func (b *builder) applyImageRow(t jsxTable, row []string, idCol, qualityCol int, sizes map[int]string) {
	name, note := splitAdditionalSizes(cellAt(row, idCol))
	id := slugID(name)
	if id == "" {
		return
	}
	m := b.model(id, KindImage)
	m.AddSource(t.Source)
	m.AddNote(note)
	dims := catalog.Dims{DimTier: TierStandard}
	if qualityCol >= 0 {
		dims = dims.With(DimQuality, strings.ToLower(cellAt(row, qualityCol)))
	}
	for col, size := range sizes {
		a := parseAmount(cellAt(row, col))
		if !a.Found {
			continue
		}
		m.AddPrice(catalog.Price{
			Metric:   MetricImageOutput,
			Unit:     UnitPerImage,
			Amount:   a.Value,
			Currency: currency,
			Dims:     dims.With(DimSize, size),
			Note:     a.Note,
		})
	}
}

// splitAdditionalSizes separates the model name from the caveat OpenAI packs
// into the same cell.
func splitAdditionalSizes(cell string) (name, note string) {
	if !strings.Contains(cell, additionalSizes) {
		return strings.TrimSpace(cell), ""
	}
	return strings.TrimSpace(strings.ReplaceAll(cell, additionalSizes, "")), additionalSizes
}
