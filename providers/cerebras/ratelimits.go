package cerebras

import (
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// RateLimitsURL states, per model and per plan, how many requests and how many
// tokens a caller may spend in a minute, an hour and a day.
const RateLimitsURL = baseURL + "/support/rate-limits.md"

// Numeric keys the rate limit tables populate. A key without a suffix is the
// limit a paying caller is held to, and the suffix names the free trial, which
// is the same split the two ceilings are already recorded under.
const (
	LimitRequestsPerMinute     = "requests_per_minute"
	LimitRequestsPerMinuteFree = "requests_per_minute_free"
	LimitTokensPerMinute       = "tokens_per_minute"
	LimitTokensPerMinuteFree   = "tokens_per_minute_free"
	LimitTokensPerHour         = "tokens_per_hour"
	LimitTokensPerHourFree     = "tokens_per_hour_free"
	LimitTokensPerDay          = "tokens_per_day"
	LimitTokensPerDayFree      = "tokens_per_day_free"
	LimitImagesPerRequest      = "images_per_request"
	LimitImagesPerRequestFree  = "images_per_request_free"
)

// tierSuffixes name the plan a table of limits is stated for. A plan Cerebras
// provisions per organization rather than publishing, which is what the
// enterprise tab says of itself, has no table and so needs no entry.
var tierSuffixes = map[string]string{
	"free trial":                "_free",
	"developer (pay as you go)": "",
	"developer":                 "",
}

// rateColumns map a column of the rate limit tables onto the key it fills,
// before the plan's suffix is appended.
var rateColumns = map[string]string{
	"rpm": LimitRequestsPerMinute,
	"tpm": LimitTokensPerMinute,
	"tph": LimitTokensPerHour,
	"tpd": LimitTokensPerDay,
}

// applyRateLimits reads the rate limit page.
//
// Cerebras states one table per plan, told apart by the tab it stands in, and
// names the model in each row by identifier. A row for a model no document has
// named is skipped rather than creating one, because the page is a table of
// limits and not a statement of what is served.
func (b *builder) applyRateLimits(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		suffix, ok := tierSuffixes[t.Section]
		if !ok {
			continue
		}
		modelCol := columnOf(t.Headers, "model")
		if modelCol < 0 {
			continue
		}
		for _, row := range t.Rows {
			m, ok := b.models[clean(cellAt(row, modelCol))]
			if !ok {
				continue
			}
			m.AddSource(doc.URL)
			applyRateRow(m, t.Headers, row, suffix)
		}
	}
}

// applyRateRow records one model's limits under one plan.
func applyRateRow(
	m *catalog.Model,
	headers, row []string,
	suffix string,
) {
	for i, header := range headers {
		key, ok := rateColumns[strings.ToLower(clean(header))]
		if !ok {
			continue
		}
		m.SetLimit(key+suffix, parseCount(cellAt(row, i)))
	}
}
