package together

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// parse runs the parser over the catalog page, read from disk so the test
// never touches the network.
func parse(t *testing.T) map[string]catalog.Model {
	t.Helper()
	body, err := os.ReadFile(
		filepath.Join("testdata", "serverless-models.md"),
	)
	if err != nil {
		t.Fatal(err)
	}
	models, err := New().Parse([]catalog.Document{
		{URL: CatalogURL, Body: body},
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

// TestParseModalities covers the one thing the tables say about modality,
// which is which table a model is listed in.
func TestParseModalities(t *testing.T) {
	byID := parse(t)
	cases := []struct {
		id  string
		in  []string
		out []string
	}{
		{"thinkingmachines/Inkling", []string{"text"}, []string{"text"}},
		{
			"Qwen/Qwen3.5-9B",
			[]string{"image", "text"},
			[]string{"text"},
		},
		{"black-forest-labs/FLUX.2-max", []string{"text"}, []string{"image"}},
		{"cartesia/sonic", []string{"text"}, []string{"audio"}},
	}
	for _, c := range cases {
		m, ok := byID[c.id]
		if !ok {
			t.Errorf("%s: not parsed", c.id)
			continue
		}
		if got := sorted(m.Lists[ListInputModalities]); !slices.Equal(
			got,
			c.in,
		) {
			t.Errorf("%s: got input %v, want %v", c.id, got, c.in)
		}
		if got := sorted(m.Lists[ListOutputModalities]); !slices.Equal(
			got,
			c.out,
		) {
			t.Errorf("%s: got output %v, want %v", c.id, got, c.out)
		}
	}
}

// sorted returns a list in order, since a parser is free to produce one in
// any order and the store sorts on the way out.
func sorted(values []string) []string {
	out := slices.Clone(values)
	slices.Sort(out)
	return out
}

// TestParseVisionMerges covers a model listed in two tables, which is one
// model that takes images rather than two models.
func TestParseVisionMerges(t *testing.T) {
	m, ok := parse(t)["MiniMaxAI/MiniMax-M3"]
	if !ok {
		t.Fatal("MiniMaxAI/MiniMax-M3: not parsed")
	}
	if m.Kind != KindChat {
		t.Errorf("got kind %q, want chat", m.Kind)
	}
	if !slices.Contains(m.Lists[ListInputModalities], ModalityImage) {
		t.Errorf("got input %v", m.Lists[ListInputModalities])
	}
	if got := m.Limits[LimitContextWindow]; got != 524288 {
		t.Errorf("got context %d", got)
	}
}

// TestParseChatRow covers the rates and capabilities a chat row states.
func TestParseChatRow(t *testing.T) {
	m, ok := parse(t)["thinkingmachines/Inkling"]
	if !ok {
		t.Fatal("thinkingmachines/Inkling: not parsed")
	}
	want := map[catalog.Metric]float64{
		MetricInputTokens:       1,
		MetricCachedInputTokens: 0.17,
		MetricOutputTokens:      4.05,
	}
	for metric, amount := range want {
		found := false
		for _, p := range m.Prices {
			if p.Metric == metric && p.Amount == amount &&
				p.Unit == UnitPer1MTokens {
				found = true
			}
		}
		if !found {
			t.Errorf("no %s at %v in %v", metric, amount, m.Prices)
		}
	}
	for _, feature := range []string{
		FeatureFunctionCalling,
		FeatureStructuredOutputs,
	} {
		if !slices.Contains(m.Lists[ListFeatures], feature) {
			t.Errorf("missing %q in %v", feature, m.Lists[ListFeatures])
		}
	}
}

// TestParseNoOutputBound pins that Together states no output ceiling for any
// model, so that one appearing later is noticed rather than assumed.
func TestParseNoOutputBound(t *testing.T) {
	for id, m := range parse(t) {
		for key := range m.Limits {
			if key != LimitContextWindow {
				t.Errorf("%s: unexpected limit %q", id, key)
			}
		}
	}
}
