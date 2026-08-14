package ollama

import (
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// The rates a cloud model's page quotes, and the denominator it quotes them
// against. It states one denominator for all three.
const (
	MetricInputTokens       catalog.Metric = "input_tokens"
	MetricCachedInputTokens catalog.Metric = "cached_input_tokens"
	MetricOutputTokens      catalog.Metric = "output_tokens"

	UnitPer1MTokens catalog.Unit = "per_1m_tokens"
)

// DimDeployment says which build a rate belongs to. Every priced model is also
// distributed to run on the reader's own machine, where it costs nothing, so a
// rate left undimensioned would read as the price of running the model at all.
const DimDeployment = "deployment"

// deploymentCloud is the only deployment Ollama charges for.
const deploymentCloud = "cloud"

const currencyUSD = "USD"

// costRe matches the head of the card a cloud model's page opens with.
// Requiring the denominator is what makes the amounts under it readable: the
// card states "/1M tokens" once and then three bare amounts.
var costRe = regexp.MustCompile(
	`(?is)>Cost</span>\s*<span[^>]*>\s*/1M tokens\s*</span>`,
)

// contextCard opens the card that follows the rates, and so ends them.
const contextCard = `>Context</div>`

// rateRe matches one amount of the cost card and the token it counts. The
// amount is set in tabular figures, which nothing else on the page is.
var rateRe = regexp.MustCompile(
	`(?is)tabular-nums[^>]*>\s*\$([\d.]+)\s*</div>\s*` +
		`<div[^>]*>\s*([a-z ]+?)\s*</div>`,
)

// rateMetrics map the word under an amount onto what it counts.
var rateMetrics = map[string]catalog.Metric{
	"input":  MetricInputTokens,
	"cached": MetricCachedInputTokens,
	"output": MetricOutputTokens,
}

// usageRe matches the card the pages without a rate carry instead: how much of
// a plan's allowance the model draws, drawn as four bars and named in words.
var usageRe = regexp.MustCompile(
	`(?is)>Usage</div>(.{0,900}?)leading-5">\s*([a-z ]+?)\s*</span>`,
)

// filledBarRe matches one filled bar of the usage card. As many are filled as
// the level, which is how the page states the number its pricing page names.
var filledBarRe = regexp.MustCompile(`rounded-full bg-neutral-900`)

// applyModelPage reads a model's own page, which is fetched for the models
// Ollama runs itself.
//
// It carries the one thing the library and the tag listings do not: what the
// model costs. Most cloud models state only how heavily they draw on a plan's
// allowance, and a few state a rate per million tokens, so both are read and
// the level is recorded even where a rate is stated.
func (b *builder) applyModelPage(doc catalog.Document) {
	m, ok := b.models[path.Base(doc.URL)]
	if !ok {
		return
	}
	body := string(doc.Body)
	read := applyCost(m, body)
	if applyUsageLevel(m, body) {
		read = true
	}
	if read {
		m.AddSource(doc.URL)
	}
}

// applyCost records the rates a page quotes, reporting whether it quoted any.
func applyCost(m *catalog.Model, body string) bool {
	head := costRe.FindStringIndex(body)
	if head == nil {
		return false
	}
	card := body[head[1]:]
	if end := strings.Index(card, contextCard); end >= 0 {
		card = card[:end]
	}
	recorded := false
	for _, rate := range rateRe.FindAllStringSubmatch(card, -1) {
		metric, ok := rateMetrics[strings.ToLower(rate[2])]
		if !ok {
			continue
		}
		amount, err := strconv.ParseFloat(rate[1], 64)
		if err != nil {
			continue
		}
		m.AddPrice(catalog.Price{
			Metric:   metric,
			Unit:     UnitPer1MTokens,
			Amount:   amount,
			Currency: currencyUSD,
			Dims:     catalog.Dims{DimDeployment: deploymentCloud},
		})
		recorded = true
	}
	return recorded
}

// applyUsageLevel records how heavily a model draws on a plan, reporting
// whether the page stated it.
func applyUsageLevel(m *catalog.Model, body string) bool {
	card := usageRe.FindStringSubmatch(body)
	if card == nil {
		return false
	}
	m.SetAttr(AttrUsageLevel, card[2])
	m.SetAttr(
		AttrUsageRank,
		strconv.Itoa(len(filledBarRe.FindAllString(card[1], -1))),
	)
	return true
}
