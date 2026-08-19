package google

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// LimitBatchEnqueued prefixes the number of tokens a project may have waiting
// in batch jobs for one model. Google states one figure per usage tier, so the
// tier is part of the key.
const LimitBatchEnqueued = "batch_enqueued_tokens_tier_"

// tierHeadingRe matches the heading introducing one usage tier's table.
var tierHeadingRe = regexp.MustCompile(`(?i)^tier\s+(\d)$`)

// applyRateLimits reads the rate limit page for the one figure it states per
// model. Everything else on that page is stated per project or per tier rather
// than per model: the spend ceiling, the usage tier qualifications, the
// concurrent batch count and the priority multiplier all belong to the account
// and not to anything the catalog holds.
//
// The tables are read as a running state of which tier is in force, since the
// tier is a heading above the table rather than a column inside it.
func (b *builder) applyRateLimits(doc catalog.Document) {
	tier := ""
	for _, at := range marks(string(doc.Body)) {
		if at.heading != "" {
			tier = ""
			if match := tierHeadingRe.FindStringSubmatch(
				strings.TrimSpace(at.heading),
			); match != nil {
				tier = match[1]
			}
			continue
		}
		if tier == "" {
			continue
		}
		cells := rowCells(at.row)
		if len(cells) != 2 {
			continue
		}
		m := b.named(cells[0])
		if m == nil {
			continue
		}
		count := parseCount(cells[1])
		if count == 0 {
			continue
		}
		m.AddSource(doc.URL)
		m.SetLimit(LimitBatchEnqueued+tier, count)
	}
}

// named returns the model a document refers to by a name rather than by an
// endpoint. The rate limit page writes both: most of its rows name the model
// the way the index does, and a few name the endpoint itself, so the endpoint
// is tried first and the index's names second. A row naming neither, which is
// how the page writes a model the pricing page does not price and the
// pictogram it hangs off the image models, matches nothing and is skipped.
func (b *builder) named(name string) *catalog.Model {
	key := slugID(nbspReplacer.Replace(name))
	if m, ok := b.models[key]; ok {
		return m
	}
	if id, ok := b.byName[key]; ok {
		return b.models[id]
	}
	return nil
}
