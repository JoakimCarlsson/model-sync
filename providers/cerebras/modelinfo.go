package cerebras

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Metrics Cerebras bills on.
const (
	MetricInputTokens  catalog.Metric = "input_tokens"
	MetricOutputTokens catalog.Metric = "output_tokens"
)

// UnitPer1MTokens is the only denominator Cerebras quotes against.
const UnitPer1MTokens catalog.Unit = "per_1m_tokens"

// currency is the only currency Cerebras quotes.
const currency = "USD"

// Numeric keys the model pages populate. Cerebras allows a longer output on a
// paid plan than a free one, as it does a longer context.
const (
	LimitMaxOutputTokens     = "max_output_tokens"
	LimitMaxOutputTokensFree = "max_output_tokens_free"
)

// Enumeration keys the model pages populate.
const (
	ListFeatures         = "features"
	ListEndpoints        = "endpoints"
	ListInputModalities  = "input_modalities"
	ListOutputModalities = "output_modalities"
)

// AttrModelCard records where the weights this model serves are published.
const AttrModelCard = "model_card_url"

// featureNames map the capability Cerebras names onto the catalog's
// vocabulary. Only the names that differ are listed; the rest are Cerebras'
// own words with their spacing reduced to an identifier.
var featureNames = map[string]string{
	"tool calling":          "function_calling",
	"parallel tool calling": "parallel_tool_calls",
	"image inputs":          "image_input",
}

// Patterns over the model information block. Each page ends with one call
// carrying every fact about the model as an attribute, which is where the
// rates, the output bound and the capabilities are stated.
var (
	blockRe = regexp.MustCompile(`(?s)<ModelInfo\b(.*?)/>`)
	// propRe matches one attribute, either a bare string or a braced value.
	propRe = regexp.MustCompile(`(?s)(\w+)=(?:"([^"]*)"|\{(.*))`)
	// fieldRe matches one field of a braced object.
	fieldRe = regexp.MustCompile(`(\w+)\s*:\s*"([^"]*)"`)
	// listRe matches one entry of a braced list, or of a list-valued field.
	listRe = regexp.MustCompile(`"([^"]*)"`)
	// amountRe matches a rate and, where Cerebras writes one, the denominator
	// it is quoted against.
	amountRe = regexp.MustCompile(`\$\s*([\d,]*\.?\d+)\s*(?:/\s*([\w ]+))?`)
)

// applyModelPage reads one model's page.
func (b *builder) applyModelPage(doc catalog.Document) {
	block := blockRe.FindStringSubmatch(string(doc.Body))
	if block == nil {
		return
	}
	props := parseProps(block[1])
	id := props["modelId"]
	if id == "" {
		return
	}
	m := b.model(id, KindChat)
	m.AddSource(doc.URL)
	m.SetAttr(AttrModelCard, props["modelCardUrl"])
	applyTiered(
		m,
		props["contextLength"],
		LimitContextWindowFree,
		LimitContextWindow,
	)
	applyTiered(
		m,
		props["maxOutput"],
		LimitMaxOutputTokensFree,
		LimitMaxOutputTokens,
	)
	applyPricing(m, props["pricing"])
	for _, name := range listRe.FindAllStringSubmatch(props["features"], -1) {
		m.AddList(ListFeatures, featureName(name[1]))
	}
	for _, name := range listRe.FindAllStringSubmatch(props["endpoints"], -1) {
		m.AddList(ListEndpoints, name[1])
	}
	applyFormats(m, props["inputOutput"])
}

// applyTiered records a bound Cerebras states once per plan.
func applyTiered(m *catalog.Model, value, free, paid string) {
	for _, field := range fieldRe.FindAllStringSubmatch(value, -1) {
		switch field[1] {
		case "freeTier":
			m.SetLimit(free, parseCount(field[2]))
		case "paidTiers":
			m.SetLimit(paid, parseCount(field[2]))
		}
	}
}

// applyPricing records the two rates a model page states.
//
// One page writes its amounts with the denominator and another without, so a
// missing denominator is read as the per-token one every Cerebras rate uses.
func applyPricing(m *catalog.Model, value string) {
	for _, field := range fieldRe.FindAllStringSubmatch(value, -1) {
		metric := MetricInputTokens
		if field[1] == "outputPrice" {
			metric = MetricOutputTokens
		} else if field[1] != "inputPrice" {
			continue
		}
		match := amountRe.FindStringSubmatch(field[2])
		if match == nil {
			continue
		}
		amount, err := strconv.ParseFloat(
			strings.ReplaceAll(match[1], ",", ""),
			64,
		)
		if err != nil {
			continue
		}
		m.AddPrice(catalog.Price{
			Metric:   metric,
			Unit:     UnitPer1MTokens,
			Amount:   amount,
			Currency: currency,
		})
	}
}

// applyFormats records what a model takes and what it emits.
func applyFormats(m *catalog.Model, value string) {
	for _, field := range strings.Split(value, "inputFormats")[1:] {
		before, after, _ := strings.Cut(field, "outputFormats")
		addFormats(m, ListInputModalities, before)
		addFormats(m, ListOutputModalities, after)
	}
}

// addFormats records every format named in one list.
func addFormats(m *catalog.Model, key, value string) {
	for _, name := range listRe.FindAllStringSubmatch(value, -1) {
		m.AddList(key, strings.ToLower(name[1]))
	}
}

// featureName rewrites a capability into the catalog's vocabulary.
func featureName(name string) string {
	key := strings.ToLower(strings.TrimSpace(name))
	if mapped, ok := featureNames[key]; ok {
		return mapped
	}
	return strings.ReplaceAll(key, " ", "_")
}

// parseProps reads the attributes of the model information block.
//
// An attribute's value is either a quoted string or a braced expression, and a
// braced expression runs until its own brace closes, so the braces are
// counted rather than matched with a pattern.
func parseProps(block string) map[string]string {
	out := map[string]string{}
	rest := block
	for {
		match := propRe.FindStringSubmatchIndex(rest)
		if match == nil {
			return out
		}
		name := rest[match[2]:match[3]]
		if match[4] >= 0 {
			out[name] = rest[match[4]:match[5]]
			rest = rest[match[1]:]
			continue
		}
		value, after := untilClose(rest[match[6]:])
		out[name] = value
		rest = after
	}
}

// untilClose splits a braced expression from what follows it, having already
// consumed the opening brace.
func untilClose(s string) (value, rest string) {
	depth := 1
	for i, r := range s {
		switch r {
		case '{':
			depth++
		case '}':
			if depth--; depth == 0 {
				return s[:i], s[i+1:]
			}
		}
	}
	return s, ""
}
