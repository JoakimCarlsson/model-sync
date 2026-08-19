package groq

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Pages stating what holds across the platform rather than for one model.
const (
	BatchURL        = baseURL + "/docs/batch.md"
	ServiceTiersURL = baseURL + "/docs/service-tiers.md"
	FlexURL         = baseURL + "/docs/flex-processing.md"
)

// DimServiceTier says which of Groq's service tiers a rate belongs to.
const DimServiceTier = "service_tier"

// TierBatch is the tier the batch API bills under.
const TierBatch = "batch"

// batchDiscount is the share of the synchronous rate a batch request is
// billed at, which the batch page states as a discount of fifty percent.
const batchDiscount = 0.5

// Notes the platform pages state, kept because they qualify the rates and the
// tiers recorded beside them.
const (
	batchNote = "batch requests are billed at half the synchronous rate; " +
		"the batch discount does not stack with prompt caching, and all " +
		"batch tokens are billed at the batch rate regardless of cache status"
	flexNote = "flex processing is available for all models to paid " +
		"customers with ten times the on-demand rate limits, priced the " +
		"same as on-demand"
)

// serviceTierRe matches one tier in the list the service tiers page opens
// with, which names each in code and then describes it.
var serviceTierRe = regexp.MustCompile("(?m)^[*] \x60([a-z_]+)\x60:")

// batchMetrics are the rates the batch discount applies to. Groq states the
// discount against the synchronous rate and says in the same breath that a
// cached token is billed at the batch rate too, so the cached rate is not
// halved a second time.
var batchMetrics = map[catalog.Metric]bool{
	MetricInputTokens:  true,
	MetricOutputTokens: true,
	MetricAudio:        true,
	MetricSpeech:       true,
}

// applyBatch reads the batch page.
//
// The page names the models the batch API accepts, one table per endpoint, and
// states one rate for all of them: half of what the same request costs
// synchronously. So each model's rates are recorded a second time under the
// batch tier, which is what the reader comparing a batch job against a
// synchronous one needs and what the sentence alone does not give.
//
// This is read after every document that states a rate, because it has none of
// its own to state and works from the ones already recorded.
func (b *builder) applyBatch(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		idCol := columnOf(t.Headers, colModelID)
		if idCol < 0 {
			continue
		}
		for _, row := range t.Rows {
			m, ok := b.models[clean(cellAt(row, idCol))]
			if !ok {
				continue
			}
			m.AddSource(doc.URL)
			m.SetAttr(AttrBatch, "true")
			m.AddNote(batchNote)
			addBatchPrices(m)
		}
	}
}

// addBatchPrices records each of a model's synchronous rates again at the
// batch rate.
func addBatchPrices(m *catalog.Model) {
	var batched []catalog.Price
	for _, p := range m.Prices {
		if !batchMetrics[p.Metric] || p.Dims[DimServiceTier] != "" {
			continue
		}
		p.Amount *= batchDiscount
		p.Dims = p.Dims.With(DimServiceTier, TierBatch)
		batched = append(batched, p)
	}
	for _, p := range batched {
		m.AddPrice(p)
	}
}

// applyServiceTiers reads the pages naming the tiers a request may ask for.
//
// The tiers are a property of the API rather than of one model, and the flex
// page says so in as many words: every model can be asked for on the flex
// tier. So both pages are recorded against every model, the tiers as a list
// and what flex costs as a note, since the flex page states that its rate is
// the on-demand one rather than a rate of its own.
func (b *builder) applyServiceTiers(doc catalog.Document) {
	body := string(doc.Body)
	var tiers []string
	for _, match := range serviceTierRe.FindAllStringSubmatch(body, -1) {
		tiers = append(tiers, match[1])
	}
	note := ""
	if strings.Contains(body, "Flex Processing") {
		note = flexNote
	}
	if len(tiers) == 0 && note == "" {
		return
	}
	for _, id := range b.order {
		m := b.models[id]
		m.AddSource(doc.URL)
		m.AddList(ListServiceTiers, tiers...)
		m.AddList(ListParameters, "service_tier")
		m.AddNote(note)
	}
}
