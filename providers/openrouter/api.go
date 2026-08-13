package openrouter

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// listing is the shape of the models endpoint.
type listing struct {
	Data []entry `json:"data"`
}

// entry is one model as OpenRouter publishes it.
type entry struct {
	ID            string       `json:"id"`
	CanonicalSlug string       `json:"canonical_slug"`
	HuggingFaceID string       `json:"hugging_face_id"`
	Name          string       `json:"name"`
	Created       int64        `json:"created"`
	Description   string       `json:"description"`
	ContextLength int64        `json:"context_length"`
	Architecture  architecture `json:"architecture"`
	TopProvider   topProvider  `json:"top_provider"`
	// Pricing is left undecoded because its members are not one type: most
	// are decimal strings, one is a list of conditional rates, and a key
	// added later must be noticed rather than dropped.
	Pricing             map[string]json.RawMessage `json:"pricing"`
	PerRequestLimits    map[string]json.RawMessage `json:"per_request_limits"`
	SupportedParameters []string                   `json:"supported_parameters"`
	SupportedVoices     []string                   `json:"supported_voices"`
	KnowledgeCutoff     string                     `json:"knowledge_cutoff"`
	ExpirationDate      string                     `json:"expiration_date"`
	Reasoning           *reasoning                 `json:"reasoning"`
}

type architecture struct {
	InputModalities  []string `json:"input_modalities"`
	OutputModalities []string `json:"output_modalities"`
	Tokenizer        string   `json:"tokenizer"`
	InstructType     string   `json:"instruct_type"`
}

type topProvider struct {
	ContextLength       int64 `json:"context_length"`
	MaxCompletionTokens int64 `json:"max_completion_tokens"`
	IsModerated         bool  `json:"is_moderated"`
}

type reasoning struct {
	Mandatory bool `json:"mandatory"`
}

// override is a rate that replaces the standard one once a request is large
// enough, which is how OpenRouter expresses long-context pricing.
type override struct {
	MinPromptTokens int64             `json:"min_prompt_tokens"`
	Rates           map[string]string `json:"-"`
}

// UnmarshalJSON reads an override, whose rate keys sit beside its threshold
// rather than under a member of their own.
func (o *override) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	o.Rates = map[string]string{}
	for key, value := range raw {
		if key == "min_prompt_tokens" {
			if err := json.Unmarshal(value, &o.MinPromptTokens); err != nil {
				return fmt.Errorf("min_prompt_tokens: %w", err)
			}
			continue
		}
		var rate string
		if err := json.Unmarshal(value, &rate); err == nil {
			o.Rates[key] = rate
		}
	}
	return nil
}

// applyListing reads the endpoint's response.
func (b *builder) applyListing(doc catalog.Document) error {
	var list listing
	if err := json.Unmarshal(doc.Body, &list); err != nil {
		return fmt.Errorf("decode %s: %w", doc.URL, err)
	}
	for _, e := range list.Data {
		b.applyEntry(e, doc.URL)
	}
	return nil
}

// applyEntry records one model.
func (b *builder) applyEntry(e entry, source string) {
	if e.ID == "" {
		return
	}
	m := b.model(e.ID, kindFor(e.Architecture.OutputModalities))
	m.Name = e.Name
	m.AddSource(source)

	m.SetAttr(AttrSummary, summaryOf(e.Description))
	m.SetAttr(AttrAuthor, authorOf(e.ID))
	m.SetAttr(AttrCanonicalSlug, e.CanonicalSlug)
	m.SetAttr(AttrHuggingFaceID, e.HuggingFaceID)
	m.SetAttr(AttrTokenizer, e.Architecture.Tokenizer)
	m.SetAttr(AttrInstructType, e.Architecture.InstructType)
	m.SetAttr(AttrKnowledgeCutoff, isoDate(e.KnowledgeCutoff))
	m.SetAttr(AttrExpirationDate, isoDate(e.ExpirationDate))
	m.SetAttr(AttrReleased, isoFromUnix(e.Created))
	if e.TopProvider.IsModerated {
		m.SetAttr(AttrModerated, "true")
	}
	if e.Reasoning != nil && e.Reasoning.Mandatory {
		m.SetAttr(AttrReasoningMandatory, "true")
	}

	m.SetLimit(LimitContextWindow, e.ContextLength)
	m.SetLimit(LimitProviderContext, e.TopProvider.ContextLength)
	m.SetLimit(LimitMaxOutputTokens, e.TopProvider.MaxCompletionTokens)
	applyRequestLimits(m, e.PerRequestLimits)

	m.AddList(ListInputModalities, e.Architecture.InputModalities...)
	m.AddList(ListOutputModalities, e.Architecture.OutputModalities...)
	m.AddList(ListParameters, e.SupportedParameters...)
	for _, parameter := range e.SupportedParameters {
		m.AddList(ListFeatures, parameterFeatures[parameter]...)
	}
	m.AddList(ListVoices, e.SupportedVoices...)

	b.applyPricing(m, e.Pricing)
}

