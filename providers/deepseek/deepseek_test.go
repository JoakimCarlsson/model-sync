package deepseek

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// fixtures returns the document to parse, read from disk so the test never
// touches the network. It is the page whole, since the thing worth covering is
// that reading stops at the second of its two tables.
func fixtures(t *testing.T) []catalog.Document {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", "pricing.html"))
	if err != nil {
		t.Fatal(err)
	}
	return []catalog.Document{{URL: PricingURL, Body: body}}
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

// TestParseOnlyTheModels covers the trap the page sets: both its tables head
// their first cell "MODEL", and the second heads its columns with the
// denominations instead. Reading past the first would enter "1M OUTPUT TOKENS"
// into the catalog as a model.
func TestParseOnlyTheModels(t *testing.T) {
	byID := parse(t)
	want := []string{"deepseek-v4-flash", "deepseek-v4-pro"}
	got := make([]string, 0, len(byID))
	for id := range byID {
		got = append(got, id)
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("got models %q, want %q", got, want)
	}
}

// TestParseRates covers the three rates the current table states per model, and
// that a cache hit is separated from a cache miss rather than charged as a cache
// write.
func TestParseRates(t *testing.T) {
	byID := parse(t)
	cases := []struct {
		id     string
		metric catalog.Metric
		amount float64
	}{
		{"deepseek-v4-flash", MetricCachedInputTokens, 0.0028},
		{"deepseek-v4-flash", MetricInputTokens, 0.14},
		{"deepseek-v4-flash", MetricOutputTokens, 0.28},
		{"deepseek-v4-pro", MetricCachedInputTokens, 0.003625},
		{"deepseek-v4-pro", MetricInputTokens, 0.435},
		{"deepseek-v4-pro", MetricOutputTokens, 0.87},
	}
	for _, c := range cases {
		m := byID[c.id]
		found := false
		for _, p := range m.Prices {
			if p.Metric != c.metric || p.Unit != UnitPer1MTokens {
				continue
			}
			found = true
			if p.Amount != c.amount {
				t.Errorf(
					"%s: got %s %v, want %v",
					c.id,
					c.metric,
					p.Amount,
					c.amount,
				)
			}
		}
		if !found {
			t.Errorf("%s: no %s rate", c.id, c.metric)
		}
	}
	for id, m := range byID {
		if len(m.Prices) != 3 {
			t.Errorf(
				"%s: got %d prices, want the 3 currently charged",
				id,
				len(m.Prices),
			)
		}
	}
}

// TestParseScheduleNotCharged covers the second table, which a footnote dates to
// 16 August 2026. Its six amounts are between one and a half and four times the
// current ones, so reading any of them as a rate would misprice every call made
// before that date.
func TestParseScheduleNotCharged(t *testing.T) {
	scheduled := []float64{
		0.007, 0.22, 0.66, 0.014, 0.44, 1.32, 0.022, 1.98, 0.044, 3.96,
	}
	for id, m := range parse(t) {
		for _, p := range m.Prices {
			if slices.Contains(scheduled, p.Amount) {
				t.Errorf(
					"%s: recorded %v, a rate not charged until August 2026",
					id,
					p.Amount,
				)
			}
		}
	}
}

// TestParseSharedRow covers a row stating one value for both models, which the
// table writes once rather than twice.
func TestParseSharedRow(t *testing.T) {
	for id, m := range parse(t) {
		if got := m.Limits[LimitContextWindow]; got != 1_000_000 {
			t.Errorf("%s: got context window %d, want 1000000", id, got)
		}
		if got := m.Limits[LimitMaxOutputTokens]; got != 384_000 {
			t.Errorf("%s: got max output %d, want 384000", id, got)
		}
	}
}

// TestParsePerModelRows covers the rows that differ between the two models,
// including the concurrency limit, which is the one bound the page states per
// model rather than for both.
func TestParsePerModelRows(t *testing.T) {
	byID := parse(t)
	cases := []struct {
		id          string
		version     string
		concurrency int64
	}{
		{"deepseek-v4-flash", "DeepSeek-V4-Flash-0731", 2500},
		{"deepseek-v4-pro", "DeepSeek-V4-Pro-0813", 500},
	}
	for _, c := range cases {
		m := byID[c.id]
		if got := m.Attrs[AttrModelVersion]; got != c.version {
			t.Errorf("%s: got version %q, want %q", c.id, got, c.version)
		}
		if got := m.Limits[LimitConcurrency]; got != c.concurrency {
			t.Errorf(
				"%s: got concurrency %d, want %d",
				c.id,
				got,
				c.concurrency,
			)
		}
	}
}

// TestParseFeaturesAndEndpoints covers the capability rows, which are marked
// with a tick per model, and the two rows naming an API rather than something
// the model can do.
func TestParseFeaturesAndEndpoints(t *testing.T) {
	for id, m := range parse(t) {
		for _, want := range []string{"function_calling", "json_mode"} {
			if !slices.Contains(m.Lists[ListFeatures], want) {
				t.Errorf(
					"%s: got features %q, want %q among them",
					id,
					m.Lists[ListFeatures],
					want,
				)
			}
		}
		for _, want := range []string{"Anthropic", "Responses"} {
			if !slices.Contains(m.Lists[ListEndpoints], want) {
				t.Errorf(
					"%s: got endpoints %q, want %q among them",
					id,
					m.Lists[ListEndpoints],
					want,
				)
			}
		}
		if slices.Contains(m.Lists[ListFeatures], "anthropic_api") {
			t.Errorf("%s: an API recorded as a capability", id)
		}
	}
}

// TestParseNoNameOrModality pins what DeepSeek does not publish. Its table heads
// each column with the identifier and states a model version beside it, which is
// the build that identifier resolves to rather than a name.
func TestParseNoNameOrModality(t *testing.T) {
	for id, m := range parse(t) {
		if m.Name != "" {
			t.Errorf("%s: got name %q, want none published", id, m.Name)
		}
		for _, key := range []string{
			"input_modalities",
			"output_modalities",
		} {
			if got := m.Lists[key]; len(got) != 0 {
				t.Errorf("%s: got %s %q, want none published", id, key, got)
			}
		}
	}
}
