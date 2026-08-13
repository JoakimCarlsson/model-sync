package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// fixture is a catalog holding one of every case the report distinguishes: a
// model with everything, a model with nothing, and one withdrawn model under
// each of the two keys a lifecycle is recorded under.
func fixture() *catalog.Catalog {
	full := catalog.Model{
		ID:       "full",
		Provider: "acme",
		Name:     "Full",
		Kind:     "chat",
		Prices:   []catalog.Price{{Metric: "input_tokens", Amount: 1}},
		Limits: map[string]int64{
			limitContextWindow:   1000,
			limitMaxOutputTokens: 100,
		},
		Lists: map[string][]string{
			catalog.ListFeatures: {
				"streaming",
				catalog.CapabilityReasoning,
				catalog.CapabilityStructuredOutputs,
				catalog.CapabilityFunctionCalling,
			},
			listInputModalities:  {"text"},
			listOutputModalities: {"text"},
		},
	}
	bare := catalog.Model{ID: "bare", Provider: "acme", Kind: "chat"}
	gone := catalog.Model{
		ID:       "gone",
		Provider: "acme",
		Kind:     "chat",
		Attrs:    map[string]string{"state": "retired"},
	}
	shutdown := catalog.Model{
		ID:       "shutdown",
		Provider: "acme",
		Kind:     "chat",
		Attrs:    map[string]string{"lifecycle_state": "shutdown"},
	}
	legacy := catalog.Model{
		ID:       "legacy",
		Provider: "acme",
		Kind:     "chat",
		Attrs:    map[string]string{"lifecycle_state": "legacy"},
	}
	vectors := catalog.Model{
		ID:       "vectors",
		Provider: "acme",
		Kind:     "embedding",
		Lists:    map[string][]string{listDimensions: {"1024"}},
	}
	cat := &catalog.Catalog{}
	cat.Add("acme", "Acme", []catalog.Model{
		full, bare, gone, shutdown, legacy, vectors,
	})
	return cat
}

func TestLiveExcludesWithdrawnModels(t *testing.T) {
	cases := []struct {
		attrs map[string]string
		want  bool
	}{
		{nil, true},
		{map[string]string{"state": "active"}, true},
		{map[string]string{"state": "retired"}, false},
		{map[string]string{"state": "deprecated"}, false},
		{map[string]string{"state": "shutdown"}, false},
		{map[string]string{"lifecycle_state": "shutdown"}, false},
		{map[string]string{"lifecycle_state": "legacy"}, true},
		{map[string]string{"lifecycle_state": "legacy (some regions)"}, true},
		{map[string]string{"state": "Deprecated "}, false},
		{map[string]string{"state": "older"}, true},
	}
	for _, c := range cases {
		if got := live(catalog.Model{Attrs: c.attrs}); got != c.want {
			t.Errorf("live(%v) = %v, want %v", c.attrs, got, c.want)
		}
	}
}

func TestMeasureCountsLiveModelsByKind(t *testing.T) {
	rows := measure(fixture(), "", "", false)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want one per kind: %+v", len(rows), rows)
	}
	chat, embedding := rows[0], rows[1]
	if chat.kind != "chat" || embedding.kind != "embedding" {
		t.Fatalf("rows out of order: %q, %q", chat.kind, embedding.kind)
	}
	if chat.live != 3 {
		t.Errorf("live chat = %d, want 3 of 5", chat.live)
	}
	for _, c := range []struct {
		field string
		got   int
	}{
		{"priced", chat.priced},
		{"context", chat.context},
		{"maxOut", chat.maxOut},
		{"features", chat.features},
		{"reason", chat.reason},
		{"structured", chat.structured},
		{"tools", chat.tools},
		{"inMod", chat.inMod},
		{"outMod", chat.outMod},
		{"named", chat.named},
	} {
		if c.got != 1 {
			t.Errorf("chat %s = %d, want 1", c.field, c.got)
		}
	}
	if embedding.embed != 1 || embedding.dims != 1 {
		t.Errorf(
			"embedding dims = %d of %d, want 1 of 1",
			embedding.dims,
			embedding.embed,
		)
	}
	if chat.embed != 0 {
		t.Errorf("chat counted %d embedding models", chat.embed)
	}
}

// A model stating a capability in its vendor's words is not counted as
// stating it. The column measures how far the vocabulary has converged, so a
// synonym has to read as the gap it is.
func TestCountRejectsVendorSynonyms(t *testing.T) {
	got := count(catalog.Model{
		Kind: "chat",
		Lists: map[string][]string{
			catalog.ListFeatures: {"json_mode", "formatted_output", "tools"},
		},
	})
	if got.features != 1 {
		t.Errorf("features = %d, want the list counted as populated", got.features)
	}
	if got.structured != 0 || got.tools != 0 {
		t.Errorf(
			"structured = %d, tools = %d, want a synonym to count as neither",
			got.structured,
			got.tools,
		)
	}
}

func TestMeasureAllIncludesWithdrawnModels(t *testing.T) {
	rows := measure(fixture(), "chat", "", true)
	if len(rows) != 1 || rows[0].live != 5 {
		t.Fatalf("got %+v, want one row of 5 chat models", rows)
	}
}

func TestMeasureFilters(t *testing.T) {
	if rows := measure(fixture(), "", "nobody", false); len(rows) != 0 {
		t.Errorf("filtering on an absent provider gave %+v", rows)
	}
	rows := measure(fixture(), "embedding", "acme", false)
	if len(rows) != 1 || rows[0].kind != "embedding" {
		t.Errorf("got %+v, want only the embedding row", rows)
	}
}

func TestRenderMarksShortfalls(t *testing.T) {
	var buf bytes.Buffer
	rows := measure(fixture(), "chat", "", false)
	if err := render(&buf, rows, "chat", false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "1!") {
		t.Errorf("no shortfall marked in:\n%s", out)
	}
	if strings.Contains(out, "all kinds") {
		t.Errorf("a single kind should need no subtotal:\n%s", out)
	}
}
