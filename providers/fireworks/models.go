package fireworks

import (
	"slices"
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

// Sections of the pricing page that list models.
const (
	sectionTextVision = "text and vision models"
	sectionEmbeddings = "embeddings"
)

// tierColumns maps a rate column onto the serving path it prices.
var tierColumns = map[string]string{
	"standard": TierStandard,
	"priority": TierPriority,
}

// applyPricing reads the serverless pricing page.
func (b *builder) applyPricing(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		switch t.Section {
		case sectionTextVision:
			b.applyTripleTable(t)
		case sectionEmbeddings:
			b.applySingleTable(t)
		}
	}
}

// applyTripleTable reads the table whose cells hold three amounts per serving
// path.
func (b *builder) applyTripleTable(t table) {
	for _, row := range t.Rows {
		ref, ok := splitModelCell(cellAt(row, 0))
		if !ok {
			continue
		}
		m := b.model(ref.ID, KindChat)
		m.AddSource(t.Source)
		if m.Name == "" {
			m.Name = ref.Name
		}
		m.SetAttr(AttrModelURL, ref.URL)
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

// applySingleTable reads a table whose cells hold one amount, skipping the
// rows that price a parameter count band rather than a model.
func (b *builder) applySingleTable(t table) {
	for _, row := range t.Rows {
		cell := cellAt(row, 0)
		if isBand(cell) {
			continue
		}
		id, name := clean(cell), clean(cell)
		if ref, ok := splitModelCell(cell); ok {
			id, name = ref.ID, ref.Name
		} else {
			id = slugID(id)
		}
		if id == "" {
			continue
		}
		amounts := parseTriple(cellAt(row, 1))
		if len(amounts) == 0 {
			continue
		}
		m := b.model(id, KindEmbedding)
		m.AddSource(t.Source)
		if m.Name == "" {
			m.Name = name
		}
		m.AddPrice(catalog.Price{
			Metric:   MetricInputTokens,
			Unit:     UnitPer1MTokens,
			Amount:   amounts[0],
			Currency: currency,
		})
	}
}

// embeddingMatch pairs a priced embedding model with the guide row naming the
// same model, which is the row that links it to a page.
type embeddingMatch struct {
	ID  string
	Ref modelRef
}

// applyEmbeddingsGuide gives the embedding model the identifier it is served
// under and the page it is described on.
//
// The pricing page prices it as "Qwen3 8B" and links nothing, so on the
// pricing page alone it is a name and a rate. The guide writes the name out,
// links the model's page and states the identifier the API takes, so the model
// is re-keyed onto that identifier and its page is read like any other. The
// name is taken from the guide as well: "Qwen3 8B" is what the price list
// calls it and also what Fireworks calls a chat model it serves, and the guide
// is the document writing the two apart.
func (b *builder) applyEmbeddingsGuide(doc catalog.Document) {
	for _, match := range b.matchEmbeddings(doc) {
		m := b.models[match.ID]
		m.Name = match.Ref.Name
		m.SetAttr(AttrModelURL, match.Ref.URL)
		m.AddSource(doc.URL)
		b.rename(match.ID, match.Ref.ID)
	}
}

// matchEmbeddings pairs every embedding model the pricing page priced without
// linking with the guide row naming it.
func (b *builder) matchEmbeddings(doc catalog.Document) []embeddingMatch {
	refs := guideModelRefs(doc)
	var out []embeddingMatch
	for _, id := range b.order {
		m := b.models[id]
		if m.Kind != KindEmbedding || m.Attrs[AttrModelURL] != "" {
			continue
		}
		for _, ref := range refs {
			if !namesSameModel(m.Name, ref.Name) {
				continue
			}
			out = append(out, embeddingMatch{ID: id, Ref: ref})
			break
		}
	}
	return out
}

// guideModelRefs returns the models the embeddings guide both links and states
// are served on serverless.
//
// The guide's tables also list models that run only on a deployment of the
// caller's own, which the pricing page quotes no rate for, and its reranking
// tables list models whose names an embedding model's name is a subset of. A
// row is therefore read only when it links a model, says serverless, and is
// not under the reranking heading.
func guideModelRefs(doc catalog.Document) []modelRef {
	var out []modelRef
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		if strings.Contains(t.Section, "rerank") {
			continue
		}
		for _, row := range t.Rows {
			ref, ok := splitModelCell(cellAt(row, 0))
			if !ok || !mentionsServerless(row) {
				continue
			}
			out = append(out, ref)
		}
	}
	return out
}

// mentionsServerless reports whether a row says the model is served without a
// deployment of the caller's own.
func mentionsServerless(row []string) bool {
	for _, cell := range row {
		if strings.Contains(strings.ToLower(cell), "serverless") {
			return true
		}
	}
	return false
}

// namesSameModel reports whether the guide's name is the pricing page's name
// spelled out. The pricing table writes "Qwen3 8B" where the guide writes
// "Qwen3 Embedding 8B", so a match is every word of the priced name appearing
// in the listed one, in order.
func namesSameModel(priced, listed string) bool {
	want, have := nameWords(priced), nameWords(listed)
	if len(want) == 0 {
		return false
	}
	for _, word := range want {
		at := slices.Index(have, word)
		if at < 0 {
			return false
		}
		have = have[at+1:]
	}
	return true
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
