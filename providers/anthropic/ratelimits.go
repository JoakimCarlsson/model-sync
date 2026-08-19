package anthropic

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// RateLimitsURL is the page stating how much of a model an organization may
// use per minute. Anthropic publishes a full table per usage tier, and the
// tiers are named rather than numbered: Start, Build and Scale carry published
// numbers, and Custom is arranged with an account team and carries none.
const RateLimitsURL = baseURL + "/api/rate-limits.md"

// rateCounter is one of the three things Anthropic meters per minute. It
// keeps input and output apart rather than adding them into one tokens per
// minute figure, and a catalog that merged them would be inventing a number no
// document states.
type rateCounter struct {
	prefix string
	match  string
}

// rateCounters are matched on the abbreviation each column header ends with,
// which is the part of the header that does not change.
var rateCounters = []rateCounter{
	{LimitRPMPrefix, "(rpm)"},
	{LimitITPMPrefix, "(itpm)"},
	{LimitOTPMPrefix, "(otpm)"},
}

var (
	// tabTitleRe matches the element the rate tables are grouped under. The
	// tier a table states appears nowhere in the table itself, only in the tab
	// that holds it, so the tab is tracked the way scanTables tracks headings.
	tabTitleRe = regexp.MustCompile(`(?i)<Tab title="([^"]+)">`)
	// markerRe matches a row label's trailing footnote markers, which
	// Anthropic writes as escaped asterisks so they survive as text.
	markerRe = regexp.MustCompile(`\*+$`)
	// footnoteLineRe matches the italicized line a marker refers to.
	footnoteLineRe = regexp.MustCompile(`^\*((?:\\\*)+)\s+(.*?)\*?$`)
	// combinedRe matches the footnote naming the models that share one limit.
	combinedRe = regexp.MustCompile(
		`(?i)applies to combined traffic across (.+?)\.(?:\s|$)`,
	)
)

// applyRateLimits records the per-minute ceilings each usage tier allows.
//
// Two rows of each table name a model family rather than a model, because
// Anthropic meters one bucket across a generation: Claude Opus 4.x is one
// limit shared by four models. The footnote the row's marker points at names
// them, so the row is expanded onto each and the sharing is kept as a note.
// Without the footnote the row names nothing that exists and is dropped, which
// is the right outcome: a family is not a model and the limit is not any one
// model's.
func (b *builder) applyRateLimits(doc catalog.Document) {
	body := string(doc.Body)
	combined := combinedModels(body)
	for _, t := range tieredTables(body, doc.URL) {
		tier := tierName(t.Section)
		if tier == "" || !headerIs(t, 0, "model") {
			continue
		}
		b.applyRateLimitTable(t, tier, combined)
	}
}

// applyRateLimitTable records one tier's table.
func (b *builder) applyRateLimitTable(
	t mdTable,
	tier string,
	combined map[string][]string,
) {
	cols := map[int]string{}
	for i, header := range t.Headers {
		for _, counter := range rateCounters {
			if strings.HasSuffix(
				strings.ToLower(clean(header)),
				counter.match,
			) {
				cols[i] = counter.prefix + tier
			}
		}
	}
	if len(cols) == 0 {
		return
	}
	for _, row := range t.Rows {
		label, marker := splitMarker(cellAt(row, 0))
		for _, name := range rateLimitNames(label, marker, combined) {
			b.applyRateLimitRow(t, row, cols, name, marker, combined)
		}
	}
}

// applyRateLimitRow records one model's ceilings from a row.
func (b *builder) applyRateLimitRow(
	t mdTable,
	row []string,
	cols map[int]string,
	name, marker string,
	combined map[string][]string,
) {
	m, ok := b.models[b.resolve(name)]
	if !ok {
		return
	}
	m.AddSource(t.Source)
	for column, key := range cols {
		m.SetLimit(key, parseCount(cellAt(row, column)))
	}
	if shared, ok := combined[marker]; ok {
		m.AddNote(
			"rate limits are one bucket shared across " +
				strings.Join(shared, ", "),
		)
	}
}

// rateLimitNames turns a row's label into the models it states a limit for.
// A label naming a model states it for that model; a label naming a family
// states it for every model the family's footnote lists.
func rateLimitNames(
	label, marker string,
	combined map[string][]string,
) []string {
	if shared, ok := combined[marker]; ok {
		return shared
	}
	refs := splitModelCell(label)
	if len(refs) == 0 {
		return nil
	}
	return []string{refs[0].Name}
}

// splitMarker separates a row label from the footnote markers glued to it,
// which is how Anthropic flags a row that states a shared limit.
func splitMarker(cell string) (label, marker string) {
	text := clean(cell)
	marker = markerRe.FindString(text)
	return strings.TrimSpace(strings.TrimSuffix(text, marker)), marker
}

// combinedModels reads the footnotes naming the models that share one limit,
// keyed by the marker that points at each. Anthropic distinguishes two such
// footnotes on this page by the number of asterisks alone.
func combinedModels(body string) map[string][]string {
	out := map[string][]string{}
	for _, line := range strings.Split(body, "\n") {
		match := footnoteLineRe.FindStringSubmatch(strings.TrimSpace(line))
		if match == nil {
			continue
		}
		names := combinedRe.FindStringSubmatch(match[2])
		if names == nil {
			continue
		}
		out[strings.ReplaceAll(match[1], `\`, "")] = splitNames(names[1])
	}
	return out
}

// tierName reduces a tab title such as "Start tier" to the tier it names, and
// reports nothing for a tab holding no published numbers.
func tierName(title string) string {
	name, ok := strings.CutSuffix(strings.ToLower(clean(title)), " tier")
	if !ok || name == "custom" {
		return ""
	}
	return name
}

// tieredTables walks a document and returns every pipe table together with the
// tab title in force above it, which is where this page states the tier a
// table belongs to. It is the same walk scanTables makes, over a different
// marker, because these tables are indistinguishable from one another.
func tieredTables(body, source string) []mdTable {
	var (
		out     []mdTable
		title   string
		current *mdTable
	)
	for _, raw := range strings.Split(skipFrontmatter(body), "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "|") {
			if current == nil {
				out = append(out, mdTable{Section: title, Source: source})
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
		if match := tabTitleRe.FindStringSubmatch(line); match != nil {
			title = match[1]
		}
	}
	return out
}
