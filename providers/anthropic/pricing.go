package anthropic

import (
	"fmt"
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

// applyPricing reads the rate tables, the overhead table and the tool prices
// stated in prose.
func (b *builder) applyPricing(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		if tier, ok := tierFor(t.Section); ok {
			b.applyRateTable(t, tier)
			continue
		}
		b.applyToolOverhead(t)
	}
	b.applyToolPricing(string(doc.Body), doc.URL)
	b.applyFreeToolPricing(string(doc.Body), doc.URL)
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
	// allowance matches the quantity Anthropic grants before the rate starts
	// applying, where it grants one. It is pricing that does not reduce to the
	// amount, so it is kept as the price's note rather than dropped: a rate
	// charged only past a monthly allowance is not the rate a reader would
	// otherwise compute.
	allowance *regexp.Regexp
	// allowanceNote is the sentence that quantity is substituted into.
	allowanceNote string
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
		allowance: regexp.MustCompile(
			`([\d,]+) free hours of usage per month`,
		),
		allowanceNote: "%s hours per organization each month are free",
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

// allowanceOf states the free allowance a rate applies past, and states nothing
// where the document states nothing: a rate whose allowance is withdrawn loses
// the note rather than keeping a stale one.
func allowanceOf(pattern *regexp.Regexp, note, body string) string {
	if pattern == nil {
		return ""
	}
	match := pattern.FindStringSubmatch(body)
	if match == nil {
		return ""
	}
	return fmt.Sprintf(note, match[1])
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
		if m.Name == "" {
			m.Name = rate.name
		}
		m.AddSource(source)
		m.AddPrice(catalog.Price{
			Metric:   rate.metric,
			Unit:     rate.unit,
			Amount:   value,
			Currency: currency,
			Dims:     catalog.Dims{DimTier: TierStandard},
			Note:     allowanceOf(rate.allowance, rate.allowanceNote, text),
		})
	}
}

// Free rates. A tool Anthropic states a charge of zero for is priced, not
// unpriced: the sentence saying a tool costs nothing beyond tokens is as much
// a published rate as the one saying it costs ten dollars, and a catalog that
// recorded only the second would leave a reader unable to tell a free tool
// from one whose price nobody has read.
var freeRates = []struct {
	id      string
	metric  catalog.Metric
	unit    catalog.Unit
	pattern *regexp.Regexp
	note    string
}{
	{
		id:     "web-fetch",
		metric: MetricToolCall,
		unit:   UnitPerRequest,
		pattern: regexp.MustCompile(
			`web fetch tool is available on the Claude API at no additional cost`,
		),
		note: "no additional charge beyond standard token costs",
	},
}

// applyFreeToolPricing records the rates stated as an absence of one.
func (b *builder) applyFreeToolPricing(body, source string) {
	text := strings.ReplaceAll(body, "**", "")
	for _, rate := range freeRates {
		if !rate.pattern.MatchString(text) {
			continue
		}
		m, ok := b.models[rate.id]
		if !ok {
			continue
		}
		m.AddSource(source)
		m.AddPrice(catalog.Price{
			Metric:   rate.metric,
			Unit:     rate.unit,
			Amount:   0,
			Currency: currency,
			Dims:     catalog.Dims{DimTier: TierStandard},
			Note:     rate.note,
		})
	}
}

// toolSystemPromptRe matches one token count of the tool use overhead table,
// whose cells hold two numbers where the model behaves two ways.
var toolSystemPromptRe = regexp.MustCompile(`([\d,]+) tokens`)

// applyToolOverhead records the size of the system prompt Anthropic prepends
// whenever a request carries any tool at all.
//
// It is a bound rather than a rate, so it is a limit and not a price: the
// tokens are billed at the model's own input rate and the table states how
// many of them there are. Two numbers are stated per model, because a request
// that lets Claude decide whether to call a tool is told less than one that
// requires a call, and both are kept under their own key.
func (b *builder) applyToolOverhead(t mdTable) {
	choices, counts :=
		columnOf(t, "tool choice"),
		columnOf(t, "tool use system prompt token count")
	if choices < 0 || counts < 0 {
		return
	}
	for _, row := range t.Rows {
		b.applyToolOverheadRow(t, row, choices, counts)
	}
}

// applyToolOverheadRow records one model's two overheads.
func (b *builder) applyToolOverheadRow(
	t mdTable,
	row []string,
	choices, counts int,
) {
	found := toolSystemPromptRe.FindAllStringSubmatch(cellAt(row, counts), -1)
	if len(found) < 2 {
		return
	}
	keys := [2]string{LimitToolSystemPromptAuto, LimitToolSystemPromptAny}
	choice := strings.ToLower(cellAt(row, choices))
	if strings.Index(choice, "any") < strings.Index(choice, "auto") {
		keys = [2]string{LimitToolSystemPromptAny, LimitToolSystemPromptAuto}
	}
	for _, ref := range splitModelCell(cellAt(row, 0)) {
		m, ok := b.models[b.resolve(ref.Name)]
		if !ok {
			continue
		}
		m.AddSource(t.Source)
		for i, key := range keys {
			m.SetLimit(key, parseCount(found[i][1]))
		}
	}
}
