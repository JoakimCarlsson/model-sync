package deepseek

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Metrics DeepSeek bills on.
const (
	MetricInputTokens       catalog.Metric = "input_tokens"
	MetricCachedInputTokens catalog.Metric = "cached_input_tokens"
	MetricOutputTokens      catalog.Metric = "output_tokens"
)

// UnitPer1MTokens is the only denominator DeepSeek quotes.
const UnitPer1MTokens catalog.Unit = "per_1m_tokens"

// KindChat is the only kind DeepSeek publishes.
const KindChat catalog.Kind = "chat"

// Scalar keys the pricing page populates.
const (
	AttrModelVersion = "model_version"
	AttrThinkingMode = "thinking_mode"
)

// Numeric keys the pricing page populates.
const (
	LimitContextWindow   = "context_window"
	LimitMaxOutputTokens = "max_output_tokens"
	LimitConcurrency     = "concurrency_limit"
)

// Enumeration keys the pricing table populates.
const (
	// ListFeatures holds the capabilities marked as supported.
	ListFeatures = catalog.ListFeatures
	// ListEndpoints holds the APIs a model answers on, which DeepSeek marks
	// as supported in the same column as its capabilities.
	ListEndpoints = "endpoints"
)

// Enumeration keys the Responses API guide populates.
const (
	ListInputModalities  = "input_modalities"
	ListOutputModalities = "output_modalities"
)

// Modalities DeepSeek names a content part for.
const (
	ModalityText  = "text"
	ModalityImage = "image"
)

// featureNames map a row label onto the catalog's vocabulary. DeepSeek heads a
// row with prose, so the label is not an identifier and is translated into
// one; anything not listed keeps DeepSeek's own words with its punctuation and
// spacing reduced.
var featureNames = map[string]string{
	"tool calls":                   catalog.CapabilityFunctionCalling,
	"json output":                  catalog.CapabilityStructuredOutputs,
	"chat prefix completion（beta）": "prefix",
}

// endpointLabels are the rows naming an API a model answers on rather than
// something the model can do.
var endpointLabels = map[string]string{
	"anthropic api": "Anthropic",
	"responses api": "Responses",
}

// labelWordRe matches whatever in a row label is not part of an identifier,
// including the full-width brackets DeepSeek writes a qualifier in.
var labelWordRe = regexp.MustCompile(`[^a-z0-9]+`)

// featureName rewrites a row label into the catalog's vocabulary.
func featureName(label string) string {
	if name, ok := featureNames[label]; ok {
		return name
	}
	return strings.Trim(labelWordRe.ReplaceAllString(label, "_"), "_")
}

// supported is the mark DeepSeek uses for a capability a model has.
const supported = "✓"

// rateRows maps a row label onto what that row's amounts are charged for.
var rateRows = map[string]catalog.Metric{
	"1m input tokens (cache hit)":  MetricCachedInputTokens,
	"1m input tokens (cache miss)": MetricInputTokens,
	"1m output tokens":             MetricOutputTokens,
}

var (
	rowRe    = regexp.MustCompile(`(?is)<tr[^>]*>(.*?)</tr>`)
	cellRe   = regexp.MustCompile(`(?is)<t[hd][^>]*>(.*?)</t[hd]\s*>`)
	tagRe    = regexp.MustCompile(`(?s)<[^>]*>`)
	amountRe = regexp.MustCompile(`\$\s*([\d,]*\.?\d+)`)
	countRe  = regexp.MustCompile(`(?i)([\d,]*\.?\d+)\s*([km])?`)
)

// text strips markup and collapses whitespace.
func text(html string) string {
	return strings.Join(strings.Fields(tagRe.ReplaceAllString(html, " ")), " ")
}

