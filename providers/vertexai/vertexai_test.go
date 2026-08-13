package vertexai

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// e5PageURL is the page the fixture was taken from. A model page reaches a
// model through its identifier rather than through its URL, but the URL is
// recorded as the source, so the test uses the real one.
const e5PageURL = modelPagePre + "maas/e5/multilingual-e5-large"

// fixtures returns the documents to parse, read from disk so the test never
// touches the network and needs no Google credential. The billing catalog is
// reduced to one SKU, since what it contributes to a model is a rate and the
// shape of that JSON is the same for all 60 of them. The model page keeps only
// its specification table, which is the only part read.
func fixtures(t *testing.T) []catalog.Document {
	t.Helper()
	docs := []catalog.Document{}
	for _, f := range []struct{ url, file string }{
		{catalogURL, "catalog.json"},
		{e5PageURL, "model-e5.html"},
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

// TestParseSKUReachesModel covers the join the whole provider rests on: a SKU
// names a model in prose, and the model page names the identifier the API
// answers to.
func TestParseSKUReachesModel(t *testing.T) {
	byID := parse(t)
	m, ok := byID["multilingual-e5-large-instruct"]
	if !ok {
		t.Fatalf("not parsed, got %v", ids(byID))
	}
	if len(m.Prices) != 1 {
		t.Fatalf("got %d prices, want 1", len(m.Prices))
	}
	p := m.Prices[0]
	if p.Metric != MetricInputTokens || p.Unit != UnitPer1MTokens {
		t.Errorf("got %s %s", p.Metric, p.Unit)
	}
	if p.Amount != 0.025 {
		t.Errorf("got amount %v, want 0.025", p.Amount)
	}
	if !slices.Contains(m.Sources, e5PageURL) {
		t.Errorf("got sources %v", m.Sources)
	}
}

// TestParseEmbeddingSpecs covers the two rows an embedding model's page states
// under headings of its own. Its input bound is called a maximum sequence
// length rather than a context window, and its vector width is stated as a
// ceiling, "Up to 1,024", which is the only place Vertex publishes one.
func TestParseEmbeddingSpecs(t *testing.T) {
	m := parse(t)["multilingual-e5-large-instruct"]
	if got := m.Limits[LimitContextWindow]; got != 512 {
		t.Errorf("got context window %d, want 512", got)
	}
	if got := m.Lists[ListDimensions]; !slices.Equal(got, []string{"1024"}) {
		t.Errorf("got widths %q, want [1024]", got)
	}
	if got := m.Limits[LimitMaxOutputTokens]; got != 0 {
		t.Errorf("got max output %d, want none for an embedding model", got)
	}
}

// TestParseModalityDirections covers the modality table, which names every
// modality it knows of and says of each whether it flows in, out or neither. An
// embedding page marks "Embeddings" as its output, which is read as the text the
// model works in, so both sides come back set.
func TestParseModalityDirections(t *testing.T) {
	m := parse(t)["multilingual-e5-large-instruct"]
	if got := m.Lists[ListInputModalities]; !slices.Equal(
		got,
		[]string{"text"},
	) {
		t.Errorf("got input %q, want [text]", got)
	}
	if got := m.Lists[ListOutputModalities]; !slices.Equal(
		got,
		[]string{"text"},
	) {
		t.Errorf("got output %q, want [text]", got)
	}
}

// TestParseUnsupportedNotRecorded covers the rule that makes the pages readable
// at all: they list what a model cannot do as plainly as what it can, so only
// the supported entries may be kept.
func TestParseUnsupportedNotRecorded(t *testing.T) {
	m := parse(t)["multilingual-e5-large-instruct"]
	for _, name := range []string{"image", "audio", "video"} {
		if slices.Contains(m.Lists[ListInputModalities], name) {
			t.Errorf("recorded %q, which the page marks unsupported", name)
		}
	}
	for _, feature := range m.Lists[ListFeatures] {
		if feature == "provisioned_throughput" || feature == "batch_inference" {
			t.Errorf("recorded %q, which is a way to buy the model", feature)
		}
	}
}

// ids returns the identifiers of a model index, for a failure message.
func ids(byID map[string]catalog.Model) []string {
	out := make([]string, 0, len(byID))
	for id := range byID {
		out = append(out, id)
	}
	slices.Sort(out)
	return out
}
