package groq

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
		{ModelsURL, "models.md"},
		{
			baseURL + "/docs/compound/systems/compound.md",
			"compound.md",
		},
		{
			baseURL + "/docs/model/minimaxai/minimax-m2.7.md",
			"minimax-m2.7.md",
		},
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

// TestParseSystemPage covers a compound system, whose page is filed under a
// path of its own and whose rate is whatever its parts cost.
func TestParseSystemPage(t *testing.T) {
	m, ok := parse(t)["groq/compound"]
	if !ok {
		t.Fatal("groq/compound: not parsed")
	}
	if m.Attrs[AttrSystem] != "true" {
		t.Errorf("got attrs %v", m.Attrs)
	}
	if len(m.Prices) != 0 {
		t.Errorf("got %d prices, want none", len(m.Prices))
	}
	if len(m.Notes) == 0 {
		t.Fatal("no note saying why")
	}
	if !slices.Contains(m.Lists[ListFeatures], "web_search") {
		t.Errorf("got features %v", m.Lists[ListFeatures])
	}
	if got := m.Limits[LimitContextWindow]; got != 131072 {
		t.Errorf("got context %d", got)
	}
}

// TestParseEnterpriseModel covers the model whose rate cell says to ask, which
// is recorded as an access badge rather than as a rate.
func TestParseEnterpriseModel(t *testing.T) {
	m, ok := parse(t)["minimaxai/minimax-m2.7"]
	if !ok {
		t.Fatal("minimaxai/minimax-m2.7: not parsed")
	}
	if m.Attrs[AttrAccess] == "" {
		t.Errorf("got attrs %v", m.Attrs)
	}
	if len(m.Prices) != 0 {
		t.Errorf("got %d prices, want none", len(m.Prices))
	}
	if got := m.Limits[LimitMaxOutputTokens]; got != 131072 {
		t.Errorf("got max output %d", got)
	}
}

// TestParseRates covers the table's rate cell, which states a pair of amounts
// for a chat model and one for an audio model.
func TestParseRates(t *testing.T) {
	byID := parse(t)
	m, ok := byID["openai/gpt-oss-120b"]
	if !ok {
		t.Fatal("openai/gpt-oss-120b: not parsed")
	}
	found := map[catalog.Metric]bool{}
	for _, p := range m.Prices {
		found[p.Metric] = true
		if p.Unit != UnitPer1MTokens {
			t.Errorf("got unit %q", p.Unit)
		}
	}
	for _, metric := range []catalog.Metric{
		MetricInputTokens,
		MetricOutputTokens,
	} {
		if !found[metric] {
			t.Errorf("no %s rate in %v", metric, m.Prices)
		}
	}
}
