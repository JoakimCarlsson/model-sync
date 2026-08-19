package anthropic

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Metrics Anthropic bills on.
const (
	MetricInputTokens       catalog.Metric = "input_tokens"
	MetricCachedInputTokens catalog.Metric = "cached_input_tokens"
	MetricCacheWriteTokens  catalog.Metric = "cache_write_tokens"
	MetricOutputTokens      catalog.Metric = "output_tokens"
	MetricToolCall          catalog.Metric = "tool_call"
	MetricRuntime           catalog.Metric = "runtime"
)

// Units Anthropic quotes amounts against. MTok is its own word for a million
// tokens and appears inside the price cell rather than in a heading.
const (
	UnitPerMTok        catalog.Unit = "per_1m_tokens"
	UnitPer1KSearches  catalog.Unit = "per_1k_searches"
	UnitPerHour        catalog.Unit = "per_hour"
	UnitPerSessionHour catalog.Unit = "per_session_hour"
	// UnitPerRequest is the denominator of a tool Anthropic counts calls of
	// and charges nothing for.
	UnitPerRequest catalog.Unit = "per_request"
)

// Kinds of model Anthropic publishes.
const (
	KindChat catalog.Kind = "chat"
	KindTool catalog.Kind = "tool"
)

// Dimension keys Anthropic's prices vary along. Cache lifetime is a dimension
// here because a five minute write and a one hour write are different rates
// for the same metric.
const (
	DimTier     = "tier"
	DimCacheTTL = "cache_ttl"
)

// Service tiers Anthropic prices separately.
const (
	TierStandard = "standard"
	TierBatch    = "batch"
	TierFast     = "fast"
)

// Scalar keys the overview populates.
const (
	AttrSummary         = "summary"
	AttrReleaseDate     = "release_date"
	AttrAvailability    = "availability"
	AttrLatency         = "comparative_latency"
	AttrKnowledgeCutoff = "knowledge_cutoff"
	AttrTrainingCutoff  = "training_data_cutoff"
	AttrBedrockID       = "aws_bedrock_id"
	AttrVertexID        = "google_cloud_id"
)

// Scalar keys the thinking and effort guides populate. Anthropic offers two
// thinking modes and states per model which of them a request may ask for and
// which one it gets when it asks for nothing, so both are recorded: a model
// whose thinking is always on is a different thing from one that defaults to
// it and accepts being told not to.
const (
	AttrThinkingTypes   = "thinking_types"
	AttrThinkingDefault = "thinking_default"
	AttrDefaultEffort   = "default_effort"
)

// Numeric keys the overview populates.
const (
	LimitContextWindow   = "context_window"
	LimitMaxOutputTokens = "max_output_tokens"
	// LimitMaxOutputTokensBatch is the higher ceiling the Batches API allows
	// behind a beta header. It is a second key rather than a replacement
	// because the comparison table's Max output row is the synchronous
	// Messages API's and stays true of it.
	LimitMaxOutputTokensBatch = "max_output_tokens_batch"
)

// Numeric keys the rate limit page populates, one per usage tier. Anthropic
// names its tiers rather than numbering them and meters three counters rather
// than one, splitting input from output, so the key carries the tier's own
// name and the counter's own abbreviation instead of collapsing either.
const (
	LimitRPMPrefix  = "rpm_tier_"
	LimitITPMPrefix = "itpm_tier_"
	LimitOTPMPrefix = "otpm_tier_"
)

// Numeric keys the pricing page and the context window guide populate that are
// bounds rather than rates. The tool use system prompt is the block Anthropic
// prepends whenever a request carries any tool at all, and it is two sizes
// rather than one because a forced tool choice states more.
const (
	LimitToolSystemPromptAuto = "tool_use_system_prompt_tokens_auto"
	LimitToolSystemPromptAny  = "tool_use_system_prompt_tokens_any"
	LimitMaxImagesPerRequest  = "max_images_per_request"
)

// Enumeration keys the documents populate.
const (
	ListFeatures         = catalog.ListFeatures
	ListAliases          = "aliases"
	ListInputModalities  = "input_modalities"
	ListOutputModalities = "output_modalities"
	ListVersions         = "versions"
	ListParameters       = catalog.ListParameters
	// ListEffortLevels enumerates the values Anthropic states its effort
	// parameter accepts on a model. Two of the five are stated per model and
	// three are stated of the parameter, so the list is per model either way.
	ListEffortLevels = "effort_levels"
	// ListBetaHeaders holds the anthropic-beta values a tool requires. It is a
	// list because Anthropic keeps an older header working alongside a new
	// one.
	ListBetaHeaders = "beta_headers"
)

// Capabilities Anthropic states per model outside the comparison table, in the
// compatibility block each feature guide opens with or in a sentence naming
// the models a feature reaches. None of them is one of the catalog's canonical
// values, because none of them is a capability another vendor also publishes:
// they are Anthropic's own features, and the name here is Anthropic's own.
const (
	FeatureComputerUse  = "computer_use"
	FeatureCompaction   = "compaction"
	FeatureTaskBudgets  = "task_budgets"
	FeatureFastMode     = "fast_mode"
	FeaturePriorityTier = "priority_tier"
)

