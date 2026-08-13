package voyage

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// parse runs the parser over the documents, read from disk so the test never
// touches the network.
func parse(t *testing.T) map[string]catalog.Model {
	t.Helper()
	docs := make([]catalog.Document, 0, len(documentURLs))
	for _, url := range documentURLs {
		name := filepath.Base(url)
		body, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		docs = append(docs, catalog.Document{URL: url, Body: body})
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

// TestParseModalities covers the one thing Voyage's pages say about modality,
// which is which page a model is documented on.
func TestParseModalities(t *testing.T) {
	byID := parse(t)
	cases := map[string][]string{
		"voyage-4":              {"text"},
		"voyage-multimodal-3.5": {"image", "text"},
		"rerank-2.5":            {"text"},
		"voyage-context-4":      {"text"},
	}
	for id, want := range cases {
		m, ok := byID[id]
		if !ok {
			t.Errorf("%s: not parsed", id)
			continue
		}
		got := slices.Clone(m.Lists[ListInputModalities])
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("%s: got %v, want %v", id, got, want)
		}
		if out := m.Lists[ListOutputModalities]; !slices.Equal(
			out,
			[]string{"text"},
		) {
			t.Errorf("%s: got output %v, want [text]", id, out)
		}
	}
}

// TestParseModalitiesUnstated covers a model that survives only in the rate
// tables, which carries neither side rather than an output on its own.
func TestParseModalitiesUnstated(t *testing.T) {
	m, ok := parse(t)["voyage-code-2"]
	if !ok {
		t.Skip("voyage-code-2: not in the fixtures")
	}
	if len(m.Lists[ListInputModalities]) != 0 ||
		len(m.Lists[ListOutputModalities]) != 0 {
		t.Errorf("voyage-code-2: got %v", m.Lists)
	}
}

// TestParseOpenWeightModel covers the model Voyage documents without selling,
// which is why it carries no rate.
func TestParseOpenWeightModel(t *testing.T) {
	m, ok := parse(t)["voyage-4-nano"]
	if !ok {
		t.Fatal("voyage-4-nano: not parsed")
	}
	if m.Attrs[AttrOpenWeights] != "true" {
		t.Errorf("got attrs %v", m.Attrs)
	}
	if len(m.Prices) != 0 {
		t.Errorf("got %d prices, want none", len(m.Prices))
	}
	if len(m.Notes) != 1 {
		t.Errorf("got notes %v, want one saying why there is no rate", m.Notes)
	}
}

// TestParsePricing covers a rate stated twice per row, against two
// denominators that do not always agree.
func TestParsePricing(t *testing.T) {
	m, ok := parse(t)["voyage-4"]
	if !ok {
		t.Fatal("voyage-4: not parsed")
	}
	units := map[catalog.Unit]bool{}
	for _, p := range m.Prices {
		units[p.Unit] = true
	}
	for _, unit := range []catalog.Unit{UnitPer1KTokens, UnitPer1MTokens} {
		if !units[unit] {
			t.Errorf("no rate per %s in %v", unit, m.Prices)
		}
	}
	if m.Limits[LimitContextWindow] == 0 {
		t.Error("no context window")
	}
	if len(m.Lists[ListDimensions]) == 0 {
		t.Error("no embedding dimensions")
	}
}
