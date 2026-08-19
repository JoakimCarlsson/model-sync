package cerebras

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// PublicModelsURL answers, without a key, with the models Cerebras serves on
// its public endpoints. It is the one place the two bounds the documentation
// rounds are stated exactly, and the only one stating the repository the
// weights come from, the rates to the token and the standing of the model.
const PublicModelsURL = "https://api.cerebras.ai/public/v1/models"

// Scalar keys the public model list populates.
const (
	AttrSummary       = "summary"
	AttrAuthor        = "author"
	AttrQuantization  = "quantization"
	AttrHuggingFaceID = "hugging_face_id"
	AttrOpenWeights   = "open_weights"
	AttrTokenizer     = "tokenizer"
	AttrInstructType  = "instruct_type"
	AttrReleaseDate   = "release_date"
)

// Enumeration keys the public model list populates.
const (
	ListParameters  = catalog.ListParameters
	ListDatacenters = "datacenter_locations"
)

// publicListing is the shape of the public model list.
type publicListing struct {
	Data []publicModel `json:"data"`
}

// publicModel is one model as the endpoint states it.
type publicModel struct {
	ID            string          `json:"id"`
	Created       int64           `json:"created"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	OwnedBy       string          `json:"owned_by"`
	HuggingFaceID string          `json:"hugging_face_id"`
	Quantization  string          `json:"quantization"`
	Deprecated    bool            `json:"deprecated"`
	Preview       bool            `json:"preview"`
	Pricing       publicPricing   `json:"pricing"`
	Capabilities  map[string]bool `json:"capabilities"`
	Parameters    map[string]bool `json:"supported_parameters"`
	Architecture  publicArch      `json:"architecture"`
	Limits        publicLimits    `json:"limits"`
	Datacenters   []string        `json:"datacenter_locations"`
}

// publicPricing is the per-token rate, quoted against a single token rather
// than against the million tokens every document quotes.
type publicPricing struct {
	Prompt     string `json:"prompt"`
	Completion string `json:"completion"`
}

// publicArch is what the model is, as opposed to what it does.
type publicArch struct {
	Modality     string `json:"modality"`
	Tokenizer    string `json:"tokenizer"`
	InstructType string `json:"instruct_type"`
}

// publicLimits are the ceilings and the rates a caller is held to. The two
// rate fields are stated as null for every model today, and are read so that a
// model Cerebras does state them for carries them.
type publicLimits struct {
	MaxContextLength    int64  `json:"max_context_length"`
	MaxCompletionTokens int64  `json:"max_completion_tokens"`
	RequestsPerMinute   *int64 `json:"requests_per_minute"`
	TokensPerMinute     *int64 `json:"tokens_per_minute"`
}

// publicCapabilities maps a capability flag of the public list onto the
// catalog's vocabulary, in a slice rather than a map so that the order a model
// records them in does not depend on map iteration.
//
// Two flags are dropped rather than translated. "tools" repeats
// "function_calling" and "response_format" repeats "structured_outputs", and
// recording either would put a second word for one capability in the list.
var publicCapabilities = []struct {
	Flag    string
	Feature string
}{
	{"streaming", FeatureStreaming},
	{"function_calling", catalog.CapabilityFunctionCalling},
	{"parallel_tool_calls", FeatureParallelToolCalls},
	{"tool_choice", FeatureToolChoice},
	{"structured_outputs", catalog.CapabilityStructuredOutputs},
	{"json_mode", catalog.CapabilityJSONMode},
	{"reasoning", catalog.CapabilityReasoning},
}

// Capabilities of the public list that name a modality rather than a feature.
const capabilityVision = "vision"

// modalityWords maps a word of the architecture's modality onto the modality
// it names.
var modalityWords = map[string]string{
	"text":   "text",
	"vision": "image",
	"image":  "image",
	"audio":  "audio",
}

// applyPublicModels reads the public model list.
//
// It is read before every document, because it states the two ceilings to the
// token where the catalog and the model pages round them to "131k" and "40k",
// because it names the repository the weights are published in, and because it
// is Cerebras answering for itself which models it currently sells and under
// which standing rather than a page describing them.
func (b *builder) applyPublicModels(doc catalog.Document) error {
	var list publicListing
	if err := json.Unmarshal(doc.Body, &list); err != nil {
		return fmt.Errorf("decode %s: %w", doc.URL, err)
	}
	for _, e := range list.Data {
		if e.ID == "" {
			continue
		}
		m := b.model(e.ID, KindChat)
		m.AddSource(doc.URL)
		if m.Name == "" {
			m.Name = e.Name
		}
		applyPublicScalars(m, e)
		applyPublicPricing(m, e.Pricing)
		applyPublicCapabilities(m, e)
		m.SetLimit(LimitContextWindow, e.Limits.MaxContextLength)
		m.SetLimit(LimitMaxOutputTokens, e.Limits.MaxCompletionTokens)
		if e.Limits.RequestsPerMinute != nil {
			m.SetLimit(LimitRequestsPerMinute, *e.Limits.RequestsPerMinute)
		}
		if e.Limits.TokensPerMinute != nil {
			m.SetLimit(LimitTokensPerMinute, *e.Limits.TokensPerMinute)
		}
		m.AddList(ListDatacenters, e.Datacenters...)
	}
	return nil
}

// applyPublicScalars records what the list states about the model itself.
func applyPublicScalars(m *catalog.Model, e publicModel) {
	m.SetAttr(AttrSummary, e.Description)
	m.SetAttr(AttrAuthor, e.OwnedBy)
	m.SetAttr(AttrQuantization, e.Quantization)
	m.SetAttr(AttrState, publicState(e))
	m.SetAttr(AttrHuggingFaceID, e.HuggingFaceID)
	m.SetAttr(AttrTokenizer, e.Architecture.Tokenizer)
	m.SetAttr(AttrInstructType, e.Architecture.InstructType)
	m.SetAttr(AttrReleaseDate, unixDate(e.Created))
	if e.HuggingFaceID != "" {
		m.SetAttr(AttrOpenWeights, "true")
	}
}

// publicState reads the standing of a model off the two flags the list states
// it with.
func publicState(e publicModel) string {
	switch {
	case e.Deprecated:
		return StateDeprecated
	case e.Preview:
		return StatePreview
	}
	return StateActive
}

// applyPublicPricing records the rates the list quotes against a single token.
//
// Every Cerebras document quotes a rate against the million tokens, so the
// amount is converted and rounded back to the precision a document writes, or
// a rate the documents write as "$0.99" would be recorded twice, once as
// itself and once as the float the multiplication leaves behind.
func applyPublicPricing(m *catalog.Model, p publicPricing) {
	for _, rate := range []struct {
		Metric catalog.Metric
		Value  string
	}{
		{MetricInputTokens, p.Prompt},
		{MetricOutputTokens, p.Completion},
	} {
		amount, err := strconv.ParseFloat(strings.TrimSpace(rate.Value), 64)
		if err != nil || amount == 0 {
			continue
		}
		m.AddPrice(catalog.Price{
			Metric:   rate.Metric,
			Unit:     UnitPer1MTokens,
			Amount:   math.Round(amount*1e6*1e6) / 1e6,
			Currency: currency,
		})
	}
}

// applyPublicCapabilities records what the list says the model can do and what
// it takes.
func applyPublicCapabilities(m *catalog.Model, e publicModel) {
	for _, c := range publicCapabilities {
		if e.Capabilities[c.Flag] {
			m.AddList(ListFeatures, c.Feature)
		}
	}
	if e.Capabilities[capabilityVision] {
		m.AddList(ListInputModalities, "image")
	}
	for _, word := range strings.Split(e.Architecture.Modality, "+") {
		if modality, ok := modalityWords[strings.TrimSpace(word)]; ok {
			m.AddList(ListInputModalities, modality)
		}
	}
	for _, name := range sortedTrue(e.Parameters) {
		m.AddList(ListParameters, name)
	}
}

// sortedTrue returns the keys a flag map sets, in order, so that nothing in
// the output depends on map iteration.
func sortedTrue(flags map[string]bool) []string {
	out := make([]string, 0, len(flags))
	for name, ok := range flags {
		if ok {
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out
}

// unixDate renders a second count as a date, and nothing at all for the zero
// the list writes when it has no date to state.
func unixDate(seconds int64) string {
	if seconds <= 0 {
		return ""
	}
	return time.Unix(seconds, 0).UTC().Format("2006-01-02")
}
