package fireworks

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// parse runs the parser over the fixtures, read from disk so the test never
// touches the network.
func parse(t *testing.T) map[string]catalog.Model {
	t.Helper()
	files := []struct {
		url  string
		file string
	}{
		{PricingURL, "pricing.md"},
		{modelPagePre + "fireworks/minimax-m3", "minimax-m3.html"},
		{StructuredOutputsURL, "structured-outputs.md"},
	}
	docs := make([]catalog.Document, 0, len(files))
	for _, f := range files {
		body, err := os.ReadFile(filepath.Join("testdata", f.file))
		if err != nil {
			t.Fatal(err)
		}
		docs = append(docs, catalog.Document{URL: f.url, Body: body})
	}
	models, err := New().Parse(docs)
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]catalog.Model, len(models))
	for _, m := range models {
		byID[m.ID] = m
	}
	return byID
}

// TestParseServingPaths covers the two rates one cell holds, which Fireworks
// separates by which path serves the request.
func TestParseServingPaths(t *testing.T) {
	m, ok := parse(t)["fireworks/minimax-m3"]
	if !ok {
		t.Fatal("fireworks/minimax-m3: not parsed")
	}
	tiers := map[string]bool{}
	for _, p := range m.Prices {
		if p.Metric == MetricInputTokens {
			tiers[p.Dims[DimTier]] = true
		}
	}
	if len(tiers) < 2 {
		t.Errorf("got input rates for %v, want one per serving path", tiers)
	}
}

// TestParseModelPage covers the record a model's page carries as embedded
// JSON, which is the only place Fireworks states what a model holds.
func TestParseModelPage(t *testing.T) {
	m, ok := parse(t)["fireworks/minimax-m3"]
	if !ok {
		t.Fatal("fireworks/minimax-m3: not parsed")
	}
	if got := m.Limits[LimitContextWindow]; got != 512000 {
		t.Errorf("got context %d, want 512000", got)
	}
	if !slices.Contains(m.Lists[ListFeatures], "function_calling") {
		t.Errorf("got features %v", m.Lists[ListFeatures])
	}
	if !slices.Contains(m.Lists[ListInputModalities], "text") {
		t.Errorf("got input modalities %v", m.Lists[ListInputModalities])
	}
}

// TestParseNoOutputBound pins that Fireworks states no output ceiling for any
// model, so that one appearing later is noticed rather than assumed.
func TestParseNoOutputBound(t *testing.T) {
	for id, m := range parse(t) {
		if _, ok := m.Limits["max_output_tokens"]; ok {
			t.Errorf("%s: got an output ceiling", id)
		}
	}
}

// TestParseStructuredOutputs covers the capability Fireworks states for every
// model at once rather than as a flag on each. It reaches models the pricing
// page priced without linking to a page of their own, since the guide needs no
// model record, and stops at the models the guide is about: an embedding model
// returns a vector, which no schema constrains.
func TestParseStructuredOutputs(t *testing.T) {
	chat, embedding := 0, 0
	for id, m := range parse(t) {
		has := slices.Contains(
			m.Lists[ListFeatures],
			catalog.CapabilityStructuredOutputs,
		)
		if m.Kind == KindEmbedding {
			if has {
				t.Errorf("%s: an embedding model claiming structured output", id)
			}
			embedding++
			continue
		}
		if !has {
			t.Errorf("%s: got features %q, want structured output", id, m.Lists[ListFeatures])
		}
		chat++
	}
	if chat == 0 || embedding == 0 {
		t.Fatalf("got %d chat and %d embedding, want both covered", chat, embedding)
	}
}
