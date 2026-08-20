package bedrock

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// currency is the currency the published rates are read in.
const currency = "USD"

// providerID and providerName identify this source in the catalog.
const (
	providerID   = "bedrock"
	providerName = "AWS Bedrock"
)

// PriceListURL is the machine-readable price list AWS bills from.
const PriceListURL = "https://pricing.us-east-1.amazonaws.com" +
	"/offers/v1.0/aws/AmazonBedrock/current/index.json"

const docsBase = "https://docs.aws.amazon.com/bedrock/latest/userguide/"

// Documents AWS publishes that this parser reads besides the price list.
const (
	// ContentsURL is the user guide's table of contents, which is the index
	// of the model cards.
	ContentsURL = docsBase + "toc-contents.json"
	// cardPre prefixes one model's card.
	cardPre = docsBase + "model-card-"
	// LifecycleURL is the page dating the retirement of every model AWS is
	// withdrawing, which is where a legacy model's exact dates are stated.
	LifecycleURL = docsBase + "model-lifecycle.md"
	// BatchURL and LatencyURL are the two pages naming, per model, the
	// Regions a serving path may be used in.
	BatchURL   = docsBase + "batch-inference-supported.md"
	LatencyURL = docsBase + "latency-optimized-inference.md"
	// QuotasURL is the page stating a default quota per model, which the
	// guide does for one endpoint and refers to the Service Quotas console
	// for the rest.
	QuotasURL = docsBase + "quotas-mantle.md"
)

// SpecURLs are the pages stating a model's specification as a labelled list.
// AWS writes one for each of the models Amazon built itself, and they are the
// only pages stating how wide a vector an embedding model returns.
var SpecURLs = []string{
	docsBase + "titan-embedding-models.md",
	docsBase + "titan-multiemb-models.md",
	docsBase + "titan-image-models.md",
}

// guideURLs are the pages read besides the cards, in the order they are
// fetched.
func guideURLs() []string {
	return append([]string{
		LifecycleURL,
		BatchURL,
		LatencyURL,
		QuotasURL,
	}, SpecURLs...)
}

// fetchWorkers bounds the concurrent requests made for the model cards.
const fetchWorkers = 8

// Provider reads AWS Bedrock's price list. The zero value is not usable; call
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

// Fetch retrieves the price list, the user guide's contents, and one model
// card per model the contents index.
//
// The price list is a billing document: it says what a model costs and nothing
// about the model. The cards say the rest — the context window, the output
// ceiling, the modalities, the capabilities and the identifiers the model
// answers to — and state no rate.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	prices, err := p.get(ctx, PriceListURL)
	if err != nil {
		return nil, err
	}
	docs := []catalog.Document{prices}
	contents, err := p.get(ctx, ContentsURL)
	if err != nil {
		return docs, err
	}
	cards, failures := p.getAll(ctx, cardURLs(contents))
	docs = append(docs, cards...)
	guide, guideFailures := p.getAll(ctx, guideURLs())
	docs = append(docs, guide...)
	return docs, errors.Join(append(failures, guideFailures...)...)
}

// cardHrefRe matches a link from the contents to one model's card.
var cardHrefRe = regexp.MustCompile(`"(model-card-[a-z0-9.-]+)\.html"`)

// cardURLs derives the model cards the contents index, asking for each in
// markdown, which AWS serves beside the rendered page and which states the
// same facts without the page around them.
func cardURLs(contents catalog.Document) []string {
	var urls []string
	for _, match := range cardHrefRe.FindAllStringSubmatch(
		string(contents.Body),
		-1,
	) {
		url := docsBase + match[1] + ".md"
		if !slices.Contains(urls, url) {
			urls = append(urls, url)
		}
	}
	slices.Sort(urls)
	return urls
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

// Parse decodes the price list first, because it is the only document naming
// the models, then reads each card onto the model it describes.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	var failures []error
	for _, doc := range docs {
		if doc.URL != PriceListURL {
			continue
		}
		if err := b.applyPriceList(doc); err != nil {
			failures = append(failures, err)
		}
	}
	var cards []catalog.Document
	for _, doc := range docs {
		if strings.HasPrefix(doc.URL, cardPre) {
			cards = append(cards, doc)
		}
	}
	b.applyCards(cards)
	for _, doc := range docs {
		b.applyGuide(doc)
	}
	return b.result(), errors.Join(failures...)
}

// applyGuide records one of the pages read besides the cards, each of which
// states one fact about many models where a card states many about one. They
// are read last, because every one of them is joined to a model on an
// identifier that only a card supplies.
func (b *builder) applyGuide(doc catalog.Document) {
	switch doc.URL {
	case LifecycleURL:
		b.applyLifecycle(doc)
	case BatchURL:
		b.applySupport(doc, featureBatch, ListBatchRegions, ListBatchProfiles)
	case LatencyURL:
		b.applySupport(doc, featureLatency, "", ListLatencyRegions)
	case QuotasURL:
		b.applyQuotas(doc)
	default:
		if slices.Contains(SpecURLs, doc.URL) {
			b.applySpec(doc)
		}
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
