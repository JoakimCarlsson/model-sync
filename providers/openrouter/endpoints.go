package openrouter

import (
	"encoding/json"
	"fmt"

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
	ProviderName        string   `json:"provider_name"`
	MaxCompletionTokens int64    `json:"max_completion_tokens"`
	SupportedParameters []string `json:"supported_parameters"`
}

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
		applyEndpoints(m, d.Data.Endpoints)
		m.AddSource(doc.URL)
	}
	return nil
}

// applyEndpoints fills from the upstreams what the listing left unstated, and
// only that: where the listing answered, the upstream OpenRouter fronts is the
// answer, and the others are neither a correction of it nor an addition to it.
//
// The upstreams disagree about the completion ceiling, sometimes by two orders
// of magnitude, because each states what it will itself return rather than what
// the model can produce. The largest is recorded, since it is the longest
// answer the model is published as able to give and any smaller figure is a
// fact about one seller rather than about the model.
func applyEndpoints(m *catalog.Model, endpoints []endpoint) {
	if m.Limits[LimitMaxOutputTokens] <= 0 {
		var ceiling int64
		for _, e := range endpoints {
			ceiling = max(ceiling, e.MaxCompletionTokens)
		}
		m.SetLimit(LimitMaxOutputTokens, ceiling)
	}
	if len(m.Lists[ListFeatures]) > 0 {
		return
	}
	for _, e := range endpoints {
		for _, parameter := range e.SupportedParameters {
			m.AddList(ListFeatures, parameterFeatures[parameter]...)
		}
	}
}
