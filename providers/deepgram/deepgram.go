package deepgram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// currency is the only currency Deepgram quotes.
const currency = "USD"

// providerID and providerName identify this source in the catalog.
const (
	providerID   = "deepgram"
	providerName = "Deepgram"
)

// PricingURL is the only page stating Deepgram's rates.
const PricingURL = "https://deepgram.com/pricing"

// The documentation pages saying what a model can do. Deepgram states a
// capability once per product rather than once per model, so each of these
// pages answers for every model the pricing page sells under that product.
const (
	// STTStreamingURL is the feature overview for transcribing a live
	// connection.
	STTStreamingURL = docsBase + "stt-streaming-feature-overview.md"
	// STTBatchURL is the feature overview for transcribing a finished
	// recording, which Deepgram calls pre-recorded.
	STTBatchURL = docsBase + "stt-pre-recorded-feature-overview.md"
	// FluxURL is the feature overview for the Flux models, which support a
	// different set from the rest of speech to text and so have a page of
	// their own.
	FluxURL = docsBase + "flux/feature-overview.md"
	// SpeechURL is the feature overview for text to speech.
	SpeechURL = docsBase + "tts-feature-overview.md"
	// SpeechLimitsURL is the text-to-speech guide, which is where the ceiling
	// on the text one request may carry is stated.
	SpeechLimitsURL = docsBase + "text-to-speech.md"
	// AgentURL is the feature overview for the Voice Agent API.
	AgentURL = docsBase + "voice-agent-feature-overview.md"
)

// docsBase is where Deepgram serves its documentation. Every page is published
// as markdown at the same path with .md appended, which is what these read.
const docsBase = "https://developers.deepgram.com/docs/"

// cacheFiles are where each fetched document is kept.
var cacheFiles = map[string]string{
	PricingURL:      "deepgram_pricing.html",
	STTStreamingURL: "deepgram_stt_streaming.md",
	STTBatchURL:     "deepgram_stt_batch.md",
	FluxURL:         "deepgram_flux.md",
	SpeechURL:       "deepgram_tts.md",
	SpeechLimitsURL: "deepgram_tts_limits.md",
	AgentURL:        "deepgram_agent.md",
}

// documentURLs are fetched in this order, the pricing page first because it is
// the only one naming the models the rest describe.
var documentURLs = []string{
	PricingURL,
	STTStreamingURL,
	STTBatchURL,
	FluxURL,
	SpeechURL,
	SpeechLimitsURL,
	AgentURL,
}

// Provider reads Deepgram's pricing page. The zero value is not usable; call
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

// Fetch retrieves the pricing page and every documentation page describing
// what a model can do.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	docs := make([]catalog.Document, 0, len(documentURLs))
	for _, url := range documentURLs {
		doc, err := p.get(ctx, url)
		if err != nil {
			return docs, err
		}
		docs = append(docs, doc)
	}
	return docs, nil
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
		return catalog.Document{}, fmt.Errorf("fetch %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return catalog.Document{}, fmt.Errorf("read %s: %w", url, err)
	}
	p.writeCache(url, body)
	return catalog.Document{URL: url, Body: body}, nil
}

// Parse reads the pricing page first, because it is the only document naming
// the models, then reads the documentation onto what it established.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	for _, doc := range docs {
		if doc.URL == PricingURL {
			b.applyPricing(doc)
		}
	}
	for _, doc := range docs {
		if doc.URL != PricingURL {
			b.applyDocs(doc)
		}
	}
	return b.result(), nil
}

// readCache returns a previously fetched document.
func (p *Provider) readCache(url string) ([]byte, bool) {
	if p.CacheDir == "" {
		return nil, false
	}
	body, err := os.ReadFile(filepath.Join(p.CacheDir, cacheFiles[url]))
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
	_ = os.WriteFile(filepath.Join(p.CacheDir, cacheFiles[url]), body, 0o644)
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

// each calls fn for every model accumulated so far.
func (b *builder) each(fn func(m *catalog.Model)) {
	for _, id := range b.order {
		fn(b.models[id])
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
