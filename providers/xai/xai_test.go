package xai

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
		{ModelsURL, "models.md"},
		{modelPagePre + "grok-4.6.md", "grok-4.6.md"},
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

// TestParsePromptBands covers the two rates a model has for the same tokens,
// which xAI separates by how large the request is.
func TestParsePromptBands(t *testing.T) {
	m, ok := parse(t)["grok-4.6"]
	if !ok {
		t.Fatal("grok-4.6: not parsed")
	}
	bands := map[string]float64{}
	for _, p := range m.Prices {
		if p.Metric == MetricInputTokens {
			bands[p.Dims[DimPromptBand]] = p.Amount
		}
	}
	if len(bands) != 2 {
		t.Errorf("got input rates %v, want one per band", bands)
	}
	if got := m.Limits[LimitContextWindow]; got == 0 {
		t.Error("no context window")
	}
	if !slices.Contains(m.Lists[ListInputModalities], "image") {
		t.Errorf("got input modalities %v", m.Lists[ListInputModalities])
	}
}

// TestParseToolsWithoutRates covers the three tools whose rate cell holds
// words rather than an amount, which are kept as a note so that a tool without
// a price does not read as a free one.
func TestParseToolsWithoutRates(t *testing.T) {
	byID := parse(t)
	for _, id := range []string{
		"image_generation",
		"view_image",
		"view_x_video",
	} {
		m, ok := byID[id]
		if !ok {
			t.Errorf("%s: not parsed", id)
			continue
		}
		if len(m.Prices) != 0 {
			t.Errorf("%s: got %d prices, want none", id, len(m.Prices))
		}
		if len(m.Notes) == 0 {
			t.Errorf("%s: no note saying why", id)
		}
	}
}

// TestParseToolRates covers the tools that are charged per call.
func TestParseToolRates(t *testing.T) {
	m, ok := parse(t)["web_search"]
	if !ok {
		t.Fatal("web_search: not parsed")
	}
	if len(m.Prices) != 1 {
		t.Fatalf("got %d prices, want 1", len(m.Prices))
	}
	p := m.Prices[0]
	if p.Metric != MetricToolCall || p.Unit != UnitPer1KCalls || p.Amount != 5 {
		t.Errorf("got %s %s %v", p.Metric, p.Unit, p.Amount)
	}
}
