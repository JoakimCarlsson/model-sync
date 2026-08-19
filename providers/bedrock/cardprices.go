package bedrock

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Dimension keys a card's pricing table varies a rate along.
const (
	// DimOption is how a request reaches the model: within one Region, across
	// the Regions of one geography, or anywhere in the world. AWS charges the
	// last of the three less and states no Region for any of them, so this is
	// what a card's rates vary along where the price list's vary by Region.
	DimOption = "inference_option"
	// DimScope carries what else a card names a table of rates for, which is
	// the partition it applies to on the one card pricing GovCloud apart.
	DimScope = "scope"
)

// ContextShort is the band of a rate AWS meters below the prompt length it
// prices apart. It is recorded only where a card states two bands, since a
// model priced once is not priced short.
const ContextShort = "short"

var (
	// cardUnitRe matches the denominator a card's pricing section states
	// under its tables. Nothing is read from a table stating none: a column
	// of amounts says what a thing costs and never what it is counted in.
	cardUnitRe = regexp.MustCompile(
		`(?i)All prices are per 1 million tokens`,
	)
	// cardTierRe matches the service tier a card's pricing section says its
	// rates are for.
	cardTierRe = regexp.MustCompile(
		`(?i)Pricing shown is for the (\w+) tier`,
	)
	// cardAmountRe matches an amount, which a card writes with its currency
	// sign and writes as a dash where the rate does not exist.
	cardAmountRe = regexp.MustCompile(`^\$([\d.,]+)$`)
	// cardBandRe matches the prompt length a card bands its rates by.
	cardBandRe = regexp.MustCompile(`(?i)^(short|long) context window`)
)

// priceColumns map a column of a card's pricing table onto what it counts,
// matched in this order because the cache columns are input columns too.
var priceColumns = []struct {
	fragment string
	metric   catalog.Metric
}{
	{"cache write", MetricCacheWriteTokens},
	{"cache read", MetricCachedInputTokens},
	{"output", MetricOutputTokens},
	{"input", MetricInputTokens},
}

// applyCardPrices records the rates a card states itself.
//
// Most cards price nothing and refer the reader to the pricing page, and the
// price list is where their rates are read from. The cards of the models
// reached through the newest endpoints state a table instead, and for several
// of those the price list carries no meter at all, so this is the only place
// AWS states what they cost. The two do not overlap where both exist: the
// list's rates vary by Region and these vary by how a request is routed, so
// they are recorded under a dimension of their own rather than merged.
func applyCardPrices(m *catalog.Model, t table, c card) {
	body := string(c.doc.Body)
	if !cardUnitRe.MatchString(body) {
		return
	}
	tier := ""
	if match := cardTierRe.FindStringSubmatch(body); match != nil {
		tier = serviceTiers[strings.ToLower(match[1])]
	}
	base := catalog.Dims{}.
		With(DimTier, tier).
		With(DimContext, priceBand(t.caption)).
		With(DimScope, priceScope(t.caption))
	for _, row := range t.rows {
		applyPriceRow(m, t, row, base)
	}
}

// applyPriceRow records one routing option's rates.
func applyPriceRow(
	m *catalog.Model,
	t table,
	row []string,
	base catalog.Dims,
) {
	option := slug(cell(row, 0))
	if option == "" {
		return
	}
	for i := range row {
		metric, ok := priceMetric(t.heading(i))
		if !ok {
			continue
		}
		amount, ok := cardAmount(cell(row, i))
		if !ok {
			continue
		}
		m.AddPrice(catalog.Price{
			Metric:   metric,
			Unit:     UnitPer1MTokens,
			Amount:   amount,
			Currency: currency,
			Dims: base.
				With(DimOption, option).
				With(DimCacheTTL, cacheTTL(t.heading(i))),
		})
	}
}

// priceMetric reads what a column of a card's pricing table counts. The
// heading divides the metric from the qualification with an em dash, which is
// dropped along with every other rune that is not a letter.
func priceMetric(heading string) (catalog.Metric, bool) {
	words := strings.ToLower(nonWordRe.ReplaceAllString(heading, " "))
	for _, entry := range priceColumns {
		if strings.Contains(words, entry.fragment) {
			return entry.metric, true
		}
	}
	return "", false
}

// nonWordRe matches everything a heading is not read by.
var nonWordRe = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// cardAmount reads an amount out of a cell, which is absent where AWS writes
// a dash rather than a figure.
func cardAmount(value string) (float64, bool) {
	match := cardAmountRe.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil {
		return 0, false
	}
	amount, err := strconv.ParseFloat(
		strings.ReplaceAll(match[1], ",", ""),
		64,
	)
	if err != nil || amount == 0 {
		return 0, false
	}
	return amount, true
}

// priceBand reads the prompt length band a table of rates applies to.
func priceBand(caption string) string {
	match := cardBandRe.FindStringSubmatch(caption)
	if match == nil {
		return ""
	}
	if strings.EqualFold(match[1], "long") {
		return ContextLong
	}
	return ContextShort
}

// priceScope reads what else a caption restricts a table of rates to, which
// is the partition on the one card pricing GovCloud apart from the rest.
func priceScope(caption string) string {
	if caption == "" || cardBandRe.MatchString(caption) {
		return ""
	}
	return slug(caption)
}
