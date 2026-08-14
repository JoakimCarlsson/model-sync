package cohere

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// VaultPricingURL states what an instance of a model costs on Cohere's
// dedicated deployment platform. It is the one document quoting a rate for the
// Command A family, which the marketing pricing page sells no card for.
const VaultPricingURL = "https://docs.cohere.com/docs/model-vault/standard/pricing.md"

// Headings of the two tables the dedicated deployment page quotes rates in.
const (
	vaultModelHeading = "model"
	vaultTierHeading  = "performance tier"
)

// vaultRates map a rate column onto the denominator its amounts are quoted
// against and, where the heading names the tier instead of a column doing it,
// onto that tier.
//
// The page states its rates in two tables of different shapes. The one for the
// models sold self-serve gives each model a row per tier and three columns of
// denominators; the one for the generative models gives each model one row and
// names the larger tier in the heading of a second hourly column.
var vaultRates = map[string]struct {
	Unit catalog.Unit
	Tier string
}{
	"hourly rate":    {UnitPerHour, ""},
	"monthly rate":   {UnitPerMonth, ""},
	"annual rate":    {UnitPerYear, ""},
	"xl hourly rate": {UnitPerHour, TierXL},
}

// vaultAmountRe matches the amount in a rate cell. The page escapes the dollar
// sign and separates thousands, and writes a dash where a model is not offered
// at that rate.
var vaultAmountRe = regexp.MustCompile(`\$\s*([\d,]+(?:\.\d+)?)`)

// applyVaultPricing reads the dedicated deployment rates.
//
// An instance is billed for the time it is held rather than for anything a
// request carries, so every amount here is hosting and is marked with the
// platform it belongs to, the same way the rates the marketing page states for
// the same platform are. The two documents overlap on the models sold
// self-serve and agree on them, and an identical rate is recorded once.
func (b *builder) applyVaultPricing(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		modelCol := columnOf(t.Headers, vaultModelHeading)
		if modelCol < 0 {
			continue
		}
		tierCol := columnOf(t.Headers, vaultTierHeading)
		for _, row := range t.Rows {
			product := clean(cellAt(row, modelCol))
			b.nameFromCard(product)
			tier := strings.ToLower(clean(cellAt(row, tierCol)))
			b.addVaultRow(doc, product, tier, t.Headers, row)
		}
	}
}

// addVaultRow records every rate one row of a dedicated deployment table
// states.
func (b *builder) addVaultRow(
	doc catalog.Document,
	product, tier string,
	headers, row []string,
) {
	for i, header := range headers {
		quoted, ok := vaultRates[strings.ToLower(clean(header))]
		if !ok {
			continue
		}
		value := vaultAmount(cellAt(row, i))
		if value == 0 {
			continue
		}
		named := tier
		if quoted.Tier != "" {
			named = quoted.Tier
		}
		for _, id := range b.identify(product) {
			b.price(doc, id, catalog.Price{
				Metric: MetricHosting,
				Unit:   quoted.Unit,
				Amount: value,
				Dims: catalog.Dims{
					DimDeployment: DeploymentVault,
				}.With(DimTier, named),
			})
		}
	}
}

// vaultAmount reads the rate one cell quotes.
func vaultAmount(cell string) float64 {
	match := vaultAmountRe.FindStringSubmatch(cell)
	if match == nil {
		return 0
	}
	return amount(strings.ReplaceAll(match[1], ",", ""))
}
