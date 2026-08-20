package openrouter

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// detail is the shape of a model's endpoint document.
type detail struct {
	Data detailData `json:"data"`
}

type detailData struct {
	ID        string     `json:"id"`
	Endpoints []endpoint `json:"endpoints"`
}

// endpoint is one upstream serving the model, as OpenRouter publishes it.
type endpoint struct {
	ProviderName            string                     `json:"provider_name"`
	Tag                     string                     `json:"tag"`
	Quantization            string                     `json:"quantization"`
	ContextLength           int64                      `json:"context_length"`
	MaxCompletionTokens     int64                      `json:"max_completion_tokens"`
	MaxPromptTokens         int64                      `json:"max_prompt_tokens"`
	SupportedParameters     []string                   `json:"supported_parameters"`
	SupportsImplicitCaching bool                       `json:"supports_implicit_caching"`
	SupportsVoiceCloning    bool                       `json:"supports_voice_cloning"`
	Pricing                 map[string]json.RawMessage `json:"pricing"`
}

// quantizationUnknown is what OpenRouter writes where the seller has not said
// what precision it serves the weights at, which is a statement that the fact
// is missing rather than a precision.
const quantizationUnknown = "unknown"

// applyDetail reads an endpoint document into the models that linked to it.
func (b *builder) applyDetail(doc catalog.Document) error {
	var d detail
	if err := json.Unmarshal(doc.Body, &d); err != nil {
		return fmt.Errorf("decode %s: %w", doc.URL, err)
	}
	for _, id := range b.details[doc.URL] {
		m, ok := b.models[id]
		if !ok {
			continue
		}
		b.applyEndpoints(m, d.Data.Endpoints)
		m.AddSource(doc.URL)
	}
	return nil
}

// applyEndpoints records the sellers of a model: who they are, what precision
// they serve it at, what they charge, how much they will read and write, and
// which parameters they take.
//
// The listing describes a model through one seller, the one OpenRouter fronts
// for it, and that seller's rate stays the model's unqualified rate because it
// is what a caller who names nothing pays. Every other seller's rates are
// recorded beside it, qualified by the seller's name, so the fan the
// marketplace really offers is in the catalog without any of it being read as
// the model's own price.
//
// The ceilings are treated differently from the rates. A ceiling is a fact
// about what the model can produce or accept, so the largest any seller states
// is recorded, since each seller states what it will itself return rather than
// what the model is able to. The parameters and the capabilities they imply
// are unioned for the same reason: a caller may route to any seller, so a
// capability one seller offers is a capability the model has here.
func (b *builder) applyEndpoints(m *catalog.Model, endpoints []endpoint) {
	var ceiling, prompt, context int64
	offers := offerNumbers(endpoints)
	for at, e := range endpoints {
		ceiling = max(ceiling, servedCeiling(e))
		prompt = max(prompt, e.MaxPromptTokens)
		context = max(context, e.ContextLength)
		m.AddList(ListEndpoints, e.ProviderName)
		m.AddList(ListEndpointTags, e.Tag)
		if e.Quantization != quantizationUnknown {
			m.AddList(ListQuantizations, e.Quantization)
		}
		m.AddList(ListParameters, e.SupportedParameters...)
		if generative(m) {
			for _, parameter := range e.SupportedParameters {
				m.AddList(ListFeatures, parameterFeatures[parameter]...)
			}
		}
		if e.SupportsImplicitCaching {
			m.SetAttr(AttrImplicitCaching, "true")
			m.AddList(ListFeatures, FeaturePromptCaching)
		}
		if e.SupportsVoiceCloning {
			m.AddList(ListFeatures, FeatureVoiceCloning)
		}
		m.AddList(ListFeatures, pricedFeatures(e.Pricing)...)
		applyPricing(
			m,
			e.Pricing,
			endpointDims(e).With(DimEndpointOffer, offers[at]),
		)
		b.applySeller(m, e.ProviderName)
	}
	m.SetLimit(LimitContextWindow, context)
	m.SetLimit(LimitMaxOutputTokens, withinWindow(m, ceiling))
	m.SetLimit(LimitMaxInputTokens, prompt)
}

// servedCeiling is the largest completion one seller will actually return,
// which is its stated ceiling or its window, whichever is smaller.
//
// Sellers state the two independently and some state a ceiling their own
// window cannot hold: DeepInfra offers l3-lunaris-8b with an 8,192 token
// window and a 16,384 token ceiling, and Azure offers gpt-3.5-turbo-0613 with
// 4,095 and 4,096. A request asking for the ceiling fails in both cases, so
// what the seller serves is the window.
func servedCeiling(e endpoint) int64 {
	if e.ContextLength > 0 && e.MaxCompletionTokens > e.ContextLength {
		return e.ContextLength
	}
	return e.MaxCompletionTokens
}

