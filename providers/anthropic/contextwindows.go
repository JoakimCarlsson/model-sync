package anthropic

import (
	"regexp"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// ContextWindowsURL is the guide to what a context window holds. It is not
// read for the window itself, which the comparison table states per model and
// this page states per group, but for the one bound only it states: how many
// images or PDF pages a single request may carry.
const ContextWindowsURL = baseURL + "/build-with-claude/context-windows.md"

// imageLimitRe matches the sentence stating that bound. Anthropic writes it as
// one number qualified by another, giving the larger figure and then the
// smaller one for models with the smaller window, so both numbers and the
// window that selects between them come out of the same sentence.
var imageLimitRe = regexp.MustCompile(
	`A single request can include up to ([\d,]+) images or PDF pages ` +
		`\(([\d,]+) for models with a ([\d.,]+[kKmM]?)-token context window\)`,
)

// applyImageLimit records how many images or PDF pages one request may carry.
//
// Anthropic states this per context window rather than per model: the larger
// figure for everything, less a smaller figure for models with the smaller
// window. Every model's window is already known from the comparison table, so
// the sentence resolves to a number for each without anything being assumed
// about any of them: the qualified figure where the window is the one the
// qualification names, and the unqualified figure otherwise. A model with no
// window recorded is left alone, since nothing selects a branch for it.
func (b *builder) applyImageLimit(doc catalog.Document) {
	match := imageLimitRe.FindStringSubmatch(clean(string(doc.Body)))
	if match == nil {
		return
	}
	standard, reduced := parseCount(match[1]), parseCount(match[2])
	window := parseTokenCount(match[3] + " tokens")
	if standard == 0 || reduced == 0 || window == 0 {
		return
	}
	for _, id := range b.order {
		m := b.models[id]
		if m.Kind != KindChat || withdrawn(m) {
			continue
		}
		switch m.Limits[LimitContextWindow] {
		case 0:
		case window:
			m.SetLimit(LimitMaxImagesPerRequest, reduced)
			m.AddSource(doc.URL)
		default:
			m.SetLimit(LimitMaxImagesPerRequest, standard)
			m.AddSource(doc.URL)
		}
	}
}
