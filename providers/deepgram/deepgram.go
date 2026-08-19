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
	// IntelligenceURL is the feature overview for the four things Deepgram
	// reads out of a transcript, which is the only page saying whether each
	// runs on a live connection or only on a recording.
	IntelligenceURL = docsBase + "stt-intelligence-feature-overview.md"
	// SpeechURL is the feature overview for text to speech.
	SpeechURL = docsBase + "tts-feature-overview.md"
	// SpeechLimitsURL is the text-to-speech guide, which is where the ceiling
	// on the text one request may carry is stated.
	SpeechLimitsURL = docsBase + "text-to-speech.md"
	// AgentURL is the feature overview for the Voice Agent API.
	AgentURL = docsBase + "voice-agent-feature-overview.md"
)

// The documents naming the model options a request may ask for, which the
// pricing page never does, and saying what each of them understands.
const (
	// OptionsURL is the models and languages overview. It is the only
	// document listing every speech-to-text model option Deepgram serves and
	// the language codes each of them accepts.
	OptionsURL = docsBase + "models-languages-overview.md"
	// OptionDetailURL is the model parameter reference, which restates the
	// options as a request parameter and adds what the overview leaves out:
	// the size of each Whisper model and who may ask for a custom one.
	OptionDetailURL = docsBase + "model.md"
	// WhisperURL is the Whisper Cloud guide, which lists the languages and
	// the Deepgram features that Whisper answers for, neither of which the
	// overview states.
	WhisperURL = docsBase + "deepgram-whisper-cloud.md"
	// BatchLimitsURL is the pre-recorded transcription guide, which states
	// the ceiling on a submitted file.
	BatchLimitsURL = docsBase + "pre-recorded-audio.md"
)

// The documents describing what Deepgram speaks with, and how much of it at
// once.
const (
	// VoicesURL is the Aura voice catalog, one table per language, naming
	// every voice and the accent, age and gender it speaks in.
	VoicesURL = docsBase + "tts-models.md"
	// FluxVoicesURL is the same catalog for Flux TTS, which is served on a
	// different endpoint and documented apart from Aura.
	FluxVoicesURL = docsBase + "flux-tts/voices.md"
	// SpeechOptionsURL is the text-to-speech models and languages overview,
	// which is what says which languages each generation of voice covers.
	SpeechOptionsURL = docsBase + "tts-models-languages-overview.md"
	// RateLimitsURL is the concurrency reference. It is the only document
	// stating how many requests a plan may have in flight at once, and the
	// only one saying which of streaming and pre-recorded each model serves.
	RateLimitsURL = "https://developers.deepgram.com/reference/" +
		"api-rate-limits.md"
)

// docsBase is where Deepgram serves its documentation. Every page is published
// as markdown at the same path with .md appended, which is what these read.
const docsBase = "https://developers.deepgram.com/docs/"

// cacheFiles are where each fetched document is kept.
var cacheFiles = map[string]string{
	PricingURL:       "deepgram_pricing.html",
	STTStreamingURL:  "deepgram_stt_streaming.md",
	STTBatchURL:      "deepgram_stt_batch.md",
	FluxURL:          "deepgram_flux.md",
	IntelligenceURL:  "deepgram_intelligence.md",
	SpeechURL:        "deepgram_tts.md",
	SpeechLimitsURL:  "deepgram_tts_limits.md",
	AgentURL:         "deepgram_agent.md",
	OptionsURL:       "deepgram_options.md",
	OptionDetailURL:  "deepgram_model_option.md",
	WhisperURL:       "deepgram_whisper.md",
	BatchLimitsURL:   "deepgram_prerecorded.md",
	VoicesURL:        "deepgram_voices.md",
	FluxVoicesURL:    "deepgram_flux_voices.md",
	SpeechOptionsURL: "deepgram_tts_options.md",
	RateLimitsURL:    "deepgram_rate_limits.md",
}

// documentURLs are fetched in this order, the pricing page first because it is
// the only one naming what Deepgram sells.
var documentURLs = []string{
	PricingURL,
	OptionsURL,
	OptionDetailURL,
	WhisperURL,
	RateLimitsURL,
	STTStreamingURL,
	STTBatchURL,
	FluxURL,
	IntelligenceURL,
	SpeechURL,
	SpeechLimitsURL,
	SpeechOptionsURL,
	VoicesURL,
	FluxVoicesURL,
	BatchLimitsURL,
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

// parsePhases are the readers run over the fetched documents, in the order
// they must run in. What a document can be read for depends on what an earlier
// one established: the pricing page is the only one naming what Deepgram
// sells, the overview is the only one naming the model options it serves, and
// the concurrency reference is the only one saying which of streaming and
// pre-recorded each of those options answers on, which is what decides which
// feature overview describes them.
var parsePhases = []struct {
	url   string
	apply func(*builder, catalog.Document)
}{
	{PricingURL, (*builder).applyPricing},
	{OptionsURL, (*builder).applyOptions},
	{OptionDetailURL, (*builder).applyOptionDetail},
	{WhisperURL, (*builder).applyWhisper},
	{PricingURL, (*builder).applyFAQ},
	{RateLimitsURL, (*builder).applyRateLimits},
	{STTStreamingURL, (*builder).applyDocs},
	{STTBatchURL, (*builder).applyDocs},
	{FluxURL, (*builder).applyDocs},
	{IntelligenceURL, (*builder).applyIntelligence},
	{SpeechURL, (*builder).applyDocs},
	{AgentURL, (*builder).applyDocs},
	{SpeechLimitsURL, (*builder).applyDocs},
	{SpeechOptionsURL, (*builder).applySpeechOptions},
	{VoicesURL, (*builder).applyVoices},
	{FluxVoicesURL, (*builder).applyVoices},
	{BatchLimitsURL, (*builder).applyBatchLimits},
}

// Parse runs each reader over the document it reads, in the order the readers
// depend on one another. A document that did not arrive is skipped rather than
// treated as an empty one, so a failed fetch leaves what the others say
// standing instead of wiping it.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	byURL := make(map[string]catalog.Document, len(docs))
	for _, doc := range docs {
		byURL[doc.URL] = doc
	}
	for _, phase := range parsePhases {
		doc, ok := byURL[phase.url]
		if !ok || len(doc.Body) == 0 {
			continue
		}
		phase.apply(b, doc)
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
	// denied are the capabilities a document says a model does not have. A
	// feature overview answers for a whole product, and one page contradicts
	// it per model: the Whisper guide lists the Deepgram features Whisper
	// supports and the ones it does not, so what it denies has to outlast the
	// page that grants it to everything.
	denied map[string]map[string]bool
}

func newBuilder() *builder {
	return &builder{
		models: map[string]*catalog.Model{},
		denied: map[string]map[string]bool{},
	}
}

// deny records that a model does not have a capability, whatever a page
// describing its product says.
func (b *builder) deny(id, feature string) {
	if feature == "" {
		return
	}
	if b.denied[id] == nil {
		b.denied[id] = map[string]bool{}
	}
	b.denied[id][feature] = true
}

// addFeature records a capability unless a document has already said this
// model does not have it.
func (b *builder) addFeature(m *catalog.Model, features ...string) {
	for _, f := range features {
		if b.denied[m.ID][f] {
			continue
		}
		m.AddList(catalog.ListFeatures, f)
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
