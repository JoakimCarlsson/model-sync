package fireworks

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// table is one markdown table with the heading above it.
type table struct {
	Section string
	Headers []string
	Rows    [][]string
	Source  string
}

// Sections of the pricing page that state a rate.
const (
	sectionTextVision = "text and vision models"
	sectionOtherBase  = "other base models"
	sectionEmbeddings = "embeddings"
)

// tierColumns maps a rate column onto the serving path it prices.
var tierColumns = map[string]string{
	"standard": TierStandard,
	"priority": TierPriority,
}

// applyPricing reads the serverless pricing page.
//
// The page has three rate cards. The first names models and links each to its
// page, which is the join and needs no matching on names. The second and third
// price by parameter count, in bands that apply to whatever the first card did
// not name, so those rows are held until every model's parameter count is
// known and then matched to the models they cover.
func (b *builder) applyPricing(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		switch {
		case t.Section == sectionTextVision:
			b.applyTripleTable(t)
		case strings.HasPrefix(t.Section, sectionOtherBase):
			b.textBands = append(b.textBands, bandRows(t)...)
		case t.Section == sectionEmbeddings:
			b.embeddingBands = append(b.embeddingBands, bandRows(t)...)
			b.pendingNamed = append(b.pendingNamed, namedRows(t, doc.URL)...)
		}
	}
	b.applyBatchDiscount(doc)
	b.pricingSource = doc.URL
}

// applyTripleTable reads the table whose cells hold three amounts per serving
// path.
func (b *builder) applyTripleTable(t table) {
	for _, row := range t.Rows {
		ref, ok := splitModelCell(cellAt(row, 0))
		if !ok {
			continue
		}
		id := b.resolveRef(ref)
		m := b.model(id, KindChat)
		m.AddSource(t.Source)
		b.priced[id] = true
		if m.Name == "" {
			m.Name = ref.Name
		}
		for i, header := range t.Headers {
			tier, ok := tierColumns[strings.ToLower(clean(header))]
			if !ok {
				continue
			}
			dims := catalog.Dims{DimTier: tier}.With(DimServing, ref.Serving)
			for at, amount := range parseTriple(cellAt(row, i)) {
				if at >= len(tripleOrder) {
					break
				}
				m.AddPrice(catalog.Price{
					Metric:   tripleOrder[at],
					Unit:     UnitPer1MTokens,
					Amount:   amount,
					Currency: currency,
					Dims:     dims,
				})
			}
		}
	}
}

// resolveRef says which model a rate card row prices.
//
// A row links the model's page in the console, which addresses it by a path
// that is usually the one it is served under and sometimes not: a model can be
// linked as one name and served as another. So the link is tried as an
// identifier first, and where no model answers to it, as the address of a page
// in the library, which is the document that stated what the model is served
// as. A link that is neither is taken at face value, so that a model priced
// before the library lists it is still priced.
func (b *builder) resolveRef(ref modelRef) string {
	if _, ok := b.models[ref.ID]; ok {
		return ref.ID
	}
	if id, ok := b.byPage[libraryHost+modelPathPrefix+ref.ID]; ok {
		return id
	}
	return ref.ID
}

// bandRows reads the rows of a rate card that price a parameter count band.
func bandRows(t table) []band {
	var out []band
	for _, row := range t.Rows {
		amounts := parseTriple(strings.Join(row[1:], " "))
		if len(amounts) == 0 {
			continue
		}
		if parsed, ok := parseBand(clean(cellAt(row, 0)), amounts); ok {
			out = append(out, parsed)
		}
	}
	return out
}

// namedRow is a rate card row naming a model without linking it.
type namedRow struct {
	Name   string
	Amount float64
	Source string
}

// namedRows reads the rows of a rate card that name a model rather than a
// band. The embeddings card has one: Fireworks prices its embedding model
// under a shortened name and links nothing, so the row has to be matched to a
// model by that name once the library has been read.
func namedRows(t table, source string) []namedRow {
	var out []namedRow
	for _, row := range t.Rows {
		label := clean(cellAt(row, 0))
		amounts := parseTriple(strings.Join(row[1:], " "))
		if len(amounts) == 0 {
			continue
		}
		if _, isBand := parseBand(label, amounts); isBand {
			continue
		}
		out = append(out, namedRow{
			Name:   label,
			Amount: amounts[0],
			Source: source,
		})
	}
	return out
}

// batchRe matches the sentence stating what a job billed asynchronously pays,
// which Fireworks writes as a share of the rate rather than as an amount.
var batchRe = regexp.MustCompile(
	`(?i)batch inference.{0,40}?billed at\s*\*{0,2}(\d+)%`,
)

