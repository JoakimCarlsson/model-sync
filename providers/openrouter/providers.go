package openrouter

import (
	"encoding/json"
	"fmt"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// ProvidersURL lists the upstreams OpenRouter routes to, with the facts that
// belong to the seller rather than to the model: where it is registered and
// where it runs.
const ProvidersURL = "https://openrouter.ai/api/v1/providers"

// providerList is the shape of the providers endpoint.
type providerList struct {
	Data []providerInfo `json:"data"`
}

// providerInfo is one upstream. The policy and status URLs are read but not
// recorded against a model: they are the seller's documents, and a model
// served by thirty sellers would otherwise carry thirty links that say nothing
// about it.
type providerInfo struct {
	Name         string   `json:"name"`
	Slug         string   `json:"slug"`
	Headquarters string   `json:"headquarters"`
	Datacenters  []string `json:"datacenters"`
}

// applyProviders records the sellers, to be joined onto the models when their
// endpoint documents are read.
func (b *builder) applyProviders(doc catalog.Document) error {
	var list providerList
	if err := json.Unmarshal(doc.Body, &list); err != nil {
		return fmt.Errorf("decode %s: %w", doc.URL, err)
	}
	for _, info := range list.Data {
		b.providers[info.Name] = info
	}
	b.providerSource = doc.URL
	return nil
}

// applySeller records where a model's seller sits and where it serves from.
//
// Both are country codes, and both are recorded as a set across the sellers,
// because the question they answer is which jurisdictions a request for this
// model can be served from rather than which one a given seller is in.
func (b *builder) applySeller(m *catalog.Model, name string) {
	info, ok := b.providers[name]
	if !ok {
		return
	}
	m.AddList(ListHeadquarters, info.Headquarters)
	m.AddList(ListDatacenters, info.Datacenters...)
	m.AddSource(b.providerSource)
}
