package deepseek

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// rateRows maps a section label onto what the amounts beneath it are charged
// for. The label sits one cell to the left of the row's own label, because
// each of these three sections spans a peak row and an off-peak row.
var rateRows = map[string]catalog.Metric{
	"1m input tokens (cache hit)":  MetricCachedInputTokens,
	"1m input tokens (cache miss)": MetricInputTokens,
	"1m output tokens":             MetricOutputTokens,
}

// periods map the label of a rate row onto the value of DimPeriod it states.
var periods = map[string]string{
	"peak":     PeriodPeak,
	"off-peak": PeriodOffPeak,
}

// noteRe matches the footnote the pricing table hangs off its PRICING
// section, which is where the peak window itself is stated.
var noteRe = regexp.MustCompile(`(?is)<p>\(1\)\s*(.*?)</p>`)

// footnoteRe matches the reference marker DeepSeek appends to a row label.
var footnoteRe = regexp.MustCompile(`\(\d+\)$`)

// thinkingRe matches a thinking mode row stating that the model thinks.
//
// The row is prose rather than a tick, because DeepSeek has more to say than
// yes: its models support thinking and non-thinking modes and default to one
// of them. The whole sentence is kept as an attribute, and this reads the part
// of it that is the capability every other provider states in one word.
var thinkingRe = regexp.MustCompile(`(?i)\bsupports\b.*\bthinking\b`)

// applyPricing reads the pricing page.
//
// The page is one table laid out with a model per column, so its heading row
// names the models and every row below states one fact about each. A row can
// be preceded by a spanning section label, and the three pricing sections span
// two rows apiece, so the section is carried forward on the builder.
//
// A second heading row would mean a second table, and the reading stops there
// rather than taking that table's column headings for models.
func (b *builder) applyPricing(doc catalog.Document) {
	body := string(doc.Body)
	if match := noteRe.FindStringSubmatch(body); match != nil {
		b.priceNote = text(match[1])
	}
	var ids []string
	for _, match := range rowRe.FindAllStringSubmatch(body, -1) {
		cells := rowCells(match[1])
		if len(cells) < 2 {
			continue
		}
		if strings.EqualFold(cells[0], "model") {
			if len(ids) > 0 {
				return
			}
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
// The row is read from the right because up to two spanning labels can precede
// the row's own label, so the position of the values is fixed but the position
// of the label is not. Whatever sits one cell further left is the section, and
// a section naming a rate is remembered, since the row beneath it carries only
// a period and its amounts.
func (b *builder) applyRow(cells, ids []string) {
	count := min(len(ids), len(cells)-1)
	if count < 1 {
		return
	}
	values := cells[len(cells)-count:]
	label := rowLabel(cells[len(cells)-count-1])
	if i := len(cells) - count - 2; i >= 0 {
		b.rate = rateRows[rowLabel(cells[i])]
	}
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
	if period, ok := periods[label]; ok {
		b.applyRate(m, period, value)
		return
	}
	switch label {
	case "base url (openai format)":
		m.SetAttr(AttrBaseURL, value)
		m.AddList(ListEndpoints, EndpointOpenAI)
	case "base url (anthropic format)":
		m.SetAttr(AttrAnthropicBaseURL, value)
		m.AddList(ListEndpoints, EndpointAnthropic)
	case "model version":
		m.SetAttr(AttrModelVersion, value)
		m.SetAttr(AttrDefaultSnapshot, value)
	case "thinking mode":
		m.SetAttr(AttrThinkingMode, value)
		if thinkingRe.MatchString(value) {
			m.AddList(ListFeatures, catalog.CapabilityReasoning)
		}
	case "context length":
		m.SetLimit(LimitContextWindow, parseCount(value))
	case "max output":
		m.SetLimit(LimitMaxOutputTokens, parseCount(value))
	case "concurrency limit":
		m.SetLimit(LimitConcurrency, parseCount(value))
	case "json output":
		if strings.Contains(value, supported) {
			m.AddList(
				ListFeatures,
				catalog.CapabilityStructuredOutputs,
				catalog.CapabilityJSONMode,
			)
		}
	case "fim completion（beta）":
		m.AddList(ListFeatures, FeatureFIMCompletion)
		m.SetAttr(AttrFIMModes, value)
	default:
		if !strings.Contains(value, supported) || label == "" {
			return
		}
		if endpoint, ok := endpointLabels[label]; ok {
			m.AddList(ListEndpoints, endpoint)
			return
		}
		m.AddList(ListFeatures, featureName(label))
	}
}

// applyRate records one amount against the period it is charged in.
//
// DeepSeek states no rate that is not qualified by a period, so the period is
// a dimension on every price rather than a separate metric, and the footnote
// naming the hours the peak period covers rides on each of them.
func (b *builder) applyRate(m *catalog.Model, period, value string) {
	if b.rate == "" {
		return
	}
	amount, ok := parseAmount(value)
	if !ok {
		return
	}
	m.AddPrice(catalog.Price{
		Metric:   b.rate,
		Unit:     UnitPer1MTokens,
		Amount:   amount,
		Currency: currency,
		Dims:     catalog.Dims{DimPeriod: period},
		Note:     b.priceNote,
	})
}

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
