package berget

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// defaultCurrency is what Berget quotes when it states no currency of its
// own. It is euros, not dollars, which is why every price carries the
// currency it was quoted in.
const defaultCurrency = "EUR"

// providerID and providerName identify this source in the catalog.
const (
	providerID   = "berget"
	providerName = "Berget"
)

// ModelsURL is the endpoint listing every model Berget serves.
const ModelsURL = "https://api.berget.ai/v1/models"

// cacheFile is where a fetched response is kept.
const cacheFile = "berget_models.json"

// Provider reads Berget's model API. The zero value is not usable; call New.
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

// Fetch retrieves the model listing.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	if body, ok := p.readCache(); ok {
		return []catalog.Document{{URL: ModelsURL, Body: body}}, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ModelsURL, nil)
	if err != nil {
		return nil, err
	}
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", ModelsURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: %s", ModelsURL, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", ModelsURL, err)
	}
	p.writeCache(body)
	return []catalog.Document{{URL: ModelsURL, Body: body}}, nil
}

// Parse decodes the listing.
func (p *Provider) Parse(docs []catalog.Document) ([]catalog.Model, error) {
	b := newBuilder()
	var failures []error
	for _, doc := range docs {
		if err := b.applyListing(doc); err != nil {
			failures = append(failures, err)
		}
	}
	return b.result(), errors.Join(failures...)
}

// readCache returns a previously fetched response.
func (p *Provider) readCache() ([]byte, bool) {
	if p.CacheDir == "" {
		return nil, false
	}
	body, err := os.ReadFile(filepath.Join(p.CacheDir, cacheFile))
	return body, err == nil
}

// writeCache stores a response, ignoring failures because the cache is an
// optimization and never the source of truth.
func (p *Provider) writeCache(body []byte) {
	if p.CacheDir == "" {
		return
	}
	if err := os.MkdirAll(p.CacheDir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(p.CacheDir, cacheFile), body, 0o644)
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