// applyRequestLimits records any per-request ceiling, whose keys OpenRouter
// does not fix in advance.
func applyRequestLimits(m *catalog.Model, limits map[string]json.RawMessage) {
	for key, raw := range limits {
		var value int64
		if err := json.Unmarshal(raw, &value); err != nil {
			var text string
			if json.Unmarshal(raw, &text) != nil {
				continue
			}
			parsed, err := strconv.ParseInt(text, 10, 64)
			if err != nil {
				continue
			}
			value = parsed
		}
		m.SetLimit(strings.ToLower(key), value)
	}
}

// applyPricing records every rate a model carries, including the conditional
// ones that replace the standard rate for a large enough request.
func (b *builder) applyPricing(
	m *catalog.Model,
	pricing map[string]json.RawMessage,
) {
	free := isFree(pricing)
	for key, raw := range pricing {
		if key == "overrides" {
			applyOverrides(m, raw)
			continue
		}
		var rate string
		if err := json.Unmarshal(raw, &rate); err != nil {
			continue
		}
		if free && billedAlways[key] {
			addZeroRate(m, key)
			continue
		}
		addRate(m, key, rate, nil)
	}
	if free && len(pricing) > 0 {
		m.SetAttr(AttrFree, "true")
	}
}

// billedAlways are the keys every model is charged on, which is what makes a
// zero in them a rate of nothing rather than a charge that does not apply.
var billedAlways = map[string]bool{"prompt": true, "completion": true}

// isFree reports whether a model is charged nothing for what every model is
// charged for.
func isFree(pricing map[string]json.RawMessage) bool {
	for key, raw := range pricing {
		if !billedAlways[key] {
			continue
		}
		var rate string
		if err := json.Unmarshal(raw, &rate); err != nil {
			continue
		}
		if !isZeroRate(rate) {
			return false
		}
	}
	return true
}

// addZeroRate records that a free model is charged nothing.
//
// A zero rate is otherwise dropped, because OpenRouter writes zero both for a
// model that is free and for a charge that does not apply to it. On the two
// keys every model is billed on, the ambiguity is gone: a model charged nothing
// for its prompt and nothing for its completion is free, and saying so as a
// rate of zero is what tells it apart from a model whose rate is unknown.
func addZeroRate(m *catalog.Model, key string) {
	scaling, known := priceKeys[key]
	if !known {
		return
	}
	m.AddPrice(catalog.Price{
		Metric:   scaling.metric,
		Unit:     scaling.unit,
		Amount:   0,
		Currency: currency,
		Dims:     scaling.dims,
	})
}

// applyOverrides records the conditional rates.
func applyOverrides(m *catalog.Model, raw json.RawMessage) {
	var overrides []override
	if err := json.Unmarshal(raw, &overrides); err != nil {
		return
	}
	for _, o := range overrides {
		if o.MinPromptTokens <= 0 {
			continue
		}
		dims := catalog.Dims{
			DimMinPromptTokens: strconv.FormatInt(o.MinPromptTokens, 10),
		}
		for key, rate := range o.Rates {
			addRate(m, key, rate, dims)
		}
	}
}

// addRate records one rate, scaled to the catalog's denominator.
//
// A zero rate is not recorded. OpenRouter writes zero both for a model that is
// free and for a charge that does not apply to it, and a catalog full of zero
// rates for capabilities a model does not have would say something the source
// does not.
func addRate(m *catalog.Model, key, rate string, dims catalog.Dims) {
	scaling, known := priceKeys[key]
	if !known {
		if !isZeroRate(rate) {
			m.AddNote("unmapped pricing key " + key + ": " + rate)
		}
		return
	}
	amount, ok := scaleRate(rate, scaling.factor)
	if !ok {
		return
	}
	m.AddPrice(catalog.Price{
		Metric:   scaling.metric,
		Unit:     scaling.unit,
		Amount:   amount,
		Currency: currency,
		Dims:     scaling.dims.Merge(dims),
	})
}
