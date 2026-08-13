package cohere

import (
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Kinds of model Cohere publishes.
const (
	KindChat          catalog.Kind = "chat"
	KindEmbedding     catalog.Kind = "embedding"
	KindRerank        catalog.Kind = "rerank"
	KindTranscription catalog.Kind = "transcription"
)

// Standing a model can be in, read from the overview's status column.
const (
	StateLive       = "live"
	StateDeprecated = "deprecated"
	StateRetired    = "retired"
)

// Scalar keys the overview populates.
const (
	AttrSummary          = "summary"
	AttrState            = "state"
	AttrDeprecatedOn     = "deprecated_on"
	AttrRetirementDate   = "retirement_date"
	AttrModality         = "modality"
	AttrSimilarityMetric = "similarity_metric"
	AttrMaxFileSize      = "max_file_size"
	AttrDefaultDimension = "default_embedding_dimension"
	AttrFamily           = "family"
)

// platformAttrs maps a cloud column onto the key recording the identifier the
// model answers to there.
var platformAttrs = map[string]string{
	"amazon bedrock model id":          "amazon_bedrock_id",
	"amazon sagemaker":                 "amazon_sagemaker_id",
	"azure ai foundry":                 "azure_ai_foundry_id",
	"oracle oci generative ai service": "oracle_oci_id",
}

// Numeric keys the overview populates.
const (
	LimitContextWindow   = "context_window"
	LimitMaxOutputTokens = "max_output_tokens"
)

// Enumeration keys the overview populates.
const (
	ListEndpoints        = "endpoints"
	ListInputModalities  = "input_modalities"
	ListOutputModalities = "output_modalities"
	ListDimensions       = "embedding_dimensions"
)

// ModalityText is what a chat model returns. It is the catalog's word for it,
// and the same word the modality column uses for what a model takes.
const ModalityText = "text"

// sectionKinds maps a family heading onto what its models do.
var sectionKinds = map[string]catalog.Kind{
	"command": KindChat,
	"embed":   KindEmbedding,
	"rerank":  KindRerank,
	"audio":   KindTranscription,
	"aya":     KindChat,
}

var (
	linkRe  = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	tagRe   = regexp.MustCompile(`(?s)<[^>]*>`)
	countRe = regexp.MustCompile(`(?i)([\d,]+)\s*([km])?`)
)

// clean strips markdown decoration from a cell value.
func clean(cell string) string {
	s := linkRe.ReplaceAllString(cell, "$1")
	s = tagRe.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "`", "")
	return strings.Join(strings.Fields(s), " ")
}

// parseCount reads a quantity such as "128,000" or "256k".
func parseCount(cell string) int64 {
	match := countRe.FindStringSubmatch(clean(cell))
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
	case "m":
		n *= 1_000_000
	}
	return int64(n)
}

// applyOverview reads the model overview page.
func (b *builder) applyOverview(doc catalog.Document) {
	names := displayNames(string(doc.Body))
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		kind, ok := sectionKinds[family(t.Section)]
		if !ok {
			continue
		}
		idCol := columnOf(t.Headers, "model name")
		if idCol < 0 {
			continue
		}
		for _, row := range t.Rows {
			id := clean(cellAt(row, idCol))
			if id == "" {
				continue
			}
			m := b.model(id, kind)
			m.AddSource(t.Source)
			m.SetAttr(AttrFamily, family(t.Section))
			if m.Name == "" {
				m.Name = names[undated(id)]
			}
			b.applyRow(m, t, row)
			setOutputModality(m)
		}
	}
}

// setOutputModality records what a chat model gives back.
//
// The modality column states only what a model takes. What it returns is in the
// paragraph above the table: Command "takes a user instruction (or command) and
// generates text following the instruction", and the Aya models answer on the
// same Chat endpoint, one of them taking images and text and giving back "a
// single coherent response". Both families return text.
//
// Nothing is recorded for embed or rerank, which return a vector and a set of
// relevance scores. Neither is a modality, and naming them here would put a
// value in a field that no other provider's embedding models carry.
//
// A model the overview states no input modality for gets no output modality
// either. The nightly builds appear only in the table of platform identifiers,
// which has no modality column, and recording that one returns text while saying
// nothing about what it takes would read as a model that takes nothing.
func setOutputModality(m *catalog.Model) {
	if m.Kind != KindChat || len(m.Lists[ListInputModalities]) == 0 {
		return
	}
	m.AddList(ListOutputModalities, ModalityText)
}

// docLinkRe matches a link from the overview's opening summary to a model,
// which is where the model's display name is written.
var docLinkRe = regexp.MustCompile(`\[([A-Z][^\]]*)\]\(([^)]+)\)`)

// datedRe matches the release date Cohere suffixes an identifier with.
var datedRe = regexp.MustCompile(`-\d{2}-\d{4}$`)

