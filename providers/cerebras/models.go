package cerebras

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// KindChat is the only kind Cerebras serves.
const KindChat catalog.Kind = "chat"

// States the public model list distinguishes a model by. It carries a flag
// for a model in preview and a flag for one on its way out, and a model
// carrying neither is one Cerebras is selling without qualification.
const (
	StateActive     = "active"
	StatePreview    = "preview"
	StateDeprecated = "deprecated"
)

// Scalar keys the catalog populates.
const (
	AttrState = "state"
	// AttrParameterCount is the size of the model in weights, which is not the
	// list of request parameters its API accepts. Those are an enumeration and
	// are recorded under catalog.ListParameters.
	AttrParameterCount = "parameter_count"
	AttrTokensPerSec   = "tokens_per_second"
	AttrModelURL       = "model_url"
)

// Numeric keys the catalog populates. Cerebras allows a longer context on a
// paid plan than a free one, so both ceilings are recorded.
const (
	LimitContextWindow     = "context_window"
	LimitContextWindowFree = "context_window_free"
)

var (
	linkRe = regexp.MustCompile(`\[([^\]]*)\]\(([^)]*)\)`)
	tagRe  = regexp.MustCompile(`(?s)<[^>]*>`)
	// supRe matches a footnote marker with the digit inside it, which is not
	// part of the name it is attached to.
	supRe = regexp.MustCompile(`(?is)<sup\b[^>]*>.*?</sup\s*>`)
	// tabRe matches the tab a table stands in, which is how Cerebras heads a
	// table that shares a heading with the tables beside it.
	tabRe   = regexp.MustCompile(`(?i)<Tab\s+title="([^"]*)"`)
	countRe = regexp.MustCompile(
		`(?i)([\d,]*\.?\d+)\s*(k|m|b|billion|million)?`,
	)
)

// clean strips markdown decoration and the footnote markers Cerebras puts in
// model names.
func clean(cell string) string {
	s := linkRe.ReplaceAllString(cell, "$1")
	s = supRe.ReplaceAllString(s, "")
	s = tagRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, `\~`, "~")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "`", "")
	return strings.Join(strings.Fields(s), " ")
}

// parseCount reads a quantity such as "131k" or "120 billion".
func parseCount(value string) int64 {
	match := countRe.FindStringSubmatch(strings.TrimSpace(clean(value)))
	if match == nil {
		return 0
	}
	n, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", ""), 64)
	if err != nil {
		return 0
	}
	switch strings.ToLower(match[2]) {
	case "k":
		n *= 1_000
	case "m", "million":
		n *= 1_000_000
	case "b", "billion":
		n *= 1_000_000_000
	}
	return int64(n)
}

// modelPageURLs derives the per-model pages the catalog links to. The link is
// in the model name cell, alongside the identifier the model is called by.
func modelPageURLs(index catalog.Document) []string {
	var urls []string
	for _, t := range scanTables(string(index.Body), index.URL) {
		nameCol := columnOf(t.Headers, "model name")
		if nameCol < 0 {
			continue
		}
		for _, row := range t.Rows {
			target := linkTarget(cellAt(row, nameCol))
			if !strings.HasPrefix(target, "/models/") {
				continue
			}
			url := baseURL + target + ".md"
			if !slices.Contains(urls, url) {
				urls = append(urls, url)
			}
		}
	}
	slices.Sort(urls)
	return urls
}

// applyCatalog reads the model catalog page.
//
// Every table naming a model identifier is read, whatever heading it stands
// under. Cerebras has written the catalog as one table and as a table per
// standing at different times, and a reader keyed on the headings it happened
// to use silently stops reading the day they change.
func (b *builder) applyCatalog(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		idCol := columnOf(t.Headers, "model id")
		if idCol < 0 {
			continue
		}
		var (
			nameCol    = columnOf(t.Headers, "model name")
			paramCol   = columnOf(t.Headers, "parameters")
			contextCol = columnOf(t.Headers, "context (free / paid)")
			speedCol   = columnOf(t.Headers, "speed (tokens/s)")
		)
		for _, row := range t.Rows {
			id := clean(cellAt(row, idCol))
			if id == "" {
				continue
			}
			m := b.model(id, KindChat)
			m.AddSource(t.Source)
			if m.Name == "" {
				m.Name = clean(cellAt(row, nameCol))
			}
			m.SetAttr(AttrParameterCount, clean(cellAt(row, paramCol)))
			m.SetAttr(AttrTokensPerSec, clean(cellAt(row, speedCol)))
			m.SetAttr(AttrModelURL, linkTarget(cellAt(row, nameCol)))
			applyContext(m, cellAt(row, contextCol))
		}
	}
}

// AttrDeprecatedOn is the date Cerebras has said a model will go on. It is a
// date rather than a state, because the model is served until it arrives and
// recording it as withdrawn would drop from the catalog something still sold.
const AttrDeprecatedOn = "deprecated_on"

// deprecationRe matches the notice the catalog opens with when a model has a
// date to go by. It names the model as the catalog's own tables name it.
var deprecationRe = regexp.MustCompile(
	`(?i)\*\*([^*]+)\*\*\s+is scheduled for deprecation on ([^.\n]+)`,
)

// applyDeprecations reads the notice above the tables.
func (b *builder) applyDeprecations(doc catalog.Document) {
	body := string(doc.Body)
	for _, match := range deprecationRe.FindAllStringSubmatch(body, -1) {
		name := clean(match[1])
		for _, m := range b.models {
			if m.Name == name {
				m.SetAttr(AttrDeprecatedOn, isoDate(match[2]))
				m.AddSource(doc.URL)
			}
		}
	}
}

// isoDate normalizes the date the notice writes in prose.
func isoDate(value string) string {
	text := clean(value)
	if t, err := time.Parse("January 2, 2006", text); err == nil {
		return t.Format("2006-01-02")
	}
	return text
}

// applyContext reads the "65k / 131k" cell stating the free and paid ceilings.
func applyContext(m *catalog.Model, cell string) {
	free, paid, ok := strings.Cut(clean(cell), "/")
	if !ok {
		m.SetLimit(LimitContextWindow, parseCount(cell))
		return
	}
	m.SetLimit(LimitContextWindowFree, parseCount(free))
	m.SetLimit(LimitContextWindow, parseCount(paid))
}

// linkTarget returns the page a cell links to.
func linkTarget(cell string) string {
	match := linkRe.FindStringSubmatch(cell)
	if match == nil {
		return ""
	}
	return strings.TrimSpace(match[2])
}

// table is one markdown table with the heading above it.
type table struct {
	Section string
	Headers []string
	Rows    [][]string
	Source  string
}

// scanTables walks a document and returns every pipe table, tracking the
// nearest preceding heading, or the tab the table stands in where Cerebras
// puts several tables under one heading and tells them apart by tab.
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
		if match := tabRe.FindStringSubmatch(line); match != nil {
			section = strings.ToLower(clean(match[1]))
			continue
		}
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

// columnOf returns the index of the column with the given heading, or -1.
func columnOf(headers []string, name string) int {
	for i, h := range headers {
		if strings.EqualFold(clean(h), name) {
			return i
		}
	}
	return -1
}
