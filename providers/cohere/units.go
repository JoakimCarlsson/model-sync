package cohere

import (
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// currency is the only currency Cohere quotes.
const currency = "USD"

// Metrics Cohere bills on.
const (
	MetricInputTokens   catalog.Metric = "input_tokens"
	MetricOutputTokens  catalog.Metric = "output_tokens"
	MetricImageInput    catalog.Metric = "image_input"
	MetricSearchQueries catalog.Metric = "search_queries"
)

// Units Cohere quotes amounts against.
const (
	UnitPer1MTokens catalog.Unit = "per_1m_tokens"
	UnitPer1KSearch catalog.Unit = "per_1k_searches"
)

// modalityNames map the wording of the overview's modality column onto the
// catalog's vocabulary. Cohere names a mixed document by the formats it may
// contain, which is the two modalities it already states rather than a third.
var modalityNames = map[string]string{
	"text":                           "text",
	"images":                         "image",
	"image":                          "image",
	"audio":                          "audio",
	"mixed texts/images (i.e. pdfs)": "file",
}

// modalityName rewrites one modality into the catalog's vocabulary, keeping
// Cohere's own word for anything the table names that this does not cover.
func modalityName(value string) string {
	if name, ok := modalityNames[strings.ToLower(strings.TrimSpace(value))]; ok {
		return name
	}
	return strings.ToLower(strings.TrimSpace(value))
}

// cardModels map the product name Cohere prints on a pricing card onto the
// identifier that product is called by.
//
// The rate cards name products, not models: a card headed "Embed 4" states the
// rate of the model the API calls embed-v4.0. Where the pricing page names a
// model precisely, as its legacy rates do, no entry is needed and the name is
// reduced to an identifier instead.
var cardModels = map[string][]string{
	"command r":     {"command-r-08-2024"},
	"command r7b":   {"command-r7b-12-2024"},
	"embed 4":       {"embed-v4.0"},
	"rerank 4 fast": {"rerank-v4.0-fast"},
	"rerank 4 pro":  {"rerank-v4.0-pro"},
	"aya expanse":   {"c4ai-aya-expanse-8b", "c4ai-aya-expanse-32b"},
}

// cardLabels map the wording of a rate's label onto what it counts. Cohere
// labels the ordinary rates "Input" and "Output" on one card and "Cost" on
// another, and names only the exception within a card.
var cardLabels = map[string]catalog.Metric{
	"input":      MetricInputTokens,
	"cost":       MetricInputTokens,
	"output":     MetricOutputTokens,
	"image cost": MetricImageInput,
}

// cardUnits map the denominator a card quotes against onto a unit and, where
// the denominator says what is being counted, onto a metric.
var cardUnits = map[string]struct {
	Unit   catalog.Unit
	Metric catalog.Metric
}{
	"1m tokens":   {UnitPer1MTokens, ""},
	"1k searches": {UnitPer1KSearch, MetricSearchQueries},
}
