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

// States Voyage distinguishes. A legacy model is still served and still
// priced; it simply carries no free allowance and sits under a heading saying
// a newer model is better.
const (
	StateActive = "active"
	StateLegacy = "legacy"
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
	AttrHuggingFaceID    = "hugging_face_id"
	AttrTokenizer        = "tokenizer"
	AttrReleaseDate      = "release_date"
	AttrAnnouncementURL  = "announcement_url"
	AttrReplacement      = "recommended_replacement"
	AttrBatchWindow      = "batch_completion_window"
	AttrDeprecated       = "deprecated"
)

// noteOpenWeights records why the models Voyage publishes the weights of carry
// no rate. Voyage does not serve them, so there is nothing for it to charge
// for, and without this they would read as models it serves for free.
const noteOpenWeights = "weights published; Voyage states no rate " +
	"because it does not serve the model"

// Numeric keys the documents populate.
//
// The request bounds are stated per endpoint rather than per model, and are
// recorded against every model that endpoint serves. An input is one text for
// the embedding endpoint, one interleaved sequence for the multimodal one and
// one document for the reranker, which is why they share a key: the question
// a consumer asks of all three is how many items fit in one call.
const (
	LimitContextWindow  = "context_window"
	LimitChunkContext   = "chunk_context_window"
	LimitFreeTokens     = "free_tokens"
	LimitInputsPerReq   = "max_inputs_per_request"
	LimitTokensPerReq   = "max_tokens_per_request"
	LimitTokensPerInput = "max_tokens_per_input"
	LimitChunksPerReq   = "max_chunks_per_request"
	LimitQueryTokens    = "max_query_tokens"
	LimitImagePixels    = "max_image_pixels"
	LimitImageMB        = "max_image_megabytes"
	LimitVideoMB        = "max_video_megabytes"
	LimitPixelsPerToken = "pixels_per_token"
	LimitVideoPixels    = "video_pixels_per_token"
	LimitMinBillPixels  = "min_billable_pixels_per_image"
	LimitMaxBillPixels  = "max_billable_pixels_per_image"
	LimitFileRetention  = "file_retention_days"
	LimitRPM            = "requests_per_minute"
	LimitTPM            = "tokens_per_minute"
)

// Enumeration keys the documents populate.
const (
	ListDimensions       = "embedding_dimensions"
	ListInputModalities  = "input_modalities"
	ListOutputModalities = "output_modalities"
	ListFeatures         = catalog.ListFeatures
	ListParameters       = catalog.ListParameters
	ListEndpoints        = "endpoints"
	ListOutputDtypes     = "output_dtypes"
)

// Capabilities Voyage states per model. None of the canonical capability values
// applies to anything it sells: they describe what a model answering in words
// can do, and nothing here answers in words.
const (
	// FeatureReducibleDims is set where a model offers more than one width, so
	// that a caller can ask for a shorter vector than the default.
	FeatureReducibleDims = "reducible_embedding_dimensions"
	// FeatureQuantizedOutput is set where a model can return its vector in
	// something narrower than a 32-bit float.
	FeatureQuantizedOutput = "quantized_embeddings"
	// FeatureInputTypes is set where a model distinguishes a query from a
	// document, Voyage prepending a different retrieval prompt for each.
	FeatureInputTypes = "input_type_prompting"
	// FeatureTruncation is set where an over-length input is cut to fit
	// instead of being refused.
	FeatureTruncation = "input_truncation"
	// FeatureAutoChunking is set where the model will cut a whole document
	// into chunks itself rather than being handed them.
	FeatureAutoChunking = "auto_chunking"
	// FeatureInstructions is set where the query may carry an instruction
	// steering what counts as relevant.
	FeatureInstructions = "instruction_following"
)

// Modalities Voyage's capability pages account for.
const (
	ModalityText  = "text"
	ModalityImage = "image"
	ModalityVideo = "video"
)

// modalitiesFor reports what the models in one table take.
//
// Voyage states this by which page a model is documented on and nowhere else:
// the multimodal page is the one whose models vectorize text and pictures
// together, and every other page is text. MongoDB's overview gathers every
// family onto one page, where the same thing is said by the heading above a
// table rather than by the address of the page holding it.
func modalitiesFor(url, section string) []string {
	if strings.Contains(url, "multimodal") ||
		strings.Contains(section, "multimodal") {
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
// Both sides are set together, so a model that survives in the rate tables and
// in no model table at all carries neither. A consumer reading one side alone
// cannot tell an unstated modality from a model that takes or returns nothing.
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
	// quantizedRe matches the sentence naming the models that can return a
	// narrower vector. The list runs to the end of the line rather than to a
	// full stop, because two of the models named have a stop in their name.
	quantizedRe = regexp.MustCompile("(?i)`ubinary` are supported by ([^\n]+)")
	// videoInputRe matches the sentence naming the models that take video,
	// which the multimodal page states only to withhold it from the rest of
	// its own table.
	videoInputRe = regexp.MustCompile(
		`(?i)videos? inputs? (?:are|is) only supported by ([^\n]+)`,
	)
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

// modelIDs returns the identifiers in a passage that name a model. A sentence
// granting something to a list of models writes each of them as code, and
// writes the parameter it is talking about the same way.
func modelIDs(text string) []string {
	var out []string
	for _, id := range backtickedIDs(text) {
		if strings.HasPrefix(id, "voyage-") || strings.HasPrefix(id, "rerank") {
			out = append(out, id)
		}
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
