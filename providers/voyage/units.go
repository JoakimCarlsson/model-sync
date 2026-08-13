package voyage

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Metrics Voyage bills on. There is no output metric: an embedding or a
// reranking is charged entirely by what goes into it.
const (
	MetricInputTokens catalog.Metric = "input_tokens"
	MetricPixels      catalog.Metric = "pixels"
	MetricStorage     catalog.Metric = "storage"
)

// Units Voyage quotes amounts against.
const (
	UnitPer1KTokens catalog.Unit = "per_1k_tokens"
	UnitPer1MTokens catalog.Unit = "per_1m_tokens"
	UnitPer1BPixels catalog.Unit = "per_1b_pixels"
	UnitPerGBMonth  catalog.Unit = "per_gb_month"
)

// Kinds of model Voyage publishes.
const (
	KindEmbedding catalog.Kind = "embedding"
	KindRerank    catalog.Kind = "rerank"
	KindTool      catalog.Kind = "tool"
)

// States Voyage distinguishes. An older model is still served; it simply
// carries no free allowance.
const (
	StateCurrent = "current"
	StateOlder   = "older"
)

// Scalar keys the documents populate.
const (
	AttrSummary          = "summary"
	AttrState            = "state"
	AttrFreeAllowance    = "free_allowance"
	AttrEstPerRequest    = "estimated_price_per_request"
	AttrBatchDiscount    = "batch_discount"
	AttrDefaultDimension = "default_embedding_dimension"
	AttrOpenWeights      = "open_weights"
)

// noteOpenWeights records why the models Voyage publishes the weights of carry
// no rate. Voyage does not serve them, so there is nothing for it to charge
// for, and without this they would read as models it serves for free.
const noteOpenWeights = "weights published; Voyage states no rate " +
	"because it does not serve the model"

// Numeric keys the documents populate.
const (
	LimitContextWindow = "context_window"
	LimitChunkContext  = "chunk_context_window"
	LimitFreeTokens    = "free_tokens"
)

// Enumeration keys the documents populate.
const (
	ListDimensions       = "embedding_dimensions"
	ListInputModalities  = "input_modalities"
	ListOutputModalities = "output_modalities"
)

// Modalities Voyage's capability pages account for.
const (
	ModalityText  = "text"
	ModalityImage = "image"
)

// pageModalities reports what the models on one capability page take.
//
// Voyage states this by which page a model is documented on and nowhere else:
// the multimodal page is the one whose models vectorize text and pictures
// together, and every other page is text.
func pageModalities(url string) []string {
	if strings.Contains(url, "multimodal") {
		return []string{ModalityText, ModalityImage}
	}
	return []string{ModalityText}
}

// addModalities records what a model takes and what it gives back.
//
// No Voyage page states an output modality. Every model here vectorizes or
// scores text, so the medium it works in is text on both sides, and that is
// what is recorded rather than the shape of the return value: an embedding is a
// vector and a reranking is a set of scores, but neither is a modality and the
// catalog has no word for either.
//
// Both sides are set together, so a model surviving only in the rate tables
// carries neither. A consumer reading one side alone cannot tell an unstated
// modality from a model that takes or returns nothing.
func addModalities(m *catalog.Model, in []string) {
	if len(in) == 0 {
		return
	}
	m.AddList(ListInputModalities, in...)
	m.AddList(ListOutputModalities, ModalityText)
}

var (
	linkRe    = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	breakRe   = regexp.MustCompile(`(?i)<br\s*/?>`)
	htmlTagRe = regexp.MustCompile(`(?s)<[^>]*>`)
	amountRe  = regexp.MustCompile(`\$\s*([\d,]*\.?\d+)`)
	countRe   = regexp.MustCompile(
		`(?i)^([\d,]*\.?\d+)\s*(thousand|million|billion|[kmb])?\b`,
	)
	dimensionRe = regexp.MustCompile(`(\d+)\s*(\(default\))?`)
	backtickRe  = regexp.MustCompile("`([^`]+)`")
)

// clean strips the decoration Voyage wraps around cell values. Its markdown
// carries MDX anchor elements, escaped footnote markers and backticked
// identifiers.
func clean(cell string) string {
	s := breakRe.ReplaceAllString(cell, "\n")
	s = linkRe.ReplaceAllString(s, "$1")
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, `\*`, "")
	return strings.Join(strings.Fields(s), " ")
}

// splitModels reads a model cell, which names more than one model when they
// share a rate. Voyage separates them with a line break element.
func splitModels(cell string) []string {
	var out []string
	for _, part := range breakRe.Split(cell, -1) {
		for _, line := range strings.Split(part, "\n") {
			if id := clean(line); id != "" {
				out = append(out, id)
			}
		}
	}
	return out
}

// parseAmount reads a price cell.
func parseAmount(cell string) (float64, bool) {
	match := amountRe.FindStringSubmatch(clean(cell))
	if match == nil {
		return 0, false
	}
	value, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", ""), 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

// parseCount reads a quantity, which Voyage writes both as a grouped number
// and as a word, as in "32,000" and "200 million".
func parseCount(value string) int64 {
	match := countRe.FindStringSubmatch(strings.TrimSpace(clean(value)))
	if match == nil {
		return 0
	}
	n, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", ""), 64)
	if err != nil {
		return 0
	}
	switch strings.ToLower(match[2]) {
	case "thousand", "k":
		n *= 1_000
	case "million", "m":
		n *= 1_000_000
	case "billion", "b":
		n *= 1_000_000_000
	}
	return int64(n)
}

// parseDimensions reads an embedding dimension cell such as
// "1024 (default), 256, 512, 2048", returning every dimension offered and the
// one used when none is asked for.
func parseDimensions(cell string) (dimensions []string, defaultDim string) {
	for _, match := range dimensionRe.FindAllStringSubmatch(clean(cell), -1) {
		dimensions = append(dimensions, match[1])
		if match[2] != "" {
			defaultDim = match[1]
		}
	}
	return dimensions, defaultDim
}

// backtickedIDs returns every identifier written as code in a passage, which
// is how Voyage lists the models a term applies to when it states them in a
// sentence rather than a table.
func backtickedIDs(text string) []string {
	var out []string
	for _, match := range backtickRe.FindAllStringSubmatch(text, -1) {
		out = append(out, strings.TrimSpace(match[1]))
	}
	return out
}

// kindFor reports what a model is from its identifier, which is the only
// signal on pages that mix them.
func kindFor(id string) catalog.Kind {
	if strings.HasPrefix(id, "rerank") {
		return KindRerank
	}
	return KindEmbedding
}
