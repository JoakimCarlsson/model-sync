package groq

import (
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// RateLimitsURL is the page stating a limit per model in each of the six
// counters Groq measures usage in.
const RateLimitsURL = baseURL + "/docs/rate-limits.md"

// baseLimitsNote records where the numbers under the base keys come from and
// why they are not the numbers the models table states. Nothing reading the
// aggregate can see a package comment, and the two documents disagree.
const baseLimitsNote = "the rate limits page calls these the base limits; " +
	"the models table states higher figures under the developer plan"

// headModelID opens the row naming the counters, which is written as a row of
// its own above the models rather than as the table's header.
const headModelID = "MODEL ID"

// applyRateLimits reads the rate limits page.
//
// The page is one row per model and one column per counter, but it is not one
// table: the header sits in a table of its own and every model's row opens a
// table of its own, because each is written with its own rule under it. So the
// rows are read as rows and the columns are taken from the header row wherever
// it appears, rather than by scanning for a table.
//
// The counters are Groq's own codes, expanded by the page itself: requests and
// tokens per minute and per day, and audio seconds per hour and per day.
func (b *builder) applyRateLimits(doc catalog.Document) {
	var order []string
	for _, raw := range strings.Split(string(doc.Body), "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := splitRow(line)
		if isSeparator(cells) {
			continue
		}
		if strings.EqualFold(clean(cellAt(cells, 0)), headModelID) {
			order = counterOrder(cells)
			continue
		}
		b.applyRateLimitRow(cells, order, doc.URL)
	}
}

// applyRateLimitRow records one model's limits.
func (b *builder) applyRateLimitRow(
	cells, order []string,
	source string,
) {
	if len(order) == 0 {
		return
	}
	m, ok := b.models[clean(cellAt(cells, 0))]
	if !ok {
		return
	}
	var recorded bool
	for i, key := range order {
		if key == "" {
			continue
		}
		value := parseCount(cellAt(cells, i+1))
		if value == 0 {
			continue
		}
		m.SetLimit(key, value)
		recorded = true
	}
	if recorded {
		m.AddSource(source)
		m.AddNote(baseLimitsNote)
	}
}

// counterOrder reads which counter each column after the model holds.
func counterOrder(cells []string) []string {
	out := make([]string, 0, len(cells))
	for _, cell := range cells[1:] {
		out = append(out, baseRateLimitKeys[strings.ToLower(clean(cell))])
	}
	return out
}
