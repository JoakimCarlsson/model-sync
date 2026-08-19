package vertexai

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// PricingURL is the page every model page points at for its rates.
//
// The billing catalog is read in preference to it, and for the models Vertex
// meters under its own service it is not read at all: the page states one
// model in several tables with only a tab to tell them apart, where a SKU
// carries the region and the usage type as fields. But Vertex bills for the
// models it serves for other labs through those labs' own services, so the
// Claude, Grok and Mistral releases it serves have no SKU under the Vertex
// service, and this page is the only place Google publishes a rate for them. A
// model the catalog already priced is not read here, so the two documents
// never state the same rate twice.
const PricingURL = docsBase +
	"/gemini-enterprise-agent-platform/generative-ai/pricing"

// Metrics and dimensions the pricing page states that the billing catalog
// does not.
const (
	// MetricCacheWriteTokens is what writing a prompt into the cache costs,
	// which the catalog meters as input and the page states apart.
	MetricCacheWriteTokens catalog.Metric = "cache_write_tokens"
	// DimRegion is which endpoint a rate holds for. The page tabs the rate
	// tables by endpoint where the rate varies between them.
	DimRegion = "region"
	// DimCacheTTL is how long the cached prompt is kept, which the page prices
	// separately at five minutes and at an hour.
	DimCacheTTL = "cache_ttl"
)

// Context bands the pricing page prices a model in. The catalog marks the
// longer band "(Long)" and never says where it begins; the page heads the two
// columns with the threshold, so the band is recorded by what the page says
// rather than by the catalog's word for it.
const (
	ContextUpTo200K = "up_to_200k_input_tokens"
	ContextOver200K = "over_200k_input_tokens"
)

var (
	// pricingMarkerRe matches, in the order the page writes them, the heading
	// that opens a publisher's rates, the panel that says which endpoint the
	// rates under it hold for, and a rate table. A table is read against the
	// panel it sits in, and a heading closes the last panel, because only the
	// Claude rates are tabbed by endpoint and a table under any other heading
	// holds wherever the model is sold.
	pricingMarkerRe = regexp.MustCompile(
		`(?is)<h2[^>]*>|role="tabpanel" aria-labelledby="(tab-[^"]+)"|` +
			`<table class="nooFgd[^"]*">(.*?)</table>`,
	)
	// pricingTabRe matches the name of one such endpoint, which the page keeps
	// in the tab that selects it.
	pricingTabRe = regexp.MustCompile(
		`id="(tab-[^"]+)"[^>]*track-metadata-eventdetail="([^"]*)"`,
	)
	pricingRowRe  = regexp.MustCompile(`(?is)<tr[^>]*>(.*?)</tr>`)
	pricingHeadRe = regexp.MustCompile(`(?is)<th[^>]*>(.*?)</th>`)
	pricingCellRe = regexp.MustCompile(`(?is)<td[^>]*>(.*?)</td>`)
	// pricingAmountRe matches the first amount a cell states. A cell may state
	// a second in brackets, Mistral OCR quoting a page as well as a million
	// tokens, and that quote is kept as the rate's note rather than read as
	// another rate in a unit the column does not name.
	pricingAmountRe = regexp.MustCompile(`\$\s*([0-9]+(?:\.[0-9]+)?)`)
	// pricingNoteRe matches that bracketed second quote.
	pricingNoteRe = regexp.MustCompile(`\((or [^)]*)\)`)
	// pricingBandRe matches the threshold a price column is headed with.
	pricingBandRe = regexp.MustCompile(`(?i)(=&lt;|&gt;|=<|>)\s*200K input`)
	// pricingNameRe matches whatever in a model name is not part of an
	// identifier, so that the name a rate table writes and the identifier a
	// model page states reduce to the same thing.
	pricingNameRe = regexp.MustCompile(`[^a-z0-9]+`)
)

// pricingMetric says what a row of the rate table counts. The page names the
// row for the direction, for the cache and for the serving path at once, so
// the words are weighed rather than the phrase matched.
func pricingMetric(kind string) (catalog.Metric, bool) {
	switch {
	case strings.Contains(kind, "write"):
		return MetricCacheWriteTokens, true
	case strings.Contains(kind, "hit"):
		return MetricCachedInputTokens, true
	case strings.Contains(kind, "output"):
		return MetricOutputTokens, true
	case strings.Contains(kind, "input"):
		return MetricInputTokens, true
	}
	return "", false
}

