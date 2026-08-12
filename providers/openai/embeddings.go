package openai

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Keys the embeddings guide populates.
const (
	AttrDefaultDimension = "default_embedding_dimension"
	FeatureReducibleDims = "reducible_embedding_dimensions"
)

// embeddingDimRe matches the sentence stating the width of an embedding.
//
// OpenAI writes this nowhere else. Its embedding model pages list modalities,
// endpoints and rates but never say how long the vector is, and the guide's
// own model table gives pages-per-dollar and a benchmark score instead. The
// number exists in one sentence of prose, so that is where it is read from.
var embeddingDimRe = regexp.MustCompile(
	"(?i)embedding vector is `?(\\d+)`? for `?([\\w.-]+)`?" +
		" or `?(\\d+)`? for `?([\\w.-]+)`?",
)

// applyEmbeddingsGuide reads the embeddings guide.
//
// Unlike providers that offer a fixed set of widths, OpenAI exposes a
// dimensions parameter that shortens the vector to any smaller length. There
// is therefore no list of dimensions to record, only the default and the fact
// that it can be reduced, and recording a list would invent a set of discrete
// options OpenAI does not publish.
func (b *builder) applyEmbeddingsGuide(doc catalog.Document) {
	body := string(doc.Body)
	for _, match := range embeddingDimRe.FindAllStringSubmatch(body, -1) {
		b.setDimension(doc.URL, match[2], match[1])
		b.setDimension(doc.URL, match[4], match[3])
	}
	for _, t := range scanMarkdownTables(doc) {
		b.applyEmbeddingTable(t)
	}
}

// setDimension records the default width of one model's embedding.
func (b *builder) setDimension(source, id, dimension string) {
	id = strings.Trim(strings.TrimSpace(id), "`")
	if id == "" || dimension == "" {
		return
	}
	m := b.model(id, KindEmbedding)
	m.AddSource(source)
	m.SetAttr(AttrDefaultDimension, dimension)
	m.AddList(ListFeatures, FeatureReducibleDims)
}

// applyEmbeddingTable reads the guide's model table, whose only fact not
// available elsewhere is the longest input a model accepts.
func (b *builder) applyEmbeddingTable(t mdTable) {
	idCol := columnOf(t.Headers, []string{"model"})
	maxCol := columnOf(t.Headers, []string{"max input"})
	if idCol < 0 || maxCol < 0 {
		return
	}
	for _, row := range t.Rows {
		id := strings.Trim(cellAt(row, idCol), "`")
		if id == "" {
			continue
		}
		m := b.model(id, KindEmbedding)
		m.AddSource(t.Source)
		m.SetLimit(LimitContextWindow, parseCount(cellAt(row, maxCol)))
	}
}