// modalityWords map a word of the overview's modality sentence onto the
// catalog's vocabulary.
var modalityWords = map[string]string{
	"text":  "text",
	"image": "image",
	"audio": "audio",
	"video": "video",
	"pdf":   "file",
}

var (
	linkRe   = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	mdxTagRe = regexp.MustCompile(`(?s)<[^>]*>`)
	priceRe  = regexp.MustCompile(
		`\$([\d,]+(?:\.\d+)?)\s*(?:/\s*([A-Za-z][\w \-]*))?`,
	)
	// modalitySentenceRe matches the sentence stating what every current model
	// takes and returns, which the comparison table has no row for.
	modalitySentenceRe = regexp.MustCompile(
		`(?i)all current claude models support ([^.]+)\.`,
	)
	// modalityClauseRe matches one clause of that sentence, which names the
	// direction after the modalities travelling in it.
	modalityClauseRe = regexp.MustCompile(`(?i)([a-z ,]+?)(input|output)\b`)
	// sharedSpecsRe matches the sentence giving a model's bounds by naming
	// another model rather than by stating them. It is how Anthropic documents
	// a model held back from general release: Claude Mythos 5 has no column in
	// the comparison table and is described only as sharing Claude Fable 5's.
	sharedSpecsRe = regexp.MustCompile(
		"(?i)\\(`([a-z0-9.-]+)`\\) shares ([A-Z][A-Za-z0-9. ]+?)'s specs",
	)
	// batchOutputRe matches the note raising the output ceiling on the Batches
	// API, which the comparison table does not cover: its Max output row is
	// the synchronous API's, and this note names the models that exceed it
	// there and by how much.
	batchOutputRe = regexp.MustCompile(
		`(?i)Message Batches API[^,]*, (.+?) support up to ([\d.,]+[kKmM]?) output tokens`,
	)
	// wordRe matches one word of such a clause.
	wordRe         = regexp.MustCompile(`[a-z]+`)
	footnoteRe     = regexp.MustCompile(`^(.*[a-zA-Z)])\d{1,2}$`)
	footnoteYearRe = regexp.MustCompile(`^(.*\d{4})\d{1,2}$`)
	tokenSizeRe    = regexp.MustCompile(`(?i)^([\d.,]+)\s*([kKmM])?\s*tokens?$`)
	ordinalRe      = regexp.MustCompile(`(\d{1,2})(st|nd|rd|th)\b`)
	countRe        = regexp.MustCompile(`^([\d,]+)`)
)

// stripOrdinal removes the English ordinal suffix Anthropic wrote its earliest
// changelog headings with, turning "February 27th, 2025" into a date the
// layouts recognize. Later headings carry no suffix and pass through
// untouched.
func stripOrdinal(value string) string {
	return ordinalRe.ReplaceAllString(value, "$1")
}