// parseAmount reads a rate cell.
func parseAmount(cell string) (float64, bool) {
	match := amountRe.FindStringSubmatch(cell)
	if match == nil {
		return 0, false
	}
	value, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", ""), 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

// parseCount reads a quantity such as "1M" or "384K".
func parseCount(cell string) int64 {
	match := countRe.FindStringSubmatch(cell)
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

// applyPricing reads the pricing page.
//
// The page carries two tables and only the first describes models. It is laid
// out with a model per column, so its heading row names them and every row
// below states one fact about each. The second is the off-peak discount table,
// which transposes that: it heads its columns with the denominations and its
// rows with the models. Both tables head their first cell "MODEL", so reading
// past the first would take the denominations for models and enter three of
// them into the catalog.
//
// The second heading therefore ends the reading. What it introduces is a
// discount this parser does not record.
func (b *builder) applyPricing(doc catalog.Document) {
	var ids []string
	for _, match := range rowRe.FindAllStringSubmatch(string(doc.Body), -1) {
		cells := rowCells(match[1])
		if len(cells) < 2 {
			continue
		}
		if strings.EqualFold(cells[0], "model") {
			if len(ids) > 0 {
				return
			}
			ids = cells[1:]
			for _, id := range ids {
				b.model(id, KindChat).AddSource(doc.URL)
			}
			continue
		}
		if len(ids) == 0 {
			continue
		}
		b.applyRow(cells, ids)
	}
}

// applyRow records one fact about every model.
//
// The row is read from the right because a spanning section label can precede
// the row's own label, so the position of the values is fixed but the position
// of the label is not.
func (b *builder) applyRow(cells, ids []string) {
	count := min(len(ids), len(cells)-1)
	if count < 1 {
		return
	}
	values := cells[len(cells)-count:]
	label := rowLabel(cells[len(cells)-count-1])
	for i, id := range ids {
		value := values[0]
		if i < len(values) {
			value = values[i]
		}
		b.applyValue(id, label, value)
	}
}

// applyValue records one cell against one model.
func (b *builder) applyValue(id, label, value string) {
	if value == "" {
		return
	}
	m := b.model(id, KindChat)
	if metric, ok := rateRows[label]; ok {
		if amount, ok := parseAmount(value); ok {
			m.AddPrice(catalog.Price{
				Metric:   metric,
				Unit:     UnitPer1MTokens,
				Amount:   amount,
				Currency: currency,
			})
		}
		return
	}
	switch label {
	case "model version":
		m.SetAttr(AttrModelVersion, value)
	case "thinking mode":
		m.SetAttr(AttrThinkingMode, value)
		if thinkingRe.MatchString(value) {
			m.AddList(ListFeatures, catalog.CapabilityReasoning)
		}
	case "context length":
		m.SetLimit(LimitContextWindow, parseCount(value))
	case "max output":
		m.SetLimit(LimitMaxOutputTokens, parseCount(value))
	case "concurrency limit":
		m.SetLimit(LimitConcurrency, parseCount(value))
	default:
		if !strings.Contains(value, supported) || label == "" {
			return
		}
		if endpoint, ok := endpointLabels[label]; ok {
			m.AddList(ListEndpoints, endpoint)
			return
		}
		m.AddList(ListFeatures, featureName(label))
	}
}

// thinkingRe matches a thinking mode row stating that the model thinks.
//
// The row is prose rather than a tick, because DeepSeek has more to say than
// yes: its models support thinking and non-thinking modes and default to one
// of them. The whole sentence is kept as an attribute, and this reads the part
// of it that is the capability every other provider states in one word.
var thinkingRe = regexp.MustCompile(`(?i)\bsupports\b.*\bthinking\b`)

// footnoteRe matches the reference marker DeepSeek appends to a row label.
var footnoteRe = regexp.MustCompile(`\(\d+\)$`)

// rowLabel normalizes the cell naming a row, dropping the footnote marker that
// would otherwise make "Concurrency Limit(2)" a different label from the one
// it is.
func rowLabel(cell string) string {
	return strings.ToLower(
		strings.TrimSpace(
			footnoteRe.ReplaceAllString(strings.TrimSpace(cell), ""),
		),
	)
}

// headingRe matches a section heading of the change log, which is where
// DeepSeek writes a model's name.
var headingRe = regexp.MustCompile(`(?is)<h[23][^>]*>(.*?)</h[23]\s*>`)

// anchorRe matches the permalink the site puts inside every heading, which is
// not part of the heading's text.
var anchorRe = regexp.MustCompile(`(?is)<a\b.*?</a\s*>`)

// updateSuffix is what the change log appends to a heading naming a model.
const updateSuffix = " update"

// applyChangeLog reads each model's name.
//
// The pricing table heads its columns with the identifier, so the name is not
// on it. The change log heads the entry for a release with the model's name,
// written as DeepSeek writes it rather than as the identifier is spelled, and
// a heading is taken only where it is the identifier of a model the pricing
// page already stated. That is what keeps "DeepSeek-V4", the heading of the
// release that introduced both models, and the headings of the models
// withdrawn before them, from naming anything.
func (b *builder) applyChangeLog(doc catalog.Document) {
	for _, match := range headingRe.FindAllStringSubmatch(string(doc.Body), -1) {
		name := text(anchorRe.ReplaceAllString(match[1], ""))
		id := strings.TrimSuffix(strings.ToLower(name), updateSuffix)
		m, ok := b.models[id]
		if !ok || m.Name != "" {
			continue
		}
		m.Name = name[:len(id)]
		m.AddSource(doc.URL)
	}
}

// contentParts map a content part onto the modality it carries and the
// enumeration that modality belongs in.
var contentParts = map[string]struct {
	list     string
	modality string
}{
	"input_text":  {ListInputModalities, ModalityText},
	"input_image": {ListInputModalities, ModalityImage},
	"output_text": {ListOutputModalities, ModalityText},
}

// messageRow heads the row of the input item table stating what a message
// carries, and unsupported marks a sentence denying support.
const (
	messageRow  = "message"
	unsupported = "not supported"
)

// applyResponsesGuide reads what the models take and what they return.
//
// DeepSeek states no modality against a model, because both models answer the
// one API. What it does state is the content parts that API carries, in a
// table of input items whose message row names input_text and output_text and
// then says, in the sentence after, that image and file inputs are not
// supported. The row is therefore read a sentence at a time and a sentence
// denying support is skipped, so that the input_image named inside that one is
// not read as something the models accept.
func (b *builder) applyResponsesGuide(doc catalog.Document) {
	for _, match := range rowRe.FindAllStringSubmatch(string(doc.Body), -1) {
		cells := rowCells(match[1])
		if len(cells) < 2 || !strings.EqualFold(cells[0], messageRow) {
			continue
		}
		for _, sentence := range strings.Split(cells[1], ". ") {
			if strings.Contains(strings.ToLower(sentence), unsupported) {
				continue
			}
			b.applyContentParts(doc.URL, sentence)
		}
	}
}

// applyContentParts records the modality of every content part one sentence
// names, against every model, since the sentence describes the API rather than
// a model.
func (b *builder) applyContentParts(source, sentence string) {
	for name, part := range contentParts {
		if !strings.Contains(sentence, name) {
			continue
		}
		for _, m := range b.models {
			m.AddList(part.list, part.modality)
			m.AddSource(source)
		}
	}
}

// rowCells returns the text of one row's cells.
func rowCells(row string) []string {
	matches := cellRe.FindAllStringSubmatch(row, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, text(m[1]))
	}
	return out
}
