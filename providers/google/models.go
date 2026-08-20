package google

import (
	"html"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Metrics Google bills on.
const (
	MetricInputTokens       catalog.Metric = "input_tokens"
	MetricOutputTokens      catalog.Metric = "output_tokens"
	MetricCachedInputTokens catalog.Metric = "cached_input_tokens"
	MetricVideoOutput       catalog.Metric = "video_output"
	MetricImageOutput       catalog.Metric = "image_output"
	MetricAudioOutput       catalog.Metric = "audio_output"
	MetricToolCall          catalog.Metric = "tool_call"
	// MetricCacheStorage is what Google charges for holding a cache rather
	// than for reading it, which it states inside the same cell as the read
	// rate and against a denominator of its own.
	MetricCacheStorage catalog.Metric = "cache_storage_tokens"
)

// Units Google quotes amounts against.
const (
	UnitPer1MTokens   catalog.Unit = "per_1m_tokens"
	UnitPerSecond     catalog.Unit = "per_second"
	UnitPerMinute     catalog.Unit = "per_minute"
	UnitPerImage      catalog.Unit = "per_image"
	UnitPerRequest    catalog.Unit = "per_request"
	UnitPer1KRequests catalog.Unit = "per_1k_requests"
	// UnitPerFrame is what a video is counted in where Google prices reading
	// one rather than generating it.
	UnitPerFrame catalog.Unit = "per_frame"
	// UnitPer1MTokensPerHour is the denominator a cache is held against, time
	// being part of what a storage rate is charged on.
	UnitPer1MTokensPerHour catalog.Unit = "per_1m_tokens_per_hour"
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
	KindMusic     catalog.Kind = "music"
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
	{"lyria", KindMusic},
	{"image", KindImage},
	{"video", KindVideo},
	{"audio", KindAudio},
}

// kindMetrics name what a model of each kind sells, for the rows that price a
// model rather than one facet of its billing.
var kindMetrics = map[catalog.Kind]catalog.Metric{
	KindMusic: MetricAudioOutput,
}

// nameKind reads what a model does out of the identifier it answers to, which
// is what settles the models no rate of theirs distinguishes. Everything else
// is chat until a rate says otherwise.
func nameKind(id string) catalog.Kind {
	for _, entry := range nameKinds {
		if strings.Contains(id, entry.fragment) {
			return entry.kind
		}
	}
	return KindChat
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
	m.Kind = nameKind(m.ID)
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
var modelHeadingRe = regexp.MustCompile(
	`(?i)^(gemini|imagen|veo|gemma|lyria)\b`,
)

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
// million tokens, per second, per image and per request. The fragments are
// checked in order, since Google quotes a request rate by the thousand as
// often as singly and the longer reading is the one meant.
var headerUnits = []struct {
	fragment string
	unit     catalog.Unit
}{
	{"per 1m tokens", UnitPer1MTokens},
	{"per second", UnitPerSecond},
	{"per image", UnitPerImage},
	{"per 1,000 requests", UnitPer1KRequests},
	{"per 1k requests", UnitPer1KRequests},
	{"per request", UnitPerRequest},
}

// modalities are the inputs Google prices separately on one model.
var modalities = []string{"text", "image", "audio", "video"}

// variants are the quality levels Google prices separately under one heading,
// naming them in the row label rather than in a heading of their own.
var variants = []string{"standard", "fast", "lite", "ultra", "pro", "clip"}

// DimVariant separates those levels, and DimResolution the output sizes a
// single rate cell can price differently.
const (
	DimVariant    = "variant"
	DimResolution = "resolution"
)

// DimTool names the tool a grounding row prices. Google prices two under one
// model at the same rate against the same denominator, and without the tool
// the two rows are the same row twice.
const DimTool = "tool"

// groundingPrefix opens the label of every row pricing a tool rather than the
// model itself.
const groundingPrefix = "grounding with "

// DimContextBand separates the two rates a model charges by how long the
// prompt is, which Google states beside the amount rather than in a column.
const DimContextBand = "context_band"

// DimEffectiveUntil and DimEffectiveFrom carry the dates an introductory rate
// runs between. Google prices several models twice in one cell, once through
// the end of the promotion and once from the day after, and the two amounts
// are one rate at two times rather than two rates.
const (
	DimEffectiveUntil = "effective_until"
	DimEffectiveFrom  = "effective_from"
)

// cellUnits map a denominator stated beside an amount onto a unit. A column
// heading states the denominator of the rows below it, and a cell overrides it
// where what the amount buys is not what the column counts: a grounded search
// request is not a token and neither is an hour of cache storage.
var cellUnits = []struct {
	fragment string
	unit     catalog.Unit
}{
	{"tokens per hour", UnitPer1MTokensPerHour},
	{"per 1,000 requests", UnitPer1KRequests},
	{"1,000 search queries", UnitPer1KRequests},
	{"per image", UnitPerImage},
	{"per second", UnitPerSecond},
	{"per frame", UnitPerFrame},
	{"/min", UnitPerMinute},
	{"per minute", UnitPerMinute},
}

// storageMarker is how Google says an amount buys storage rather than a read.
const storageMarker = "storage price"

var (
	// bandRe matches the prompt length an amount applies up to or from, which
	// Google writes after the amount as a clause rather than as a column.
	bandRe = regexp.MustCompile(`(?i)prompts?\s*(<=|>)\s*([\d,]+)\s*k`)
	// effectiveRe matches the day an introductory rate runs through, or the
	// day the rate replacing it starts on.
	effectiveRe = regexp.MustCompile(
		`(?i)\b(through|starting)\s+([A-Za-z]+ \d{1,2}, \d{4})`,
	)
)

// bandNames map the comparison Google writes onto a name for the band.
var bandNames = map[string]string{"<=": "lte", ">": "gt"}

var (
	headingRe = regexp.MustCompile(`(?is)<h[234][^>]*>(.*?)</h[234]>`)
	// headingGroupRe matches the block heading a model, which carries the name
	// Google sells it under and, beneath it, every endpoint the API answers to
	// under that name.
	headingGroupRe = regexp.MustCompile(
		`(?is)<div class="heading-group">(.*?)</div\s*>`,
	)
	codeRe   = regexp.MustCompile(`(?is)<code[^>]*>(.*?)</code\s*>`)
	rowRe    = regexp.MustCompile(`(?is)<tr[^>]*>(.*?)</tr>`)
	cellRe   = regexp.MustCompile(`(?is)<t[hd][^>]*>(.*?)</t[hd]\s*>`)
	tagRe    = regexp.MustCompile(`(?s)<[^>]*>`)
	amountRe = regexp.MustCompile(`\$\s*([\d,]*\.?\d+)`)
)

// freeOfCharge is how Google writes a rate of zero.
const freeOfCharge = "free of charge"

// text strips markup, resolves the entities Google writes a quote, an
// apostrophe and a comparison as, and collapses whitespace. The entities are
// resolved last, so that one standing for an angle bracket cannot be read as
// the markup it is not.
func text(markup string) string {
	stripped := tagRe.ReplaceAllString(markup, " ")
	return html.UnescapeString(strings.Join(strings.Fields(stripped), " "))
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
		codes   = headingCodes(body)
		ids     []string
		tier    string
		columns []column
	)
	for _, at := range marks(body) {
		switch {
		case at.heading != "":
			if modelHeadingRe.MatchString(at.heading) {
				ids = b.begin(at.heading, codes[at.heading], doc.URL)
				tier = ""
				continue
			}
			if name, ok := tiers[strings.ToLower(at.heading)]; ok {
				tier = name
			}
		case len(ids) > 0:
			cells := rowCells(at.row)
			if len(cells) < 2 {
				continue
			}
			if header, ok := planHeader(cells); ok {
				columns = fillUnits(header)
				continue
			}
			b.applyRow(ids, tier, columns, cells)
		}
	}
}

// begin starts a model heading, returning the endpoints its tables price.
//
// Google states them beneath the heading, one <code> apiece, and they are the
// identifiers the API answers to and the model pages are addressed by. A
// heading stating none is a model Google prices without naming an endpoint for
// it, and is held under the name it is sold as.
//
// An endpoint the index says Google has withdrawn is not begun at all, so no
// row of its tables is read and no page is attached to it. The rates outlast
// the model: the pricing page still heads Gemini 2.0 Flash and still tabulates
// what it charged, while the index marks it shut down, and what is left is a
// price for something that no longer answers. Returning the endpoints kept is
// what stops the rows below the heading landing anywhere.
func (b *builder) begin(heading string, codes []string, src string) []string {
	ids := codes
	if len(ids) == 0 {
		ids = []string{slugID(heading)}
	}
	kept := make([]string, 0, len(ids))
	for _, id := range ids {
		entry := b.entry(id, heading)
		if slices.Contains(withdrawnStates, entry.state) {
			continue
		}
		m := b.model(id, nameKind(id))
		m.AddSource(src)
		if entry != (indexEntry{}) {
			m.AddSource(ModelsURL)
		}
		if m.Name == "" {
			m.Name = modelName(heading, ids, entry)
		}
		m.SetAttr(AttrState, b.stateOf(id, entry))
		kept = append(kept, id)
	}
	if len(kept) > 0 {
		b.groups = append(b.groups, kept)
	}
	return kept
}

// stateOf reports the lifecycle one endpoint is in. The withdrawal the index
// hangs off a model's name comes first, being the strongest thing Google says
// about a model, and the availability its card is marked with second.
func (b *builder) stateOf(id string, entry indexEntry) string {
	if entry.state != "" {
		return entry.state
	}
	return b.cardState[id]
}

// entry returns what the model index states about one endpoint, falling back
// to what it states about the family for the endpoints it does not list one by
// one.
func (b *builder) entry(id, heading string) indexEntry {
	if e, ok := b.index[id]; ok {
		return e
	}
	return b.index[indexKey(heading)]
}

// modelName is what to call one of the endpoints a pricing heading covers. A
// heading covering one names it, pictogram aside. A heading covering several
// names the family, and the index is the only document telling those apart,
// listing "Veo 3.1" and "Veo 3.1 Lite" as models of their own; the family name
// stands in for the endpoints the index does not list.
func modelName(heading string, ids []string, entry indexEntry) string {
	if len(ids) > 1 && entry.name != "" {
		return entry.name
	}
	return headingName(heading)
}

// headingName is the name a pricing heading states, without the pictogram
// Google hangs off the end of the ones it is fond of.
func headingName(heading string) string {
	return strings.TrimRightFunc(heading, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != ')'
	})
}

