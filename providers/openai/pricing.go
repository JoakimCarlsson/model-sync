package openai

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// mdTable is one markdown pipe table from the pricing page, carried together
// with the surrounding prose that qualifies it. OpenAI states the tier and the
// default denominator as bare lines above the table rather than inside it.
type mdTable struct {
	Headers []string
	Rows    [][]string
	Kind    catalog.Kind
	Tier    string
	Unit    catalog.Unit
	Source  string
}

// scanMarkdownTables walks a document top to bottom, tracking the section,
// tier and denominator in force, and returns every pipe table it finds.
func scanMarkdownTables(doc catalog.Document) []mdTable {
	var (
		out     []mdTable
		kind    catalog.Kind
		tier    = TierStandard
		unit    catalog.Unit
		lines   = strings.Split(string(doc.Body), "\n")
		current *mdTable
	)
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "|") {
			if current == nil {
				out = append(out, mdTable{Kind: kind, Tier: tier, Unit: unit, Source: doc.URL})
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
		if line == "" {
			continue
		}
		if k, ok := sectionKind(line); ok {
			kind, tier, unit = k, TierStandard, ""
			continue
		}
		if t, ok := tierFor(line); ok {
			tier = t
			continue
		}
		if u, ok := unitHint(line); ok {
			unit = u
		}
	}
	return out
}

// splitRow splits a pipe row into trimmed cells, dropping the empty fields the
// leading and trailing pipes produce.
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
		t := strings.Trim(c, ":- ")
		if t != "" {
			return false
		}
	}
	return len(cells) > 0
}

// role is what a column contributes to the rows it appears in.
type role int

const (
	roleSkip role = iota
	roleID
	roleDim
	rolePrice
	roleCategory
)

// column is the meaning of one table column.
type column struct {
	role   role
	metric catalog.Metric
	unit   catalog.Unit
	dimKey string
	dims   catalog.Dims
}

// headerColumn maps one of OpenAI's column headers onto its meaning. The
// context prefixes are handled by stripping them and recursing, because
// OpenAI writes the same four metrics twice per row, once per context band.
func headerColumn(header string) column {
	h := strings.ToLower(strings.Join(strings.Fields(header), " "))
	for prefix, band := range map[string]string{"short context ": "short", "long context ": "long"} {
		if strings.HasPrefix(h, prefix) {
			inner := headerColumn(strings.TrimPrefix(h, prefix))
			inner.dims = inner.dims.With(DimContext, band)
			return inner
		}
	}
	switch h {
	case "model", "tool":
		return column{role: roleID}
	case "category":
		return column{role: roleCategory}
	case "modality":
		return column{role: roleDim, dimKey: DimModality}
	case "quality":
		return column{role: roleDim, dimKey: DimQuality}
	case "size":
		return column{role: roleDim, dimKey: DimSize}
	case "use case":
		return column{role: roleDim, dimKey: DimUseCase}
	case "details":
		return column{role: roleDim, dimKey: DimDetail}
	case "portrait", "landscape":
		return column{role: roleDim, dimKey: h}
	case "input":
		return column{role: rolePrice, metric: MetricInputTokens}
	case "cached input":
		return column{role: rolePrice, metric: MetricCachedInputTokens}
	case "cache writes":
		return column{role: rolePrice, metric: MetricCacheWriteTokens}
	case "output", "output / cost":
		return column{role: rolePrice, metric: MetricOutputTokens}
	case "training":
		return column{role: rolePrice, metric: MetricTraining}
	case "price per second":
		return column{role: rolePrice, metric: MetricVideoOutput, unit: UnitPerSecond}
	case "estimated cost", "pricing":
		return column{role: rolePrice, metric: MetricUsage}
	}
	return column{role: roleSkip}
}

// applyPricingTable turns one table into models and prices on the builder.
func (b *builder) applyPricingTable(t mdTable) {
	cols := make([]column, len(t.Headers))
	for i, h := range t.Headers {
		cols[i] = headerColumn(h)
	}
	for _, row := range t.Rows {
		b.applyPricingRow(t, cols, row)
	}
}