// applyBatchDiscount records the share the pricing page states a batched
// request is billed at. The page states it once, for every model and both
// directions of traffic, so it is held and applied to the rates each model
// ends up with rather than to any one of them here.
func (b *builder) applyBatchDiscount(doc catalog.Document) {
	match := batchRe.FindSubmatch(doc.Body)
	if match == nil {
		return
	}
	if share, ok := parseAmount("$" + string(match[1])); ok {
		b.batchShare = share / 100
	}
}

// resolveNamed matches the rate card rows that named a model without linking
// it against the models the library established.
//
// The name is shortened: the card prices "Qwen3 8B" where the library titles
// the model "Qwen3 Embedding 8B", and a chat model of exactly the shorter name
// is served as well. So a row is only matched against models that answer the
// endpoint the card is a card for, and only where exactly one of them is
// named; two would mean the card cannot say which, and the rate is then left
// unrecorded rather than put on a guess.
func (b *builder) resolveNamed() {
	for _, row := range b.pendingNamed {
		var found []string
		for _, id := range b.order {
			if b.models[id].Kind != KindEmbedding {
				continue
			}
			if namesSameModel(row.Name, b.models[id].Name) {
				found = append(found, id)
			}
		}
		if len(found) != 1 {
			continue
		}
		m := b.models[found[0]]
		m.AddSource(row.Source)
		m.AddPrice(catalog.Price{
			Metric:   MetricInputTokens,
			Unit:     UnitPer1MTokens,
			Amount:   row.Amount,
			Currency: currency,
			Dims:     catalog.Dims{DimTier: TierStandard},
		})
		b.priced[found[0]] = true
	}
}

// applyBands prices the serverless models the rate cards did not name, by the
// band their parameter count puts them in.
//
// Fireworks says outright that this is how the rest of the library is priced:
// any text or vision model it does not price individually costs what its size
// and architecture cost. The band applies only to models it can be charged
// for, which is the ones the library says are served on serverless; a model
// that only runs on GPUs of the caller's own is billed by the hour and has no
// per-token rate at all.
func (b *builder) applyBands() {
	for _, id := range b.order {
		m := b.models[id]
		if b.priced[id] || !servesServerless(m) {
			continue
		}
		bands := b.textBands
		if m.Kind == KindEmbedding || m.Kind == KindRerank {
			bands = b.embeddingBands
		}
		params := parameterCount(m.Attrs[AttrParameterCount])
		moe := m.Attrs[AttrMixtureOfExperts] == "true"
		match, ok := bandFor(bands, params, moe)
		if !ok || len(match.Amounts) == 0 {
			continue
		}
		dims := catalog.Dims{
			DimTier:     TierStandard,
			DimSizeBand: match.Label,
		}
		metrics := []catalog.Metric{MetricInputTokens, MetricOutputTokens}
		if m.Kind != KindChat {
			metrics = metrics[:1]
		}
		for _, metric := range metrics {
			m.AddPrice(catalog.Price{
				Metric:   metric,
				Unit:     UnitPer1MTokens,
				Amount:   match.Amounts[0],
				Currency: currency,
				Dims:     dims,
			})
		}
		m.AddSource(b.pricingSource)
	}
}

// applyBatch records what each rate becomes for a request submitted as a
// batch. Fireworks states the share once and states which traffic it applies
// to: what a caller sends and what the model generates, not the part of the
// input served from cache, which is discounted on its own terms.
func (b *builder) applyBatch() {
	if b.batchShare <= 0 {
		return
	}
	for _, id := range b.order {
		m := b.models[id]
		for _, p := range append([]catalog.Price(nil), m.Prices...) {
			if p.Metric != MetricInputTokens &&
				p.Metric != MetricOutputTokens {
				continue
			}
			if p.Dims[DimTier] != TierStandard || p.Dims[DimBatch] != "" {
				continue
			}
			m.AddPrice(catalog.Price{
				Metric:   p.Metric,
				Unit:     p.Unit,
				Amount:   p.Amount * b.batchShare,
				Currency: p.Currency,
				Dims:     p.Dims.With(DimBatch, "true"),
			})
		}
	}
}

// servesServerless reports whether the library said the model runs on the
// shared fleet, which is the only place Fireworks bills it per token.
func servesServerless(m *catalog.Model) bool {
	for _, v := range m.Lists[ListDeployment] {
		if v == DeploymentServerless {
			return true
		}
	}
	return false
}

// scanTables walks a document and returns every pipe table, tracking the
// nearest preceding heading.
func scanTables(body, source string) []table {
	var (
		out     []table
		section string
		current *table
	)
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "|") {
			if current == nil {
				out = append(out, table{Section: section, Source: source})
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
			continue
		}
		current = nil
		if after, ok := strings.CutPrefix(line, "#"); ok {
			section = strings.ToLower(
				clean(strings.TrimSpace(strings.TrimLeft(after, "# "))),
			)
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
