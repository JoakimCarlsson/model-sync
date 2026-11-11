package cerebras

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Patterns reading the summary paragraph a model page now opens with, in
// place of the model information block it used to carry.
var (
	proseIDRe       = regexp.MustCompile("Model ID:\\s*`([^`]+)`")
	proseContextRe  = regexp.MustCompile(`(?i)Context window:(.*?)\.(?:\s|$)`)
	proseOutputRe   = regexp.MustCompile(`(?i)Max(?:imum)? output:(.*?)\.(?:\s|$)`)
	proseCapsRe     = regexp.MustCompile(`(?i)Capabilities:(.*?)\.(?:\s|$)`)
	proseTierRe     = regexp.MustCompile(`(?i)(\d[\d,.]*\s*[km]?)\s*tokens(?:\s*\(([\d,]+)\))?[^,]*?\b(free|paid)`)
	prosePriceRe    = regexp.MustCompile(`(?i)\$\s*([\d,]*\.?\d+)\s*per million (input|output) tokens`)
	proseListJoinRe = regexp.MustCompile(`(?i)^\s*and\s+`)
)

// applyProsePage reads a model page written as prose: the identifier, the
// free and paid ceilings, the rates and the capabilities its summary
// paragraph states. A page stating no identifier is ignored.
func (b *builder) applyProsePage(doc catalog.Document) {
	body := string(doc.Body)
	id := proseIDRe.FindStringSubmatch(body)
	if id == nil {
		return
	}
	m := b.model(id[1], KindChat)
	m.AddSource(doc.URL)
	applyProseTiers(m, proseContextRe, body, LimitContextWindowFree, LimitContextWindow)
	applyProseTiers(m, proseOutputRe, body, LimitMaxOutputTokensFree, LimitMaxOutputTokens)
	applyProsePricing(m, body)
	applyProseCapabilities(m, body)
}

// applyProseTiers records the free and paid values of the sentence label
// matches in body, preferring the exact count a value writes in parentheses
// over the rounded one before it. free and paid name the limits recorded.
func applyProseTiers(m *catalog.Model, label *regexp.Regexp, body, free, paid string) {
	sentence := label.FindStringSubmatch(body)
	if sentence == nil {
		return
	}
	for _, tier := range proseTierRe.FindAllStringSubmatch(sentence[1], -1) {
		count := parseCount(tier[1])
		if tier[2] != "" {
			count = parseCount(tier[2])
		}
		key := paid
		if strings.EqualFold(tier[3], "free") {
			key = free
		}
		m.SetLimit(key, count)
	}
}

// applyProsePricing records the input and output rates the summary quotes
// per million tokens.
func applyProsePricing(m *catalog.Model, body string) {
	for _, match := range prosePriceRe.FindAllStringSubmatch(body, -1) {
		amount, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", ""), 64)
		if err != nil {
			continue
		}
		metric := MetricInputTokens
		if strings.EqualFold(match[2], "output") {
			metric = MetricOutputTokens
		}
		m.AddPrice(catalog.Price{
			Metric:   metric,
			Unit:     UnitPer1MTokens,
			Amount:   amount,
			Currency: currency,
		})
	}
}

// applyProseCapabilities records the capabilities the summary lists, the
// ones naming an input modality as that modality.
func applyProseCapabilities(m *catalog.Model, body string) {
	sentence := proseCapsRe.FindStringSubmatch(body)
	if sentence == nil {
		return
	}
	for _, name := range strings.Split(sentence[1], ",") {
		name = strings.TrimSpace(proseListJoinRe.ReplaceAllString(name, ""))
		if name == "" {
			continue
		}
		if modality, ok := capabilityModalities[strings.ToLower(name)]; ok {
			m.AddList(ListInputModalities, modality)
			continue
		}
		m.AddList(ListFeatures, featureName(name))
	}
}
