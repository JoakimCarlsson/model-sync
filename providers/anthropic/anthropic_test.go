package anthropic

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
		{DeprecationsURL, "deprecations.md"},
		{OverviewURL, "overview.md"},
		{PricingURL, "pricing.md"},
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

// TestParseModalities covers the sentence that is the only place Anthropic
// states what a model takes and returns.
func TestParseModalities(t *testing.T) {
	byID := parse(t)
	for id, m := range byID {
		switch {
		case m.Kind != KindChat, m.Attrs[AttrState] == stateRetired:
			if len(m.Lists[ListInputModalities]) != 0 {
				t.Errorf("%s: got modalities on a %s model", id, m.Kind)
			}
		default:
			in := sorted(m.Lists[ListInputModalities])
			if !slices.Equal(in, []string{"image", "text"}) {
				t.Errorf("%s: got input %v", id, in)
			}
			out := m.Lists[ListOutputModalities]
			if !slices.Equal(out, []string{"text"}) {
				t.Errorf("%s: got output %v", id, out)
			}
		}
	}
}

// TestParseModalitiesReachPricedOnlyModel covers the model the overview
// describes in prose and only the pricing page names, which is why the
// sentence is applied after every document has been read.
func TestParseModalitiesReachPricedOnlyModel(t *testing.T) {
	m, ok := parse(t)["claude-mythos-5"]
	if !ok {
		t.Fatal("claude-mythos-5: not parsed")
	}
	if len(m.Lists[ListInputModalities]) == 0 {
		t.Error("no input modalities")
	}
	if len(m.Prices) == 0 {
		t.Error("no prices")
	}
}

// TestParseOverview covers the transposed comparison table.
func TestParseOverview(t *testing.T) {
	byID := parse(t)
	m, ok := byID["claude-opus-5"]
	if !ok {
		t.Fatal("claude-opus-5: not parsed")
	}
	if m.Name == "" {
		t.Error("no display name")
	}
	if got := m.Limits[LimitContextWindow]; got == 0 {
		t.Error("no context window")
	}
	if got := m.Limits[LimitMaxOutputTokens]; got == 0 {
		t.Error("no output ceiling")
	}
	if !slices.Contains(m.Lists[ListFeatures], FeatureReasoning) {
		t.Errorf("got features %v", m.Lists[ListFeatures])
	}
}

// TestParseRetired covers the deprecations page, which is the only document
// saying a model no longer serves.
func TestParseRetired(t *testing.T) {
	m, ok := parse(t)["claude-3-opus-20240229"]
	if !ok {
		t.Fatal("claude-3-opus-20240229: not parsed")
	}
	if m.Attrs[AttrState] != stateRetired {
		t.Errorf("got state %q, want retired", m.Attrs[AttrState])
	}
}

// sorted returns a list in order, since a parser is free to produce one in any
// order and the store sorts on the way out.
func sorted(values []string) []string {
	out := slices.Clone(values)
	slices.Sort(out)
	return out
}
