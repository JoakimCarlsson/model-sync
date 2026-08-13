package azure

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// fixtures returns the documents to parse, read from disk so the test never
// touches the network.
func fixtures(t *testing.T) []catalog.Document {
	t.Helper()
	docs := []catalog.Document{}
	for _, f := range []struct {
		url  string
		file string
	}{
		{retailPricesURL, "meters.json"},
		{ModelsURL, "models.html"},
	} {
		body, err := os.ReadFile(filepath.Join("testdata", f.file))
		if err != nil {
			t.Fatal(err)
		}
		docs = append(docs, catalog.Document{URL: f.url, Body: body})
	}
	return docs
}

// parse runs the parser over the fixtures and indexes the result.
func parse(t *testing.T) map[string]catalog.Model {
	t.Helper()
	models, err := New().Parse(fixtures(t))
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]catalog.Model, len(models))
	for _, m := range models {
		byID[m.ID] = m
	}
	return byID
}

// TestParseCollections covers the capability tables, which are the only place
// Azure states what the models it does not own hold and can do.
func TestParseCollections(t *testing.T) {
	byID := parse(t)
	cases := []struct {
		id      string
		context int64
		maxOut  int64
		in      []string
		out     []string
	}{
		{"command-a", 131072, 8182, []string{"text"}, []string{"text"}},
		{"v4-pro", 1000000, 384000, []string{"text"}, []string{"text"}},
		{
			"llama-3.3-70b",
			128000,
			8192,
			[]string{"text"},
			[]string{"text"},
		},
	}
	for _, c := range cases {
		m, ok := byID[c.id]
		if !ok {
			t.Errorf("%s: not parsed", c.id)
			continue
		}
		if got := m.Limits[LimitContextWindow]; got != c.context {
			t.Errorf("%s: got context %d, want %d", c.id, got, c.context)
		}
		if got := m.Limits[LimitMaxOutputTokens]; got != c.maxOut {
			t.Errorf("%s: got max output %d, want %d", c.id, got, c.maxOut)
		}
		if got := m.Lists[ListInputModalities]; len(got) != len(c.in) {
			t.Errorf("%s: got input modalities %v", c.id, got)
		}
		if got := m.Lists[ListOutputModalities]; len(got) != len(c.out) {
			t.Errorf("%s: got output modalities %v", c.id, got)
		}
	}
}

// TestParseCollectionCapabilities covers the bullets that are not bounds.
func TestParseCollectionCapabilities(t *testing.T) {
	byID := parse(t)
	m := byID["command-a"]
	for _, feature := range []string{"function_calling", "json_mode"} {
		if !has(m.Lists[ListFeatures], feature) {
			t.Errorf(
				"command-a: missing feature %q in %v",
				feature,
				m.Lists[ListFeatures],
			)
		}
	}
	if !has(m.Lists[ListLanguages], "pt-br") {
		t.Errorf("command-a: got languages %v", m.Lists[ListLanguages])
	}
	rerank := byID["rerank-v4-pro"]
	if has(rerank.Lists[ListFeatures], "function_calling") {
		t.Error("rerank-v4-pro: tool calling is stated as no")
	}
	if got := rerank.Limits[LimitContextWindow]; got != 0 {
		t.Errorf("rerank-v4-pro: got context %d, want none stated", got)
	}
}

// TestParsePartnerDeploymentsUndocumented pins the decision that a meter
// serving a documented model on a partner's deployment keeps the rate alone.
func TestParsePartnerDeploymentsUndocumented(t *testing.T) {
	m, ok := parse(t)["fw-kimi-k2.5"]
	if !ok {
		t.Fatal("fw-kimi-k2.5: not parsed")
	}
	if len(m.Prices) == 0 {
		t.Error("fw-kimi-k2.5: no price")
	}
	if len(m.Limits) != 0 || len(m.Lists) != 0 {
		t.Errorf("fw-kimi-k2.5: got %v and %v", m.Limits, m.Lists)
	}
}

// TestParseEmbeddings covers the embedding tables, which head one bound with
// the column the older chat tables state a pair under, and the alias that
// reaches the model a SKU abbreviates past recognition.
func TestParseEmbeddings(t *testing.T) {
	byID := parse(t)
	for _, c := range []struct {
		id        string
		context   int64
		dimension string
	}{
		{"text-embedding-3-small", 8192, "1536"},
		{"embedding-ada", 8192, "1536"},
	} {
		m, ok := byID[c.id]
		if !ok {
			t.Errorf("%s: not parsed", c.id)
			continue
		}
		if got := m.Limits[LimitContextWindow]; got != c.context {
			t.Errorf("%s: got context %d, want %d", c.id, got, c.context)
		}
		if !has(m.Lists[ListDimensions], c.dimension) {
			t.Errorf("%s: got dimensions %v", c.id, m.Lists[ListDimensions])
		}
	}
}

// TestParseMeters covers the reading of a SKU: what is billed, at what
// denominator, on which deployment, and which model it belongs to.
func TestParseMeters(t *testing.T) {
	byID := parse(t)
	m, ok := byID["gpt-5-mini"]
	if !ok {
		t.Fatal("gpt-5-mini: not parsed")
	}
	want := map[catalog.Metric]float64{
		"input_tokens":  0.25,
		"output_tokens": 1,
	}
	for metric, amount := range want {
		found := false
		for _, p := range m.Prices {
			if p.Metric == metric && p.Amount == amount {
				found = true
			}
		}
		if !found {
			t.Errorf("gpt-5-mini: no %s at %v in %v", metric, amount, m.Prices)
		}
	}
	if got := m.Limits[LimitContextWindow]; got == 0 {
		t.Error("gpt-5-mini: no context window")
	}
}

// TestParseSKUWindow covers the window Azure states only inside a meter's own
// name, for a model it has stopped documenting.
func TestParseSKUWindow(t *testing.T) {
	m, ok := parse(t)["gpt-4-32k"]
	if !ok {
		t.Fatal("gpt-4-32k: not parsed")
	}
	if got := m.Limits[LimitContextWindow]; got != 32000 {
		t.Errorf("got context %d, want 32000", got)
	}
}

// has reports whether a list contains a value.
func has(values []string, want string) bool {
	return slices.Contains(values, want)
}
