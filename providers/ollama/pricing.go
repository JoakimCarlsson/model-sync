package ollama

import (
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// PricingURL is the page tabulating the rate of every cloud model, including
// the models whose own page quotes no rate and the off-peak rates no model
// page states.
const PricingURL = baseURL + "/pricing"

// DimPeriod says which part of the week a rate applies in, for the models the
// pricing table quotes a cheaper off-peak rate for.
const DimPeriod = "period"

// periodOffPeak is the period the table marks "(Off-Peak)".
const periodOffPeak = "off_peak"

// DimTag says which build of a model a rate belongs to, for the models the
// pricing table prices per build rather than once.
const DimTag = "tag"

// offPeakSuffix marks a row of the table as the off-peak rate of its model.
const offPeakSuffix = "(Off-Peak)"

// pricingRowRe matches one row of the pricing table: the model it links, the
// label it shows and its input, cached input and output amounts, each of
// which may be a dash where the table states none.
var pricingRowRe = regexp.MustCompile(
	`(?is)<tr>\s*<td[^>]*>\s*<a href="/library/([^"]+)"[^>]*>([^<]+)</a>\s*</td>` +
		`\s*<td[^>]*>\s*([^<]*?)\s*</td>` +
		`\s*<td[^>]*>\s*([^<]*?)\s*</td>` +
		`\s*<td[^>]*>\s*([^<]*?)\s*</td>`,
)

// pricingColumns are the metrics of the table's amount columns, in order.
var pricingColumns = []catalog.Metric{
	MetricInputTokens,
	MetricCachedInputTokens,
	MetricOutputTokens,
}

// applyPricing reads the pricing table, recording each row's amounts against
// the model it links, per million tokens as the table's heading states. A row
// labelled with a build prices that build alone, and a row marked off-peak
// prices the off-peak period alone.
func (b *builder) applyPricing(doc catalog.Document) {
	for _, row := range pricingRowRe.FindAllStringSubmatch(string(doc.Body), -1) {
		m, ok := b.models[path.Base(row[1])]
		if !ok {
			continue
		}
		dims := pricingDims(strings.TrimSpace(text(row[2])))
		recorded := false
		for i, metric := range pricingColumns {
			amount, ok := parseDollars(row[3+i])
			if !ok {
				continue
			}
			m.AddPrice(catalog.Price{
				Metric:   metric,
				Unit:     UnitPer1MTokens,
				Amount:   amount,
				Currency: currencyUSD,
				Dims:     dims,
			})
			recorded = true
		}
		if recorded {
			m.AddSource(doc.URL)
		}
	}
}

// pricingDims derives the dimensions of a row from its label: always the
// cloud deployment, plus the period when the label marks it off-peak and the
// build when the label names one.
func pricingDims(label string) catalog.Dims {
	dims := catalog.Dims{DimDeployment: deploymentCloud}
	if strings.HasSuffix(label, offPeakSuffix) {
		dims[DimPeriod] = periodOffPeak
		label = strings.TrimSpace(strings.TrimSuffix(label, offPeakSuffix))
	}
	if _, tag, ok := strings.Cut(label, ":"); ok && tag != "" {
		dims[DimTag] = tag
	}
	return dims
}

// parseDollars reads an amount written as "$0.15", reporting false for a dash
// or anything else that states no amount.
func parseDollars(cell string) (float64, bool) {
	digits, ok := strings.CutPrefix(strings.TrimSpace(cell), "$")
	if !ok {
		return 0, false
	}
	amount, err := strconv.ParseFloat(digits, 64)
	if err != nil {
		return 0, false
	}
	return amount, true
}
