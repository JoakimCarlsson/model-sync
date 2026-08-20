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
	DefaultParameters   map[string]json.Number     `json:"default_parameters"`
	SupportedVoices     []string                   `json:"supported_voices"`
	KnowledgeCutoff     string                     `json:"knowledge_cutoff"`
	ExpirationDate      string                     `json:"expiration_date"`
	Reasoning           *reasoning                 `json:"reasoning"`
	AliasTarget         *aliasTarget               `json:"alias_target"`
	Benchmarks          *benchmarks                `json:"benchmarks"`
	Links               links                      `json:"links"`
}

type architecture struct {
	Modality         string   `json:"modality"`
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

// reasoning is attached to a model that thinks before it answers, and to no
// other, which makes its presence the statement of the capability and its
// members the statement of how far the caller may turn the thinking up.
type reasoning struct {
	Mandatory        bool     `json:"mandatory"`
	DefaultEnabled   *bool    `json:"default_enabled"`
	DefaultEffort    string   `json:"default_effort"`
	SupportedEfforts []string `json:"supported_efforts"`
}

// benchmarks are the third-party scores OpenRouter attaches to a model. Only
// the indices are recorded: the arena rows beside them are one row per arena
// and per category, and they state a standing against other models rather than
// anything about the model itself.
type benchmarks struct {
	ArtificialAnalysis *indices `json:"artificial_analysis"`
}

// indices are the scores Artificial Analysis publishes for a model, as
// OpenRouter restates them.
type indices struct {
	Intelligence *json.Number `json:"intelligence_index"`
	Coding       *json.Number `json:"coding_index"`
	Agentic      *json.Number `json:"agentic_index"`
}

// aliasTarget is what a moving identifier such as x-ai/grok-latest currently
// resolves to.
type aliasTarget struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// links are the sub-resources of a model, of which the endpoint document is
// the only one carrying facts the listing does not.
type links struct {
	Details string `json:"details"`
}

// detailURL is the absolute address of a model's endpoint document.
func detailURL(e entry) string {
	if e.Links.Details == "" {
		return ""
	}
	return baseURL + e.Links.Details
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
		if url := detailURL(e); url != "" {
			b.details[url] = append(b.details[url], e.ID)
		}
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
	m.SetAttr(AttrReleaseDate, isoFromUnix(e.Created))
	m.SetAttr(AttrModality, e.Architecture.Modality)
	if e.TopProvider.IsModerated {
		m.SetAttr(AttrModerated, "true")
	}
	if e.AliasTarget != nil {
		m.SetAttr(AttrAliasTarget, e.AliasTarget.Slug)
	}
	applyReasoning(m, e.Reasoning)
	applyDefaults(m, e.DefaultParameters)
	applyEmbedding(m, e.Description)
	applyBenchmarks(m, e.Benchmarks)

	m.SetLimit(LimitContextWindow, e.ContextLength)
	m.SetLimit(LimitProviderContext, e.TopProvider.ContextLength)
	m.SetLimit(
		LimitMaxOutputTokens,
		withinWindow(m, e.TopProvider.MaxCompletionTokens),
	)
	applyRequestLimits(m, e.PerRequestLimits)

	addModalities(m, ListInputModalities, e.Architecture.InputModalities)
	addModalities(m, ListOutputModalities, e.Architecture.OutputModalities)
	m.AddList(ListParameters, e.SupportedParameters...)
	m.AddList(ListFeatures, featuresOf(e, generative(m))...)
	m.AddList(ListVoices, e.SupportedVoices...)

	applyPricing(m, e.Pricing, nil)
}

// addModalities records the media a model takes or returns, with the medium
// alongside the vendor's word where OpenRouter names the task instead.
func addModalities(m *catalog.Model, key string, modalities []string) {
	for _, modality := range modalities {
		m.AddList(key, modality)
		m.AddList(key, modalityAliases[strings.ToLower(modality)])
	}
}

// applyDefaults records the sampling settings a model is served with when the
// caller asks for none, exactly as published.
func applyDefaults(m *catalog.Model, defaults map[string]json.Number) {
	for name, value := range defaults {
		m.SetAttr(AttrDefaultPrefix+name, value.String())
	}
}

// applyBenchmarks records the indices, leaving the arena standings out.
func applyBenchmarks(m *catalog.Model, b *benchmarks) {
	if b == nil || b.ArtificialAnalysis == nil {
		return
	}
	scores := map[string]*json.Number{
		"intelligence_index": b.ArtificialAnalysis.Intelligence,
		"coding_index":       b.ArtificialAnalysis.Coding,
		"agentic_index":      b.ArtificialAnalysis.Agentic,
	}
	for name, value := range scores {
		if value == nil {
			continue
		}
		m.SetAttr(AttrBenchmarkPrefix+name, value.String())
	}
}

// generative reports whether a model writes an answer, which is the test for
// reading its parameter list as a statement of capability. The three kinds
// that do are the ones that return text, images, or both text and speech.
//
// OpenRouter forwards the same parameter list to models that could not use it:
// an embedding model is published as accepting temperature, top_k and min_p,
// and a transcription model as accepting response_format, which there names
// the shape of the transcript rather than a schema the model is held to.
// Reading those lists the way a chat model's is read would put structured
// output on every reranker in the catalog. The rates and the reasoning object
// are read for every model, because neither is boilerplate: they are stated
// per model and only where they apply.
func generative(m *catalog.Model) bool {
	switch m.Kind {
	case KindChat, KindImage, KindAudio:
		return true
	}
	return false
}

// applyReasoning records how far a model's thinking can be turned up, and that
// it thinks at all.
func applyReasoning(m *catalog.Model, r *reasoning) {
	if r == nil {
		return
	}
	if r.Mandatory {
		m.SetAttr(AttrReasoningMandatory, "true")
	}
	if r.DefaultEnabled != nil {
		m.SetAttr(
			AttrReasoningDefaultEnabled,
			strconv.FormatBool(*r.DefaultEnabled),
		)
	}
	m.SetAttr(AttrReasoningDefaultEffort, r.DefaultEffort)
	m.AddList(ListReasoningEfforts, r.SupportedEfforts...)
}

// featuresOf reports what a listing entry says a model can do.
//
// OpenRouter states it three ways and never as a capability list. A parameter
// its API will forward implies the capability the parameter drives; a charge it
// levies implies the capability being charged for, since nothing is billed for
// reading a cache or running a search unless the model does it; and the
// reasoning object is attached to a model that thinks and to no other.
//
// Only the first of the three is conditional on what the model generates. See
// generative for why.
func featuresOf(e entry, generative bool) []string {
	var features []string
	if generative {
		for _, parameter := range e.SupportedParameters {
			features = append(features, parameterFeatures[parameter]...)
		}
	}
	features = append(features, pricedFeatures(e.Pricing)...)
	if e.Reasoning != nil {
		features = append(features, catalog.CapabilityReasoning)
	}
	return features
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

// applyPricing records every rate a pricing object carries, including the
// conditional ones that replace the standard rate for a large enough request.
//
// The same object is published twice, once on the model for the seller
// OpenRouter fronts and once per seller in the endpoint document. The dims
// name the seller when the rates are a seller's, and are empty when they are
// the model's own, so the rate a caller who names nothing pays stays
// unqualified.
func applyPricing(
	m *catalog.Model,
	pricing map[string]json.RawMessage,
	dims catalog.Dims,
) {
	free := isFree(pricing)
	for key, raw := range pricing {
		if key == "overrides" {
			applyOverrides(m, raw, dims)
			continue
		}
		var rate string
		if err := json.Unmarshal(raw, &rate); err != nil {
			continue
		}
		if free && billedAlways[key] {
			addZeroRate(m, key, dims)
			continue
		}
		addRate(m, key, rate, dims)
	}
	if free && len(pricing) > 0 && len(dims) == 0 {
		m.SetAttr(AttrFree, "true")
	}
}

// billedAlways are the keys every model is charged on, which is what makes a
// zero in them a rate of nothing rather than a charge that does not apply.
var billedAlways = map[string]bool{"prompt": true, "completion": true}

// isFree reports whether a model is charged nothing for anything.
//
// The test is every published rate rather than the two every model carries,
// because a model billed per image is charged zero per token and is not free:
// its zero says tokens are not how it is billed. Only where nothing at all is
// charged does a zero on the prompt and completion keys mean a rate of
// nothing.
func isFree(pricing map[string]json.RawMessage) bool {
	for _, raw := range pricing {
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
func addZeroRate(m *catalog.Model, key string, dims catalog.Dims) {
	scaling, known := priceKeys[key]
	if !known {
		return
	}
	m.AddPrice(catalog.Price{
		Metric:   scaling.metric,
		Unit:     scaling.unit,
		Amount:   0,
		Currency: currency,
		Dims:     scaling.dims.Merge(dims),
	})
}

// applyOverrides records the conditional rates.
func applyOverrides(m *catalog.Model, raw json.RawMessage, dims catalog.Dims) {
	var overrides []override
	if err := json.Unmarshal(raw, &overrides); err != nil {
		return
	}
	for _, o := range overrides {
		if o.MinPromptTokens <= 0 {
			continue
		}
		conditional := dims.With(
			DimMinPromptTokens,
			strconv.FormatInt(o.MinPromptTokens, 10),
		)
		for key, rate := range o.Rates {
			addRate(m, key, rate, conditional)
		}
	}
}

// addRate records one rate, scaled to the catalog's denominator.
//
// A zero rate is not recorded. OpenRouter writes zero both for a model that is
// free and for a charge that does not apply to it, and a catalog full of zero
// rates for capabilities a model does not have would say something the source
// does not.
//
// A negative rate is not a rate. OpenRouter writes "-1" on its routers, whose
// cost is whatever the model the request was routed to charges, and scaling
// that to a denominator yields minus a million dollars per million tokens. It
// is recorded as a variable rate instead: the metric and the unit are known,
// the amount is not, and the catalog says so with a null rather than with a
// sign a consumer would multiply.
func addRate(m *catalog.Model, key, rate string, dims catalog.Dims) {
	scaling, known := priceKeys[key]
	if !known {
		if !isZeroRate(rate) {
			m.AddNote("unmapped pricing key " + key + ": " + rate)
		}
		return
	}
	if isRoutedRate(rate) {
		m.AddPrice(catalog.Price{
			Metric:   scaling.metric,
			Unit:     scaling.unit,
			Variable: true,
			Currency: currency,
			Dims:     scaling.dims.Merge(dims),
			Note:     routedNote,
		})
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

// routedNote says what a variable rate on this provider means, since the model
// carrying it is a router and not a model.
const routedNote = "routed model; billed at the destination model's rate"
