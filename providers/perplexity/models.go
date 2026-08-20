package perplexity

import (
	"regexp"
	"slices"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// EmbeddingsURL is the Embeddings guide, whose model table is the pricing
// page's with three more columns: the input bound, whether the vector can be
// truncated, and what it is quantized to.
const EmbeddingsURL = baseURL + "/docs/embeddings/quickstart.md"

// table is one markdown table lifted out of a document, with the context it
// was found in. Perplexity nests most of its tables inside MDX tab elements,
// so the columns say what a table is but only the tab and the headings above
// it say who it is about.
type table struct {
	Headers []string
	Rows    [][]string
	Source  string
	// Section is the most recent second-level heading.
	Section string
	// Heading is the most recent heading of any level.
	Heading string
	// Tab is the title of the enclosing tab element, if any.
	Tab string
}

// tokenColumns are the per-token rate columns of the Sonar table.
var tokenColumns = []struct {
	header string
	metric catalog.Metric
	unit   catalog.Unit
}{
	{"input tokens ($/1m)", MetricInputTokens, UnitPer1MTokens},
	{"output tokens ($/1m)", MetricOutputTokens, UnitPer1MTokens},
	{"citation tokens ($/1m)", MetricCitationTokens, UnitPer1MTokens},
	{"reasoning tokens ($/1m)", MetricReasoningTokens, UnitPer1MTokens},
	{"search queries ($/1k)", MetricSearchQueries, UnitPer1KRequests},
}

// brokeredColumns are the rate columns of the Agent API's model tables, where
// the heading states the denominator and the cells hold bare numbers.
var brokeredColumns = []struct {
	header string
	metric catalog.Metric
}{
	{"input ($/1m)", MetricInputTokens},
	{"output ($/1m)", MetricOutputTokens},
	{"cache ($/1m)", MetricCachedInputTokens},
}

// contextSizes are the request-fee columns, one per depth of web search.
var contextSizes = map[string]string{
	"low context size":    "low",
	"medium context size": "medium",
	"high context size":   "high",
}

// applyDocument reads a pricing document, dispatching each table on the
// columns it has. Perplexity nests most of them inside tab elements, so the
// surrounding headings do not reliably say what a table is; its columns do.
func (b *builder) applyDocument(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		switch {
		case columnOf(t.Headers, "input tokens ($/1m)") >= 0:
			b.applySonarTokens(t)
		case columnOf(t.Headers, "low context size") >= 0:
			b.applyRequestFees(t)
		case columnOf(t.Headers, "dimensions") >= 0:
			b.applyEmbeddings(t)
		case columnOf(t.Headers, "input ($/1m)") >= 0:
			b.applyBrokered(t)
		case columnOf(t.Headers, "tool") >= 0:
			b.applyTools(t)
		case columnOf(t.Headers, "price per 1k requests") >= 0:
			b.applySearchAPI(t)
		}
	}
}

// applySonarTokens reads the per-token rates of the Sonar models.
func (b *builder) applySonarTokens(t table) {
	for _, row := range t.Rows {
		m, ok := b.rowModel(t, row, KindChat)
		if !ok {
			continue
		}
		for _, col := range tokenColumns {
			at := columnOf(t.Headers, col.header)
			if at < 0 {
				continue
			}
			if amount, ok := parseAmount(cellAt(row, at)); ok {
				m.AddPrice(catalog.Price{
					Metric:   col.metric,
					Unit:     col.unit,
					Amount:   amount,
					Currency: currency,
				})
			}
		}
	}
}

// applyRequestFees reads the per-request fee, which varies by how much of the
// web the query is allowed to read.
func (b *builder) applyRequestFees(t table) {
	for _, row := range t.Rows {
		m, ok := b.rowModel(t, row, KindChat)
		if !ok {
			continue
		}
		for header, size := range contextSizes {
			at := columnOf(t.Headers, header)
			if at < 0 {
				continue
			}
			if amount, ok := parseAmount(cellAt(row, at)); ok {
				m.AddPrice(catalog.Price{
					Metric:   MetricRequest,
					Unit:     UnitPer1KRequests,
					Amount:   amount,
					Currency: currency,
					Dims:     catalog.Dims{DimContextSize: size},
				})
			}
		}
	}
}

