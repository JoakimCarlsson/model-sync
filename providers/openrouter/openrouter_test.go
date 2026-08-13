package openrouter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// parse runs the parser over the listing, read from disk so the test never
// touches the network.
func parse(t *testing.T) map[string]catalog.Model {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", "models.json"))
	if err != nil {
		t.Fatal(err)
	}
	models, err := New().Parse([]catalog.Document{
		{URL: ModelsURL, Body: body},
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

// rate returns the amount of one rate, or reports that it is absent.
func rate(m catalog.Model, metric catalog.Metric, dims catalog.Dims) (
	float64,
	bool,
) {
	for _, p := range m.Prices {
		if p.Metric == metric && p.Dims.Key() == dims.Key() {
			return p.Amount, true
		}
	}
	return 0, false
}

// TestParseFreeModels covers a model OpenRouter charges nothing for, which
// records as a rate of zero rather than as no rate at all.
func TestParseFreeModels(t *testing.T) {
	byID := parse(t)
	for _, id := range []string{
		"openai/gpt-oss-20b:free",
		"openrouter/free",
		"google/lyria-3-pro-preview",
	} {
		m, ok := byID[id]
		if !ok {
			t.Errorf("%s: not parsed", id)
			continue
		}
		if m.Attrs[AttrFree] != "true" {
			t.Errorf("%s: not marked free", id)
		}
		for _, metric := range []catalog.Metric{
			MetricInputTokens,
			MetricOutputTokens,
		} {
			amount, ok := rate(m, metric, nil)
			if !ok {
				t.Errorf("%s: no %s rate", id, metric)
				continue
			}
			if amount != 0 {
				t.Errorf("%s: got %s %v, want 0", id, metric, amount)
			}
		}
	}
}

// TestParsePaidModel covers the scaling and the conditional rates, and that a
// paid model is not marked free.
func TestParsePaidModel(t *testing.T) {
	m, ok := parse(t)["openai/gpt-5.6-sol"]
	if !ok {
		t.Fatal("openai/gpt-5.6-sol: not parsed")
	}
	if m.Attrs[AttrFree] != "" {
		t.Error("marked free")
	}
	if amount, ok := rate(m, MetricInputTokens, nil); !ok || amount != 5 {
		t.Errorf("got input %v %v, want 5", amount, ok)
	}
	long := catalog.Dims{DimMinPromptTokens: "272000"}
	if amount, ok := rate(m, MetricInputTokens, long); !ok || amount != 10 {
		t.Errorf("got long-context input %v %v, want 10", amount, ok)
	}
}

// TestParseZeroRatesStayUnrecorded covers the other half of the rule: a zero
// on a key that does not apply to every model is not a rate of nothing.
func TestParseZeroRatesStayUnrecorded(t *testing.T) {
	m, ok := parse(t)["google/lyria-3-pro-preview"]
	if !ok {
		t.Fatal("google/lyria-3-pro-preview: not parsed")
	}
	for _, p := range m.Prices {
		switch p.Metric {
		case MetricInputTokens, MetricOutputTokens:
		default:
			t.Errorf("unexpected rate %s", p.Metric)
		}
	}
}

// TestParseCapabilities covers what the listing states beyond the rates.
func TestParseCapabilities(t *testing.T) {
	m, ok := parse(t)["anthropic/claude-opus-5"]
	if !ok {
		t.Fatal("anthropic/claude-opus-5: not parsed")
	}
	if m.Name == "" {
		t.Error("no display name")
	}
	if got := m.Limits[LimitContextWindow]; got == 0 {
		t.Error("no context window")
	}
	if len(m.Lists[ListInputModalities]) == 0 {
		t.Error("no input modalities")
	}
	if len(m.Lists[ListFeatures]) == 0 {
		t.Error("no features")
	}
}
