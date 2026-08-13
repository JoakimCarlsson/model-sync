package openai

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// modelPage is where one model's page is addressed from.
const modelPagePre = baseURL + "/api/docs/models/"

// parse runs the parser over the fixtures, read from disk so the test never
// touches the network.
func parse(t *testing.T) map[string]catalog.Model {
	t.Helper()
	files := []struct {
		url  string
		file string
	}{
		{PricingURL, "pricing.md"},
		{modelPagePre + "gpt-5.6-sol.md", "gpt-5.6-sol.md"},
		{
			modelPagePre + "daybreak-blue-latest.md",
			"daybreak-blue-latest.md",
		},
		{modelPagePre + "gpt-oss-120b.md", "gpt-oss-120b.md"},
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

// TestParseAliasRates covers the two models OpenAI prices only by saying they
// cost what the model they point at costs.
func TestParseAliasRates(t *testing.T) {
	byID := parse(t)
	alias, ok := byID["daybreak-blue-latest"]
	if !ok {
		t.Fatal("daybreak-blue-latest: not parsed")
	}
	target, ok := byID["gpt-5.6-sol"]
	if !ok {
		t.Fatal("gpt-5.6-sol: not parsed")
	}
	if len(alias.Prices) != len(target.Prices) {
		t.Errorf(
			"got %d prices, want the %d of its target",
			len(alias.Prices),
			len(target.Prices),
		)
	}
	if !slices.Contains(alias.Notes, notePricedAs+"gpt-5.6-sol") {
		t.Errorf("got notes %v", alias.Notes)
	}
	if alias.Attrs[AttrDefaultSnapshot] != "gpt-5.6-sol" {
		t.Errorf("got snapshot %q", alias.Attrs[AttrDefaultSnapshot])
	}
}

// TestParseUnpricedStayUnpriced pins the models OpenAI states no rate for: a
// row of dashes in the rate table, and an open-weight model the table leaves
// out altogether.
func TestParseUnpricedStayUnpriced(t *testing.T) {
	byID := parse(t)
	for _, id := range []string{"gpt-5.4-cyber", "gpt-oss-120b"} {
		m, ok := byID[id]
		if !ok {
			t.Errorf("%s: not parsed", id)
			continue
		}
		if len(m.Prices) != 0 {
			t.Errorf("%s: got %d prices, want none", id, len(m.Prices))
		}
	}
}

// TestParsePricing covers the rate table itself, including the long-context
// rates stated as a second pair of columns.
func TestParsePricing(t *testing.T) {
	m, ok := parse(t)["gpt-5.6-sol"]
	if !ok {
		t.Fatal("gpt-5.6-sol: not parsed")
	}
	found := map[catalog.Metric]bool{}
	for _, p := range m.Prices {
		found[p.Metric] = true
	}
	for _, metric := range []catalog.Metric{
		MetricInputTokens,
		MetricOutputTokens,
		MetricCachedInputTokens,
	} {
		if !found[metric] {
			t.Errorf("no %s rate in %v", metric, m.Prices)
		}
	}
	if m.Limits[LimitContextWindow] == 0 {
		t.Error("no context window")
	}
}