// applyEmbeddings reads an embedding table, which states the vector width
// beside the rate. The pricing page states the width and the rate and stops
// there; the Embeddings guide restates both and adds the input bound, whether
// the vector can be truncated and what it is quantized to.
func (b *builder) applyEmbeddings(t table) {
	priceCol := columnOf(t.Headers, "price ($/1m tokens)")
	dimCol := columnOf(t.Headers, "dimensions")
	for _, row := range t.Rows {
		m, ok := b.rowModel(t, row, KindEmbedding)
		if !ok {
			continue
		}
		if width := clean(cellAt(row, dimCol)); width != "" {
			m.SetAttr(AttrDefaultDimension, width)
			m.AddList(ListDimensions, width)
		}
		if strings.Contains(m.ID, "context") {
			m.SetAttr(AttrContextualized, "true")
		}
		b.embedding = appendUnique(b.embedding, m.ID)
		b.applyEmbeddingGuide(m, t, row)
		if amount, ok := parseAmount(cellAt(row, priceCol)); ok {
			m.AddPrice(catalog.Price{
				Metric:   MetricInputTokens,
				Unit:     UnitPer1MTokens,
				Amount:   amount,
				Currency: currency,
			})
		}
	}
}

// applyEmbeddingGuide reads the columns only the Embeddings guide carries. It
// also records what an embedding model works in, which no column states and
// the guide says in prose: these embed text, and the vector they answer with
// is the return value rather than a medium the catalog has a word for, so text
// stands on both sides the way it does for a reranker.
func (b *builder) applyEmbeddingGuide(m *catalog.Model, t table, row []string) {
	if t.Source != EmbeddingsURL {
		return
	}
	m.AddList(ListInputModalities, ModalityText)
	m.AddList(ListOutputModalities, ModalityText)
	window := countRe.FindStringSubmatch(clean(cellAt(
		row,
		columnOf(t.Headers, "context"),
	)))
	if window != nil {
		m.SetLimit(LimitContextWindow, parseTokens(window[1], window[2]))
	}
	if strings.EqualFold(
		clean(cellAt(row, columnOf(t.Headers, "mrl"))),
		"yes",
	) {
		m.AddList(ListFeatures, FeatureMatryoshka)
	}
	for _, format := range strings.Split(
		clean(cellAt(row, columnOf(t.Headers, "quantization"))),
		"/",
	) {
		m.AddList(ListQuantizations, strings.ToLower(strings.TrimSpace(format)))
	}
}

// applyBrokered reads the models Perplexity resells, which it namespaces under
// the lab that made them.
func (b *builder) applyBrokered(t table) {
	docsCol := columnOf(t.Headers, "docs")
	for _, row := range t.Rows {
		m, ok := b.rowModel(t, row, KindChat)
		if !ok {
			continue
		}
		m.SetAttr(AttrAuthor, brokeredAuthor(t.Tab, m.ID))
		m.SetAttr(AttrSummary, clean(cellAt(row, docsCol)))
		card := hrefOf(cellAt(row, docsCol))
		m.SetAttr(AttrModelCardURL, card)
		m.SetAttr(AttrHuggingFaceID, huggingFaceID(card))
		if !slices.Contains(b.agent, m.ID) {
			b.agent = append(b.agent, m.ID)
		}
		for _, col := range brokeredColumns {
			at := columnOf(t.Headers, col.header)
			if at < 0 {
				continue
			}
			cell := clean(cellAt(row, at))
			if discountRe.MatchString(cell) {
				m.SetAttr(AttrCacheDiscount, cell)
				continue
			}
			base := catalog.Dims{}.With(DimAPI, APIAgent)
			for _, rate := range rateBands(cell) {
				m.AddPrice(catalog.Price{
					Metric:   col.metric,
					Unit:     UnitPer1MTokens,
					Amount:   rate.amount,
					Currency: currency,
					Dims:     base.With(DimPromptBand, rate.band),
				})
			}
		}
	}
}