// applyPricingRow resolves the row's model, the dimensions qualifying it, and
// then one price per price column that holds an amount.
func (b *builder) applyPricingRow(t mdTable, cols []column, row []string) {
	q, ok := rowID(cols, row)
	if !ok {
		return
	}
	kind := t.Kind
	dims := catalog.Dims{}.With(DimTier, t.Tier).Merge(q.Dims)
	for i, col := range cols {
		cell := cellAt(row, i)
		switch col.role {
		case roleDim:
			if cell != "" && cell != "-" {
				dims = dims.With(col.dimKey, cell)
			}
		case roleCategory:
			if k, ok := categoryKind(cell); ok {
				kind = k
			}
		}
	}
	m := b.model(q.ID, kind)
	m.AddSource(t.Source)
	m.AddNote(q.Note)
	for i, col := range cols {
		if col.role != rolePrice {
			continue
		}
		a := parseAmount(cellAt(row, i))
		if !a.Found {
			if a.Note != "" && !addMemoryPrices(m, a.Note, dims) {
				m.AddNote(a.Note)
			}
			continue
		}
		unit := firstUnit(a.Unit, col.unit, t.Unit)
		m.AddPrice(catalog.Price{
			Metric:   metricFor(t.Kind, col.metric, unit),
			Unit:     unit,
			Amount:   a.Value,
			Currency: currency,
			Dims:     dims.Merge(col.dims),
			Note:     a.Note,
		})
	}
}

// memoryPriceRe matches one of the several rates OpenAI packs into the
// container row, each naming the memory size it applies to.
var memoryPriceRe = regexp.MustCompile(`(\d+)\s*GB\s+\$([\d.]+)`)

// addMemoryPrices expands the container row, which states four rates in one
// cell as "1 GB $0.03, 4 GB $0.12, 16 GB $0.48, 64 GB $1.92 per 20-minute
// session per container". It reports whether the cell was one of these.
func addMemoryPrices(m *catalog.Model, cell string, dims catalog.Dims) bool {
	matches := memoryPriceRe.FindAllStringSubmatchIndex(cell, -1)
	if len(matches) == 0 {
		return false
	}
	note := strings.TrimSpace(strings.TrimLeft(cell[matches[len(matches)-1][1]:], ", "))
	for _, at := range matches {
		value, err := strconv.ParseFloat(cell[at[4]:at[5]], 64)
		if err != nil {
			continue
		}
		m.AddPrice(catalog.Price{
			Metric:   MetricUsage,
			Unit:     UnitPerSession,
			Amount:   value,
			Currency: currency,
			Dims:     dims.With(DimMemory, cell[at[2]:at[3]]+"gb"),
			Note:     note,
		})
	}
	return true
}

// metricFor specializes a column's metric where the section makes it more
// precise: a tool row priced per call is a tool call, and one priced per
// gigabyte-day is storage.
func metricFor(kind catalog.Kind, m catalog.Metric, unit catalog.Unit) catalog.Metric {
	if kind != KindTool || m != MetricUsage {
		return m
	}
	if unit == UnitPerGBDay {
		return MetricStorage
	}
	return MetricToolCall
}

// rowID locates and interprets the row's identifier cell.
func rowID(cols []column, row []string) (qualifier, bool) {
	for i, col := range cols {
		if col.role != roleID {
			continue
		}
		cell := cellAt(row, i)
		if cell == "" || cell == "-" {
			return qualifier{}, false
		}
		q := splitQualifier(cell)
		return q, q.ID != ""
	}
	return qualifier{}, false
}

// cellAt returns a row's cell, tolerating rows shorter than the header.
func cellAt(row []string, i int) string {
	if i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

// firstUnit returns the first denominator that is known: the one the cell
// stated, then the column's, then the section's.
func firstUnit(units ...catalog.Unit) catalog.Unit {
	for _, u := range units {
		if u != "" {
			return u
		}
	}
	return ""
}
