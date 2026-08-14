package berget

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// ListParameters is the enumeration the specification's request fields land
// in. It is kept apart from the capability list because a parameter names what
// the API accepts and not what the model can do.
const ListParameters = catalog.ListParameters

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

// transcriptionFeatures map a parameter of the transcription endpoint onto the
// capability that accepting it demonstrates. They are the only two parameters
// read as capabilities: both name something the model does with the audio
// rather than a knob on the request, and both are what another provider states
// as a capability outright, so a consumer asking which models diarize would
// otherwise miss Berget's three.
var transcriptionFeatures = map[string]string{
	"align":    catalog.CapabilityWordTimestamps,
	"diarize":  catalog.CapabilityDiarization,
	"hotwords": catalog.CapabilityKeyterms,
}

// spec is the part of the specification this reads: the request body of each
// endpoint, which is where a parameter is named.
type spec struct {
	Components struct {
		Schemas map[string]schema `json:"schemas"`
	} `json:"components"`
}

// schema is one request body, read for the names of its fields alone.
type schema struct {
	Properties map[string]json.RawMessage `json:"properties"`
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
	for _, m := range b.models {
		params, ok := byKind[m.Kind]
		if !ok {
			continue
		}
		m.AddSource(doc.URL)
		m.AddList(ListParameters, params...)
		if m.Kind != KindTranscription {
			continue
		}
		for _, param := range params {
			m.AddList(ListFeatures, transcriptionFeatures[param])
		}
	}
	return nil
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
