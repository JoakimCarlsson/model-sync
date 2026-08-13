package mistral

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Columns of the deprecation table, in the order Mistral writes them.
const (
	colName        = 0
	colVersion     = 1
	colAPI         = 2
	colDates       = 3
	colAlternative = 4
	columnCount    = 5
)

var (
	rowRe  = regexp.MustCompile(`(?is)<tr[^>]*>(.*?)</tr>`)
	cellRe = regexp.MustCompile(`(?is)<t[hd][^>]*>(.*?)</t[hd]\s*>`)
	tagRe  = regexp.MustCompile(`(?s)<[^>]*>`)
	// dateRe matches one of the two dates packed into a single cell.
	dateRe = regexp.MustCompile(`\d{1,2}/\d{1,2}/\d{4}`)
)

// text strips markup and the arrow Mistral appends to a linked model name.
func text(html string) string {
	s := tagRe.ReplaceAllString(html, " ")
	s = strings.ReplaceAll(s, "↗", "")
	return strings.Join(strings.Fields(s), " ")
}

// applyDeprecations reads the deprecation table on the index page. The
// per-model pages state one lifecycle date each, whichever has passed, so this
// table is what supplies the retirement date of a model that is deprecated but
// still serving.
func (b *builder) applyDeprecations(doc catalog.Document) {
	for _, match := range rowRe.FindAllStringSubmatch(string(doc.Body), -1) {
		cells := rowCells(match[1])
		if len(cells) < columnCount {
			continue
		}
		id := cells[colAPI]
		if id == "" || strings.EqualFold(cells[colName], "model") {
			continue
		}
		m := b.model(id, "")
		m.AddSource(doc.URL)
		if m.Name == "" {
			m.Name = cells[colName]
		}
		m.SetAttr(AttrVersion, cells[colVersion])
		m.SetAttr(AttrReplacement, cells[colAlternative])
		deprecated, retired := splitDates(cells[colDates])
		m.SetAttr(AttrDeprecatedOn, deprecated)
		m.SetAttr(AttrRetirementDate, retired)
	}
}

// splitDates separates the deprecation date from the retirement date, which
// Mistral renders as two lines and serves as one cell with nothing between
// them. The first is the deprecation and the second the retirement.
func splitDates(cell string) (deprecated, retired string) {
	found := dateRe.FindAllString(cell, -1)
	if len(found) > 0 {
		deprecated = isoDate(found[0])
	}
	if len(found) > 1 {
		retired = isoDate(found[1])
	}
	return deprecated, retired
}

// rowCells returns the text of one row's cells.
func rowCells(row string) []string {
	matches := cellRe.FindAllStringSubmatch(row, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, text(m[1]))
	}
	return out
}