// headingCodes maps each model heading of the pricing page onto the endpoints
// stated beneath it. The two sit in one block, but the page is read as a
// stream of headings and rows, so the block is indexed by its heading first.
func headingCodes(body string) map[string][]string {
	out := map[string][]string{}
	for _, group := range headingGroupRe.FindAllStringSubmatch(body, -1) {
		heading := text(first(headingRe, group[1]))
		if heading == "" {
			continue
		}
		out[heading] = codesIn(group[1])
	}
	return out
}

// pricingCodes returns every endpoint the pricing page names, which is what
// addresses the page Google publishes for it.
func pricingCodes(doc catalog.Document) []string {
	var out []string
	for _, codes := range headingCodes(string(doc.Body)) {
		for _, code := range codes {
			if !slices.Contains(out, code) {
				out = append(out, code)
			}
		}
	}
	slices.Sort(out)
	return out
}

// codesIn returns the endpoints a fragment of markup names.
func codesIn(html string) []string {
	var out []string
	for _, match := range codeRe.FindAllStringSubmatch(html, -1) {
		if id := text(match[1]); id != "" && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

// codesFor picks the endpoints a row prices.
//
// A heading covering several endpoints states the quality level in the row's
// label and nowhere else, so a row naming one goes to the endpoint whose
// identifier carries that word, and the level Google leaves out of an
// identifier, its standard, goes to the endpoint carrying none of them. A row
// naming no level prices every endpoint under the heading, which is how the
// one model reached by two endpoint names is priced.
func codesFor(ids []string, variant string) []string {
	if variant == "" || len(ids) == 1 {
		return ids
	}
	for _, id := range ids {
		if strings.Contains(id, variant) {
			return []string{id}
		}
	}
	for _, id := range ids {
		if nameVariant(id) == "" {
			return []string{id}
		}
	}
	return nil
}

// nameVariant reports the quality level an identifier carries, if any.
func nameVariant(id string) string {
	for _, variant := range variants {
		if strings.Contains(id, variant) {
			return variant
		}
	}
	return ""
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

// applyRow records one row's amounts against each plan, for each endpoint the
// row prices.
func (b *builder) applyRow(
	ids []string,
	tier string,
	columns []column,
	cells []string,
) {
	label := strings.ToLower(cells[0])
	for _, id := range codesFor(ids, variantOf(label)) {
		m := b.models[id]
		if m == nil {
			continue
		}
		metric, ok := metricFor(label, m.Kind)
		if !ok {
			continue
		}
		refineKind(m, metric)
		addRates(m, metric, label, tier, columns, cells)
	}
}

// addRates records one row's amounts against each plan of one model.
func addRates(
	m *catalog.Model,
	metric catalog.Metric,
	label, tier string,
	columns []column,
	cells []string,
) {
	for i, cell := range cells[1:] {
		col := column{}
		if i < len(columns) {
			col = columns[i]
		}
		if col.unit == "" {
			continue
		}
		dims := catalog.Dims{}.
			With(DimTool, toolOf(label)).
			With(DimTier, tier).
			With(DimPlan, col.plan).
			With(DimModality, modalityOf(label)).
			With(DimVariant, variantOf(label))
		for _, r := range parseRates(cell) {
			m.AddPrice(catalog.Price{
				Metric:   r.metricOr(metric),
				Unit:     r.unitOr(col.unit),
				Amount:   r.amount,
				Currency: currency,
				Dims:     dims.Merge(r.dims),
				Note:     r.note,
			})
		}
	}
}

// metricOr returns what one amount is charged for, which is the row's subject
// unless the cell said the amount buys something else.
func (r rate) metricOr(metric catalog.Metric) catalog.Metric {
	if r.metric != "" {
		return r.metric
	}
	return metric
}

// unitOr returns the denominator one amount is quoted against, which is the
// column's unless the cell stated one of its own.
func (r rate) unitOr(unit catalog.Unit) catalog.Unit {
	if r.unit != "" {
		return r.unit
	}
	return unit
}

// toolOf reports which tool a row prices, for the rows charging for a tool
// call rather than for the model's own tokens.
func toolOf(label string) string {
	_, name, ok := strings.Cut(label, groundingPrefix)
	if !ok {
		return ""
	}
	return strings.ReplaceAll(slugID(name), "-", "_")
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

// metricFor maps a row label onto what its amounts are charged for. A label
// naming a quality level and nothing else prices the model itself rather than
// one facet of its billing, which is how Google prices the music models, and
// what such a model sells is read from its kind.
func metricFor(label string, kind catalog.Kind) (catalog.Metric, bool) {
	for _, entry := range rowMetrics {
		if strings.Contains(label, entry.fragment) {
			return entry.metric, true
		}
	}
	if variantOf(label) == "" {
		return "", false
	}
	metric, ok := kindMetrics[kind]
	return metric, ok
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

// rate is one amount from a cell, with everything the cell says about when
// that amount applies rather than another in the same cell.
type rate struct {
	amount float64
	metric catalog.Metric
	unit   catalog.Unit
	dims   catalog.Dims
	note   string
}

var (
	// qualifierRe matches the parenthesis Google puts after an amount to say
	// what that amount buys, as in "$0.40 (720p and 1080p) $0.60 (4k)" and
	// "$3.00 or $0.005/min (audio)". It is not anchored, because the clause
	// stating the denominator comes between the amount and the parenthesis on
	// the models billed by the minute.
	qualifierRe = regexp.MustCompile(`\(([^)]*)\)`)
	// imageSizeRe matches the size an amount buys where Google states it as
	// the denominator rather than in a parenthesis: "$0.045 per 0.5K image",
	// "$0.24 per 4K image", "$0.134 per 1K/2K image". Read as the column's
	// per-million-token rate these were four amounts for one thing, since the
	// size they differ by was in the words the column heading overrode.
	imageSizeRe = regexp.MustCompile(
		`(?i)per\s+([\d.]+k(?:\s*/\s*[\d.]+k)*)\s+image`,
	)
	// alternativeRe matches what Google writes between two ways of quoting one
	// rate: "$3.00 or $0.005/min (audio)" is one rate per million tokens and
	// the same rate per minute, and the parenthesis after the second qualifies
	// both.
	alternativeRe = regexp.MustCompile(`(?i)^\s*,?\s*or\s*$`)
)

// modalityWords are the media a parenthesis can name. A parenthesis naming
// only these says which medium an amount is charged for; one naming anything
// else says what size of picture it buys.
var modalityWords = map[string]bool{
	"text":     true,
	"image":    true,
	"images":   true,
	"audio":    true,
	"video":    true,
	"thinking": true,
	"and":      true,
	"or":       true,
}

// parseRates reads every amount in a cell along with the clause qualifying it,
// which runs from that amount to the next one.
//
// A cell states more than one amount for four reasons and they are not
// interchangeable: the output size an amount buys, the prompt length it
// applies to, the day an introductory rate runs out, and a storage rate quoted
// beside the read rate it accompanies. Reading the amounts alone left a model
// carrying two rates for the same thing with nothing to tell them apart, so
// each amount carries the clause that qualifies it. A cell reading "Free of
// charge" yields the rate of zero it means.
func parseRates(cell string) []rate {
	if amount, ok := parseAmount(cell); ok && !strings.Contains(cell, "$") {
		return []rate{{amount: amount}}
	}
	locations := amountRe.FindAllStringSubmatchIndex(cell, -1)
	note := noteOf(cell)
	out := make([]rate, 0, len(locations))
	tails := make([]string, 0, len(locations))
	for i, at := range locations {
		value, err := strconv.ParseFloat(
			strings.ReplaceAll(cell[at[2]:at[3]], ",", ""),
			64,
		)
		if err != nil {
			continue
		}
		end := len(cell)
		if i+1 < len(locations) {
			end = locations[i+1][0]
		}
		tail := cell[at[1]:end]
		r := qualify(tail)
		r.amount, r.note = value, note
		out = append(out, r)
		tails = append(tails, tail)
	}
	shareAlternatives(out, tails)
	return out
}

// qualify reads the clause following one amount for everything it says about
// when that amount applies.
func qualify(tail string) rate {
	r := rate{dims: catalog.Dims{}}
	if match := qualifierRe.FindStringSubmatch(tail); match != nil {
		r.dims = r.dims.With(qualifierKey(match[1]), slugID(match[1]))
	}
	if match := imageSizeRe.FindStringSubmatch(tail); match != nil {
		r.unit = UnitPerImage
		r.dims = r.dims.With(DimResolution, slugID(match[1]))
	}
	if match := bandRe.FindStringSubmatch(tail); match != nil {
		r.dims = r.dims.With(
			DimContextBand,
			bandNames[match[1]]+"-"+strings.ReplaceAll(match[2], ",", "")+"k",
		)
	}
	if match := effectiveRe.FindStringSubmatch(tail); match != nil {
		key := DimEffectiveFrom
		if strings.EqualFold(match[1], "through") {
			key = DimEffectiveUntil
		}
		r.dims = r.dims.With(key, isoDate(match[2]))
	}
	lower := strings.ToLower(tail)
	if strings.Contains(lower, storageMarker) {
		r.metric = MetricCacheStorage
	}
	for _, entry := range cellUnits {
		if strings.Contains(lower, entry.fragment) {
			r.unit = entry.unit
			break
		}
	}
	return r
}

// qualifierKey says which dimension a parenthesis states. Google writes the
// medium and the picture size in the same place: "(audio)" and "(text and
// thinking)" name what is being charged for, "(720p and 1080p)" and "(4k)" how
// large the result is.
func qualifierKey(content string) string {
	for _, word := range strings.FieldsFunc(
		strings.ToLower(content),
		func(r rune) bool { return r < 'a' || r > 'z' },
	) {
		if !modalityWords[word] {
			return DimResolution
		}
	}
	return DimModality
}

// shareAlternatives copies a qualifier back onto the amounts it also applies
// to.
//
// Google quotes one rate two ways and qualifies it once: "$3.00 or $0.005/min
// (audio)" is three dollars a million tokens or half a cent a minute, both for
// audio, and only the second carries the word saying so. Read apart, the first
// was an unqualified rate, and the model's text, audio and video rates then
// collided into one contradiction with four amounts in it.
func shareAlternatives(rates []rate, tails []string) {
	for i := len(rates) - 2; i >= 0; i-- {
		if !alternativeRe.MatchString(tails[i]) {
			continue
		}
		for key, value := range rates[i+1].dims {
			if key == DimResolution || key == DimModality {
				rates[i].dims = rates[i].dims.With(key, value)
			}
		}
	}
}

// noteOf keeps the allowance a cell states ahead of its amount, which is where
// Google states the requests it grounds free of charge each month.
func noteOf(cell string) string {
	at := amountRe.FindStringIndex(cell)
	if at == nil {
		return ""
	}
	before := strings.TrimSpace(cell[:at[0]])
	if !strings.Contains(strings.ToLower(before), "free") {
		return ""
	}
	return strings.TrimRight(strings.TrimSuffix(before, "then"), " ,")
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

// lessThanReplacer restores the comparison Google writes bare inside a pricing
// cell. "prompts <= 200k tokens" is not markup, but everything from its angle
// bracket to the next one reads as a tag, so the clause saying which prompts
// the amount applies to was stripped along with it and the cell was left
// stating two amounts with nothing to tell them apart.
var lessThanReplacer = strings.NewReplacer("<=", "&lt;=")

// rowCells returns the text of one row's cells.
func rowCells(row string) []string {
	matches := cellRe.FindAllStringSubmatch(row, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, text(lessThanReplacer.Replace(m[1])))
	}
	return out
}
