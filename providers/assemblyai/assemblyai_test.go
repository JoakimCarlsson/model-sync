package assemblyai

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
		{PricingURL, "pricing.html"},
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

// TestParseAddOnRate covers the add-on the models page prices only by pointing
// at the pricing page, where it is quoted once per model it runs with.
func TestParseAddOnRate(t *testing.T) {
	m, ok := parse(t)["medical-mode"]
	if !ok {
		t.Fatal("medical-mode: not parsed")
	}
	if len(m.Prices) == 0 {
		t.Fatal("no prices")
	}
	for _, p := range m.Prices {
		if p.Amount != 0.15 {
			t.Errorf("got %v, want 0.15", p.Amount)
		}
		if p.Unit != UnitPerHour {
			t.Errorf("got unit %q", p.Unit)
		}
		if p.Dims[DimMode] == "" {
			t.Errorf("got dims %v", p.Dims)
		}
	}
}

// TestParseMetricsDiffer covers the distinction the two rate tables draw: an
// hour of audio submitted is not an hour of connection held open.
func TestParseMetricsDiffer(t *testing.T) {
	byID := parse(t)
	cases := map[string]catalog.Metric{
		"universal-3.5-pro":           MetricAudio,
		"universal-3.5-pro-streaming": MetricSession,
	}
	for id, metric := range cases {
		m, ok := byID[id]
		if !ok {
			t.Errorf("%s: not parsed", id)
			continue
		}
		if len(m.Prices) == 0 {
			t.Errorf("%s: no prices", id)
			continue
		}
		if got := m.Prices[0].Metric; got != metric {
			t.Errorf("%s: got metric %q, want %q", id, got, metric)
		}
	}
}

// TestParseModalities covers what every AssemblyAI model does, which is the
// one thing its whole catalog has in common.
func TestParseModalities(t *testing.T) {
	for id, m := range parse(t) {
		if !slices.Equal(m.Lists[ListInputModalities], []string{"audio"}) {
			t.Errorf("%s: got input %v", id, m.Lists[ListInputModalities])
		}
		if !slices.Equal(m.Lists[ListOutputModalities], []string{"text"}) {
			t.Errorf("%s: got output %v", id, m.Lists[ListOutputModalities])
		}
	}
}

// TestParseCards covers the MDX cards, which are where the capabilities are.
func TestParseCards(t *testing.T) {
	m, ok := parse(t)["universal-3.5-pro"]
	if !ok {
		t.Fatal("universal-3.5-pro: not parsed")
	}
	if m.Name != "Universal-3.5 Pro" {
		t.Errorf("got name %q", m.Name)
	}
	if len(m.Lists[ListCapabilities]) == 0 {
		t.Error("no capabilities")
	}
	if m.Attrs[AttrMode] != ModePrerecorded {
		t.Errorf("got mode %q", m.Attrs[AttrMode])
	}
}