// parseCount reads a plain quantity such as "5,000" or "286 tokens", which is
// how the rate limit and tool overhead tables write a number that is a count
// rather than a size.
func parseCount(cell string) int64 {
	match := countRe.FindStringSubmatch(clean(cell))
	if match == nil {
		return 0
	}
	n, err := strconv.ParseInt(strings.ReplaceAll(match[1], ",", ""), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// clean strips the decoration Anthropic wraps around cell values: markdown
// links, MDX elements such as Tooltip, bold markers, and backticks.
func clean(cell string) string {
	s := linkRe.ReplaceAllString(cell, "$1")
	s = mdxTagRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, `\`, "")
	return strings.Join(strings.Fields(s), " ")
}

// dropFootnote removes the footnote marker Anthropic appends directly to a
// value with no separator, which turns "May 2026" into "May 20262" and
// "Pricing" into "Pricing1". The year form is tried first: a marker after a
// four digit year leaves five or more digits, whereas an unmarked year leaves
// exactly four and is returned untouched.
func dropFootnote(value string) string {
	for _, re := range []*regexp.Regexp{footnoteYearRe, footnoteRe} {
		if match := re.FindStringSubmatch(value); match != nil {
			return strings.TrimSpace(match[1])
		}
	}
	return value
}

// dropIDFootnote removes a footnote marker from a cloud identifier, but only
// when removing it leaves a string ending in the model's API identifier. An
// identifier that genuinely ends in digits, such as one suffixed -v1:0, does
// not match and is left alone.
func dropIDFootnote(value, apiID string) string {
	for n := 1; n <= 2 && n < len(value); n++ {
		candidate := value[:len(value)-n]
		if strings.HasSuffix(candidate, apiID) {
			return candidate
		}
	}
	return value
}

// dateLayouts are the date formats Anthropic writes, paired with the precision
// to render each at. Lifecycle dates carry a day, cutoffs only a month.
var dateLayouts = []struct{ in, out string }{
	{"2006-01-02", "2006-01-02"},
	{"January 2, 2006", "2006-01-02"},
	{"Jan 2, 2006", "2006-01-02"},
	{"January 2006", "2006-01"},
	{"Jan 2006", "2006-01"},
}

// isoDate rewrites a date into its machine readable form, keeping the
// precision it was written at, so "February 19, 2026" becomes 2026-02-19 and
// "May 2026" becomes 2026-05. A value in no recognized format is returned
// unchanged rather than dropped.
func isoDate(value string) string {
	text := strings.TrimSpace(value)
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout.in, text); err == nil {
			return t.Format(layout.out)
		}
	}
	return text
}

// retirementHedges are the ways Anthropic says a retirement date is a floor
// rather than a commitment.
var retirementHedges = []string{"not sooner than", "no sooner than"}

// splitRetirement separates a retirement date from the hedge in front of it,
// so that "Not sooner than June 9, 2027" yields a usable date and keeps the
// fact that the date can move.
func splitRetirement(value string) (date, hedge string) {
	text := strings.TrimSpace(value)
	for _, prefix := range retirementHedges {
		if len(text) > len(prefix) &&
			strings.EqualFold(text[:len(prefix)], prefix) {
			return isoDate(text[len(prefix):]), prefix
		}
	}
	return isoDate(text), ""
}

// unitFor maps Anthropic's denominator wording onto a unit.
func unitFor(text string) (catalog.Unit, bool) {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "mtok", "mtoks", "m tok":
		return UnitPerMTok, true
	case "hour", "hr":
		return UnitPerHour, true
	case "session-hour", "session hour":
		return UnitPerSessionHour, true
	}
	return "", false
}

// amount is one parsed price cell.
type amount struct {
	Value float64
	Unit  catalog.Unit
	Found bool
}

// parseAmount reads a price cell such as "$12.50 / MTok". A cell holding no
// amount, which Anthropic writes as an em dash or leaves empty, is not found.
func parseAmount(cell string) amount {
	match := priceRe.FindStringSubmatch(clean(cell))
	if match == nil {
		return amount{}
	}
	value, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", ""), 64)
	if err != nil {
		return amount{}
	}
	out := amount{Value: value, Found: true}
	if unit, ok := unitFor(match[2]); ok {
		out.Unit = unit
	}
	return out
}

// parseTokenCount reads a token quantity such as "1M tokens", "200k tokens" or
// "128k tokens".
func parseTokenCount(value string) int64 {
	match := tokenSizeRe.FindStringSubmatch(clean(value))
	if match == nil {
		return 0
	}
	n, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", ""), 64)
	if err != nil {
		return 0
	}
	switch strings.ToLower(match[2]) {
	case "k":
		n *= 1_000
	case "m":
		n *= 1_000_000
	}
	return int64(n)
}

// FeatureReasoning is the catalog's word for a model that thinks before it
// answers. Anthropic offers two kinds of it and names neither this.
const FeatureReasoning = catalog.CapabilityReasoning

// slugID turns a display name such as "Claude Opus 4.1" into the identifier
// form Anthropic uses for aliases, which replaces the dot in a version with a
// dash. It is the fallback for models the overview does not list.
func slugID(name string) string {
	s := strings.ToLower(clean(name))
	s = strings.NewReplacer(".", "-", " ", "-", "/", "-").Replace(s)
	return strings.Trim(s, "-")
}

// splitNames reads a prose list of models, which Anthropic writes with a comma
// between each and "and" before the last, naming the vendor once and shortening
// every name after the first: "Claude Opus 5, Opus 4.8, and Sonnet 4.6". The
// shortened forms resolve anyway, because the index they are looked up in drops
// the vendor prefix.
//
// A list of two has no comma at all, only the conjunction, so the conjunction
// is a separator in its own right rather than a prefix to strip off the last
// name. No model Anthropic names contains the word.
func splitNames(text string) []string {
	var out []string
	for _, part := range strings.Split(clean(text), ",") {
		for _, piece := range strings.Split(part, " and ") {
			if name := strings.TrimSpace(piece); name != "" {
				out = append(out, name)
			}
		}
	}
	return out
}

// modelRef is one model named by a pricing row, together with whatever the
// trailing parenthesis said about it.
type modelRef struct {
	Name         string
	Availability string
}

// splitModelCell reads a model cell. Anthropic decorates the name with a
// parenthesized availability note that is usually a link, and lists two models
// separated by a slash when they share a rate.
func splitModelCell(cell string) []modelRef {
	text := clean(cell)
	availability := ""
	if open := strings.Index(text, "("); open >= 0 &&
		strings.HasSuffix(text, ")") {
		availability = strings.TrimSpace(text[open+1 : len(text)-1])
		text = strings.TrimSpace(text[:open])
	}
	var refs []modelRef
	for _, part := range strings.Split(text, " / ") {
		if name := strings.TrimSpace(part); name != "" {
			refs = append(refs, modelRef{
				Name:         name,
				Availability: availability,
			})
		}
	}
	return refs
}
