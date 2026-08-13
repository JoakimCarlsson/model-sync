package perplexity

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
	files := []struct {
		url  string
		file string
	}{
		{baseURL + "/getting-started/pricing.md", "pricing.md"},
		{baseURL + "/docs/agent-api/models.md", "agent-models.md"},
		{SonarIndexURL, "sonar-models.md"},
		{FeaturesURL, "features.md"},
		{MediaURL, "media.md"},
		{sonarModelPre + "sonar.md", "sonar.md"},
		{sonarModelPre + "sonar-pro.md", "sonar-pro.md"},
		{sonarModelPre + "sonar-reasoning-pro.md", "sonar-reasoning-pro.md"},
		{sonarModelPre + "sonar-deep-research.md", "sonar-deep-research.md"},
	}
	docs := make([]catalog.Document, 0, len(files))
	for _, f := range files {
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

// TestParseSonarCapabilities covers what the Sonar guides state for the API and
// what a model page states for itself.
func TestParseSonarCapabilities(t *testing.T) {
	byID := parse(t)
	own := []string{
		"sonar",
		"sonar-pro",
		"sonar-reasoning-pro",
		"sonar-deep-research",
	}
	for _, id := range own {
		m, ok := byID[id]
		if !ok {
			t.Errorf("%s: not parsed", id)
			continue
		}
		for _, feature := range []string{"streaming", "structured_outputs"} {
			if !slices.Contains(m.Lists[ListFeatures], feature) {
				t.Errorf(
					"%s: missing %q in %v",
					id,
					feature,
					m.Lists[ListFeatures],
				)
			}
		}
		wantIn := []string{"file", "image", "text"}
		if got := m.Lists[ListInputModalities]; !equal(got, wantIn) {
			t.Errorf("%s: got input modalities %v, want %v", id, got, wantIn)
		}
		if got := m.Lists[ListOutputModalities]; !equal(got, []string{"text"}) {
			t.Errorf("%s: got output modalities %v", id, got)
		}
		if got := m.Limits[LimitContextWindow]; got == 0 {
			t.Errorf("%s: no context window", id)
		}
	}
}

// TestParseReasoning covers the one capability a model page states for itself,
// which it states either way round.
func TestParseReasoning(t *testing.T) {
	byID := parse(t)
	reasons := map[string]bool{
		"sonar":               false,
		"sonar-pro":           false,
		"sonar-reasoning-pro": true,
		"sonar-deep-research": true,
	}
	for id, want := range reasons {
		got := slices.Contains(byID[id].Lists[ListFeatures], featureReasoning)
		if got != want {
			t.Errorf("%s: reasoning %v, want %v", id, got, want)
		}
	}
}

func TestParseContextWindows(t *testing.T) {
	byID := parse(t)
	windows := map[string]int64{
		"sonar":               128000,
		"sonar-pro":           200000,
		"sonar-reasoning-pro": 128000,
		"sonar-deep-research": 128000,
	}
	for id, want := range windows {
		if got := byID[id].Limits[LimitContextWindow]; got != want {
			t.Errorf("%s: got context %d, want %d", id, got, want)
		}
	}
}

// TestParseBrokeredModels pins that a model Perplexity brokers carries its
// rates and nothing else, because that is all Perplexity states about it.
func TestParseBrokeredModels(t *testing.T) {
	byID := parse(t)
	for _, id := range []string{
		"anthropic/claude-opus-5",
		"openai/gpt-5.6-sol",
		"xai/grok-4.6",
	} {
		m, ok := byID[id]
		if !ok {
			t.Errorf("%s: not parsed", id)
			continue
		}
		if len(m.Prices) == 0 {
			t.Errorf("%s: no prices", id)
		}
		if len(m.Limits) != 0 || len(m.Lists) != 0 {
			t.Errorf("%s: got %v and %v", id, m.Limits, m.Lists)
		}
	}
}

// TestParseEmbeddings covers the one kind whose width Perplexity publishes
// beside the rate.
func TestParseEmbeddings(t *testing.T) {
	m, ok := parse(t)["pplx-embed-v1-4b"]
	if !ok {
		t.Fatal("pplx-embed-v1-4b: not parsed")
	}
	if !slices.Contains(m.Lists[ListDimensions], "2560") {
		t.Errorf("got dimensions %v", m.Lists[ListDimensions])
	}
	if len(m.Prices) == 0 {
		t.Error("no prices")
	}
}

// equal reports whether two sorted lists hold the same values.
func equal(got, want []string) bool {
	sorted := slices.Clone(got)
	slices.Sort(sorted)
	return slices.Equal(sorted, want)
}