// applyPricingPage records the rates the page states for the models the
// billing catalog leaves unpriced.
//
// A row names the model the way Google writes it in prose, "Claude Sonnet 4.5"
// against the page's claude-sonnet-4-5 and "Opus 5" against claude-opus-5, so
// the two are reduced to their letters and digits and the row reaches the one
// model whose identifier ends in what is left. A row reaching none, as
// "Mistral Small 3.1 (25.03)" does against mistral-small-2503, is passed over:
// nothing in either document says the two are one model, and a rate on the
// wrong model is worse than no rate at all.
func (b *builder) applyPricingPage(doc catalog.Document) {
	if doc.URL != PricingURL {
		return
	}
	body := string(doc.Body)
	tabs := map[string]string{}
	for _, tab := range pricingTabRe.FindAllStringSubmatch(body, -1) {
		tabs[tab[1]] = specText(tab[2])
	}
	unpriced := b.unpricedNames()
	region := ""
	for _, marker := range pricingMarkerRe.FindAllStringSubmatch(body, -1) {
		switch {
		case marker[1] != "":
			region = tabs[marker[1]]
		case marker[2] != "":
			b.applyRateTable(marker[2], region, unpriced, doc.URL)
		default:
			region = ""
		}
	}
}

// unpricedNames indexes the models the billing catalog stated no rate for, by
// the letters and digits of their identifiers.
func (b *builder) unpricedNames() map[string]string {
	out := map[string]string{}
	for id, m := range b.models {
		if len(m.Prices) > 0 {
			continue
		}
		out[pricingName(id)] = id
	}
	return out
}

// pricingName reduces a model name to the letters and digits of it.
func pricingName(name string) string {
	return pricingNameRe.ReplaceAllString(strings.ToLower(name), "")
}

// applyRateTable reads one table of rates.
//
// The model is written once and the cell is left blank on the rows under it,
// so the name last written is the model the following rows price. The columns
// after the model and the row kind are the context bands the page prices
// apart, and a blank amount is a band the model has no rate in rather than a
// rate of nothing.
func (b *builder) applyRateTable(
	table, region string,
	unpriced map[string]string,
	source string,
) {
	var bands []string
	id := ""
	for _, row := range pricingRowRe.FindAllStringSubmatch(table, -1) {
		heads := pricingHeadRe.FindAllStringSubmatch(row[1], -1)
		if heads != nil {
			bands = readBands(heads)
			continue
		}
		cells := pricingCellRe.FindAllStringSubmatch(row[1], -1)
		if len(cells) < 3 {
			continue
		}
		if name := specText(cells[0][1]); name != "" {
			id = unpriced[pricingName(name)]
		}
		if id == "" {
			continue
		}
		b.applyRateRow(id, cells, bands, region, source)
	}
}

// readBands reads which context band each price column holds for.
func readBands(heads [][]string) []string {
	bands := make([]string, 0, len(heads))
	for _, head := range heads {
		bands = append(bands, bandOf(head[1]))
	}
	return bands
}

// bandOf reads the threshold a price column is headed with. A column headed
// with no threshold prices the model at any length.
func bandOf(head string) string {
	match := pricingBandRe.FindStringSubmatch(head)
	if match == nil {
		return ""
	}
	if strings.HasPrefix(match[1], "=") {
		return ContextUpTo200K
	}
	return ContextOver200K
}

// applyRateRow records the rates one row of the table states.
func (b *builder) applyRateRow(
	id string,
	cells [][]string,
	bands []string,
	region, source string,
) {
	kind := strings.ToLower(specText(cells[1][1]))
	metric, ok := pricingMetric(kind)
	if !ok {
		return
	}
	m := b.models[id]
	m.AddSource(source)
	for at := 2; at < len(cells); at++ {
		amount, note, ok := readAmount(cells[at][1])
		if !ok {
			continue
		}
		band := ""
		if at < len(bands) {
			band = bands[at]
		}
		m.AddPrice(catalog.Price{
			Metric:   metric,
			Unit:     UnitPer1MTokens,
			Amount:   amount,
			Currency: defaultCurrency,
			Note:     note,
			Dims: catalog.Dims{}.
				With(DimTier, rateTier(kind)).
				With(DimCacheTTL, cacheTTL(kind)).
				With(DimRegion, region).
				With(DimContext, band),
		})
	}
}

// rateTier reads the serving path a row prices, which is the batch path where
// the row says so and the standard one otherwise.
func rateTier(kind string) string {
	if strings.Contains(kind, "batch") {
		return TierBatch
	}
	return TierStandard
}

// cacheTTL reads how long a cache write is kept, which the page prices at five
// minutes and at an hour.
func cacheTTL(kind string) string {
	switch {
	case strings.HasPrefix(kind, "5m"):
		return "5m"
	case strings.HasPrefix(kind, "1h"):
		return "1h"
	}
	return ""
}

// readAmount reads the rate a cell states, and whatever the cell says besides
// it. A cell stating nothing is a band the model is not sold in.
func readAmount(cell string) (float64, string, bool) {
	text := specText(cell)
	match := pricingAmountRe.FindStringSubmatch(text)
	if match == nil {
		return 0, "", false
	}
	amount, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, "", false
	}
	note := ""
	if extra := pricingNoteRe.FindStringSubmatch(text); extra != nil {
		note = extra[1]
	}
	return amount, note, true
}