// rate is one amount of a brokered model's rate cell, with the prompt length
// it applies to where the cell states more than one.
type rate struct {
	amount float64
	band   string
}

// rateBands reads a brokered rate cell. Most hold one amount, but a model
// whose rate steps up on a long prompt has both amounts in the one cell, each
// followed by the bound it applies under: reading the first alone would price
// every long prompt at the short-prompt rate.
func rateBands(cell string) []rate {
	matches := bandRe.FindAllStringSubmatch(cell, -1)
	if len(matches) == 0 {
		amount, ok := parseAmount(cell)
		if !ok {
			return nil
		}
		return []rate{{amount: amount}}
	}
	rates := make([]rate, 0, len(matches))
	for _, match := range matches {
		amount, ok := parseAmount(match[1])
		if !ok {
			continue
		}
		rates = append(rates, rate{amount: amount, band: bandLabel(match[2])})
	}
	return rates
}

// bandLabel writes a bound the way the other providers state one, since
// Perplexity writes its lower bound with a character no consumer will type.
func bandLabel(bound string) string {
	return strings.ReplaceAll(
		strings.Join(strings.Fields(bound), ""),
		"≤",
		"<=",
	)
}

// applyTools reads the server-side tool rates, which are charged per
// invocation except the sandbox, which is charged per session.
func (b *builder) applyTools(t table) {
	priceCol := columnOf(t.Headers, "price")
	descCol := columnOf(t.Headers, "description")
	for _, row := range t.Rows {
		m, ok := b.rowModel(t, row, KindTool)
		if !ok {
			continue
		}
		m.SetAttr(AttrSummary, clean(cellAt(row, descCol)))
		b.tools = appendUnique(b.tools, m.ID)
		cell := clean(cellAt(row, priceCol))
		amount, ok := parseAmount(cell)
		if !ok {
			continue
		}
		metric, unit := MetricToolCall, UnitPerInvocation
		if strings.Contains(strings.ToLower(cell), "per session") {
			metric, unit = MetricSession, UnitPerSession
		}
		m.AddPrice(catalog.Price{
			Metric:   metric,
			Unit:     unit,
			Amount:   amount,
			Currency: currency,
		})
	}
}

// applySearchAPI reads the standalone search product's rate.
func (b *builder) applySearchAPI(t table) {
	priceCol := columnOf(t.Headers, "price per 1k requests")
	descCol := columnOf(t.Headers, "description")
	for _, row := range t.Rows {
		m, ok := b.rowModel(t, row, KindTool)
		if !ok {
			continue
		}
		m.SetAttr(AttrSummary, clean(cellAt(row, descCol)))
		b.searchAPI = appendUnique(b.searchAPI, m.ID)
		if amount, ok := parseAmount(cellAt(row, priceCol)); ok {
			m.AddPrice(catalog.Price{
				Metric:   MetricRequest,
				Unit:     UnitPer1KRequests,
				Amount:   amount,
				Currency: currency,
			})
		}
	}
}

// rowModel resolves the model a row names, which is always its first cell.
func (b *builder) rowModel(
	t table,
	row []string,
	kind catalog.Kind,
) (*catalog.Model, bool) {
	name := clean(cellAt(row, 0))
	if name == "" || name == "-" {
		return nil, false
	}
	m := b.model(slugID(name), kind)
	m.AddSource(t.Source)
	if m.Name == "" {
		m.Name = name
	}
	return m, true
}

// scanTables returns every pipe table in a document, however deeply the MDX
// around it is indented, each carrying the heading and tab it was found under.
func scanTables(body, source string) []table {
	var (
		out     []table
		current *table
		ctx     tableContext
	)
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "|") {
			current = nil
			ctx.observe(line)
			continue
		}
		if current == nil {
			out = append(out, table{
				Source:  source,
				Section: ctx.section,
				Heading: ctx.heading,
				Tab:     ctx.tab,
			})
			current = &out[len(out)-1]
		}
		cells := splitRow(line)
		switch {
		case current.Headers == nil:
			current.Headers = cells
		case isSeparator(cells):
		default:
			current.Rows = append(current.Rows, cells)
		}
	}
	return out
}

