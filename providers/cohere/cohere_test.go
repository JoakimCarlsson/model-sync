package cohere

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
	if got := byID["command-r-03-2024"].Attrs[AttrState]; got !=
		StateDeprecated {
		t.Errorf("got state %q", got)
	}
}

// TestParseDimensions covers the two shapes the embed table's width column is
// written in: a bare number, and the whole set as a sentence with one of them
// marked as the one returned by default.
func TestParseDimensions(t *testing.T) {
	byID := parse(t)
	cases := []struct {
		id     string
		widths []string
		def    string
	}{
		{"embed-v4.0", []string{"256", "512", "1024", "1536"}, "1536"},
		{"embed-english-v3.0", []string{"1024"}, "1024"},
		{"embed-english-light-v3.0", []string{"384"}, "384"},
		{"embed-multilingual-v3.0", []string{"1024"}, "1024"},
	}
	for _, c := range cases {
		m := byID[c.id]
		if got := m.Lists[ListDimensions]; !slices.Equal(got, c.widths) {
			t.Errorf("%s: got widths %q, want %q", c.id, got, c.widths)
		}
		if got := m.Attrs[AttrDefaultDimension]; got != c.def {
			t.Errorf("%s: got default width %q, want %q", c.id, got, c.def)
		}
	}
}

// TestParseModalities covers the rule that neither side of the pair is recorded
// without the other, and the sides a family states in prose rather than in a
// column. Cohere's modality column says only what a model takes; the paragraph
// above each table says what it gives back.
func TestParseModalities(t *testing.T) {
	byID := parse(t)
	cases := []struct {
		id  string
		in  []string
		out []string
	}{
		{"command-a-03-2025", []string{"text"}, []string{"text"}},
		{
			"command-a-vision-07-2025",
			[]string{"image", "text"},
			[]string{"text"},
		},
		{"c4ai-aya-vision-32b", []string{"image", "text"}, []string{"text"}},
		{"tiny-aya-global", []string{"text"}, []string{"text"}},
		{"command-nightly", nil, nil},
		{"embed-v4.0", []string{"file", "image", "text"}, nil},
		{"rerank-v4.0-pro", []string{"text"}, nil},
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
	for id, m := range byID {
		in := len(m.Lists[ListInputModalities])
		out := len(m.Lists[ListOutputModalities])
		if out > 0 && in == 0 {
			t.Errorf("%s: returns something and takes nothing", id)
		}
	}
}

// TestParseUnpricedCarryReason covers the served models the pricing page states
// no amount for. Each says so, because a package comment is not visible to
// anything reading the catalog and an unpriced model otherwise reads as free.
func TestParseUnpricedCarryReason(t *testing.T) {
	byID := parse(t)
	for _, id := range []string{
		"command-a-03-2025",
		"command-a-plus-05-2026",
		"command-a-reasoning-08-2025",
		"command-a-translate-08-2025",
		"command-a-vision-07-2025",
		"c4ai-aya-vision-32b",
		"tiny-aya-global",
		"command-nightly",
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
		if !slices.Contains(m.Notes, noteNoRate) {
			t.Errorf("%s: got notes %v, want the no-rate note", id, m.Notes)
		}
	}
	for id, m := range byID {
		if !slices.Contains(m.Notes, noteNoRate) {
			continue
		}
		if len(m.Prices) > 0 {
			t.Errorf("%s: priced and marked as having no rate", id)
		}
		if withdrawn(&m) {
			t.Errorf("%s: withdrawn and marked as having no rate", id)
		}
	}
}

// TestParseWithoutPricingPage covers the guard on that note. The overview is
// fetched without the pricing page whenever the marketing site fails, and
// marking every model unpriced then would report this parser's missing document
// as a fact about Cohere.
func TestParseWithoutPricingPage(t *testing.T) {
	docs := []catalog.Document{}
	for _, doc := range fixtures(t) {
		if doc.URL != PricingURL {
			docs = append(docs, doc)
		}
	}
	models, err := New().Parse(docs)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range models {
		if slices.Contains(m.Notes, noteNoRate) {
			t.Errorf("%s: marked unpriced with no pricing page read", m.ID)
		}
	}
}

// TestParseCardNames covers the display names Cohere states only as the product
// a rate card is headed by, and the rule that a card heading two models names
// neither of them.
func TestParseCardNames(t *testing.T) {
	byID := parse(t)
	for id, want := range map[string]string{
		"embed-v4.0":             "Embed 4",
		"rerank-v4.0-fast":       "Rerank 4 Fast",
		"rerank-v4.0-pro":        "Rerank 4 Pro",
		"command-a-plus-05-2026": "Command A+",
		"command-r-08-2024":      "Command R",
	} {
		if got := byID[id].Name; got != want {
			t.Errorf("%s: got name %q, want %q", id, got, want)
		}
	}
	if got := byID["c4ai-aya-expanse-32b"].Name; got != "" {
		t.Errorf("a card heading two models named one of them %q", got)
	}
}

// TestParseUnnamed pins the models Cohere publishes no display name for. Its
// tables state only the identifier, and the models its prose does not name keep
// none rather than one derived from the identifier.
func TestParseUnnamed(t *testing.T) {
	byID := parse(t)
	for _, id := range []string{
		"tiny-aya-global",
		"c4ai-aya-vision-32b",
		"embed-english-v3.0",
		"rerank-v3.5",
	} {
		if got := byID[id].Name; got != "" {
			t.Errorf("%s: got name %q, want none published", id, got)
		}
	}
}
