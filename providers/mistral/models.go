package mistral

import (
	"regexp"
	"strings"
	"time"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Kinds Mistral publishes. The deprecation table states no modality, so what
// a model does is read from its name, which Mistral is consistent about:
// Voxtral hears, Codestral writes code, and the rest say what they are.
const (
	KindChat          catalog.Kind = "chat"
	KindTranscription catalog.Kind = "transcription"
	KindEmbedding     catalog.Kind = "embedding"
	KindModeration    catalog.Kind = "moderation"
	KindOCR           catalog.Kind = "ocr"
)

// nameKinds map a fragment of a model's name onto what it does.
var nameKinds = []struct {
	fragment string
	kind     catalog.Kind
}{
	{"voxtral", KindTranscription},
	{"ocr", KindOCR},
	{"moderation", KindModeration},
	{"embed", KindEmbedding},
}

// kindFor reports what a model does, read from its identifier.
func kindFor(id string) catalog.Kind {
	lower := strings.ToLower(id)
	for _, entry := range nameKinds {
		if strings.Contains(lower, entry.fragment) {
			return entry.kind
		}
	}
	return KindChat
}

// StateRetired is the standing of everything in the table.
const StateRetired = "retired"

// Scalar keys the deprecation table populates.
const (
	AttrState          = "state"
	AttrVersion        = "version"
	AttrDeprecatedOn   = "deprecated_on"
	AttrRetirementDate = "retirement_date"
	AttrReplacement    = "recommended_replacement"
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

// applyDeprecations reads the deprecation table.
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
		m := b.model(id, kindFor(id))
		m.AddSource(doc.URL)
		if m.Name == "" {
			m.Name = cells[colName]
		}
		m.SetAttr(AttrState, StateRetired)
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

// isoDate rewrites the month-first dates Mistral writes into calendar order.
func isoDate(value string) string {
	trimmed := strings.TrimSpace(value)
	for _, layout := range []string{"1/2/2006", "01/02/2006"} {
		if t, err := time.Parse(layout, trimmed); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return trimmed
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
