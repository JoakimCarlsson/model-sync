package anthropic

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

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

// Fetch retrieves the documents. None is redundant: the pricing page names
// models only by display name, the overview states identifiers for current
// models only, the deprecations page is the sole source of identifiers and
// lifecycle for retired ones, the capability guides state the capabilities the
// overview's comparison table has no row for, the tool directory is the only
// page describing a tool as anything other than a rate, the changelog is the
// only page dating a model, the rate limits page is the only page bounding one
// per minute, and the thinking and context window guides each state one bound
// stated nowhere else.
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
		ReleaseNotesURL,
		RateLimitsURL,
		ThinkingURL,
		ContextWindowsURL,
		EffortURL,
		FastModeURL,
		ServiceTiersURL,
		ComputerUseURL,
		CompactionURL,
		TaskBudgetsURL,
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
	return catalog.Document{URL: url, Body: body}, nil
}
