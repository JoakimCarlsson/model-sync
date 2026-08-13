package google

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Metrics Google bills on.
const (
	MetricInputTokens       catalog.Metric = "input_tokens"
	MetricOutputTokens      catalog.Metric = "output_tokens"
	MetricCachedInputTokens catalog.Metric = "cached_input_tokens"
	MetricVideoOutput       catalog.Metric = "video_output"
	MetricImageOutput       catalog.Metric = "image_output"
	MetricToolCall          catalog.Metric = "tool_call"
)

// Units Google quotes amounts against.
const (
	UnitPer1MTokens   catalog.Unit = "per_1m_tokens"
	UnitPerSecond     catalog.Unit = "per_second"
	UnitPerImage      catalog.Unit = "per_image"
	UnitPer1KRequests catalog.Unit = "per_1k_requests"
)

// DimModality separates the rates a model charges for different kinds of
// input, which Google states as separate rows on one table.
const DimModality = "modality"

// Kinds on the Gemini pricing page. It is not only chat: the same page prices
// image and video generation, embeddings and speech.
const (
	KindChat      catalog.Kind = "chat"
	KindImage     catalog.Kind = "image"
	KindVideo     catalog.Kind = "video"
	KindEmbedding catalog.Kind = "embedding"
	KindSpeech    catalog.Kind = "speech"
	KindAudio     catalog.Kind = "audio"
)

// nameKinds map a fragment of a model's name onto what it does, for the models
// whose rates alone do not say. An embedding and a chat model are both charged
// per input token.
var nameKinds = []struct {
	fragment string
	kind     catalog.Kind
}{
	{"embedding", KindEmbedding},
	{"tts", KindSpeech},
	{"image", KindImage},
	{"video", KindVideo},
	{"audio", KindAudio},
}

// metricKinds map a rate onto what the model producing it does. A model
// charged for image output makes images whatever its name suggests.
var metricKinds = map[catalog.Metric]catalog.Kind{
	MetricImageOutput: KindImage,
	MetricVideoOutput: KindVideo,
}

// refineKind settles what a model is once a rate has been read for it. It only
// ever replaces the chat default, so the first specific reading wins.
func refineKind(m *catalog.Model, metric catalog.Metric) {
	if m.Kind != "" && m.Kind != KindChat {
		return
	}
	if kind, ok := metricKinds[metric]; ok {
		m.Kind = kind
		return
	}
	for _, entry := range nameKinds {
		if strings.Contains(m.ID, entry.fragment) {
			m.Kind = entry.kind
			return
		}
	}
}

// Dimension keys Google's prices vary along. A rate needs both: the tier is
// the serving path and the plan is what the account pays for.
const (
	DimTier = "tier"
	DimPlan = "plan"
)

// Plans the columns price.
const (
	PlanFree = "free"
	PlanPaid = "paid"
)

// tiers are the serving paths Google prices separately, each introduced by its
// own heading under a model.
var tiers = map[string]string{
	"standard": "standard",
	"batch":    "batch",
	"flex":     "flex",
	"priority": "priority",
}

// modelHeadingRe matches the heading that introduces a model rather than a
// tier or a page section.
var modelHeadingRe = regexp.MustCompile(`(?i)^(gemini|imagen|veo|gemma)\b`)

// rowMetrics maps a fragment of a row label onto what the row's amounts are
// charged for. The fragments are checked in order, since a label can contain
// more than one and the earlier entries are the more specific reading.
var rowMetrics = []struct {
	fragment string
	metric   catalog.Metric
}{
	{"context caching", MetricCachedInputTokens},
	{"grounding with", MetricToolCall},
	{"input price", MetricInputTokens},
	{"output price", MetricOutputTokens},
	{"video", MetricVideoOutput},
	{"image price", MetricImageOutput},
}

// headerUnits maps the denominator stated in a plan heading onto a unit.
// Google writes it there rather than in the row, so one page states rates per
// million tokens, per second, per image and per request.
var headerUnits = []struct {
	fragment string
	unit     catalog.Unit
}{
	{"per 1m tokens", UnitPer1MTokens},
	{"per second", UnitPerSecond},
	{"per image", UnitPerImage},
	{"per request", UnitPer1KRequests},
}

// modalities are the inputs Google prices separately on one model.
var modalities = []string{"text", "image", "audio", "video"}

