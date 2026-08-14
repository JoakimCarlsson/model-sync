package anthropic

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

const baseURL = "https://platform.claude.com/docs/en"

// Documents Anthropic publishes that this parser reads. Appending .md to a doc
// URL returns its markdown source.
const (
	DeprecationsURL = baseURL + "/about-claude/model-deprecations.md"
	OverviewURL     = baseURL + "/about-claude/models/overview.md"
	PricingURL      = baseURL + "/about-claude/pricing.md"
)

// Fetch retrieves the six pages. None is redundant: the pricing page names
// models only by display name, the overview states identifiers for current
// models only, the deprecations page is the sole source of identifiers and
// lifecycle for retired ones, the two guides state the capabilities the
// overview's comparison table has no row for, and the tool directory is the
// only page describing a server tool as anything other than a rate.
func (p *Provider) Fetch(ctx context.Context) ([]catalog.Document, error) {
	var (
		docs     []catalog.Document
		failures []error
	)
	for _, url := range []string{
		DeprecationsURL,
		OverviewURL,
		PricingURL,
		ToolReferenceURL,
		StructuredOutputsURL,
		ToolUseURL,
	} {
		doc, err := p.get(ctx, url)
		if err != nil {
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
