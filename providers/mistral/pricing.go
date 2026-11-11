package mistral

import (
	"regexp"
	"slices"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// PricingURL is Mistral's API pricing page. The marketing site's pricing tab
// redirects here.
const PricingURL = baseURL + "/inference/pricing"

// RegionalURL is the guide to regional inference, and the only document
// stating what a regional endpoint adds to a rate.
const RegionalURL = baseURL + "/inference/regional-inference"

// Patterns over the pricing page and the regional inference guide.
//
// The pricing page renders its amounts in the browser: the payload it serves
// carries only the model pages each section prices, by slug, and the copy the
// page shows. The amounts themselves are the ones the model pages state.
var (
	pricedSlugsRe = regexp.MustCompile(`"slugs":\[([^\]]*)\]`)
	batchRe       = regexp.MustCompile(
		`Batching allows you to run asynchronous inference[^"]*?at a (\d+)% discount`,
	)
	regionalRe = regexp.MustCompile(`\(a (\d+)% upcharge\)`)
)

// batchEndpoint is the path a model answers batch jobs on.
const batchEndpoint = "/v1/batch"

// applyPricingPage records which models Mistral prices and the batch discount
// a published rate may be adjusted by.
//
// The discount is a ratio rather than an amount and is recorded as such,
// because Mistral prints no product of it anywhere. It is recorded only for a
// model whose page names the batch endpoint, since a model that takes no
// batch job is never billed at the batch rate.
func (b *builder) applyPricingPage(doc catalog.Document) {
	body := flight(doc.Body)
	batch := percent(first(batchRe, body))
	for _, match := range pricedSlugsRe.FindAllStringSubmatch(body, -1) {
		for _, slug := range quoted(match[1]) {
			m := b.bySlug(slug)
			if m == nil {
				continue
			}
			m.AddSource(doc.URL)
			b.priced[m.ID] = true
			if slices.Contains(m.Lists[ListEndpoints], batchEndpoint) {
				m.SetAttr(AttrBatchDiscount, batch)
			}
		}
	}
}

// applyRegional records the surcharge a regional endpoint adds to every rate
// of the models the pricing page prices. It must follow applyPricingPage,
// which establishes that set.
func (b *builder) applyRegional(doc catalog.Document) {
	surcharge := percent(first(regionalRe, flight(doc.Body)))
	if surcharge == "" {
		return
	}
	for _, id := range b.sortedIDs() {
		if !b.priced[id] {
			continue
		}
		m := b.models[id]
		m.AddSource(doc.URL)
		m.SetAttr(AttrRegionalSurcharge, surcharge)
	}
}

// percent renders a figure the page states as a bare number.
func percent(value string) string {
	if value == "" {
		return ""
	}
	return value + "%"
}

// bySlug returns the model whose documentation page carries a slug, or nil.
func (b *builder) bySlug(slug string) *catalog.Model {
	if slug == "" {
		return nil
	}
	id, ok := b.slugs[slug]
	if !ok {
		return nil
	}
	return b.models[id]
}