// variants are the quality levels Google prices separately within one model,
// naming them in the row label rather than as a model of their own.
var variants = []string{"standard", "fast", "lite", "ultra", "pro"}

// DimVariant separates those levels, and DimResolution the output sizes a
// single rate cell can price differently.
const (
	DimVariant    = "variant"
	DimResolution = "resolution"
)

var (
	headingRe = regexp.MustCompile(`(?is)<h[234][^>]*>(.*?)</h[234]>`)
	rowRe     = regexp.MustCompile(`(?is)<tr[^>]*>(.*?)</tr>`)
	cellRe    = regexp.MustCompile(`(?is)<t[hd][^>]*>(.*?)</t[hd]\s*>`)
	tagRe     = regexp.MustCompile(`(?s)<[^>]*>`)
	amountRe  = regexp.MustCompile(`\$\s*([\d,]*\.?\d+)`)
)

// freeOfCharge is how Google writes a rate of zero.
const freeOfCharge = "free of charge"

// text strips markup and collapses whitespace.
func text(html string) string {
	return strings.Join(strings.Fields(tagRe.ReplaceAllString(html, " ")), " ")
}

// slugID turns a heading such as "Gemini 3.6 Flash" into an identifier.
func slugID(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' {
			return r
		}
		return '-'
	}, s)
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

// applyPricing reads the pricing page as a running state of which model and
// which tier the current table belongs to.
func (b *builder) applyPricing(doc catalog.Document) {
	var (
		body    = string(doc.Body)
		model   string
		tier    string
		columns []column
	)
	for _, at := range marks(body) {
		switch {
		case at.heading != "":
			if modelHeadingRe.MatchString(at.heading) {
				model, tier = slugID(at.heading), ""
				b.model(model, KindChat).AddSource(doc.URL)
				continue
			}
			if name, ok := tiers[strings.ToLower(at.heading)]; ok {
				tier = name
			}
		case model != "":
			cells := rowCells(at.row)
			if len(cells) < 2 {
				continue
			}
			if header, ok := planHeader(cells); ok {
				columns = fillUnits(header)
				continue
			}
			b.applyRow(model, tier, columns, cells)
		}
	}
}

// fillUnits gives every column of a table the denominator its rates are quoted
// against.
//
// Google states it in one column's heading only: "Paid Tier, per 1M tokens in
// USD" beside a bare "Free Tier". Both columns price the same rows, so the one
// denominator the table states is the denominator of all of them. Without this
// the free plan's rates are dropped for want of a unit, which is why every
// model Google gives away read as unpriced.
func fillUnits(columns []column) []column {
	stated := catalog.Unit("")
	for _, col := range columns {
		if col.unit != "" {
			stated = col.unit
			break
		}
	}
	for i := range columns {
		if columns[i].unit == "" {
			columns[i].unit = stated
		}
	}
	return columns
}

// column is one plan and the denominator its heading states.
type column struct {
	plan string
	unit catalog.Unit
}

// planHeader reports whether a row names the plans its columns price, and what
// each states rates against.
func planHeader(cells []string) ([]column, bool) {
	if strings.TrimSpace(cells[0]) != "" {
		return nil, false
	}
	columns := make([]column, 0, len(cells)-1)
	for _, c := range cells[1:] {
		lower := strings.ToLower(c)
		col := column{plan: slugID(c), unit: unitForHeader(c)}
		switch {
		case strings.Contains(lower, "free"):
			col.plan = PlanFree
		case strings.Contains(lower, "paid"):
			col.plan = PlanPaid
		}
		columns = append(columns, col)
	}
	return columns, true
}

// applyRow records one row's amounts against each plan.
func (b *builder) applyRow(
	model, tier string,
	columns []column,
	cells []string,
) {
	label := strings.ToLower(cells[0])
	metric, ok := metricFor(label)
	if !ok {
		return
	}
	m := b.model(model, KindChat)
	for i, cell := range cells[1:] {
		col := column{}
		if i < len(columns) {
			col = columns[i]
		}
		if col.unit == "" {
			continue
		}
		dims := catalog.Dims{}.
			With(DimTier, tier).
			With(DimPlan, col.plan).
			With(DimModality, modalityOf(label)).
			With(DimVariant, variantOf(label))
		refineKind(m, metric)
		for _, r := range parseRates(cell) {
			m.AddPrice(catalog.Price{
				Metric:   metric,
				Unit:     col.unit,
				Amount:   r.amount,
				Currency: currency,
				Dims:     dims.With(DimResolution, r.resolution),
				Note:     r.note,
			})
		}
	}
}

