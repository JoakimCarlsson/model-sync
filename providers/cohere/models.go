package cohere

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Kinds of model Cohere publishes.
const (
	KindChat          catalog.Kind = "chat"
	KindEmbedding     catalog.Kind = "embedding"
	KindRerank        catalog.Kind = "rerank"
	KindTranscription catalog.Kind = "transcription"
)

// Scalar keys the overview populates.
const (
	AttrSummary          = "summary"
	AttrState            = "state"
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
	ListEndpoints  = "endpoints"
	ListModalities = "modalities"
	ListDimensions = "embedding_dimensions"
)

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
			b.applyRow(m, t, row)
		}
	}
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
			m.SetAttr(AttrState, strings.ToLower(value))
		case "description":
			m.SetAttr(AttrSummary, firstSentence(value))
		case "modality", "modalities":
			m.SetAttr(AttrModality, value)
			m.AddList(ListModalities, splitList(value)...)
		case "context length":
			m.SetLimit(LimitContextWindow, parseCount(value))
		case "maximum output tokens":
			m.SetLimit(LimitMaxOutputTokens, parseCount(value))
		case "dimensions":
			m.AddList(ListDimensions, splitList(value)...)
			m.SetAttr(AttrDefaultDimension, firstOf(splitList(value)))
		case "similarity metric":
			m.SetAttr(AttrSimilarityMetric, value)
		case "maximum file size":
			m.SetAttr(AttrMaxFileSize, value)
		case "endpoints":
			m.AddList(ListEndpoints, splitList(value)...)
		}
	}
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

// firstOf returns the first item, or the empty string.
func firstOf(items []string) string {
	if len(items) == 0 {
		return ""
	}
	return items[0]
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
