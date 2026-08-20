package ollama

import (
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// LimitContextWindow is the one numeric bound Ollama publishes, and it is on
// the model's tag listing rather than in the library.
const LimitContextWindow = "context_window"

// defaultTag is the build Ollama serves when a request names no other, and so
// the one whose context window is the model's.
const defaultTag = "latest"

// tagsPath is what a model's tag listing is filed under.
const tagsPath = "/tags"

// AttrLastUpdated is the day Ollama last changed the model, which the listing
// states twice: once as an age in words, "10 months ago", and once as the
// instant behind it. Only the instant is a date, so the age is not recorded.
const AttrLastUpdated = "last_updated"

// updatedRe matches that instant. The listing writes it as the tooltip of the
// age it shows, and it is the only tooltip on the page shaped like a date.
var updatedRe = regexp.MustCompile(
	`title="([A-Z][a-z]{2}) (\d{1,2}), (\d{4})[^"]*"`,
)

// months map the listing's spelling of a month onto its number.
var months = map[string]string{
	"Jan": "01", "Feb": "02", "Mar": "03", "Apr": "04",
	"May": "05", "Jun": "06", "Jul": "07", "Aug": "08",
	"Sep": "09", "Oct": "10", "Nov": "11", "Dec": "12",
}

// updatedDate reads the day of the instant the listing states, or the empty
// string where it states none.
func updatedDate(body []byte) string {
	match := updatedRe.FindSubmatch(body)
	if match == nil {
		return ""
	}
	month, ok := months[string(match[1])]
	if !ok {
		return ""
	}
	day := string(match[2])
	if len(day) == 1 {
		day = "0" + day
	}
	return string(match[3]) + "-" + month + "-" + day
}

// tagRowRe matches one row of the tag listing: the build it names, the context
// window it holds and the modalities it takes.
//
// The three are matched together rather than separately because the listing
// states them once per build and a model has many, so a row's own bounds are
// what tie them to each other.
//
// The thousands suffix and the input column are both optional. A window under
// a thousand is written plainly, "512 context window", which is how every
// small embedding model states its own, and requiring the suffix lost the row
// entirely rather than losing the suffix.
var tagRowRe = regexp.MustCompile(
	`(?is)href="/library/([^"]+)".{0,900}?` +
		`([\d.]+)\s*([KM])?\s*context window` +
		`(?:.{0,200}?([A-Za-z, ]+?)\s*input)?`,
)

// applyTagListing reads a model's tag listing.
//
// Every build of a model gets a row, and they differ: a quantization at one
// size may hold a shorter context than another. The build recorded is the one
// Ollama serves by default, since that is what running the model plainly
// gives, and the first row otherwise.
func (b *builder) applyTagListing(doc catalog.Document) {
	id := path.Base(strings.TrimSuffix(doc.URL, tagsPath))
	m, ok := b.models[id]
	if !ok {
		return
	}
	row := defaultRow(doc.Body)
	if row == nil {
		return
	}
	m.AddSource(doc.URL)
	m.SetAttr(AttrDefaultSnapsh, row[1])
	m.SetAttr(catalog.APIID, row[1])
	if date := updatedDate(doc.Body); date != "" {
		m.SetAttr(AttrLastUpdated, date)
	}
	m.SetLimit(LimitContextWindow, parseCount(row[2], row[3]))
	for _, name := range strings.Split(row[4], ",") {
		addModality(m, ListInputModalities, strings.TrimSpace(name))
	}
	addModality(m, ListOutputModalities, "text")
}

// defaultRow returns the listing row of the build Ollama serves when a request
// names no other, since that is what running the model plainly gives, and the
// first row otherwise.
func defaultRow(body []byte) []string {
	rows := tagRowRe.FindAllStringSubmatch(string(body), -1)
	if len(rows) == 0 {
		return nil
	}
	for _, candidate := range rows {
		if strings.HasSuffix(candidate[1], ":"+defaultTag) {
			return candidate
		}
	}
	return rows[0]
}

// parseCount reads a quantity written with a thousands or millions suffix,
// which the listing writes on every window of a thousand or more and on none
// below it.
func parseCount(digits, suffix string) int64 {
	n, err := strconv.ParseFloat(digits, 64)
	if err != nil {
		return 0
	}
	switch strings.ToUpper(suffix) {
	case "K":
		n *= 1_000
	case "M":
		n *= 1_000_000
	}
	return int64(n)
}