// displayNames reads the display name of every model the overview links to,
// keyed by the identifier the name belongs to without its release date.
//
// The tables state no name beyond the identifier, but the summary above them
// names each model in prose and links it, and the link's address is the
// identifier without its date: command-a-plus-05-2026 is the model the summary
// calls Command A+. Where the summary links out to the marketing site instead,
// the name itself reduces to the identifier, which covers the models that
// carry no date at all.
func displayNames(body string) map[string]string {
	out := map[string]string{}
	for _, match := range docLinkRe.FindAllStringSubmatch(body, -1) {
		name := clean(match[1])
		for _, key := range []string{path.Base(match[2]), slugID(name)} {
			if _, ok := out[key]; !ok {
				out[key] = name
			}
		}
	}
	return out
}

// statusRe matches the status column, which Cohere writes as a standing
// followed, for a model on its way out, by the date it takes effect.
var statusRe = regexp.MustCompile(`(?i)^(\w+)\s*(.*)$`)

// splitStatus separates the standing from the date it takes effect. Cohere
// writes them as one phrase, "Deprecated Sept 15, 2025", which would otherwise
// become a state value no two models share.
func splitStatus(value string) (state, date string) {
	match := statusRe.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil {
		return strings.ToLower(strings.TrimSpace(value)), ""
	}
	return strings.ToLower(match[1]), isoDate(match[2])
}

// dateLayouts are the date formats Cohere writes.
var dateLayouts = []string{"Jan 2, 2006", "January 2, 2006", "2006-01-02"}

// monthSpellings normalize the abbreviations Cohere writes that no calendar
// layout matches. It shortens September to four letters and nothing else.
var monthSpellings = strings.NewReplacer("Sept ", "Sep ")

// isoDate rewrites a date into calendar order.
func isoDate(value string) string {
	text := monthSpellings.Replace(strings.TrimSpace(value))
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, text); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return text
}

// undated strips the release date from an identifier.
func undated(id string) string {
	return datedRe.ReplaceAllString(id, "")
}

// family reduces a heading to the model family it introduces, so that the
// platform tables under "Using Command Models on Different Platforms" are
// attributed to the same family as the models above them.
func family(section string) string {
	for name := range sectionKinds {
		if strings.Contains(section, name) {
			return name
		}
	}
	return section
}

// applyRow records every column the row has, since the five families share a
// vocabulary without sharing a shape.
func (b *builder) applyRow(m *catalog.Model, t table, row []string) {
	for i, header := range t.Headers {
		cell := cellAt(row, i)
		value := clean(cell)
		if value == "" || value == "-" {
			continue
		}
		if key, ok := platformAttrs[strings.ToLower(clean(header))]; ok {
			m.SetAttr(key, value)
			continue
		}
		switch strings.ToLower(clean(header)) {
		case "status":
			state, date := splitStatus(value)
			m.SetAttr(AttrState, state)
			if state == StateRetired {
				m.SetAttr(AttrRetirementDate, date)
				continue
			}
			m.SetAttr(AttrDeprecatedOn, date)
		case "description":
			m.SetAttr(AttrSummary, firstSentence(value))
		case "modality", "modalities":
			m.SetAttr(AttrModality, value)
			for _, item := range splitList(value) {
				m.AddList(ListInputModalities, modalityName(item))
			}
		case "context length":
			m.SetLimit(LimitContextWindow, parseCount(value))
		case "maximum output tokens":
			m.SetLimit(LimitMaxOutputTokens, parseCount(value))
		case "dimensions":
			widths, def := parseDimensions(value)
			m.AddList(ListDimensions, widths...)
			m.SetAttr(AttrDefaultDimension, def)
		case "similarity metric":
			m.SetAttr(AttrSimilarityMetric, value)
		case "maximum file size":
			m.SetAttr(AttrMaxFileSize, value)
		case "endpoints":
			m.AddList(ListEndpoints, splitList(value)...)
		}
	}
}

// dimensionRe matches one vector width and the marker that follows the width a
// model returns unless asked for another.
var dimensionRe = regexp.MustCompile(`(\d[\d,]*)\s*(\(default\))?`)

// parseDimensions reads the vector widths an embedding model offers and which
// of them it returns by default.
//
// A model offering one width states the number alone. A model offering a choice
// states the whole set as a sentence, "One of '[256, 512, 1024, 1536
// (default)]'", which cannot be taken as the cell split on its commas: that
// leaves the prose and the brackets attached to the first and last width, and it
// makes the first width the default when the cell says the last one is.
func parseDimensions(value string) (widths []string, def string) {
	for _, match := range dimensionRe.FindAllStringSubmatch(clean(value), -1) {
		width := strings.ReplaceAll(match[1], ",", "")
		widths = append(widths, width)
		if match[2] != "" {
			def = width
		}
	}
	if def == "" && len(widths) == 1 {
		def = widths[0]
	}
	return widths, def
}

// splitList divides a comma separated cell.
func splitList(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		if item := strings.TrimSpace(part); item != "" {
			out = append(out, item)
		}
	}
	return out
}

// firstSentence trims a description to its opening sentence, since Cohere's
// run to a paragraph.
func firstSentence(value string) string {
	if sentence, _, ok := strings.Cut(value, ". "); ok {
		return sentence + "."
	}
	return value
}

// table is one markdown table with the heading above it.
type table struct {
	Section string
	Headers []string
	Rows    [][]string
	Source  string
}

// scanTables walks a document and returns every pipe table, tracking the
// nearest preceding heading.
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
