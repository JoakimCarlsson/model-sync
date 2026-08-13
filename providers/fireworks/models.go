package fireworks

import (
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// table is one markdown table with the heading above it.
type table struct {
	Section string
	Headers []string
	Rows    [][]string
	Source  string
}

// Sections of the pricing page that list models.
const (
	sectionTextVision = "text and vision models"
	sectionEmbeddings = "embeddings"
)

// tierColumns maps a rate column onto the serving path it prices.
var tierColumns = map[string]string{
	"standard": TierStandard,
	"priority": TierPriority,
}

// applyPricing reads the serverless pricing page.
func (b *builder) applyPricing(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		switch t.Section {
		case sectionTextVision:
			b.applyTripleTable(t)
		case sectionEmbeddings:
			b.applySingleTable(t)
		}
	}
}

// applyTripleTable reads the table whose cells hold three amounts per serving
// path.
func (b *builder) applyTripleTable(t table) {
	for _, row := range t.Rows {
		ref, ok := splitModelCell(cellAt(row, 0))
		if !ok {
			continue
		}
		m := b.model(ref.ID, KindChat)
		m.AddSource(t.Source)
		if m.Name == "" {
			m.Name = ref.Name
		}
		m.SetAttr(AttrModelURL, ref.URL)
		for i, header := range t.Headers {
			tier, ok := tierColumns[strings.ToLower(clean(header))]
			if !ok {
				continue
			}
			dims := catalog.Dims{DimTier: tier}.With(DimServing, ref.Serving)
			for at, amount := range parseTriple(cellAt(row, i)) {
				if at >= len(tripleOrder) {
					break
				}
				m.AddPrice(catalog.Price{
					Metric:   tripleOrder[at],
					Unit:     UnitPer1MTokens,
					Amount:   amount,
					Currency: currency,
					Dims:     dims,
				})
			}
		}
	}
}

// applySingleTable reads a table whose cells hold one amount, skipping the
// rows that price a parameter count band rather than a model.
func (b *builder) applySingleTable(t table) {
	for _, row := range t.Rows {
		cell := cellAt(row, 0)
		if isBand(cell) {
			continue
		}
		id, name := clean(cell), clean(cell)
		if ref, ok := splitModelCell(cell); ok {
			id, name = ref.ID, ref.Name
		} else {
			id = slugID(id)
		}
		if id == "" {
			continue
		}
		amounts := parseTriple(cellAt(row, 1))
		if len(amounts) == 0 {
			continue
		}
		m := b.model(id, KindEmbedding)
		m.AddSource(t.Source)
		if m.Name == "" {
			m.Name = name
		}
		m.AddPrice(catalog.Price{
			Metric:   MetricInputTokens,
			Unit:     UnitPer1MTokens,
			Amount:   amounts[0],
			Currency: currency,
		})
	}
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
