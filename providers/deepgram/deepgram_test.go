package deepgram

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// parse runs the parser over the pricing page, read from disk so the test
// never touches the network.
func parse(t *testing.T) map[string]catalog.Model {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", "pricing.html"))
	if err != nil {
		t.Fatal(err)
	}
	models, err := New().Parse([]catalog.Document{
		{URL: PricingURL, Body: body},
	})
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]catalog.Model, len(models))
	for _, m := range models {
		byID[m.ID] = m
	}
	return byID
}

// TestParseModalities covers what each product's models take and return, which
// is what the heading a table sits under says.
func TestParseModalities(t *testing.T) {
	byID := parse(t)
	cases := map[string][2][]string{
		"nova-3-monolingual": {{"audio"}, {"text"}},
		"aura-2":             {{"text"}, {"audio"}},
		"standard":           {{"audio", "text"}, {"audio", "text"}},
		"summarization":      {{"audio"}, {"text"}},
	}
	for id, want := range cases {
		m, ok := byID[id]
		if !ok {
			t.Errorf("%s: not parsed", id)
			continue
		}
		if got := m.Lists[ListInputModalities]; !slices.Equal(got, want[0]) {
			t.Errorf("%s: got input %v, want %v", id, got, want[0])
		}
		if got := m.Lists[ListOutputModalities]; !slices.Equal(got, want[1]) {
			t.Errorf("%s: got output %v, want %v", id, got, want[1])
		}
	}
}

// TestParseIncluded covers the add-on Deepgram charges nothing extra for,
// which it writes as a word rather than as an amount.
func TestParseIncluded(t *testing.T) {
	m, ok := parse(t)["smart-formatting"]
	if !ok {
		t.Fatal("smart-formatting: not parsed")
	}
	if m.Attrs[AttrIncluded] != "true" {
		t.Errorf("got attrs %v", m.Attrs)
	}
	if len(m.Prices) == 0 {
		t.Fatal("no prices")
	}
	for _, p := range m.Prices {
		if p.Amount != 0 || p.Note != noteIncluded {
			t.Errorf("got %v at %v with note %q", p.Metric, p.Amount, p.Note)
		}
	}
}

// TestParseContactSales covers the model Deepgram will only price in a
// conversation, which carries no rate for that reason.
func TestParseContactSales(t *testing.T) {
	m, ok := parse(t)["custom"]
	if !ok {
		t.Fatal("custom: not parsed")
	}
	if m.Attrs[AttrAccess] != contactSales {
		t.Errorf("got attrs %v", m.Attrs)
	}
	if len(m.Prices) != 0 {
		t.Errorf("got %d prices, want none", len(m.Prices))
	}
}

// TestParseRates covers the two plan columns and the promotional rate written
// in the same cell as the one that follows it.
func TestParseRates(t *testing.T) {
	m, ok := parse(t)["nova-3-monolingual"]
	if !ok {
		t.Fatal("nova-3-monolingual: not parsed")
	}
	plans := map[string]bool{}
	for _, p := range m.Prices {
		if p.Metric != MetricAudio {
			t.Errorf("got metric %q", p.Metric)
		}
		plans[p.Dims[DimPlan]] = true
	}
	if len(plans) < 2 {
		t.Errorf("got rates for %v, want one per plan", plans)
	}
}
