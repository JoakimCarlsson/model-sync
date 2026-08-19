package cohere

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// RateLimitsURL states how often a model may be called. Cohere publishes two
// ceilings for every model, one for the key an account is given on sign up and
// one for the key it is given on paying, and states them in two tables: the
// Chat models get a row each, and everything else gets a row per endpoint.
const RateLimitsURL = "https://docs.cohere.com/docs/rate-limits.md"

// Headings the two rate limit tables are written under.
const (
	rateModelColumn    = "model"
	rateEndpointColumn = "endpoint"
	rateTrialColumn    = "trial rate limit"
	rateProdColumn     = "production rate limit"
)

// Suffixes separating the ceiling of a free key from the ceiling of a paid
// one, since Cohere quotes both for every row.
const (
	tierTrial      = "_trial"
	tierProduction = "_production"
)

// rateCounters map the noun a ceiling is counted in onto the key recording it.
// Cohere counts calls for most endpoints and inputs for the embedding one,
// which are not the same thing: one embedding call carries up to ninety six
// texts and each of them is an input.
var rateCounters = map[string]string{
	"req":    "requests_per_minute",
	"inputs": "inputs_per_minute",
}

// rateEndpoints map an endpoint the rate limit table names onto the endpoint
// the overview's column names and, where the row bounds something other than
// a call to the endpoint the family is named for, onto the key prefix saying
// what it bounds.
//
// An embedding model answers on two endpoints and Cohere bounds three things
// across them, none of them the same thing: how many inputs a minute may be
// embedded, how many of those inputs may be images, and how many batch jobs a
// minute may be submitted. Recording the last of the three as the model's
// requests a minute would read as a ceiling on calling it at all.
//
// Two rows are left unmapped on purpose. Tokenize belongs to an endpoint no
// model answers on, and the default row states the ceiling of everything not
// named above it, which is not a statement about any model in particular.
var rateEndpoints = map[string]struct {
	Endpoint string
	Prefix   string
}{
	"embed":                {"Embed", ""},
	"embed (images)":       {"Embed", "image_"},
	"embedjob":             {"Embed Jobs", "job_"},
	"rerank":               {"Rerank", ""},
	"audio transcriptions": {"Audio Transcriptions", ""},
}

// rateRe matches one ceiling. A row Cohere answers with an invitation to
// contact its sales team states no ceiling and yields nothing.
var rateRe = regexp.MustCompile(`^([\d,]+)\s*(req|inputs)\s*/\s*min$`)

// applyRateLimits reads the rate limit page.
//
// The two tables reach a model two different ways. The Chat table names a
// product, which resolves to a model through the same table of product names
// the rate cards do, and the other names an endpoint, which reaches every
// model the overview answers there. A model neither table reaches carries no
// ceiling rather than the page's default row, because that row states what
// applies to everything not named and not what applies to a model.
func (b *builder) applyRateLimits(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		trial := columnOf(t.Headers, rateTrialColumn)
		prod := columnOf(t.Headers, rateProdColumn)
		if trial < 0 || prod < 0 {
			continue
		}
		if col := columnOf(t.Headers, rateModelColumn); col >= 0 {
			b.applyModelRates(doc, t, col, trial, prod)
			continue
		}
		if col := columnOf(t.Headers, rateEndpointColumn); col >= 0 {
			b.applyEndpointRates(doc, t, col, trial, prod)
		}
	}
}

// applyModelRates records the ceilings the Chat table states per product.
func (b *builder) applyModelRates(
	doc catalog.Document,
	t table,
	name, trial, prod int,
) {
	for _, row := range t.Rows {
		for _, id := range b.identify(clean(cellAt(row, name))) {
			m := b.models[id]
			setRate(m, "", cellAt(row, trial), tierTrial)
			setRate(m, "", cellAt(row, prod), tierProduction)
			m.AddSource(doc.URL)
		}
	}
}

// applyEndpointRates records the ceilings the second table states per
// endpoint against every model answering there.
func (b *builder) applyEndpointRates(
	doc catalog.Document,
	t table,
	name, trial, prod int,
) {
	for _, row := range t.Rows {
		heading := strings.ToLower(clean(cellAt(row, name)))
		named, ok := rateEndpoints[heading]
		if !ok {
			continue
		}
		for _, id := range b.order {
			m := b.models[id]
			if !onEndpoint(m, named.Endpoint) {
				continue
			}
			setRate(m, named.Prefix, cellAt(row, trial), tierTrial)
			setRate(m, named.Prefix, cellAt(row, prod), tierProduction)
			m.AddSource(doc.URL)
		}
	}
}

// setRate records one ceiling, naming the key after what the cell counts.
func setRate(m *catalog.Model, prefix, cell, tier string) {
	match := rateRe.FindStringSubmatch(clean(cell))
	if match == nil {
		return
	}
	counter, ok := rateCounters[match[2]]
	if !ok {
		return
	}
	m.SetLimit(prefix+counter+tier, parseCount(match[1]))
}
