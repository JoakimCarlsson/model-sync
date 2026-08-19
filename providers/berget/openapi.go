package berget

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// ListParameters is the enumeration the specification's request fields land
// in. It is kept apart from the capability list because a parameter names what
// the API accepts and not what the model can do.
const ListParameters = catalog.ListParameters

// Enumerations the specification populates beyond the request fields.
const (
	// ListEndpoints is the path a model of a given kind is called on.
	ListEndpoints = "endpoints"
	// ListResponseFormats is the set of shapes a response may be asked for,
	// read from the enumeration the response_format field is bounded by.
	ListResponseFormats = "response_formats"
	// ListAudioFormats is the set of container formats the transcription
	// endpoint states it accepts.
	ListAudioFormats = "audio_formats"
)

// LimitMaxBatchSize is the number of inputs one embedding request may carry,
// read from the bound the specification puts on the input array. It is a bound
// on the request and not on the model, which is why it is not a token count.
const LimitMaxBatchSize = "max_batch_size"

// kindSchemas name the request body each kind of model is called with. The
// specification states a parameter once per endpoint rather than once per
// model, and a model of a given kind is served by exactly one endpoint, so the
// kind is what carries a parameter to the models it applies to.
var kindSchemas = map[catalog.Kind]string{
	KindChat:          "ChatCompletionRequest",
	KindEmbedding:     "EmbeddingRequest",
	KindRerank:        "CohereRerankRequest",
	KindTranscription: "CreateTranscriptionRequest",
}

// kindPaths name the endpoint each kind of model is served on, as the
// specification's paths object spells it.
var kindPaths = map[catalog.Kind]string{
	KindChat:          "/v1/chat/completions",
	KindEmbedding:     "/v1/embeddings/",
	KindRerank:        "/v1/rerank/",
	KindTranscription: "/v1/audio/transcriptions",
}

// transcriptionFeatures map a parameter of the transcription endpoint onto the
// capability that accepting it demonstrates. They are the only three
// parameters read as capabilities: each names something the model does with
// the audio rather than a knob on the request, and each is what another
// provider states as a capability outright, so a consumer asking which models
// diarize would otherwise miss Berget's three.
var transcriptionFeatures = map[string]string{
	"align":    catalog.CapabilityWordTimestamps,
	"diarize":  catalog.CapabilityDiarization,
	"hotwords": catalog.CapabilityKeyterms,
}

// specReasoning names the models the specification says reason, and whether it
// says the reasoning can be turned off. Neither statement is a field: the
// reasoning_effort parameter is described as vendor neutral except that
// thinking cannot be disabled on models that always think, naming Kimi K3, and
// the thinking parameter is described as a Moonshot and Kimi K2 specific
// parameter controlling reasoning that Kimi K3 does not support. Berget serves
// exactly one Kimi K2 and one Kimi K3, so both sentences name a model on the
// listing. No sentence in any document names any other model as reasoning.
var specReasoning = map[string]string{
	"moonshotai/Kimi-K3":   "true",
	"moonshotai/Kimi-K2.6": "false",
}

// audioFormatsRe reads the container formats out of the transcription
// endpoint's prose, which states them in one sentence and nowhere else.
var audioFormatsRe = regexp.MustCompile(
	`(?i)Supported formats:\s*([a-z0-9, ]+)\.`,
)

// spec is the part of the specification this reads: the request body of each
// endpoint, which is where a parameter is named, and the prose of the
// transcription endpoint, which is where the audio formats are.
type spec struct {
	Components struct {
		Schemas map[string]schema `json:"schemas"`
	} `json:"components"`
	Paths map[string]map[string]operation `json:"paths"`
}

// operation is one method on one path, read for its prose alone.
type operation struct {
	Description string `json:"description"`
}

// schema is one request body, read for the names of its fields, the shapes its
// response_format field is bounded by and the size its input array may reach.
type schema struct {
	Properties map[string]property `json:"properties"`
}

