package perplexity

import (
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// table is one markdown table lifted out of a document.
type table struct {
	Headers []string
	Rows    [][]string
	Source  string
}

// tokenColumns are the per-token rate columns of the Sonar table.
var tokenColumns = []struct {
	header string
	metric catalog.Metric
	unit   catalog.Unit
}{
	{"input tokens ($/1m)", MetricInputTokens, UnitPer1MTokens},
	{"output tokens ($/1m)", MetricOutputTokens, UnitPer1MTokens},
	{"citation tokens ($/1m)", MetricCitationTokens, UnitPer1MTokens},
	{"reasoning tokens ($/1m)", MetricReasoningTokens, UnitPer1MTokens},
	{"search queries ($/1k)", MetricSearchQueries, UnitPer1KRequests},
}

// brokeredColumns are the rate columns of the Agent API's model tables, where
// the heading states the denominator and the cells hold bare numbers.
var brokeredColumns = []struct {
	header string
	metric catalog.Metric
}{
	{"input ($/1m)", MetricInputTokens},
	{"output ($/1m)", MetricOutputTokens},
	{"cache ($/1m)", MetricCachedInputTokens},
}

// contextSizes are the request-fee columns, one per depth of web search.
var contextSizes = map[string]string{
	"low context size":    "low",
	"medium context size": "medium",
	"high context size":   "high",
}

// applyDocument reads a pricing document, dispatching each table on the
// columns it has. Perplexity nests most of them inside tab elements, so the
// surrounding headings do not reliably say what a table is; its columns do.
func (b *builder) applyDocument(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		switch {
		case columnOf(t.Headers, "input tokens ($/1m)") >= 0:
			b.applySonarTokens(t)
		case columnOf(t.Headers, "low context size") >= 0:
			b.applyRequestFees(t)
		case columnOf(t.Headers, "dimensions") >= 0:
			b.applyEmbeddings(t)
		case columnOf(t.Headers, "input ($/1m)") >= 0:
			b.applyBrokered(t)
		case columnOf(t.Headers, "tool") >= 0:
			b.applyTools(t)
		case columnOf(t.Headers, "price per 1k requests") >= 0:
			b.applySearchAPI(t)
		}
	}
}

// applySonarTokens reads the per-token rates of the Sonar models.
func (b *builder) applySonarTokens(t table) {
	for _, row := range t.Rows {
		m, ok := b.rowModel(t, row, KindChat)
		if !ok {
			continue
		}
		for _, col := range tokenColumns {
			at := columnOf(t.Headers, col.header)
			if at < 0 {
				continue
			}
			if amount, ok := parseAmount(cellAt(row, at)); ok {
				m.AddPrice(catalog.Price{
					Metric:   col.metric,
					Unit:     col.unit,
					Amount:   amount,
					Currency: currency,
				})
			}
		}
	}
}

// applyRequestFees reads the per-request fee, which varies by how much of the
// web the query is allowed to read.
func (b *builder) applyRequestFees(t table) {
	for _, row := range t.Rows {
		m, ok := b.rowModel(t, row, KindChat)
		if !ok {
			continue
		}
		for header, size := range contextSizes {
			at := columnOf(t.Headers, header)
			if at < 0 {
				continue
			}
			if amount, ok := parseAmount(cellAt(row, at)); ok {
				m.AddPrice(catalog.Price{
					Metric:   MetricRequest,
					Unit:     UnitPer1KRequests,
					Amount:   amount,
					Currency: currency,
					Dims:     catalog.Dims{DimContextSize: size},
				})
			}
		}
	}
}

