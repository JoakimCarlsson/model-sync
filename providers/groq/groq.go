package groq

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// currency is the only currency Groq quotes.
const currency = "USD"

// providerID and providerName identify this source in the catalog.
const (
	providerID   = "groq"
	providerName = "Groq"
)

const baseURL = "https://console.groq.com"

// ModelsURL is the page listing every model Groq serves.
const ModelsURL = baseURL + "/docs/models.md"

// Provider reads Groq's models page. The zero value is not usable; call New.
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

// Fetch retrieves the models page, then the page of each model it links to.
//
// The table states what a model costs and holds, and nothing about what it
// takes or can do. That is on the model's own page, which states no rate.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	models, err := p.get(ctx, ModelsURL)
	if err != nil {
		return nil, err
	}
	docs := []catalog.Document{models}
	var failures []error
	for _, url := range modelPageURLs(models) {
		page, err := p.get(ctx, url)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		docs = append(docs, page)
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

// Parse reads the models page first, because it is the only document naming
// every model, then each model page onto what it established.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	for _, doc := range docs {
		if doc.URL == ModelsURL {
			b.applyModels(doc)
		}
	}
	for _, doc := range docs {
		if doc.URL != ModelsURL {
			b.applyModelPage(doc)
		}
	}
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
func (b *builder) model(id string, kind catalog.Kind) *catalog.Model {
	m, ok := b.models[id]
	if !ok {
		m = &catalog.Model{ID: id, Provider: providerID, Kind: kind}
		b.models[id] = m
		b.order = append(b.order, id)
		return m
	}
	if m.Kind == "" {
		m.Kind = kind
	}
	return m
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