// property is one request field. Only the few bounds Berget states a number or
// an enumeration in are decoded; the rest of a field is its own schema and
// says nothing this catalog has a slot for.
type property struct {
	Enum     []string            `json:"enum"`
	AnyOf    []property          `json:"anyOf"`
	Items    *property           `json:"items"`
	Type     string              `json:"type"`
	MaxItems *int64              `json:"maxItems"`
	Props    map[string]property `json:"properties"`
}

// applySpec records what each endpoint accepts onto the models it serves.
func (b *builder) applySpec(doc catalog.Document) error {
	var s spec
	if err := json.Unmarshal(doc.Body, &s); err != nil {
		return fmt.Errorf("decode %s: %w", doc.URL, err)
	}
	byKind := map[catalog.Kind][]string{}
	for kind, name := range kindSchemas {
		if params := s.Components.Schemas[name].names(); len(params) > 0 {
			byKind[kind] = params
		}
	}
	formats := s.responseFormats()
	audio := audioFormats(s.Paths[kindPaths[KindTranscription]]["post"])
	batch := s.Components.Schemas[kindSchemas[KindEmbedding]].batchSize()
	for _, m := range b.models {
		params, ok := byKind[m.Kind]
		if !ok {
			continue
		}
		m.AddSource(doc.URL)
		m.AddList(ListParameters, params...)
		m.AddList(ListEndpoints, kindPaths[m.Kind])
		m.AddList(ListResponseFormats, formats[m.Kind]...)
		if mandatory, ok := specReasoning[m.ID]; ok {
			m.AddList(ListFeatures, catalog.CapabilityReasoning)
			m.SetAttr(AttrReasoningMandatory, mandatory)
		}
		if m.Kind == KindEmbedding && batch > 0 {
			m.SetLimit(LimitMaxBatchSize, batch)
		}
		if m.Kind != KindTranscription {
			continue
		}
		m.AddList(ListAudioFormats, audio...)
		for _, param := range params {
			m.AddList(ListFeatures, transcriptionFeatures[param])
		}
	}
	return nil
}

// responseFormats returns, per kind, the shapes that kind's endpoint states a
// response may be asked for. Chat spells them as the type field of each
// alternative its response_format accepts and transcription as a flat
// enumeration, so both are read through the same walk.
func (s spec) responseFormats() map[catalog.Kind][]string {
	out := map[catalog.Kind][]string{}
	for kind, name := range kindSchemas {
		field, ok := s.Components.Schemas[name].Properties["response_format"]
		if !ok {
			continue
		}
		if values := field.formats(); len(values) > 0 {
			out[kind] = values
		}
	}
	return out
}

// formats returns the values a response_format field is bounded by.
func (p property) formats() []string {
	values := slices.Clone(p.Enum)
	for _, alt := range p.AnyOf {
		values = append(values, alt.Props["type"].Enum...)
	}
	slices.Sort(values)
	return slices.Compact(values)
}

// batchSize returns the number of inputs one request to this body may carry,
// which the specification states as the largest maxItems any array of strings
// the input field accepts allows.
func (s schema) batchSize() int64 {
	var largest int64
	for _, alt := range s.Properties["input"].AnyOf {
		if alt.Items == nil || alt.Items.Type != "string" {
			continue
		}
		if alt.MaxItems != nil {
			largest = max(largest, *alt.MaxItems)
		}
	}
	return largest
}

// audioFormats reads the container formats out of an endpoint's prose.
func audioFormats(op operation) []string {
	fields := audioFormatsRe.FindStringSubmatch(op.Description)
	if fields == nil {
		return nil
	}
	var out []string
	for _, name := range strings.Split(fields[1], ",") {
		if name = strings.TrimSpace(name); name != "" {
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out
}

// names returns the fields of a request body in a settled order, since map
// iteration would otherwise rewrite every file on each sync.
func (s schema) names() []string {
	params := make([]string, 0, len(s.Properties))
	for param := range s.Properties {
		params = append(params, param)
	}
	slices.Sort(params)
	return params
}
