package perplexity

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// currency is the only currency Perplexity quotes.
const currency = "USD"

// providerID and providerName identify this source in the catalog.
const (
	providerID   = "perplexity"
	providerName = "Perplexity"
)

const baseURL = "https://docs.perplexity.ai"

// Documents Perplexity publishes that this parser reads. The pricing page
// covers what Perplexity sells directly; the models page covers what it
// brokers from other labs.
var documentURLs = []string{
	PricingURL,
	AgentModelsURL,
	EmbeddingsURL,
	SonarIndexURL,
	RouterModelsURL,
	AgentToolsURL,
	SonarAPIURL,
	AsyncSonarURL,
	EmbeddingsPostURL,
	ContextEmbedURL,
	SearchPostURL,
	AgentRequestURL,
	RouterChatURL,
	RouterMessagesURL,
	FeaturesURL,
	MediaURL,
	AgentOutputURL,
	RateLimitsURL,
	ChangelogURL,
	MigrateURL,
}

// PricingURL is the rate card for everything Perplexity sells directly.
const PricingURL = baseURL + "/getting-started/pricing.md"

// rateDocumentURLs are the documents whose tables create models. Every other
// document states something about models these have already named, and one
// read into an identifier none of them carries has been misread.
var rateDocumentURLs = []string{
	PricingURL,
	AgentModelsURL,
	EmbeddingsURL,
	SonarIndexURL,
	RouterModelsURL,
}

// guideURLs are the documents describing the Sonar API rather than any one
// model, which are read only after the model pages have said which models the
// API serves.
var guideURLs = []string{FeaturesURL, MediaURL}

// SonarIndexURL lists the models Perplexity serves itself and links to the
// page each of them has, which is the only place a context window is stated.
const SonarIndexURL = baseURL + "/docs/sonar/models.md"

// Provider reads Perplexity's pricing documentation. The zero value is not
// usable; call New.
type Provider struct {
	// Client performs the fetches.
	Client *http.Client
	// CacheDir, when set, backs every fetch with a file on disk.
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

// Fetch retrieves both documents. Losing one degrades the run rather than
// ending it, since each covers models the other does not.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	var (
		docs     []catalog.Document
		failures []error
	)
	for _, url := range documentURLs {
		doc, err := p.get(ctx, url)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		docs = append(docs, doc)
		for _, page := range linkedPages(doc) {
			linked, err := p.get(ctx, page)
			if err != nil {
				failures = append(failures, err)
				continue
			}
			docs = append(docs, linked)
		}
	}
	return docs, errors.Join(failures...)
}

// linkedPages returns the per-model and per-tool pages an index links to.
// Each index addresses what it links to by a slug of its own, so those pages
// are derived from the index rather than listed here.
func linkedPages(doc catalog.Document) []string {
	switch doc.URL {
	case SonarIndexURL:
		return sonarModelURLs(doc)
	case AgentToolsURL:
		return toolPageURLs(doc)
	}
	return nil
}

// Parse reads the rate documents first, because they are the only ones naming
// the models, then the indexes that group them, and finally the references,
// guides and dated documents onto what those established.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	for _, doc := range docs {
		if namesModels(doc.URL) {
			b.applyDocument(doc)
		}
	}
	for _, doc := range docs {
		b.applyIndex(doc)
	}
	for _, doc := range docs {
		b.applyDetail(doc)
	}
	return b.result(), nil
}

// namesModels reports whether a document holds the rate tables a model is
// created from.
func namesModels(url string) bool {
	return slices.Contains(rateDocumentURLs, url)
}

// applyIndex reads a document that groups models already named. This has to
// happen before the references and guides, because those name no model and are
// read onto the groups these establish.
func (b *builder) applyIndex(doc catalog.Document) {
	switch {
	case strings.HasPrefix(doc.URL, sonarModelPre):
		b.applySonarPage(doc)
	case doc.URL == AgentModelsURL:
		b.applyAgentCards(doc)
	case doc.URL == RouterModelsURL:
		b.applyRouterCatalog(doc)
	case doc.URL == AgentToolsURL:
		b.applyToolIndex(doc)
	}
}

