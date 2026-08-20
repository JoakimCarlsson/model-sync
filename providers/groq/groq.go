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

// guides are the documents that state what the models page and the model pages
// do not: what the speech models take, what each tier limits, when a model
// shipped, when it stops answering, and which models the batch API accepts.
//
// They are listed here rather than discovered, because none of them is linked
// from the models page as a model page is.
var guides = []string{
	SpeechToTextURL,
	OrpheusURL,
	RateLimitsURL,
	ChangelogURL,
	DeprecationsURL,
	ServiceTiersURL,
	FlexURL,
	BatchURL,
}

// Fetch retrieves the models page, then the page of each model it links to,
// then the guides.
//
// The table states what a model costs and holds, and nothing about what it
// takes or can do. That is on the model's own page, which states no rate.
// Neither names a model Groq has withdrawn, which is why the deprecation page
// is read too: it is the only document naming one, and a model that stopped
// answering last week is a model a consumer still has code pointing at.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	models, err := p.get(ctx, ModelsURL)
	if err != nil {
		return nil, err
	}
	docs := []catalog.Document{models}
	var failures []error
	for _, url := range append(modelPageURLs(models), guides...) {
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
// every model Groq serves, then each model page onto what it established, then
// the guides.
//
// The order within the guides is the order the documents depend on each other.
// The deprecation page is read after the models page and not before, so that a
// model still listed keeps the standing its table gave it and only a model no
// table names is created from the announcement withdrawing it. The batch page
// is read last of all, because it states no rate of its own: it halves the ones
// every other document has already stated.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	for _, doc := range docs {
		if doc.URL == ModelsURL {
			b.applyModels(doc)
		}
	}
	for _, doc := range docs {
		if isModelPage(doc.URL) {
			b.applyModelPage(doc)
		}
	}
	for _, doc := range docs {
		b.applyGuide(doc)
	}
	b.applyAudioCeiling()
	return b.result(), nil
}

// applyAudioCeiling moves the completion ceiling of a model that returns only
// audio onto the key naming what it counts.
//
// It runs at the end because the table states the ceiling and the model's own
// page states the modality, and the modality is what makes the move safe: a
// model whose only output is sound has no text length for a completion ceiling
// to be about.
func (b *builder) applyAudioCeiling() {
	for _, m := range b.models {
		ceiling := m.Limits[LimitMaxOutputTokens]
		if ceiling == 0 || !audioOnly(m) {
			continue
		}
		delete(m.Limits, LimitMaxOutputTokens)
		m.Limits[LimitMaxAudioOutputTokens] = ceiling
	}
}

// audioOnly reports whether audio is the only thing a model returns.
func audioOnly(m *catalog.Model) bool {
	out := m.Lists[ListOutputModalities]
	return len(out) == 1 && out[0] == modalityAudio
}

// applyGuide reads one guide, or nothing where the document is not one.
func (b *builder) applyGuide(doc catalog.Document) {
	switch doc.URL {
	case SpeechToTextURL:
		b.applySpeechToText(doc)
	case OrpheusURL:
		b.applyOrpheus(doc)
	case RateLimitsURL:
		b.applyRateLimits(doc)
	case ChangelogURL:
		b.applyChangelog(doc)
	case DeprecationsURL:
		b.applyDeprecations(doc)
	case ServiceTiersURL, FlexURL:
		b.applyServiceTiers(doc)
	case BatchURL:
		b.applyBatch(doc)
	}
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