// splitRow splits a pipe row into trimmed cells.
func splitRow(line string) []string {
	parts := strings.Split(line, "|")
	if len(parts) > 0 && strings.TrimSpace(parts[0]) == "" {
		parts = parts[1:]
	}
	if len(parts) > 0 && strings.TrimSpace(parts[len(parts)-1]) == "" {
		parts = parts[:len(parts)-1]
	}
	cells := make([]string, len(parts))
	for i, p := range parts {
		cells[i] = strings.TrimSpace(p)
	}
	return cells
}

// isSeparator reports whether a row is the dashed rule under a header.
func isSeparator(cells []string) bool {
	for _, c := range cells {
		if strings.Trim(c, ":- ") != "" {
			return false
		}
	}
	return len(cells) > 0
}

// cellAt returns a row's cell, tolerating short rows.
func cellAt(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

// columnOf returns the index of the column with the given heading, or -1.
func columnOf(headers []string, name string) int {
	for i, h := range headers {
		if strings.EqualFold(clean(h), name) {
			return i
		}
	}
	return -1
}

// tableContext tracks the headings and tab element in force while a document
// is scanned.
type tableContext struct {
	section string
	heading string
	tab     string
}

// tabTitleRe matches the opening of an MDX tab element and its title.
var tabTitleRe = regexp.MustCompile(`(?i)<Tab\s+title="([^"]*)"`)

// observe folds one non-table line into the context.
func (c *tableContext) observe(line string) {
	if match := tabTitleRe.FindStringSubmatch(line); match != nil {
		c.tab = clean(match[1])
		return
	}
	if strings.HasPrefix(line, "</Tab>") {
		c.tab = ""
		return
	}
	if !strings.HasPrefix(line, "#") {
		return
	}
	level := len(line) - len(strings.TrimLeft(line, "#"))
	text := clean(strings.TrimSpace(strings.TrimLeft(line, "#")))
	c.heading = text
	if level == 2 {
		c.section = text
	}
}

var (
	// hrefRe matches the target of a markdown link, which is how the brokered
	// model table states where a model is documented.
	hrefRe = regexp.MustCompile(`\]\(([^)\s]+)`)
	// huggingFaceRe matches a Hugging Face repository, which is what a link to
	// that host is when it names an owner and a repository and not just an
	// owner.
	huggingFaceRe = regexp.MustCompile(
		`^https://huggingface\.co/([^/]+/[^/?#]+)$`,
	)
	// contextOrderRe matches the note stating which order a fee cell holding
	// three amounts states them in.
	contextOrderRe = regexp.MustCompile(
		`(?i)search context size \(([^)]*)\)`,
	)
)

// brokeredAuthor returns the lab a brokered model comes from. The tab it is
// filed under names that lab, and the namespace of its identifier does not:
// the Router catalog says outright that a model filed under Perplexity is one
// Perplexity serves rather than one it made.
func brokeredAuthor(tab, id string) string {
	if author := slugID(tab); author != "" {
		return author
	}
	return authorOf(id)
}

// hrefOf returns the target of the first markdown link in a cell.
func hrefOf(cell string) string {
	match := hrefRe.FindStringSubmatch(cell)
	if match == nil {
		return ""
	}
	return match[1]
}

// huggingFaceID returns the repository a documentation link points at, where
// it points at one.
func huggingFaceID(url string) string {
	match := huggingFaceRe.FindStringSubmatch(url)
	if match == nil {
		return ""
	}
	return match[1]
}

// applyProSearchFees reads the request fees of Pro Search, which is a second
// column of fees over the same three context sizes rather than a rate of its
// own. The table names the search type in its rows and states the three
// amounts of a row in one cell, in an order the note under it gives, so both
// the type and the size are recorded against every amount: the standard fee
// and the Pro Search fee are otherwise two amounts for the same thing.
func (b *builder) applyProSearch(doc catalog.Document) {
	sizes := contextOrder(string(doc.Body))
	if len(sizes) == 0 {
		return
	}
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		if columnOf(t.Headers, "request fee (per 1k)") >= 0 {
			b.applyProSearchFees(t, sizes)
		}
	}
}

