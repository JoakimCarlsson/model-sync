package cohere

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// fixtures returns the documents to parse, read from disk so the test never
// touches the network.
func fixtures(t *testing.T) []catalog.Document {
	t.Helper()
	docs := []catalog.Document{}
	for _, f := range []struct {
		url  string
		file string
	}{
		{ModelsURL, "models.md"},
		{PricingURL, "pricing.html"},
	} {
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

// priced reports the amount of one rate, identified by everything but its
// amount.
func priced(m catalog.Model, want catalog.Price) (float64, bool) {
	for _, p := range m.Prices {
		if p.Metric == want.Metric && p.Unit == want.Unit &&
			p.Dims.Key() == want.Dims.Key() {
			return p.Amount, true
		}
	}
	return 0, false
}

func TestParseCardRates(t *testing.T) {
	byID := parse(t)
	cases := []struct {
		id    string
		price catalog.Price
	}{
		{"command-r-08-2024", catalog.Price{
			Metric: MetricInputTokens, Unit: UnitPer1MTokens, Amount: 0.15,
		}},
		{"command-r-08-2024", catalog.Price{
			Metric: MetricOutputTokens, Unit: UnitPer1MTokens, Amount: 0.6,
		}},
		{"command-r7b-12-2024", catalog.Price{
			Metric: MetricInputTokens, Unit: UnitPer1MTokens, Amount: 0.0375,
		}},
		{"embed-v4.0", catalog.Price{
			Metric: MetricInputTokens, Unit: UnitPer1MTokens, Amount: 0.12,
		}},
		{"embed-v4.0", catalog.Price{
			Metric: MetricImageInput, Unit: UnitPer1MTokens, Amount: 0.47,
		}},
		{"rerank-v4.0-fast", catalog.Price{
			Metric: MetricSearchQueries, Unit: UnitPer1KSearch, Amount: 2,
		}},
		{"rerank-v4.0-pro", catalog.Price{
			Metric: MetricSearchQueries, Unit: UnitPer1KSearch, Amount: 2.5,
		}},
		{"command-r-plus-08-2024", catalog.Price{
			Metric: MetricInputTokens, Unit: UnitPer1MTokens, Amount: 2.5,
		}},
	}
	for _, c := range cases {
		got, ok := priced(byID[c.id], c.price)
		if !ok {
			t.Errorf("%s: no %s rate", c.id, c.price.Metric)
			continue
		}
		if got != c.price.Amount {
			t.Errorf(
				"%s: got %s %v, want %v",
				c.id,
				c.price.Metric,
				got,
				c.price.Amount,
			)
		}
	}
}

func TestParseVaultRates(t *testing.T) {
	byID := parse(t)
	vault := func(tier string) catalog.Dims {
		return catalog.Dims{DimDeployment: DeploymentVault, DimTier: tier}
	}
	cases := []struct {
		id     string
		price  catalog.Price
		amount float64
	}{
		{"embed-v4.0", catalog.Price{
			Metric: MetricHosting, Unit: UnitPerHour, Dims: vault("small"),
		}, 4},
		{"embed-v4.0", catalog.Price{
			Metric: MetricHosting, Unit: UnitPerMonth, Dims: vault("small"),
		}, 2500},
		{"embed-v4.0", catalog.Price{
			Metric: MetricHosting, Unit: UnitPerHour, Dims: vault("medium"),
		}, 5},
		{"rerank-v3.5", catalog.Price{
			Metric: MetricHosting, Unit: UnitPerMonth, Dims: vault("medium"),
		}, 3250},
		{"rerank-v4.0-pro", catalog.Price{
			Metric: MetricHosting, Unit: UnitPerHour, Dims: vault("large"),
		}, 10},
	}
	for _, c := range cases {
		got, ok := priced(byID[c.id], c.price)
		if !ok {
			t.Errorf("%s: no %s rate at %s", c.id, c.price.Unit, c.price.Dims)
			continue
		}
		if got != c.amount {
			t.Errorf("%s: got %v, want %v", c.id, got, c.amount)
		}
	}
}

// TestParseStartingRate covers the one rate Cohere states as a floor, in prose
// inside a card that carries no amounts of its own.
func TestParseStartingRate(t *testing.T) {
	m := parse(t)["cohere-transcribe-03-2026"]
	if len(m.Prices) != 1 {
		t.Fatalf("got %d prices, want 1", len(m.Prices))
	}
	p := m.Prices[0]
	if p.Metric != MetricHosting || p.Unit != UnitPerHour || p.Amount != 3.75 {
		t.Errorf("got %s %s %v", p.Metric, p.Unit, p.Amount)
	}
	if p.Note != noteStartingRate {
		t.Errorf("got note %q, want %q", p.Note, noteStartingRate)
	}
	if p.Dims[DimDeployment] != DeploymentVault {
		t.Errorf("got dims %v", p.Dims)
	}
}

// TestParseUnpriced pins the models Cohere states no rate for anywhere it
// publishes, so that a rate appearing later is noticed rather than assumed.
func TestParseUnpriced(t *testing.T) {
	byID := parse(t)
	for _, id := range []string{
		"command-a-03-2025",
		"command-a-plus-05-2026",
		"command-a-reasoning-08-2025",
		"c4ai-aya-vision-32b",
		"embed-english-v3.0",
		"rerank-english-v3.0",
	} {
		m, ok := byID[id]
		if !ok {
			t.Errorf("%s: not parsed", id)
			continue
		}
		if len(m.Prices) != 0 {
			t.Errorf("%s: got %d prices, want none", id, len(m.Prices))
		}
	}
}

// TestParseRetiredStaysUnpriced covers the rule that the page's questions and
// answers outlive the models they answer for.
func TestParseRetiredStaysUnpriced(t *testing.T) {
	m := parse(t)["c4ai-aya-expanse-8b"]
	if m.Attrs[AttrState] != StateRetired {
		t.Fatalf("got state %q, want retired", m.Attrs[AttrState])
	}
	if len(m.Prices) != 0 {
		t.Errorf("got %d prices, want none", len(m.Prices))
	}
}

func TestParseOverview(t *testing.T) {
	byID := parse(t)
	m := byID["command-a-03-2025"]
	if m.Name != "Command A" {
		t.Errorf("got name %q", m.Name)
	}
	if got := m.Limits[LimitContextWindow]; got != 256000 {
		t.Errorf("got context window %d", got)
	}
	if got := m.Limits[LimitMaxOutputTokens]; got != 8000 {
		t.Errorf("got max output %d", got)
	}
	if got := byID["embed-v4.0"].Lists[ListDimensions]; len(got) != 4 {
		t.Errorf("got embedding dimensions %v", got)
	}
	if got := byID["command-r-03-2024"].Attrs[AttrState]; got !=
		StateDeprecated {
		t.Errorf("got state %q", got)
	}
}
