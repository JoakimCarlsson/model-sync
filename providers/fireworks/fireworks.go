package fireworks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
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

// PricingURL is the page listing every serverless rate.
const PricingURL = "https://docs.fireworks.ai/serverless/pricing.md"

// StructuredOutputsURL is the guide stating which models can be constrained to
// a shape. A model's own page carries a flag for tool use and for image input
// and none for this, so the guide is where its scope is stated.
const StructuredOutputsURL = "https://docs.fireworks.ai/structured-responses/" +
	"structured-output-grammar-based.md"

// EmbeddingsGuideURL is the guide naming the embedding models Fireworks
// serves. The pricing page prices its embedding model under a name and links
// nothing, and this is the document tying that name to a model page.
const EmbeddingsGuideURL = "https://docs.fireworks.ai/guides/" +
	"querying-embeddings-models.md"

// ChatCompletionsURL is the reference for the request every chat model
// answers. Its reasoning_effort parameter is documented model family by model
// family, which is where Fireworks states which of its models reason.
const ChatCompletionsURL = "https://docs.fireworks.ai/api-reference/" +
	"post-chatcompletions.md"

// modelPagePre prefixes the page each priced model is linked to.
const modelPagePre = "https://app.fireworks.ai/models/"

// libraryPagePre prefixes the model library's page for a model, which states
// in prose what the console record has no field for.
const libraryPagePre = "https://fireworks.ai/models/"

// fetchWorkers bounds the concurrent requests made for the model pages.
const fetchWorkers = 8

// Provider reads Fireworks' pricing page. The zero value is not usable; call
// New.
type Provider struct {
	// Client performs the fetch.
	Client *http.Client
	// CacheDir, when set, backs the fetch with a file on disk.
	CacheDir string
}

// New returns a Provider using the default HTTP client.
func New() *Provider {
	return &Provider{Client: http.DefaultClient}
}

// ID implements catalog.Source.
func (p *Provider) ID() string { return providerID }

// Name implements catalog.Source.
func (p *Provider) Name() string { return providerName }

// Fetch retrieves the pricing page, the guides that state what it does not,
// then the page of every model either of them links to.
//
// The pricing page states rates and a link, and nothing about the model
// itself. What a model holds and can take is on the page behind that link,
// which states no rate. Several rows link to the same page, because a model
// served three ways is one model with three rates, so each page is fetched
// once. The embedding model is the exception: it is priced under a name and
// linked nowhere, so the guide to the embeddings API is read first for the
// page it is served from.
//
// A last round fetches the model library's page for the few models whose
// console record left something open, because that page states in prose what
// the record has no field for.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	pricing, err := p.get(ctx, PricingURL)
	if err != nil {
		return nil, err
	}
	guides, failures := p.getAll(ctx, []string{
		EmbeddingsGuideURL,
		StructuredOutputsURL,
		ChatCompletionsURL,
	})
	embedding := embeddingPageURLs(pricing, guides)
	pages, pageFailures := p.getAll(
		ctx,
		append(modelPageURLs(pricing), embedding...),
	)
	library, libraryFailures := p.getAll(
		ctx,
		libraryPageURLs(pages, embedding),
	)
	docs := append([]catalog.Document{pricing}, guides...)
	docs = append(docs, pages...)
	docs = append(docs, library...)
	failures = append(failures, pageFailures...)
	for _, failure := range libraryFailures {
		if !errors.Is(failure, errNoPage) {
			failures = append(failures, failure)
		}
	}
	return docs, errors.Join(failures...)
}

// errNoPage marks a document the provider publishes no page for. The model
// library carries a page for most models and not for all of them, and a model
// it omits is one fewer place to read rather than a failed refresh.
var errNoPage = errors.New("no page")

// embeddingPageURLs derives the pages of the embedding models the pricing page
// priced, which it names without linking. The guide is what links them, and
// the pricing page is what says which of them Fireworks charges for.
func embeddingPageURLs(
	pricing catalog.Document,
	guides []catalog.Document,
) []string {
	b := newBuilder()
	b.applyPricing(pricing)
	var urls []string
	for _, doc := range guides {
		if doc.URL != EmbeddingsGuideURL {
			continue
		}
		for _, match := range b.matchEmbeddings(doc) {
			urls = append(urls, match.Ref.URL)
		}
	}
	slices.Sort(urls)
	return slices.Compact(urls)
}