// applyProSearchFees reads one Pro Search fee table onto the model its section
// names. The section is what names it: Perplexity heads the section with the
// model Pro Search enhances and heads the table itself with the setting the
// rows enumerate.
func (b *builder) applyProSearchFees(t table, sizes []string) {
	at := columnOf(t.Headers, "request fee (per 1k)")
	m, ok := b.namedSonar(t.Section)
	if !ok {
		return
	}
	m.AddSource(t.Source)
	for _, row := range t.Rows {
		searchType := clean(cellAt(row, 0))
		if searchType == "" {
			continue
		}
		m.AddList(ListSearchTypes, searchType)
		b.addProSearchFee(m, searchType, sizes, cellAt(row, at))
	}
}

// addProSearchFee reads one row of the Pro Search fee table.
func (b *builder) addProSearchFee(
	m *catalog.Model,
	searchType string,
	sizes []string,
	cell string,
) {
	amounts := strings.Split(clean(cell), "/")
	if len(amounts) != len(sizes) {
		return
	}
	for i, amount := range amounts {
		value, ok := parseAmount(amount)
		if !ok || statedFee(m, sizes[i], value) {
			continue
		}
		m.AddPrice(catalog.Price{
			Metric:   MetricRequest,
			Unit:     UnitPer1KRequests,
			Amount:   value,
			Currency: currency,
			Dims: catalog.Dims{
				DimContextSize: sizes[i],
				DimSearchType:  searchType,
			},
		})
	}
}

// contextOrder returns the order a document says a three-amount fee cell
// states its context sizes in.
func contextOrder(body string) []string {
	match := contextOrderRe.FindStringSubmatch(body)
	if match == nil {
		return nil
	}
	var sizes []string
	for _, part := range strings.Split(match[1], "/") {
		size := strings.ToLower(strings.TrimSpace(part))
		if _, ok := contextSizes[size+" context size"]; !ok {
			return nil
		}
		sizes = append(sizes, size)
	}
	return sizes
}

// applyEmbeddingProse reads what the Embeddings guide states of every model it
// tabulates rather than of any row: how the vector is pooled, and that it is
// returned unnormalized, which decides how two of them may be compared.
func (b *builder) applyEmbeddingProse(doc catalog.Document) {
	body := string(doc.Body)
	pooling := poolingRe.FindStringSubmatch(body)
	normalized := normalizedRe.MatchString(body)
	for _, id := range b.embedding {
		m := b.models[id]
		m.AddSource(doc.URL)
		if pooling != nil {
			m.SetAttr(AttrPooling, strings.ToLower(pooling[1]))
		}
		if normalized {
			m.SetAttr(AttrNormalized, "false")
		}
	}
}

var (
	// poolingRe matches the guide's statement of how a vector is pooled.
	poolingRe = regexp.MustCompile(`(?i)All models use (\w+) pooling`)
	// normalizedRe matches its statement that the vector is not normalized.
	normalizedRe = regexp.MustCompile(`(?i)embeddings are \*\*unnormalized`)
)

// statedFee reports whether the standard fee table already states an amount
// for one context size. The default search type restates that table rather
// than adding to it, so recording its row again under a dimension of its own
// would carry the same rate twice.
func statedFee(m *catalog.Model, size string, amount float64) bool {
	for _, price := range m.Prices {
		if price.Metric != MetricRequest || price.Amount != amount {
			continue
		}
		if len(price.Dims) == 1 && price.Dims[DimContextSize] == size {
			return true
		}
	}
	return false
}
