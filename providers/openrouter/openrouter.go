package openrouter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sync"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// currency is the only currency OpenRouter quotes.
const currency = "USD"

// providerID and providerName identify this source in the catalog.
const (
	providerID   = "openrouter"
	providerName = "OpenRouter"
)

// modelsBase is the listing endpoint, and ModelsURL the listing this package
// reads.
//
// The two differ by one query parameter. Asked for nothing in particular the
// endpoint answers with the models that return text or images and leaves out
// the rest, which is a third of what OpenRouter brokers: every embedding
// model, every reranker, every transcriber and every speech and video model.
// Asking for all output modalities is what makes the listing the whole
// catalog.
const (
	modelsBase = "https://openrouter.ai/api/v1/models"
	ModelsURL  = modelsBase + "?output_modalities=all"
)

// baseURL is the host the listing's own links are relative to.
const baseURL = "https://openrouter.ai"

// fetchWorkers bounds the concurrent requests made for the endpoint documents.
const fetchWorkers = 8

// Provider reads OpenRouter's model API. The zero value is not usable; call
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

// Fetch retrieves the model listing, then the endpoint document of every model
// the listing left short.
//
// The listing describes a model through the one upstream OpenRouter currently
// fronts for it, and that upstream does not always state a completion ceiling
// or forward a parameter that implies a capability. The rest of the upstreams
// serving the same model are a document away, so the listing is read first for
// the models it answers and the documents are fetched only for the ones it does
// not.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	models, err := p.get(ctx, ModelsURL)
	if err != nil {
		return nil, err
	}
	urls, err := detailURLs(models)
	if err != nil {
		return []catalog.Document{models}, err
	}
	urls = append(urls, ProvidersURL)
	for _, category := range categories {
		urls = append(urls, categoryURL(category))
	}
	rest, failures := p.getAll(ctx, urls)
	return append([]catalog.Document{models}, rest...),
		errors.Join(failures...)
}

// detailURLs derives the endpoint document of every model in the listing.
// Several entries share one document, because a model and its free variant are
// one model served two ways, so each document is fetched once.
func detailURLs(models catalog.Document) ([]string, error) {
	var list listing
	if err := json.Unmarshal(models.Body, &list); err != nil {
		return nil, fmt.Errorf("decode %s: %w", models.URL, err)
	}
	var urls []string
	for _, e := range list.Data {
		url := detailURL(e)
		if url == "" || slices.Contains(urls, url) {
			continue
		}
		urls = append(urls, url)
	}
	slices.Sort(urls)
	return urls, nil
}

// getAll retrieves urls concurrently, returning the documents in the order the
// urls were given so a run is reproducible.
func (p *Provider) getAll(
	ctx context.Context,
	urls []string,
) ([]catalog.Document, []error) {
	docs := make([]catalog.Document, len(urls))
	errs := make([]error, len(urls))
	var wg sync.WaitGroup
	work := make(chan int)
	for range min(fetchWorkers, len(urls)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				docs[i], errs[i] = p.get(ctx, urls[i])
			}
		}()
	}
	for i := range urls {
		work <- i
	}
	close(work)
	wg.Wait()

	out := make([]catalog.Document, 0, len(urls))
	var failures []error
	for i := range urls {
		if errs[i] != nil {
			failures = append(failures, errs[i])
			continue
		}
		out = append(out, docs[i])
	}
	return out, failures
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

// Parse reads the documents in the order they depend on each other.
//
// The listing comes first, because it is the only document naming the models
// and the only one linking them to their endpoint documents. The provider
// listing comes next, because it says where a seller sits and the endpoint
// documents are what name a model's sellers. The endpoint documents come
// third, and the subject listings last, since they add nothing but a
// membership to a model the listing already named.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	var failures []error
	stages := []struct {
		match func(catalog.Document) bool
		apply func(catalog.Document) error
	}{
		{
			func(d catalog.Document) bool { return d.URL == ModelsURL },
			b.applyListing,
		},
		{
			func(d catalog.Document) bool { return d.URL == ProvidersURL },
			b.applyProviders,
		},
		{
			func(d catalog.Document) bool {
				return d.URL != ModelsURL && d.URL != ProvidersURL &&
					!isCategoryURL(d.URL)
			},
			b.applyDetail,
		},
		{
			func(d catalog.Document) bool { return isCategoryURL(d.URL) },
			b.applyCategory,
		},
	}
	for _, stage := range stages {
		for _, doc := range docs {
			if !stage.match(doc) {
				continue
			}
			if err := stage.apply(doc); err != nil {
				failures = append(failures, err)
			}
		}
	}
	return b.result(), errors.Join(failures...)
}

// builder accumulates models.
type builder struct {
	models map[string]*catalog.Model
	order  []string
	// details maps an endpoint document to the models the listing linked to
	// it. The document names the model it describes, but under the identifier
	// of the base model rather than of the variant that linked to it, so the
	// link is what an endpoint document is attributed by.
	details map[string][]string
	// providers holds the seller facts, keyed by the name an endpoint document
	// calls the seller, and providerSource the document they were read from.
	providers      map[string]providerInfo
	providerSource string
}

func newBuilder() *builder {
	return &builder{
		models:    map[string]*catalog.Model{},
		details:   map[string][]string{},
		providers: map[string]providerInfo{},
	}
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
