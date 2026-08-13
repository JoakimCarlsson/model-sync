package deepseek

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Metrics DeepSeek bills on.
const (
	MetricInputTokens       catalog.Metric = "input_tokens"
	MetricCachedInputTokens catalog.Metric = "cached_input_tokens"
	MetricOutputTokens      catalog.Metric = "output_tokens"
)

// UnitPer1MTokens is the only denominator DeepSeek quotes.
const UnitPer1MTokens catalog.Unit = "per_1m_tokens"

// KindChat is the only kind DeepSeek publishes.
const KindChat catalog.Kind = "chat"

// Scalar keys the pricing page populates.
const (
	AttrModelVersion = "model_version"
	AttrThinkingMode = "thinking_mode"
)

// Numeric keys the pricing page populates.
const (
	LimitContextWindow   = "context_window"
	LimitMaxOutputTokens = "max_output_tokens"
	LimitConcurrency     = "concurrency_limit"
)

// ListFeatures holds the capabilities marked as supported.
const ListFeatures = "features"

// supported is the mark DeepSeek uses for a capability a model has.
const supported = "✓"

// rateRows maps a row label onto what that row's amounts are charged for.
var rateRows = map[string]catalog.Metric{
	"1m input tokens (cache hit)":  MetricCachedInputTokens,
	"1m input tokens (cache miss)": MetricInputTokens,
	"1m output tokens":             MetricOutputTokens,
}

var (
	rowRe    = regexp.MustCompile(`(?is)<tr[^>]*>(.*?)</tr>`)
	cellRe   = regexp.MustCompile(`(?is)<t[hd][^>]*>(.*?)</t[hd]\s*>`)
	tagRe    = regexp.MustCompile(`(?s)<[^>]*>`)
	amountRe = regexp.MustCompile(`\$\s*([\d,]*\.?\d+)`)
	countRe  = regexp.MustCompile(`(?i)([\d,]*\.?\d+)\s*([km])?`)
)

// text strips markup and collapses whitespace.
func text(html string) string {
	return strings.Join(strings.Fields(tagRe.ReplaceAllString(html, " ")), " ")
}

// parseAmount reads a rate cell.
func parseAmount(cell string) (float64, bool) {
	match := amountRe.FindStringSubmatch(cell)
	if match == nil {
		return 0, false
	}
	value, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", ""), 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

// parseCount reads a quantity such as "1M" or "384K".
func parseCount(cell string) int64 {
	match := countRe.FindStringSubmatch(cell)
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
	case "m":
		n *= 1_000_000
	}
	return int64(n)
}

// applyPricing reads the pricing page.
func (b *builder) applyPricing(doc catalog.Document) {
	var ids []string
	for _, match := range rowRe.FindAllStringSubmatch(string(doc.Body), -1) {
		cells := rowCells(match[1])
		if len(cells) < 2 {
			continue
		}
		if strings.EqualFold(cells[0], "model") {
			ids = cells[1:]
			for _, id := range ids {
				b.model(id, KindChat).AddSource(doc.URL)
			}
			continue
		}
		if len(ids) == 0 {
			continue
		}
		b.applyRow(cells, ids)
	}
}

// applyRow records one fact about every model.
//
// The row is read from the right because a spanning section label can precede
// the row's own label, so the position of the values is fixed but the position
// of the label is not.
func (b *builder) applyRow(cells, ids []string) {
	count := min(len(ids), len(cells)-1)
	if count < 1 {
		return
	}
	values := cells[len(cells)-count:]
	label := rowLabel(cells[len(cells)-count-1])
	for i, id := range ids {
		value := values[0]
		if i < len(values) {
			value = values[i]
		}
		b.applyValue(id, label, value)
	}
}

// applyValue records one cell against one model.
func (b *builder) applyValue(id, label, value string) {
	if value == "" {
		return
	}
	m := b.model(id, KindChat)
	if metric, ok := rateRows[label]; ok {
		if amount, ok := parseAmount(value); ok {
			m.AddPrice(catalog.Price{
				Metric:   metric,
				Unit:     UnitPer1MTokens,
				Amount:   amount,
				Currency: currency,
			})
		}
		return
	}
	switch label {
	case "model version":
		m.SetAttr(AttrModelVersion, value)
	case "thinking mode":
		m.SetAttr(AttrThinkingMode, value)
	case "context length":
		m.SetLimit(LimitContextWindow, parseCount(value))
	case "max output":
		m.SetLimit(LimitMaxOutputTokens, parseCount(value))
	case "concurrency limit":
		m.SetLimit(LimitConcurrency, parseCount(value))
	default:
		if strings.Contains(value, supported) && label != "" {
			m.AddList(ListFeatures, label)
		}
	}
}

// footnoteRe matches the reference marker DeepSeek appends to a row label.
var footnoteRe = regexp.MustCompile(`\(\d+\)$`)

// rowLabel normalizes the cell naming a row, dropping the footnote marker that
// would otherwise make "Concurrency Limit(2)" a different label from the one
// it is.
func rowLabel(cell string) string {
	return strings.ToLower(
		strings.TrimSpace(
			footnoteRe.ReplaceAllString(strings.TrimSpace(cell), ""),
		),
	)
}

// rowCells returns the text of one row's cells.
func rowCells(row string) []string {
	matches := cellRe.FindAllStringSubmatch(row, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, text(m[1]))
	}
	return out
}
