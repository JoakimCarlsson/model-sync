package voyage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

const baseURL = "https://docs.voyageai.com/docs"

// overviewURL is the model overview MongoDB publishes, Voyage now being part of
// MongoDB. It restates the same tables in the same shape, and it is read
// because it is the only page that still carries the context length and the
// embedding width of a model Voyage's own pages have dropped while continuing
// to serve and to charge for it.
const overviewURL = "https://www.mongodb.com/docs/voyageai/models.md"

// Documents Voyage publishes that this parser reads. Appending .md to a doc
// URL returns its markdown source.
var documentURLs = []string{
	baseURL + "/pricing.md",
	baseURL + "/embeddings.md",
	baseURL + "/multimodal-embeddings.md",
	baseURL + "/contextualized-chunk-embeddings.md",
	baseURL + "/reranker.md",
	baseURL + "/batch-inference.md",
	overviewURL,
}

// Fetch retrieves every document. The pricing page is required; the others
// only add detail, so losing one degrades the run instead of ending it.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	var (
		docs     []catalog.Document
		failures []error
	)
	for _, url := range documentURLs {
		doc, err := p.get(ctx, url)
		if err != nil {
			if isPricing(url) {
				return nil, err
			}
			failures = append(failures, err)
			continue
		}
		docs = append(docs, doc)
	}
	return docs, errors.Join(failures...)
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

// readCache returns a previously fetched body.
func (p *Provider) readCache(url string) ([]byte, bool) {
	if p.CacheDir == "" {
		return nil, false
	}
	body, err := os.ReadFile(filepath.Join(p.CacheDir, cacheName(url)))
	return body, err == nil
}

// writeCache stores a body, ignoring failures because the cache is an
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
