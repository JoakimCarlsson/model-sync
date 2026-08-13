package cohere

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Patterns over the pricing page. Cohere states its rates two ways: current
// products as data behind rate cards, and withdrawn models as sentences in the
// page's questions and answers.
var (
	// cardMarker opens one rate card in the payload. The cards are separated
	// rather than matched whole, because a card runs to several thousand
	// characters of prose and a bounded repeat cannot reach across it.
	cardMarker = `"_type":"model","highlightModel":`
	// cardNameRe matches the product a card is headed by and the denominator
	// its amounts are quoted against.
	cardNameRe = regexp.MustCompile(`"modelName":"([^"]+)","per":"([^"]*)"`)
	// cardRatesRe matches the amounts a card states.
	cardRatesRe = regexp.MustCompile(`"pricings":(\[[^\]]*\])`)
	// legacyRe matches a sentence stating the rate of a withdrawn model.
	legacyRe = regexp.MustCompile(
		`([A-Za-z][A-Za-z0-9+\- ]*?) pricing is \$([\d.]+)/1M tokens for ` +
			`input and \$([\d.]+)/1M tokens for output`,
	)
	// ayaRe matches the sentence stating one rate for two research models,
	// which is the only rate the page states for more than one model at once.
	ayaRe = regexp.MustCompile(
		`(Aya Expanse) models \([^)]*\) on the API are charged at ` +
			`\$([\d.]+)/1M tokens for input and \$([\d.]+)/1M tokens for output`,
	)
	pushRe = regexp.MustCompile(
		`(?s)self\.__next_f\.push\(\[1,("(?:[^"\\]|\\.)*")\]\)`,
	)
)

// cardRate is one amount on a rate card, with the wording Cohere labels it by.
// A card that quotes one of its two amounts against a different denominator
// overrides it in place.
// A side of a card that is labelled but carries no amount states no rate, so
// the amounts are pointers: a card quoting one figure against a thousand
// searches leaves the other side empty, which is not a rate of zero.
type cardRate struct {
	InputLabel  string   `json:"inputLabel"`
	InputPrice  *float64 `json:"inputPrice"`
	OutputLabel string   `json:"outputLabel"`
	OutputPrice *float64 `json:"outputPrice"`
	OverridePer string   `json:"overridePer"`
}

// applyPricing reads the pricing page.
//
// A rate is recorded only against a model the overview already established,
// because the page names products and platforms as well as models and only the
// overview says which of those names the API answers to.
func (b *builder) applyPricing(doc catalog.Document) {
	body := flight(doc.Body)
	for card := range strings.SplitSeq(body, cardMarker) {
		name := cardNameRe.FindStringSubmatch(card)
		rates := cardRatesRe.FindStringSubmatch(card)
		if name == nil || rates == nil {
			continue
		}
		var parsed []cardRate
		if err := json.Unmarshal([]byte(rates[1]), &parsed); err != nil {
			continue
		}
		for _, r := range parsed {
			b.addCard(doc, name[1], name[2], r)
		}
	}
	for _, match := range legacyRe.FindAllStringSubmatch(body, -1) {
		b.addTokenRates(doc, match[1], match[2], match[3])
	}
	for _, match := range ayaRe.FindAllStringSubmatch(body, -1) {
		b.addTokenRates(doc, match[1], match[2], match[3])
	}
}

// addCard records both amounts of one rate card.
func (b *builder) addCard(
	doc catalog.Document,
	product, per string,
	r cardRate,
) {
	for _, side := range []struct {
		label  string
		amount *float64
	}{{r.InputLabel, r.InputPrice}, {r.OutputLabel, r.OutputPrice}} {
		metric, ok := cardLabels[strings.ToLower(strings.TrimSpace(side.label))]
		if !ok || side.amount == nil {
			continue
		}
		denominator := per
		if r.OverridePer != "" {
			denominator = r.OverridePer
		}
		quoted, ok := cardUnits[strings.ToLower(strings.TrimSpace(denominator))]
		if !ok {
			continue
		}
		if quoted.Metric != "" {
			metric = quoted.Metric
		}
		for _, id := range b.identify(product) {
			b.price(doc, id, metric, quoted.Unit, *side.amount)
		}
	}
}

// addTokenRates records the pair of per-token amounts a sentence states.
func (b *builder) addTokenRates(doc catalog.Document, product, in, out string) {
	for _, id := range b.identify(product) {
		b.price(doc, id, MetricInputTokens, UnitPer1MTokens, amount(in))
		b.price(doc, id, MetricOutputTokens, UnitPer1MTokens, amount(out))
	}
}

// price records one rate against a model the overview established. A retired
// model is left unpriced whatever the page still says about it: the page's
// questions and answers outlive the models they answer for.
func (b *builder) price(
	doc catalog.Document,
	id string,
	metric catalog.Metric,
	unit catalog.Unit,
	value float64,
) {
	m, ok := b.models[id]
	if !ok || strings.HasPrefix(m.Attrs[AttrState], "retired") {
		return
	}
	m.AddSource(doc.URL)
	m.AddPrice(catalog.Price{
		Metric:   metric,
		Unit:     unit,
		Amount:   value,
		Currency: currency,
	})
}

// identify reports which models a name on the pricing page refers to.
//
// A product name is looked up first, because a product and a withdrawn model
// can share a name: the card headed "Command R" states the rate of the model
// serving under that name today, which is command-r-08-2024, not the alias
// command-r that points at the 2024 version the page prices separately.
//
// A name the page states precisely reduces to an identifier instead, so that
// "Command R+ 08-2024" reaches command-r-plus-08-2024 without a table.
func (b *builder) identify(name string) []string {
	if ids, ok := cardModels[strings.ToLower(strings.TrimSpace(name))]; ok {
		var out []string
		for _, id := range ids {
			if _, ok := b.models[id]; ok {
				out = append(out, id)
			}
		}
		return out
	}
	if _, ok := b.models[slugID(name)]; ok {
		return []string{slugID(name)}
	}
	return nil
}

// slugID reduces a model named in prose to the identifier it is called by.
func slugID(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, "+", "-plus")
	s = strings.Join(strings.Fields(s), "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return s
}

// amount reads a decimal rate.
func amount(text string) float64 {
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0
	}
	return value
}

// flight returns the payload a rendered page carries, which the page embeds a
// piece at a time, each piece a JSON string.
func flight(body []byte) string {
	text := string(body)
	if !strings.Contains(text, "self.__next_f.push") {
		return text
	}
	var out strings.Builder
	for _, match := range pushRe.FindAllStringSubmatch(text, -1) {
		var piece string
		if err := json.Unmarshal([]byte(match[1]), &piece); err != nil {
			continue
		}
		out.WriteString(piece)
	}
	return out.String()
}