// withinWindow bounds a ceiling by the window the model is recorded with.
//
// The sellers' largest ceiling can exceed it, because a model's window is
// stated by the listing and its ceiling by whichever seller offers the most.
// The two describe different deployments: minimax-m3 is listed with a 524,288
// token window and served by one seller with 1,048,576, and a batch variant
// takes its window from its own listing entry and its sellers from the model
// behind it. What a consumer can ask this model for is bounded by the window
// this model is published with.
func withinWindow(m *catalog.Model, ceiling int64) int64 {
	window := m.Limits[LimitContextWindow]
	if window > 0 && ceiling > window {
		return window
	}
	return ceiling
}

// offerNumbers numbers the offers whose dimensions are identical and whose
// rates are not, and returns the number of each endpoint by its position, or
// an empty string where the endpoint needs none.
//
// A seller publishing one model twice under one name, one tag and one
// precision is stating two prices for what the catalog can only describe as
// one rate. Which of the two a caller reaches is OpenRouter's business and it
// publishes nothing that says, so both are kept and told apart by a number.
//
// The number comes from the rates rather than from the order the endpoints
// arrive in, which is not stable between runs: the cheaper offer is the first,
// and a run that reads the same rates writes the same numbers.
func offerNumbers(endpoints []endpoint) map[int]string {
	groups := map[string][]int{}
	for at, e := range endpoints {
		key := endpointDims(e).Key()
		groups[key] = append(groups[key], at)
	}
	out := map[int]string{}
	for _, group := range groups {
		if len(group) < 2 || !ratesDiffer(endpoints, group) {
			continue
		}
		slices.SortStableFunc(group, func(a, b int) int {
			return strings.Compare(
				rateKey(endpoints[a]),
				rateKey(endpoints[b]),
			)
		})
		for n, at := range group {
			out[at] = strconv.Itoa(n + 1)
		}
	}
	return out
}

// ratesDiffer reports whether the endpoints at these positions quote different
// rates. Where they quote the same ones there is nothing to tell apart, and
// the identical rates collapse into one entry as any repeated rate does.
func ratesDiffer(endpoints []endpoint, group []int) bool {
	first := rateKey(endpoints[group[0]])
	for _, at := range group[1:] {
		if rateKey(endpoints[at]) != first {
			return true
		}
	}
	return false
}

// rateKey encodes a seller's rates in a form that orders them: the keys in
// order, each with the amount it is quoted at, padded so that the comparison
// is by value rather than by the length of the decimal.
func rateKey(e endpoint) string {
	keys := slices.Sorted(maps.Keys(e.Pricing))
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		var rate string
		if err := json.Unmarshal(e.Pricing[key], &rate); err != nil {
			rate = string(e.Pricing[key])
		}
		amount, ok := scaleRate(rate, 1)
		if !ok {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%020.10f", key, amount))
	}
	return strings.Join(parts, ";")
}

// endpointDims name the seller a rate belongs to.
//
// The name alone does not identify it. One seller may offer the same model
// three ways at three prices, and what tells them apart is the tag: openai,
// openai/flex and openai/priority are one seller's three service levels, and
// azure/eu and azure/us are one seller's two regions. Without the tag the
// three rates would collide into a contradiction the source does not have.
//
// The quantization is part of the name rather than a note on it, because a
// seller offering one model at two precisions publishes two endpoints and two
// prices, and without the precision the two rates would look like a
// contradiction. The discount rides along where the seller states one:
// OpenRouter documents neither what it is reduced from nor whether the
// published rate already has it applied, so it qualifies the rate and is not
// arithmetic on it.
func endpointDims(e endpoint) catalog.Dims {
	dims := catalog.Dims{DimEndpointProvider: e.ProviderName}
	if e.Tag != "" {
		dims[DimEndpointTag] = e.Tag
	}
	if e.Quantization != "" && e.Quantization != quantizationUnknown {
		dims[DimQuantization] = e.Quantization
	}
	if discount, ok := discountOf(e.Pricing); ok {
		dims[DimDiscount] = discount
	}
	return dims
}

// discountOf returns the reduction stated beside a seller's rates, if any. It
// is published as a bare number rather than as the decimal string the rates
// use, and a zero is no discount.
func discountOf(pricing map[string]json.RawMessage) (string, bool) {
	raw, ok := pricing["discount"]
	if !ok {
		return "", false
	}
	var value json.Number
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	text := strings.TrimSpace(value.String())
	if text == "" || isZeroRate(text) {
		return "", false
	}
	return text, true
}

// pricedFeatures reports the capabilities a pricing object implies, which is
// the only way OpenRouter states caching and search.
func pricedFeatures(pricing map[string]json.RawMessage) []string {
	var features []string
	for key, raw := range pricing {
		var rate string
		if err := json.Unmarshal(raw, &rate); err != nil {
			continue
		}
		if isZeroRate(rate) {
			continue
		}
		features = append(features, priceFeatures[key]...)
	}
	return features
}
