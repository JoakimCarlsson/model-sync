package ollama

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// fixtures returns the documents to parse, read from disk so the test never
// touches the network. The library page is trimmed to the entries that cover
// every shape the parser tells apart, because the live page carries all 233
// models and their markup is the bulk of it.
func fixtures(t *testing.T) []catalog.Document {
	t.Helper()
	docs := []catalog.Document{}
	for _, f := range []struct{ url, file string }{
		{LibraryURL, "library.html"},
		{
			LibraryURL + "/nomic-embed-text" + tagsPath,
			"tags-nomic-embed-text.html",
		},
		{LibraryURL + "/qwen3" + tagsPath, "tags-qwen3.html"},
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

// TestParseSizesAndCapabilities covers the rule the library forces: capabilities
// and parameter sizes are tags in one list, told apart by shape.
func TestParseSizesAndCapabilities(t *testing.T) {
	byID := parse(t)
	cases := []struct {
		id       string
		sizes    []string
		features []string
	}{
		{
			"qwen3",
			[]string{
				"0.6b", "1.7b", "4b", "8b", "14b", "30b", "32b", "235b",
			},
			[]string{"function_calling", "reasoning"},
		},
		{"mixtral", []string{"8x7b", "8x22b"}, []string{"function_calling"}},
		{"gemma3n", []string{"e2b", "e4b"}, nil},
		{"nomic-embed-text", nil, nil},
	}
	for _, c := range cases {
		m := byID[c.id]
		got := slices.Clone(m.Lists[ListParameterSizes])
		want := slices.Clone(c.sizes)
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("%s: got sizes %q, want %q", c.id, got, want)
		}
		got = slices.Clone(m.Lists[ListFeatures])
		want = slices.Clone(c.features)
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("%s: got features %q, want %q", c.id, got, want)
		}
	}
}

// TestParseVisionIsAModality covers the two tags that name a modality rather
// than a capability: a model that reads images stays a chat model and records
// the modality, and vision never lands among the features.
func TestParseVisionIsAModality(t *testing.T) {
	m := parse(t)["llama4"]
	if m.Kind != KindChat {
		t.Errorf("got kind %q, want chat", m.Kind)
	}
	if !slices.Contains(m.Lists[ListInputModalities], "image") {
		t.Errorf("got input modalities %q", m.Lists[ListInputModalities])
	}
	if slices.Contains(m.Lists[ListFeatures], "vision") {
		t.Errorf("vision recorded as a feature: %q", m.Lists[ListFeatures])
	}
}

// TestParseEmbeddingKind covers the one capability that changes what a model is.
func TestParseEmbeddingKind(t *testing.T) {
	byID := parse(t)
	for _, id := range []string{
		"nomic-embed-text",
		"embeddinggemma",
		"granite-embedding",
	} {
		if got := byID[id].Kind; got != KindEmbedding {
			t.Errorf("%s: got kind %q, want embedding", id, got)
		}
	}
	if got := byID["qwen3"].Kind; got != KindChat {
		t.Errorf("qwen3: got kind %q, want chat", got)
	}
}

// TestParseTagListing covers the bound the library omits and the listing states
// once per build, and that it is read as written: Ollama says "2K", so the
// window is 2000 and not the 2048 of the model's own metadata.
func TestParseTagListing(t *testing.T) {
	byID := parse(t)
	cases := []struct {
		id      string
		context int64
	}{
		{"nomic-embed-text", 2000},
		{"qwen3", 40000},
	}
	for _, c := range cases {
		m := byID[c.id]
		if got := m.Limits[LimitContextWindow]; got != c.context {
			t.Errorf("%s: got context %d, want %d", c.id, got, c.context)
		}
		if !slices.Contains(m.Lists[ListInputModalities], "text") {
			t.Errorf("%s: got input %q", c.id, m.Lists[ListInputModalities])
		}
		if !slices.Contains(m.Sources, LibraryURL+"/"+c.id+tagsPath) {
			t.Errorf("%s: got sources %q", c.id, m.Sources)
		}
	}
	if got := byID["mixtral"].Limits[LimitContextWindow]; got != 0 {
		t.Errorf("mixtral: got context %d with no listing read", got)
	}
}

// TestParseNoPricesOrWidths pins what Ollama does not publish. It runs models on
// the reader's own machine, so a price appearing here would be wrong rather than
// welcome, and no page of its library states the width of an embedding vector.
func TestParseNoPricesOrWidths(t *testing.T) {
	for id, m := range parse(t) {
		if len(m.Prices) != 0 {
			t.Errorf("%s: got %d prices, want none", id, len(m.Prices))
		}
		if m.Name != "" {
			t.Errorf("%s: got name %q, want none published", id, m.Name)
		}
		if got := m.Limits["max_output_tokens"]; got != 0 {
			t.Errorf("%s: got max output %d, want none published", id, got)
		}
		if got := m.Lists["embedding_dimensions"]; len(got) != 0 {
			t.Errorf("%s: got widths %q, want none published", id, got)
		}
	}
}
