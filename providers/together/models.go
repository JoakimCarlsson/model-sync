package together

import (
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// table is one markdown table with the section heading above it, which is what
// says what kind of model its rows describe.
type table struct {
	Section string
	Headers []string
	Rows    [][]string
	Source  string
}

// sectionKinds maps a heading onto the kind of model listed under it. A
// heading absent from this map introduces prose or an example rather than a
// catalog, and its tables are ignored.
var sectionKinds = map[string]catalog.Kind{
	"chat models":       KindChat,
	"vision models":     KindChat,
	"image models":      KindImage,
	"video models":      KindVideo,
	"audio models":      KindAudio,
	"embedding models":  KindEmbedding,
	"rerank models":     KindRerank,
	"moderation models": KindModeration,
}

// role is what a column contributes to the rows it appears in.
type role int

const (
	roleSkip role = iota
	roleID
	roleName
	roleAuthor
	roleModality
	roleContext
	roleDimension
	roleModelSize
	roleSteps
	roleGeometry
	rolePrice
)

// column is the meaning of one column.
type column struct {
	role   role
	metric catalog.Metric
	unit   catalog.Unit
}

// headerColumn maps one of Together's column headings onto its meaning. The
// eight tables share a vocabulary even though they do not share a shape, so
// one mapping covers every modality.
func headerColumn(header string) column {
	switch strings.ToLower(clean(header)) {
	case "api model string", "model string for api":
		return column{role: roleID}
	case "model name":
		return column{role: roleName}
	case "organization":
		return column{role: roleAuthor}
	case "modality":
		return column{role: roleModality}
	case "context length", "context window":
		return column{role: roleContext}
	case "embedding dimension":
		return column{role: roleDimension}
	case "model size":
		return column{role: roleModelSize}
	case "default steps":
		return column{role: roleSteps}
	case "resolution / duration":
		return column{role: roleGeometry}
	case "input pricing (per 1m tokens)":
		return column{rolePrice, MetricInputTokens, UnitPer1MTokens}
	case "cached input pricing (per 1m tokens)":
		return column{rolePrice, MetricCachedInputTokens, UnitPer1MTokens}
	case "output pricing (per 1m tokens)":
		return column{rolePrice, MetricOutputTokens, UnitPer1MTokens}
	case "pricing (per 1m tokens)":
		return column{rolePrice, MetricInputTokens, UnitPer1MTokens}
	case "price per mp":
		return column{rolePrice, MetricImageOutput, UnitPerMegapixel}
	case "price per video":
		return column{rolePrice, MetricVideoOutput, UnitPerVideo}
	case "pricing":
		return column{role: rolePrice, metric: MetricAudio}
	}
	return column{role: roleSkip}
}

// applyCatalog reads the model catalog page.
func (b *builder) applyCatalog(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		kind, ok := sectionKinds[t.Section]
		if !ok {
			continue
		}
		b.applyTable(t, kind)
	}
}

// applyTable reads one modality's table.
func (b *builder) applyTable(t table, kind catalog.Kind) {
	cols := make([]column, len(t.Headers))
	for i, h := range t.Headers {
		cols[i] = headerColumn(h)
	}
	for _, row := range t.Rows {
		b.applyRow(t, cols, row, kind)
	}
}

// applyRow records one model.
func (b *builder) applyRow(
	t table,
	cols []column,
	row []string,
	kind catalog.Kind,
) {
	id := ""
	for i, col := range cols {
		if col.role == roleID {
			id = clean(cellAt(row, i))
		}
	}
	if id == "" {
		return
	}
	m := b.model(id, kind)
	m.AddSource(t.Source)
	dims := catalog.Dims{}
	for i, col := range cols {
		cell := cellAt(row, i)
		switch col.role {
		case roleName:
			if m.Name == "" {
				m.Name = clean(cell)
			}
		case roleAuthor:
			m.SetAttr(AttrAuthor, clean(cell))
		case roleModality:
			m.SetAttr(AttrModality, clean(cell))
			if k, ok := modalityKind(cell); ok {
				m.Kind = k
			}
		case roleContext:
			m.SetLimit(LimitContextWindow, parseCount(cell))
		case roleDimension:
			if dim := clean(cell); dim != "" && dim != "-" {
				m.SetAttr(AttrDefaultDimension, dim)
				m.AddList(ListDimensions, dim)
			}
		case roleModelSize:
			m.SetAttr(AttrModelSize, valueOrEmpty(cell))
		case roleSteps:
			m.SetAttr(AttrDefaultSteps, valueOrEmpty(cell))
		case roleGeometry:
			dims = dims.Merge(geometryDims(cell))
		}
	}
	for i, col := range cols {
		if col.role != rolePrice {
			continue
		}
		a := parseAmount(cellAt(row, i))
		if !a.Found {
			continue
		}
		unit := a.Unit
		if unit == "" {
			unit = col.unit
		}
		m.AddPrice(catalog.Price{
			Metric:   col.metric,
			Unit:     unit,
			Amount:   a.Value,
			Currency: currency,
			Dims:     dims,
		})
	}
}

// geometryDims reads the "720p / 5s" cell that says what one video purchase
// buys, which is what a per-video rate is a rate for.
func geometryDims(cell string) catalog.Dims {
	resolution, duration, ok := strings.Cut(clean(cell), "/")
	if !ok {
		return nil
	}
	return catalog.Dims{
		DimResolution: strings.ToLower(strings.TrimSpace(resolution)),
		DimDuration:   strings.ToLower(strings.TrimSpace(duration)),
	}
}

// valueOrEmpty returns a cell's value, treating the dash Together writes for
// "not applicable" as absent.
func valueOrEmpty(cell string) string {
	if text := clean(cell); text != "-" {
		return text
	}
	return ""
}