// variantOf reports the quality level a row prices, for the models Google
// prices at several levels under one name.
func variantOf(label string) string {
	for _, variant := range variants {
		if strings.Contains(label, " "+variant+" ") {
			return variant
		}
	}
	return ""
}

// metricFor maps a row label onto what its amounts are charged for.
func metricFor(label string) (catalog.Metric, bool) {
	for _, entry := range rowMetrics {
		if strings.Contains(label, entry.fragment) {
			return entry.metric, true
		}
	}
	return "", false
}

// unitForHeader reads the denominator out of a plan heading.
func unitForHeader(header string) catalog.Unit {
	lower := strings.ToLower(header)
	for _, entry := range headerUnits {
		if strings.Contains(lower, entry.fragment) {
			return entry.unit
		}
	}
	return ""
}

// modalityOf reports which input a row prices, for the models that charge
// differently by modality.
func modalityOf(label string) string {
	for _, modality := range modalities {
		if strings.HasPrefix(label, modality+" input price") {
			return modality
		}
	}
	return ""
}

// rate is one amount from a cell, with the output size it applies to when the
// cell prices several.
type rate struct {
	amount     float64
	resolution string
	note       string
}

// qualifierRe matches the parenthesis Google puts after an amount to say which
// output size it buys, as in "$0.40 (720p and 1080p) $0.60 (4k)".
var qualifierRe = regexp.MustCompile(`^\s*\(([^)]*)\)`)

// parseRates reads every amount in a cell, pairing each with the size that
// follows it. A cell stating one amount yields one rate with no size, and a
// cell reading "Free of charge" yields the rate of zero it means.
func parseRates(cell string) []rate {
	if amount, ok := parseAmount(cell); ok && !strings.Contains(cell, "$") {
		return []rate{{amount: amount}}
	}
	locations := amountRe.FindAllStringSubmatchIndex(cell, -1)
	out := make([]rate, 0, len(locations))
	for _, at := range locations {
		value, err := strconv.ParseFloat(
			strings.ReplaceAll(cell[at[2]:at[3]], ",", ""),
			64,
		)
		if err != nil {
			continue
		}
		r := rate{amount: value}
		rest := cell[at[1]:]
		if match := qualifierRe.FindStringSubmatch(rest); match != nil {
			r.resolution = slugID(match[1])
		} else if len(locations) == 1 {
			r.note = extraOf(cell)
		}
		out = append(out, r)
	}
	return out
}

// parseAmount reads the first amount in a cell, treating Google's "Free of
// charge" as the rate of zero it is rather than as a missing value.
func parseAmount(cell string) (float64, bool) {
	lower := strings.ToLower(cell)
	if strings.Contains(lower, freeOfCharge) {
		return 0, true
	}
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

// extraOf keeps whatever a cell says beyond its first amount, which is where
// Google puts a storage rate or a free allowance.
func extraOf(cell string) string {
	at := amountRe.FindStringIndex(cell)
	if at == nil {
		return ""
	}
	rest := strings.TrimSpace(cell[at[1]:])
	if !strings.Contains(rest, "$") && !strings.Contains(rest, "free") {
		return ""
	}
	return rest
}

// mark is either a heading or a table row, in the order the page states them.
type mark struct {
	heading string
	row     string
}

// marks returns the page's headings and rows in document order, which is what
// ties a table to the model and tier named above it.
func marks(body string) []mark {
	type located struct {
		at   int
		mark mark
	}
	var found []located
	for _, m := range headingRe.FindAllStringSubmatchIndex(body, -1) {
		found = append(found, located{
			at:   m[0],
			mark: mark{heading: text(body[m[2]:m[3]])},
		})
	}
	for _, m := range rowRe.FindAllStringSubmatchIndex(body, -1) {
		found = append(found, located{
			at:   m[0],
			mark: mark{row: body[m[2]:m[3]]},
		})
	}
	slices.SortStableFunc(found, func(a, b located) int { return a.at - b.at })
	out := make([]mark, 0, len(found))
	for _, f := range found {
		out = append(out, f.mark)
	}
	return out
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
