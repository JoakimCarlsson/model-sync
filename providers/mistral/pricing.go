package mistral

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// PricingURL is the API tab of Mistral's pricing page.
const PricingURL = "https://mistral.ai/pricing/api"

// Patterns over the pricing page.
//
// The page is server-rendered markup rather than a payload: each model is a
// card carrying the amounts, the documentation page the card links to, and,
// on each amount, the set of modifiers that amount may be adjusted by. The
// modifiers themselves are stated once, at the top of the page, in the control
// that applies them.
var (
	cardRe = regexp.MustCompile(
		`(?s)<div class="model-item.*?</mistral-block-card-model>`,
	)
	cardLinkRe = regexp.MustCompile(
		`href="https://docs\.mistral\.ai/models/(?:model-cards/)?([a-z0-9._-]+)"`,
	)
	cardDiscountRe = regexp.MustCompile(`data-discounts="([^"]*)"`)
	regionalRe     = regexp.MustCompile(
		`>Regional inference<[\s\S]{0,600}?>\+(\d+)%`,
	)
	cachedRe = regexp.MustCompile(
		`>Cached input tokens<[\s\S]{0,600}?>-(\d+)%`,
	)
	batchHalfRe = regexp.MustCompile(`at half price`)
)

// batchDiscount is what the page's batch tab states, which it writes as a
// fraction of the standard rate in words rather than as a percentage.
const batchDiscount = "50%"

// applyPricingPage records the modifiers Mistral applies to a published rate.
//
// The amounts on this page are the amounts the model pages already state, so
// nothing is priced from here. What only this page states is that a rate can
// be adjusted: an input rate marked as cacheable is billed a tenth for a
// repeated prompt, any rate is billed a tenth more on a regional endpoint, and
// the batch tab halves the whole card. Those are ratios rather than amounts,
// and they are recorded as such rather than multiplied out, because Mistral
// prints no product of them anywhere.
//
// A card is matched to a model by the documentation page it links to, which is
// the same page this parser read the model from. Cards Mistral prints without
// a link, and cards for a product rather than a model, match nothing and are
// left alone.
func (b *builder) applyPricingPage(doc catalog.Document) {
	body := string(doc.Body)
	regional := first(regionalRe, body)
	cached := first(cachedRe, body)
	batch := ""
	if batchHalfRe.MatchString(body) {
		batch = batchDiscount
	}
	for _, card := range cardRe.FindAllString(body, -1) {
		m := b.bySlug(first(cardLinkRe, card))
		if m == nil {
			continue
		}
		m.AddSource(doc.URL)
		if hasDiscount(card, "regional") {
			m.SetAttr(AttrRegionalSurcharge, percent(regional))
		}
		if hasDiscount(card, "cache") {
			m.SetAttr(AttrCachedDiscount, percent(cached))
		}
		m.SetAttr(AttrBatchDiscount, batch)
	}
}

// hasDiscount reports whether any amount on a card is marked as adjustable by
// one modifier. The attribute holding the set is escaped markup, so the name
// is looked for within it rather than parsed out of it.
func hasDiscount(card, name string) bool {
	for _, match := range cardDiscountRe.FindAllStringSubmatch(card, -1) {
		if strings.Contains(match[1], name) {
			return true
		}
	}
	return false
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
