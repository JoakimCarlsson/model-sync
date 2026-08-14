package perplexity

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Metrics Perplexity bills on.
const (
	MetricInputTokens       catalog.Metric = "input_tokens"
	MetricOutputTokens      catalog.Metric = "output_tokens"
	MetricCachedInputTokens catalog.Metric = "cached_input_tokens"
	MetricCitationTokens    catalog.Metric = "citation_tokens"
	MetricReasoningTokens   catalog.Metric = "reasoning_tokens"
	MetricSearchQueries     catalog.Metric = "search_queries"
	MetricRequest           catalog.Metric = "request"
	MetricToolCall          catalog.Metric = "tool_call"
	MetricSession           catalog.Metric = "session"
)

// Units Perplexity quotes amounts against.
const (
	UnitPer1MTokens   catalog.Unit = "per_1m_tokens"
	UnitPer1KRequests catalog.Unit = "per_1k_requests"
	UnitPerInvocation catalog.Unit = "per_invocation"
	UnitPerSession    catalog.Unit = "per_session"
)

// Kinds of model Perplexity publishes.
const (
	KindChat      catalog.Kind = "chat"
	KindEmbedding catalog.Kind = "embedding"
	KindTool      catalog.Kind = "tool"
)

// DimContextSize records how much of the web a request is allowed to read,
// which is the only thing separating Perplexity's three request fees.
const DimContextSize = "search_context_size"

// DimPromptBand records the prompt length a rate applies under, which is what
// separates the two amounts a brokered model's rate cell holds when the model
// costs more on a long prompt.
const DimPromptBand = "prompt_band"

// Scalar keys the pricing pages populate.
const (
	AttrSummary          = "summary"
	AttrAuthor           = "author"
	AttrDefaultDimension = "default_embedding_dimension"
	AttrContextualized   = "contextualized"
	// AttrCacheDiscount holds a cache rate stated as a reduction of the input
	// rate rather than as an amount. It is not a price, because the amount it
	// reduces varies with the prompt for the models that state it this way, so
	// resolving it here would invent a rate the table does not give.
	AttrCacheDiscount = "cache_read_discount"
)

// Enumerations the embedding table populates.
const (
	// ListDimensions holds the embedding width.
	ListDimensions = "embedding_dimensions"
	// ListQuantizations holds the encodings an embedding is returned in.
	ListQuantizations = "quantizations"
)

// FeatureMatryoshka is what Perplexity's embedding table calls MRL: the vector
// can be truncated to a shorter one that still means something.
const FeatureMatryoshka = "matryoshka"

var (
	linkRe   = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	tagRe    = regexp.MustCompile(`(?s)<[^>]*>`)
	numberRe = regexp.MustCompile(`([\d,]*\.?\d+)`)
	// countRe matches a cell holding a bare quantity, written either in full or
	// with a thousands or millions suffix.
	countRe = regexp.MustCompile(`(?i)^([\d,]+)\s*([km])?$`)
	// bandRe matches one amount of a rate cell that states a rate per prompt
	// length, which Perplexity writes as the amount and then the bound.
	bandRe = regexp.MustCompile(`([\d,]*\.?\d+)\s*\(([^)]*)\)`)
	// discountRe matches a rate cell stating a reduction of another rate rather
	// than an amount of its own.
	discountRe = regexp.MustCompile(`(?i)\d+%\s*off`)
)

// clean strips markdown and MDX decoration from a cell value.
func clean(cell string) string {
	s := linkRe.ReplaceAllString(cell, "$1")
	s = tagRe.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, `\$`, "$")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "`", "")
	return strings.Join(strings.Fields(s), " ")
}

// parseAmount reads a rate cell. Perplexity writes the currency sign in some
// tables and leaves it to the column heading in others, so the sign is
// optional and the first number wins.
func parseAmount(cell string) (float64, bool) {
	text := clean(cell)
	if text == "" || text == "-" {
		return 0, false
	}
	match := numberRe.FindStringSubmatch(text)
	if match == nil {
		return 0, false
	}
	value, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", ""), 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

// slugID turns a display name such as "Sonar Reasoning Pro" into the
// identifier the API takes.
func slugID(name string) string {
	s := strings.ToLower(clean(name))
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '/' ||
			r == '.' {
			return r
		}
		return '-'
	}, s)
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

// authorOf returns the lab a brokered model is namespaced under.
func authorOf(id string) string {
	author, _, ok := strings.Cut(id, "/")
	if !ok {
		return ""
	}
	return author
}