// applyEmbeddings reads an embedding table, which states the vector width
// beside the rate.
func (b *builder) applyEmbeddings(t table) {
	priceCol := columnOf(t.Headers, "price ($/1m tokens)")
	dimCol := columnOf(t.Headers, "dimensions")
	for _, row := range t.Rows {
		m, ok := b.rowModel(t, row, KindEmbedding)
		if !ok {
			continue
		}
		if width := clean(cellAt(row, dimCol)); width != "" {
			m.SetAttr(AttrDefaultDimension, width)
			m.AddList(ListDimensions, width)
		}
		if strings.Contains(m.ID, "context") {
			m.SetAttr(AttrContextualized, "true")
		}
		if amount, ok := parseAmount(cellAt(row, priceCol)); ok {
			m.AddPrice(catalog.Price{
				Metric:   MetricInputTokens,
				Unit:     UnitPer1MTokens,
				Amount:   amount,
				Currency: currency,
			})
		}
	}
}

// applyBrokered reads the models Perplexity resells, which it namespaces under
// the lab that made them.
func (b *builder) applyBrokered(t table) {
	docsCol := columnOf(t.Headers, "docs")
	for _, row := range t.Rows {
		m, ok := b.rowModel(t, row, KindChat)
		if !ok {
			continue
		}
		m.SetAttr(AttrAuthor, authorOf(m.ID))
		m.SetAttr(AttrSummary, clean(cellAt(row, docsCol)))
		for _, col := range brokeredColumns {
			at := columnOf(t.Headers, col.header)
			if at < 0 {
				continue
			}
			if amount, ok := parseAmount(cellAt(row, at)); ok {
				m.AddPrice(catalog.Price{
					Metric:   col.metric,
					Unit:     UnitPer1MTokens,
					Amount:   amount,
					Currency: currency,
				})
			}
		}
	}
}

// applyTools reads the server-side tool rates, which are charged per
// invocation except the sandbox, which is charged per session.
func (b *builder) applyTools(t table) {
	priceCol := columnOf(t.Headers, "price")
	descCol := columnOf(t.Headers, "description")
	for _, row := range t.Rows {
		m, ok := b.rowModel(t, row, KindTool)
		if !ok {
			continue
		}
		m.SetAttr(AttrSummary, clean(cellAt(row, descCol)))
		cell := clean(cellAt(row, priceCol))
		amount, ok := parseAmount(cell)
		if !ok {
			continue
		}
		metric, unit := MetricToolCall, UnitPerInvocation
		if strings.Contains(strings.ToLower(cell), "per session") {
			metric, unit = MetricSession, UnitPerSession
		}
		m.AddPrice(catalog.Price{
			Metric:   metric,
			Unit:     unit,
			Amount:   amount,
			Currency: currency,
		})
	}
}

// applySearchAPI reads the standalone search product's rate.
func (b *builder) applySearchAPI(t table) {
	priceCol := columnOf(t.Headers, "price per 1k requests")
	descCol := columnOf(t.Headers, "description")
	for _, row := range t.Rows {
		m, ok := b.rowModel(t, row, KindTool)
		if !ok {
			continue
		}
		m.SetAttr(AttrSummary, clean(cellAt(row, descCol)))
		if amount, ok := parseAmount(cellAt(row, priceCol)); ok {
			m.AddPrice(catalog.Price{
				Metric:   MetricRequest,
				Unit:     UnitPer1KRequests,
				Amount:   amount,
				Currency: currency,
			})
		}
	}
}

// rowModel resolves the model a row names, which is always its first cell.
func (b *builder) rowModel(
	t table,
	row []string,
	kind catalog.Kind,
) (*catalog.Model, bool) {
	name := clean(cellAt(row, 0))
	if name == "" || name == "-" {
		return nil, false
	}
	m := b.model(slugID(name), kind)
	m.AddSource(t.Source)
	if m.Name == "" {
		m.Name = name
	}
	return m, true
}

// scanTables returns every pipe table in a document, however deeply the MDX
// around it is indented.
func scanTables(body, source string) []table {
	var (
		out     []table
		current *table
	)
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "|") {
			current = nil
			continue
		}
		if current == nil {
			out = append(out, table{Source: source})
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
