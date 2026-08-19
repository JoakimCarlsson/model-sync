package openrouter

import (
	"encoding/json"
	"fmt"
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
	for _, e := range endpoints {
		ceiling = max(ceiling, e.MaxCompletionTokens)
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
		applyPricing(m, e.Pricing, endpointDims(e))
		b.applySeller(m, e.ProviderName)
	}
	m.SetLimit(LimitContextWindow, context)
	m.SetLimit(LimitMaxOutputTokens, ceiling)
	m.SetLimit(LimitMaxInputTokens, prompt)
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
