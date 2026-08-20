package googlecloud

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// providerID and providerName identify this source in the catalog.
const (
	providerID   = "googlecloud"
	providerName = "Google Cloud"
)

// currency is the only currency the price list quotes.
const currency = "USD"

// PricingURL states what a minute of audio costs, per category of model.
const PricingURL = "https://cloud.google.com/speech-to-text/pricing"

// ModelsURL lists the models the v2 API accepts, with a sentence about each.
const ModelsURL = "https://cloud.google.com/speech-to-text/v2/docs/" +
	"transcription-model"

// userAgent names this program to cloud.google.com, which answers a request
// carrying Go's default agent with the two bytes "OK" and nothing else. Any
// other agent is served the page, so this says what is asking rather than
// pretending to be a browser.
const userAgent = "model-sync (+https://github.com/joakimcarlsson/model-sync)"

// Provider reads Google Cloud's Speech-to-Text documentation. The zero value
// is not usable; call New.
type Provider struct {
	// Client performs the fetch.
	Client *http.Client
}

// New returns a Provider using the default HTTP client.
func New() *Provider {
	return &Provider{Client: http.DefaultClient}
}

// ID implements catalog.Source.
func (p *Provider) ID() string { return providerID }

// Name implements catalog.Source.
func (p *Provider) Name() string { return providerName }

// Fetch retrieves both documents. Neither links to the other and neither
// states what the other does, so a run that lost one is reported and the other
// is still read.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	var (
		docs     []catalog.Document
		failures []error
	)
	for _, url := range []string{ModelsURL, PricingURL} {
		doc, err := p.get(ctx, url)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		docs = append(docs, doc)
	}
	return docs, errors.Join(failures...)
}

// get retrieves one document.
func (p *Provider) get(
	ctx context.Context,
	url string,
) (catalog.Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return catalog.Document{}, err
	}
	req.Header.Set("User-Agent", userAgent)
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return catalog.Document{}, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return catalog.Document{}, fmt.Errorf("fetch %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return catalog.Document{}, fmt.Errorf("read %s: %w", url, err)
	}
	return catalog.Document{URL: url, Body: body}, nil
}

// Parse reads the model page first, so that a model Google documents is
// established before the price list decides which category it falls in, then
// the price list.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	for _, doc := range docs {
		if doc.URL == ModelsURL {
			b.applyModels(doc)
		}
	}
	for _, doc := range docs {
		if doc.URL == PricingURL {
			b.applyPricing(doc)
		}
	}
	b.applyUnpriced()
	return b.result(), nil
}

// builder accumulates models.
type builder struct {
	models map[string]*catalog.Model
	order  []string
}

func newBuilder() *builder {
	return &builder{models: map[string]*catalog.Model{}}
}

// model returns the entry for id, creating it if absent.
func (b *builder) model(id string) *catalog.Model {
	m, ok := b.models[id]
	if !ok {
		m = &catalog.Model{
			ID:       id,
			Provider: providerID,
			Kind:     KindTranscription,
		}
		b.models[id] = m
		b.order = append(b.order, id)
	}
	return m
}

// applyUnpriced says of a model the price list does not reach that the price
// list does not reach it, rather than leaving it looking free.
func (b *builder) applyUnpriced() {
	for _, id := range b.order {
		m := b.models[id]
		if len(m.Prices) > 0 {
			continue
		}
		m.AddNote(noteUnpriced)
		m.AddSource(PricingURL)
	}
}

// result returns the accumulated models in identifier order.
func (b *builder) result() []catalog.Model {
	ids := slices.Clone(b.order)
	slices.Sort(ids)
	out := make([]catalog.Model, 0, len(ids))
	for _, id := range ids {
		out = append(out, *b.models[id])
	}
	return out
}
