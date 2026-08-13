package google

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// modelPage is the page of the one model whose own page this test reads.
const modelPage = baseURL + "/gemini-api/docs/models/gemini-embedding-2"

// parse runs the parser over the fixtures, read from disk so the test never
// touches the network.
func parse(t *testing.T) map[string]catalog.Model {
	t.Helper()
	files := []struct {
		url  string
		file string
	}{
		{PricingURL, "pricing.html"},
		{modelPage, "gemini-embedding-2.html"},
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

// amountOf returns one rate of a model, identified by everything but its
// amount.
func amountOf(
	m catalog.Model,
	metric catalog.Metric,
	dims catalog.Dims,
) (float64, bool) {
	for _, p := range m.Prices {
		if p.Metric == metric && p.Dims.Key() == dims.Key() {
			return p.Amount, true
		}
	}
	return 0, false
}

// TestParseFreeTier covers the plan Google states a rate for in a column whose
// heading gives no denominator, which is the only rate its open models have.
func TestParseFreeTier(t *testing.T) {
	m, ok := parse(t)["gemma-4"]
	if !ok {
		t.Fatal("gemma-4: not parsed")
	}
	free := catalog.Dims{DimPlan: PlanFree}
	for _, metric := range []catalog.Metric{
		MetricInputTokens,
		MetricOutputTokens,
	} {
		amount, ok := amountOf(m, metric, free)
		if !ok {
			t.Errorf("no free %s rate in %v", metric, m.Prices)
			continue
		}
		if amount != 0 {
			t.Errorf("got %s %v, want 0", metric, amount)
		}
	}
}

// TestParsePaidTier covers a rate that carries both a plan and a tier.
func TestParsePaidTier(t *testing.T) {
	byID := parse(t)
	m, ok := byID["gemini-3-flash-preview"]
	if !ok {
		t.Fatal("gemini-3-flash: not parsed")
	}
	found := false
	for _, p := range m.Prices {
		if p.Dims[DimPlan] == PlanPaid && p.Amount > 0 {
			found = true
		}
	}
	if !found {
		t.Errorf("no paid rate in %v", m.Prices)
	}
}

// TestParseEmbeddingDimensions covers the widths a model page names, which it
// states beside a range that is not a list of them.
func TestParseEmbeddingDimensions(t *testing.T) {
	m, ok := parse(t)["gemini-embedding-2"]
	if !ok {
		t.Fatal("gemini-embedding-2: not parsed")
	}
	got := slices.Clone(m.Lists[ListDimensions])
	slices.Sort(got)
	want := []string{"1536", "3072", "768"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if m.Limits[LimitContextWindow] != 8192 {
		t.Errorf("got context %d", m.Limits[LimitContextWindow])
	}
	if len(m.Lists[ListInputModalities]) < 2 {
		t.Errorf("got input modalities %v", m.Lists[ListInputModalities])
	}
	if out := m.Lists[ListOutputModalities]; !slices.Equal(
		out,
		[]string{"text"},
	) {
		t.Errorf("got output modalities %v, want [text]", out)
	}
}

// TestParseProseModalities covers the wordings Google states a modality list
// in, where "Text embeddings" is the text an embedding model works in and the
// "with" of "Video with audio" names a second modality rather than a quality of
// the first.
func TestParseProseModalities(t *testing.T) {
	for _, c := range []struct {
		value string
		want  []string
	}{
		{"Text embeddings", []string{"text"}},
		{"Video with audio", []string{"video", "audio"}},
		{"Text, Image, Video, Audio, and PDF", []string{
			"text",
			"image",
			"video",
			"audio",
			"file",
		}},
		{"Audio (translated speech) and Text", []string{"audio", "text"}},
	} {
		m := &catalog.Model{}
		addModalities(m, ListOutputModalities, c.value)
		if got := m.Lists[ListOutputModalities]; !slices.Equal(got, c.want) {
			t.Errorf("%q: got %v, want %v", c.value, got, c.want)
		}
	}
}
