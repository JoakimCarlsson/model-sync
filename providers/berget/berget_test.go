package berget

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// fixtures returns the documents to parse, read from disk so the test never
// touches the network. Both are trimmed to the models that cover every shape
// the parser tells apart: one of each type it maps, a model with the vision
// capability, and a model under evaluation.
func fixtures(t *testing.T) []catalog.Document {
	t.Helper()
	docs := []catalog.Document{}
	for _, f := range []struct{ url, file string }{
		{ModelsURL, "models.json"},
		{OverviewURL, "overview.html"},
	} {
		body, err := os.ReadFile(filepath.Join("testdata", f.file))
		if err != nil {
			t.Fatal(err)
		}
		docs = append(docs, catalog.Document{URL: f.url, Body: body})
	}
	return docs
}

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

// priced reports the amount of one rate, identified by everything but its
// amount.
func priced(m catalog.Model, want catalog.Price) (float64, bool) {
	for _, p := range m.Prices {
		if p.Metric == want.Metric && p.Unit == want.Unit {
			return p.Amount, true
		}
	}
	return 0, false
}

// TestParseCurrency covers the thing that makes Berget unlike every other
// source here: it quotes euros, so a rate that lost its currency would read as
// dollars and understate the cost.
func TestParseCurrency(t *testing.T) {
	m := parse(t)["openai/gpt-oss-120b"]
	if len(m.Prices) == 0 {
		t.Fatal("no prices")
	}
	for _, p := range m.Prices {
		if p.Currency != defaultCurrency {
			t.Errorf(
				"got currency %q, want %q",
				p.Currency,
				defaultCurrency,
			)
		}
	}
	for _, c := range []struct {
		metric catalog.Metric
		amount float64
	}{
		{MetricInputTokens, 0.2},
		{MetricOutputTokens, 0.75},
	} {
		got, ok := priced(
			m,
			catalog.Price{Metric: c.metric, Unit: UnitPer1MTokens},
		)
		if !ok {
			t.Errorf("no %s rate", c.metric)
			continue
		}
		if got != c.amount {
			t.Errorf("got %s %v, want %v", c.metric, got, c.amount)
		}
	}
}

// TestParseModalitiesFromType covers the rule that neither document states a
// modality: what a model takes and returns comes from its type, and an
// embedding or reranking model records only what it takes because a vector and
// a relevance score are not modalities.
func TestParseModalitiesFromType(t *testing.T) {
	byID := parse(t)
	cases := []struct {
		id  string
		in  []string
		out []string
	}{
		{"openai/gpt-oss-120b", []string{"text"}, []string{"text"}},
		{"intfloat/multilingual-e5-large", []string{"text"}, nil},
		{"BAAI/bge-reranker-v2-m3", []string{"text"}, nil},
		{"KBLab/kb-whisper-large", []string{"audio"}, []string{"text"}},
		{"moonshotai/Kimi-K2.6", []string{"image", "text"}, []string{"text"}},
	}
	for _, c := range cases {
		m := byID[c.id]
		m.Sort()
		if got := m.Lists[ListInputModalities]; !slices.Equal(got, c.in) {
			t.Errorf("%s: got input %q, want %q", c.id, got, c.in)
		}
		if got := m.Lists[ListOutputModalities]; !slices.Equal(got, c.out) {
			t.Errorf("%s: got output %q, want %q", c.id, got, c.out)
		}
	}
}

// TestParseVisionIsNotAFeature covers the one capability naming a modality
// rather than an API feature.
func TestParseVisionIsNotAFeature(t *testing.T) {
	m := parse(t)["moonshotai/Kimi-K2.6"]
	if slices.Contains(m.Lists["features"], FeatureVision) {
		t.Errorf("vision recorded as a feature: %q", m.Lists["features"])
	}
	if !slices.Contains(m.Lists[ListInputModalities], "image") {
		t.Errorf("got input %q", m.Lists[ListInputModalities])
	}
}

// TestParseContextFromOverview covers the bound the endpoint omits and the
// documentation site states, keyed by the same identifier, including the two
// ways the cards write a quantity.
func TestParseContextFromOverview(t *testing.T) {
	byID := parse(t)
	cases := []struct {
		id      string
		context int64
	}{
		{"openai/gpt-oss-120b", 128000},
		{"moonshotai/Kimi-K2.6", 256000},
		{"intfloat/multilingual-e5-large", 512},
		{"BAAI/bge-reranker-v2-m3", 8192},
	}
	for _, c := range cases {
		m := byID[c.id]
		if got := m.Limits[LimitContextWindow]; got != c.context {
			t.Errorf("%s: got context %d, want %d", c.id, got, c.context)
		}
		if !slices.Contains(m.Sources, OverviewURL) {
			t.Errorf("%s: got sources %q", c.id, m.Sources)
		}
	}
	if got := byID["KBLab/kb-whisper-large"].Limits[LimitContextWindow]; got !=
		0 {
		t.Errorf("a model with no card gained a context window of %d", got)
	}
}

// TestParseVolatileFieldsIgnored covers the two fields deliberately dropped.
// They report live health and latency, and recording them would rewrite every
// file on every sync and bury the changes that matter.
func TestParseVolatileFieldsIgnored(t *testing.T) {
	for id, m := range parse(t) {
		for _, key := range []string{"up", "latency", "status"} {
			if _, ok := m.Attrs[key]; ok {
				t.Errorf("%s: recorded volatile field %q", id, key)
			}
		}
	}
}

// TestParseNoOutputBoundOrWidth pins what Berget does not publish, so that
// either appearing later reads as the endpoint having gained a field.
func TestParseNoOutputBoundOrWidth(t *testing.T) {
	for id, m := range parse(t) {
		if got := m.Limits["max_output_tokens"]; got != 0 {
			t.Errorf("%s: got max output %d, want none published", id, got)
		}
		if got := m.Lists["embedding_dimensions"]; len(got) != 0 {
			t.Errorf("%s: got widths %q, want none published", id, got)
		}
	}
}

// TestParseLifecycle covers the standing the endpoint states, which is what
// keeps a model under evaluation out of a reader's count of what is stable.
func TestParseLifecycle(t *testing.T) {
	byID := parse(t)
	if got := byID["moonshotai/Kimi-K3"].Attrs[AttrState]; got != "eval" {
		t.Errorf("got state %q, want eval", got)
	}
	if got := byID["openai/gpt-oss-120b"].Attrs[AttrState]; got != "stable" {
		t.Errorf("got state %q, want stable", got)
	}
}