// applyDetail reads a document stating something about models already grouped.
func (b *builder) applyDetail(doc catalog.Document) {
	switch {
	case doc.URL == MediaURL:
		b.applyBaseModalities(doc.URL)
		b.applyGuide(doc)
	case doc.URL == FeaturesURL:
		b.applyGuide(doc)
	case doc.URL == EmbeddingsURL:
		b.applyEmbeddingProse(doc)
	case doc.URL == PricingURL:
		b.applyProSearch(doc)
	case doc.URL == SonarAPIURL, doc.URL == AsyncSonarURL:
		b.applyReference(doc, b.sonar)
	case doc.URL == EmbeddingsPostURL, doc.URL == ContextEmbedURL:
		b.applyReference(doc, b.embedding)
	case doc.URL == SearchPostURL:
		b.applyReference(doc, b.searchAPI)
	case doc.URL == AgentOutputURL:
		b.applyAgentGuide(doc)
	case doc.URL == AgentRequestURL:
		b.applyAgentSchema(doc)
		b.applyReference(doc, b.agent)
		b.applyToolHost(doc)
	case doc.URL == RouterChatURL, doc.URL == RouterMessagesURL:
		b.applyRouterReference(doc)
	case doc.URL == MigrateURL:
		b.applyMigration(doc)
	case doc.URL == RateLimitsURL:
		b.applyRateLimits(doc)
	case doc.URL == ChangelogURL:
		b.applyChangelog(doc)
	case b.toolPages[doc.URL] != "":
		b.applyToolPage(doc)
	}
}

// applyToolHost records the endpoint a built-in tool runs inside, which no
// tool page states as a path and every one of them states as the Agent API.
func (b *builder) applyToolHost(doc catalog.Document) {
	path, ok := referencePath(string(doc.Body))
	if !ok {
		return
	}
	b.addEndpoint(b.tools, path, doc.URL)
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
	if resp.StatusCode != http.StatusOK {
		return catalog.Document{}, fmt.Errorf(
			"fetch %s: %s",
			url,
			resp.Status,
		)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return catalog.Document{}, fmt.Errorf("read %s: %w", url, err)
	}
	p.writeCache(url, body)
	return catalog.Document{URL: url, Body: body}, nil
}

// readCache returns a previously fetched document.
func (p *Provider) readCache(url string) ([]byte, bool) {
	if p.CacheDir == "" {
		return nil, false
	}
	body, err := os.ReadFile(filepath.Join(p.CacheDir, cacheName(url)))
	return body, err == nil
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

// cacheName turns a URL into a flat filename.
func cacheName(url string) string {
	trimmed := strings.TrimPrefix(url, baseURL+"/")
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

// builder accumulates models.
type builder struct {
	models map[string]*catalog.Model
	order  []string
	// sonar holds the models Perplexity serves itself, which are the ones with
	// a page of their own. The guides describing what the Sonar API can do name
	// no model, so this is what says who they are about.
	sonar []string
	// agent holds the models the Agent API serves, which are the ones its model
	// page tabulates. The Agent API's own guides name no model either.
	agent []string
	// router holds the models the Router API serves, which are the ones its
	// own catalog lists and which that catalog calls its allowlist.
	router []string
	// embedding holds the embedding models, which the rate limit page bounds
	// as a group rather than one at a time.
	embedding []string
	// tools holds the built-in tools of the Agent API, which are billed
	// products rather than models and are named by the type a request enables
	// them with.
	tools []string
	// searchAPI holds the standalone search product, which is billed per
	// request and answers on an endpoint of its own.
	searchAPI []string
	// toolPages maps a tool's reference page onto the tool it documents, since
	// a tool page is addressed by its title and not by the type a request
	// enables it with.
	toolPages map[string]string
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