// modelPageURLs derives the model pages the pricing page links to.
func modelPageURLs(pricing catalog.Document) []string {
	var urls []string
	for _, match := range modelHrefRe.FindAllStringSubmatch(
		string(pricing.Body),
		-1,
	) {
		if !slices.Contains(urls, match[0]) {
			urls = append(urls, match[0])
		}
	}
	slices.Sort(urls)
	return urls
}

// modelHrefRe matches a link from the pricing page to a model's page.
var modelHrefRe = regexp.MustCompile(
	regexp.QuoteMeta(modelPagePre) + `[A-Za-z0-9._/-]+`,
)

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

// get retrieves one document, reading from and writing to the cache directory
// when one is configured.
func (p *Provider) get(
	ctx context.Context,
	url string,
) (catalog.Document, error) {
	if body, ok := p.readCache(url); ok {
		return catalog.Document{URL: url, Body: body}, nil
	}
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
	if resp.StatusCode == http.StatusNotFound {
		return catalog.Document{}, fmt.Errorf("fetch %s: %w", url, errNoPage)
	}
	if resp.StatusCode != http.StatusOK {
		return catalog.Document{}, fmt.Errorf("fetch %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return catalog.Document{}, fmt.Errorf("read %s: %w", url, err)
	}
	p.writeCache(url, body)
	return catalog.Document{URL: url, Body: body}, nil
}

// Parse reads the documents in the order each one depends on the last. The
// pricing page comes first, because it is the only document naming the models
// and the only one pricing them; then the embeddings guide, which is what ties
// the embedding model's name to a page; then the model pages; then the library
// pages, which fill only what a model page left unstated; and last the two
// documents stating a capability for models the pricing page has already
// established.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	phases := []func(catalog.Document){
		b.applyPricing,
		b.applyEmbeddingsGuide,
		b.applyModelPage,
		b.applyLibraryPage,
		b.applyCapabilities,
	}
	for phase, apply := range phases {
		for _, doc := range docs {
			if phaseOf(doc.URL) == phase {
				apply(doc)
			}
		}
	}
	return b.result(), nil
}

// phaseOf says which pass over the documents reads url, and so which document
// a reader has already applied by the time this one is read.
func phaseOf(url string) int {
	switch {
	case url == PricingURL:
		return 0
	case url == EmbeddingsGuideURL:
		return 1
	case url == StructuredOutputsURL, url == ChatCompletionsURL:
		return 4
	case strings.HasPrefix(url, libraryPagePre):
		return 3
	default:
		return 2
	}
}

// applyCapabilities routes the two documents stating a capability against
// something other than one model.
func (b *builder) applyCapabilities(doc catalog.Document) {
	switch doc.URL {
	case StructuredOutputsURL:
		b.applyStructuredOutputs(doc)
	case ChatCompletionsURL:
		b.applyReasoning(doc)
	}
}

// readCache returns a previously fetched body.
func (p *Provider) readCache(url string) ([]byte, bool) {
	if p.CacheDir == "" {
		return nil, false
	}
	body, err := os.ReadFile(filepath.Join(p.CacheDir, cacheName(url)))
	return body, err == nil
}

// cacheName turns a URL into a flat filename.
func cacheName(url string) string {
	trimmed := strings.TrimPrefix(url, "https://")
	return providerID + "_" + strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '.', r == '-', r == '_':
			return r
		}
		return '_'
	}, trimmed)
}

// writeCache stores a document, ignoring failures because the cache is an
// optimization and never the source of truth.
func (p *Provider) writeCache(url string, body []byte) {
	if p.CacheDir == "" {
		return
	}
	if err := os.MkdirAll(p.CacheDir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(p.CacheDir, cacheName(url)), body, 0o644)
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

// rename re-keys a model, for the embedding model the pricing page priced
// under a display name before the guide stated the identifier it is served
// under.
func (b *builder) rename(from, to string) {
	m, ok := b.models[from]
	if !ok || to == "" || from == to {
		return
	}
	if _, taken := b.models[to]; taken {
		return
	}
	delete(b.models, from)
	m.ID = to
	b.models[to] = m
	b.order[slices.Index(b.order, from)] = to
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
