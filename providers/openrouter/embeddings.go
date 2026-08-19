package openrouter

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// singleWidth and listedWidths match the two ways an embedding model's
// description states the width of the vector it returns.
//
// The width is nowhere in the structured fields: no member of the model entry
// and no member of an endpoint carries it, and the only place OpenRouter
// states it is the sentence describing what the model does. Both patterns are
// anchored on the word that makes the number a width, so a parameter count or
// a context length in the same paragraph cannot be taken for one.
var (
	singleWidth  = regexp.MustCompile(`([0-9][0-9,]*)-dimensional`)
	listedWidths = regexp.MustCompile(
		`in ((?:[0-9][0-9,]*, )+(?:and )?[0-9][0-9,]*) dimensions`,
	)
)

// applyEmbedding records the width of the vector an embedding model returns,
// where its description states one.
//
// A model offering a choice of widths records the choice, and a model
// returning one width records that, since the two are different facts: the
// first says the caller may ask for less, and the second says what the caller
// gets.
func applyEmbedding(m *catalog.Model, description string) {
	if m.Kind != KindEmbedding {
		return
	}
	text := strings.Join(strings.Fields(description), " ")
	if listed := listedWidths.FindStringSubmatch(text); listed != nil {
		for _, width := range strings.Split(listed[1], ",") {
			m.AddList(ListEmbeddingDimensions, digitsOf(width))
		}
		return
	}
	if single := singleWidth.FindStringSubmatch(text); single != nil {
		m.SetAttr(AttrDefaultEmbeddingDimension, digitsOf(single[1]))
	}
}

// digitsOf strips the grouping a published number is written with, so that
// "1,024" and "1024" record as the same width.
func digitsOf(text string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, text)
}
