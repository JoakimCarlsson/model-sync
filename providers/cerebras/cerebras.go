package cerebras

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// providerID and providerName identify this source in the catalog.
const (
	providerID   = "cerebras"
	providerName = "Cerebras"
)

const baseURL = "https://inference-docs.cerebras.ai"

// CatalogURL lists every model Cerebras serves and links to its own page.
const CatalogURL = baseURL + "/models/overview.md"

// Provider reads Cerebras' model catalog. The zero value is not usable; call
// New.
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

// Fetch retrieves the catalog, the public model list, then one page per model
// the catalog links to.
//
// The catalog states what a model holds; its own page states what it costs,
// what it can do and what it takes, none of which the catalog repeats; and the
// public list is Cerebras answering which models it sells, with the two
// ceilings both documents round. A page that cannot be read costs that model
// its rates and nothing else, so the failure is reported and the rest of the
// run continues.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	index, err := p.get(ctx, CatalogURL)
	if err != nil {
		return nil, err
	}
	docs := []catalog.Document{index}
	var failures []error
	urls := append(
		[]string{
			PublicModelsURL,
			RateLimitsURL,
			ChangeLogURL,
			DeprecationsURL,
		},
		modelPageURLs(index)...,
	)
	for i := 0; i < len(urls); i++ {
		page, err := p.get(ctx, urls[i])
		if err != nil {
			failures = append(failures, err)
			continue
		}
		docs = append(docs, page)
		if urls[i] == PublicModelsURL {
			urls = append(urls, weightsURLs(page)...)
		}
	}
	return docs, errors.Join(failures...)
}

// weightsURLs returns where to read the licence of each set of weights the
// public model list names a repository for. They are derived from that list
// rather than listed here, because which weights Cerebras serves is a fact it
// states and not one this package may hold an opinion about.
func weightsURLs(list catalog.Document) []string {
	repos := weightsRepos(list)
	urls := make([]string, 0, len(repos))
	for _, repo := range repos {
		if url := weightsURL(repo); !slices.Contains(urls, url) {
			urls = append(urls, url)
		}
	}
	return urls
}

// Parse reads the public list first and the catalog second, because between
// them they say which models Cerebras serves, how exactly each is bounded and
// under which standing it is offered. Every other document adds to a model one
// of them has already named, and names no model of its own: the rate limit
// tables, the change log and the deprecation record all reach back years, and
// reading them as lists of models would fill the catalog with models nobody
// can call.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	for _, doc := range docs {
		if doc.URL != PublicModelsURL {
			continue
		}
		if err := b.applyPublicModels(doc); err != nil {
			return nil, err
		}
	}
	for _, doc := range docs {
		switch {
		case doc.URL == CatalogURL:
			b.applyCatalog(doc)
			b.applyDeprecations(doc)
		case doc.URL == RateLimitsURL:
			b.applyRateLimits(doc)
		case doc.URL == ChangeLogURL:
			b.applyChangeLog(doc)
		case doc.URL == DeprecationsURL:
			b.applyDeprecationPage(doc)
		case strings.HasPrefix(doc.URL, weightsBase):
			b.applyWeights(doc)
		case doc.URL != PublicModelsURL:
			b.applyModelPage(doc)
		}
	}
	return b.result(), nil
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
