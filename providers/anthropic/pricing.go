package anthropic

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// priceColumn is what one column of a rate table bills for.
type priceColumn struct {
	metric catalog.Metric
	dims   catalog.Dims
}

// headerColumn maps one of Anthropic's rate column headers. Cache lifetime is
// read off the header because "5m Cache Writes" and "1h Cache Writes" are the
// same metric at different lifetimes.
func headerColumn(header string) (priceColumn, bool) {
	switch strings.ToLower(clean(header)) {
	case "base input tokens", "input", "batch input":
		return priceColumn{metric: MetricInputTokens}, true
	case "output tokens", "output", "batch output":
		return priceColumn{metric: MetricOutputTokens}, true
	case "cache hits & refreshes":
		return priceColumn{metric: MetricCachedInputTokens}, true
	case "5m cache writes":
		return priceColumn{
			metric: MetricCacheWriteTokens,
			dims:   catalog.Dims{DimCacheTTL: "5m"},
		}, true
	case "1h cache writes":
		return priceColumn{
			metric: MetricCacheWriteTokens,
			dims:   catalog.Dims{DimCacheTTL: "1h"},
		}, true
	}
	return priceColumn{}, false
}

// tierFor reports which tier a rate table states, and whether the table is a
// rate table at all. Anthropic separates them by heading rather than by any
// difference in the table itself.
func tierFor(section string) (string, bool) {
	switch section {
	case "model pricing":
		return TierStandard, true
	case "batch processing":
		return TierBatch, true
	case "fast mode pricing":
		return TierFast, true
	}
	return "", false
}

// applyPricing reads the rate tables and the prose-stated tool prices.
func (b *builder) applyPricing(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		tier, ok := tierFor(t.Section)
		if !ok {
			continue
		}
		b.applyRateTable(t, tier)
	}
	b.applyToolPricing(string(doc.Body), doc.URL)
}

// applyRateTable emits one price per rate column of every row.
func (b *builder) applyRateTable(t mdTable, tier string) {
	cols := make(map[int]priceColumn, len(t.Headers))
	for i, h := range t.Headers {
		if col, ok := headerColumn(h); ok {
			cols[i] = col
		}
	}
	if len(cols) == 0 {
		return
	}
	for _, row := range t.Rows {
		for _, ref := range splitModelCell(cellAt(row, 0)) {
			b.applyRateRow(t, row, cols, tier, ref)
		}
	}
}

// applyRateRow records one model's rates from a row.
func (b *builder) applyRateRow(
	t mdTable,
	row []string,
	cols map[int]priceColumn,
	tier string,
	ref modelRef,
) {
	m := b.model(b.resolve(ref.Name), KindChat)
	m.AddSource(t.Source)
	m.SetAttr(AttrAvailability, ref.Availability)
	if m.Name == "" {
		m.Name = ref.Name
	}
	for column, col := range cols {
		a := parseAmount(cellAt(row, column))
		if !a.Found {
			continue
		}
		unit := a.Unit
		if unit == "" {
			unit = UnitPerMTok
		}
		m.AddPrice(catalog.Price{
			Metric:   col.metric,
			Unit:     unit,
			Amount:   a.Value,
			Currency: currency,
			Dims:     col.dims.With(DimTier, tier),
		})
	}
}

// Prose rates. Anthropic states its server-side tool prices in sentences
// rather than tables, so each is matched on the wording that states it. A
// rewording drops the rate, which shows up as a deletion in the data rather
// than as a stale number nobody notices.
var proseRates = []struct {
	id      string
	name    string
	metric  catalog.Metric
	unit    catalog.Unit
	pattern *regexp.Regexp
}{
	{
		id:     "web-search",
		name:   "Web search",
		metric: MetricToolCall,
		unit:   UnitPer1KSearches,
		pattern: regexp.MustCompile(
			`\$([\d.]+) per 1,000 searches`,
		),
	},
	{
		id:     "code-execution",
		name:   "Code execution",
		metric: MetricRuntime,
		unit:   UnitPerHour,
		pattern: regexp.MustCompile(
			`\$([\d.]+) USD per hour, per container`,
		),
	},
	{
		id:     "managed-agents-session",
		name:   "Claude Managed Agents session runtime",
		metric: MetricRuntime,
		unit:   UnitPerSessionHour,
		pattern: regexp.MustCompile(
			`\$([\d.]+) per session-hour`,
		),
	},
}

// applyToolPricing records the rates stated only in prose.
func (b *builder) applyToolPricing(body, source string) {
	text := strings.ReplaceAll(body, "**", "")
	for _, rate := range proseRates {
		match := rate.pattern.FindStringSubmatch(text)
		if match == nil {
			continue
		}
		value, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			continue
		}
		m := b.model(rate.id, KindTool)
		m.Name = rate.name
		m.AddSource(source)
		m.AddPrice(catalog.Price{
			Metric:   rate.metric,
			Unit:     rate.unit,
			Amount:   value,
			Currency: currency,
			Dims:     catalog.Dims{DimTier: TierStandard},
		})
	}
}
