package fireworks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// currency is the only currency Fireworks quotes.
const currency = "USD"

// providerID and providerName identify this source in the catalog.
const (
	providerID   = "fireworks"
	providerName = "Fireworks AI"
)

// fireworksAccount is the account Fireworks serves its own models from, which
// is the first half of the identifier a request carries.
const fireworksAccount = "fireworks"

// LibraryURL is the index of the model library, which is the only document
// naming every model Fireworks serves.
const LibraryURL = "https://fireworks.ai/models"

// libraryHost is what a link out of that index is relative to.
const libraryHost = "https://fireworks.ai"

// PricingURL is the page listing every serverless rate.
const PricingURL = "https://docs.fireworks.ai/serverless/pricing.md"

// PlansURL is the marketing rate card, which is where the price of a training
// job is stated.
const PlansURL = "https://fireworks.ai/pricing"

// StructuredOutputsURL is the guide stating which models can be constrained to
// a shape. A model's page in the library carries a row for tool use and one
// for image input and none for this, so the guide is where its scope is
// stated.
const StructuredOutputsURL = "https://docs.fireworks.ai/structured-responses/" +
	"structured-output-grammar-based.md"

// ChatCompletionsURL is the reference for the request every chat model
// answers. Its reasoning_effort parameter is documented model family by model
// family, which is where Fireworks states which of its models reason.
const ChatCompletionsURL = "https://docs.fireworks.ai/api-reference/" +
	"post-chatcompletions.md"

// RateLimitsURL states the ceilings a serverless caller starts at.
const RateLimitsURL = "https://docs.fireworks.ai/serverless/rate-limits.md"

// ServerlessOverviewURL states what holds for the shared fleet as a whole,
// including which models sit behind a prompt cache.
const ServerlessOverviewURL = "https://docs.fireworks.ai/serverless/overview.md"

// ServingPathsURL and USOnlyURL give the identifiers a caller sends to reach a
// model's faster or US-only serving path.
const (
	ServingPathsURL = "https://docs.fireworks.ai/serverless/serving-paths.md"
	USOnlyURL       = "https://docs.fireworks.ai/serverless/" +
		"us-only-serverless.md"
)

// ServerlessTrainingURL is the rate card of the shared trainer, which prices
// training per token for the few models it is open on.
const ServerlessTrainingURL = "https://docs.fireworks.ai/fine-tuning/" +
	"training-api/serverless.md"

// guideURLs are the documents read after the library, each stating something
// about models the library has already established.
var guideURLs = []string{
	PricingURL,
	PlansURL,
	StructuredOutputsURL,
	ChatCompletionsURL,
	RateLimitsURL,
	ServerlessOverviewURL,
	ServingPathsURL,
	USOnlyURL,
	ServerlessTrainingURL,
}

// fetchWorkers bounds the concurrent requests made for the model pages.
const fetchWorkers = 12

// Provider reads Fireworks' model library and rate cards. The zero value is
// not usable; call New.
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

// Fetch retrieves the library index, then the page of every model it links,
// then the documents that state rates and capabilities across models.
//
// The index is fetched first because it is the only list of the library, and
// nothing else Fireworks publishes enumerates it. A failure there is fatal:
// without it there are no models to read anything else onto. A single model
// page that fails is not, because the library is large and one page missing
// would otherwise throw away three hundred that did not.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	index, err := p.get(ctx, LibraryURL)
	if err != nil {
		return nil, err
	}
	pages, pageFailures := p.getAll(ctx, libraryPageURLs(index))
	guides, guideFailures := p.getAll(ctx, guideURLs)
	docs := append([]catalog.Document{index}, pages...)
	docs = append(docs, guides...)
	return docs, errors.Join(
		append(pageFailures, guideFailures...)...,
	)
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

// Parse reads the documents in the order each one depends on the last.
//
// The index comes first, because it holds what a model page will only round
// and has to be waiting when that page is read. The pricing page comes next,
// because it is the document Fireworks calls the source of truth for rates and
// a model's own page repeats one of them less precisely. Then the library
// pages, which establish the models themselves. Everything after that states
// something about models already established: what a rate card charges the
// ones it did not name, what a caller's ceilings are, which models reason, and
// what training them costs.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	phases := []func(catalog.Document){
		b.applyIndex,
		b.applyLibraryPage,
		b.applyPricing,
		b.applyGuide,
	}
	for phase, apply := range phases {
		for _, doc := range docs {
			if phaseOf(doc.URL) == phase {
				apply(doc)
			}
		}
	}
	b.resolveNamed()
	b.applyPagePrices()
	b.applyBands()
	b.applyBatch()
	return b.result(), nil
}

// phaseOf says which pass over the documents reads url, and so which document
// a reader has already applied by the time this one is read.
func phaseOf(url string) int {
	switch {
	case url == LibraryURL:
		return 0
	case strings.HasPrefix(url, LibraryURL+"/"):
		return 1
	case url == PricingURL:
		return 2
	}
	return 3
}

// applyGuide routes the documents that state something across models rather
// than about one.
func (b *builder) applyGuide(doc catalog.Document) {
	switch doc.URL {
	case StructuredOutputsURL:
		b.applyStructuredOutputs(doc)
	case ChatCompletionsURL:
		b.applyReasoning(doc)
	case RateLimitsURL:
		b.applyRateLimits(doc)
	case ServerlessOverviewURL:
		b.applyServerlessOverview(doc)
	case ServingPathsURL, USOnlyURL:
		b.applyRouters(doc)
	case ServerlessTrainingURL:
		b.applyServerlessTraining(doc)
	case PlansURL:
		b.applyTrainingPricing(doc)
	}
}

// builder accumulates models and the rate card rows that cannot be applied
// until every model is known.
type builder struct {
	models map[string]*catalog.Model
	order  []string
	// cards holds what the library index stated, against the address of the
	// page that will claim it.
	cards map[string]card
	// byPage is the model each library page established, which is how a rate
	// card row linking that page is resolved to the model it prices.
	byPage map[string]string
	// priced marks the models a rate card named outright, whose own page then
	// has nothing to add.
	priced map[string]bool
	// pagePrices are the rates the library pages quoted, held until the rate
	// cards have had their say.
	pagePrices []pagePrice
	// textBands and embeddingBands are the rate card rows that price by
	// parameter count, held until every model's count is known.
	textBands      []band
	embeddingBands []band
	// pendingNamed are the rate card rows that name a model without linking
	// it, held until the library can say which model they name.
	pendingNamed []namedRow
	// batchShare is the fraction of a rate that a batched request pays.
	batchShare float64
	// pricingSource is the page a band was read from.
	pricingSource string
}

func newBuilder() *builder {
	return &builder{
		models: map[string]*catalog.Model{},
		cards:  map[string]card{},
		byPage: map[string]string{},
		priced: map[string]bool{},
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
