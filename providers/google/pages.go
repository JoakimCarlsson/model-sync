package google

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Scalar keys the model pages populate.
const (
	AttrSummary     = "summary"
	AttrModelCode   = "model_code"
	AttrLatestUpdte = "latest_update"
	AttrModelCard   = "model_card_url"
)

// Numeric keys the model pages populate.
const (
	LimitContextWindow   = "context_window"
	LimitMaxOutputTokens = "max_output_tokens"
)

// Enumeration keys the model pages populate.
const (
	ListFeatures         = catalog.ListFeatures
	ListAliases          = "aliases"
	ListSnapshots        = "snapshots"
	ListInputModalities  = "input_modalities"
	ListOutputModalities = "output_modalities"
)

// Rows of the property table this parser reads, named as Google labels them.
const (
	rowModelCode = "model code"
	rowDataTypes = "supported data types"
	rowTokens    = "token limits"
	// rowLimits is what the video and omni pages head the same row with. They
	// state a bound in tokens like the rest and were skipped entirely for
	// being headed one word differently.
	rowLimits    = "limits"
	rowCaps      = "capabilities"
	rowConsuming = "consumption options"
	rowVersions  = "versions"
	rowUpdated   = "latest update"
	rowModelCard = "model card"
	fieldInputs  = "inputs"
	// fieldInput is the same field on an embedding model's page, which writes
	// the label in the singular.
	fieldInput    = "input"
	fieldOutput   = "output"
	fieldInLimit  = "input token limit"
	fieldOutLimit = "output token limit"
	// fieldContext and fieldTextInput are the other names Google gives the
	// input bound. Almost every page heads it "Input token limit"; the omni
	// model's heads the same count "Context window" and the video models' heads
	// the tokens their prompt may carry "Text input". Reading only the first
	// left all three with no bound at all.
	fieldContext   = "context window"
	fieldTextInput = "text input"
	// fieldDimension is the width of the vector an embedding model returns.
	fieldDimension = "output dimension size"
)

// ListDimensions holds the widths an embedding model can be asked for.
const ListDimensions = "embedding_dimensions"

// recommendedMarker introduces the widths Google names inside a field that
// otherwise states a range. A range is not a set of values a consumer can ask
// for, so only the widths named after this word are recorded.
const recommendedMarker = "recommended:"

// supported is how Google marks a capability a model has. It writes the same
// word with a qualifier for one still in preview, which is still a capability.
const supported = "supported"

// notSupported is how Google marks one it does not have. It contains the word
// above, so it is tested for first.
const notSupported = "not supported"

// featureNames map a capability Google names onto the catalog's vocabulary.
// Only the names that differ are listed; the rest are Google's own words with
// their spacing reduced to an identifier.
var featureNames = map[string]string{
	"caching":          "prompt_caching",
	"thinking":         "reasoning",
	"search grounding": "web_search",
	"batch api":        "batch",
}

// modalityNames map a supported data type onto the catalog's vocabulary.
// Google writes them in prose, in the singular or the plural and with a
// parenthesis saying what a modality is carrying, so each is reduced to its
// bare noun before being looked up.
//
// "Text embeddings" is what the embedding pages state as their output. The
// embedding is the return value rather than a modality, and the catalog has no
// word for a vector, so the phrase is read as the text those models work in.
// Dropping it left them stating what they take and nothing about what they give
// back.
var modalityNames = map[string]string{
	"text":            "text",
	"image":           "image",
	"video":           "video",
	"audio":           "audio",
	"pdf":             "file",
	"text embeddings": "text",
}

var (
	// tableRe matches the property table, which is the only table on a model
	// page and carries a class of its own.
	tableRe = regexp.MustCompile(
		`(?is)<table class="gemini-api-model-table">(.*?)</table>`,
	)
	pageRowRe  = regexp.MustCompile(`(?is)<tr[^>]*>(.*?)</tr>`)
	pageCellRe = regexp.MustCompile(`(?is)<td[^>]*>(.*?)</td\s*>`)
	// sectionRe matches one field of a cell holding several, which Google
	// writes as a bold name above its value.
	sectionRe = regexp.MustCompile(
		`(?is)<section>\s*<p><b>(.*?)</b></p>\s*<p>(.*?)</p>`,
	)
	// iconRe matches the pictogram Google puts before a row's label, whose
	// text would otherwise be read as part of the label.
	iconRe = regexp.MustCompile(
		`(?is)<span[^>]*class="google-symbols"[^>]*>.*?</span\s*>`,
	)
	// pageSupRe matches the footnote marker Google hangs off a row's label,
	// which is a reference rather than part of the label.
	pageSupRe = regexp.MustCompile(`(?is)<sup\b[^>]*>.*?</sup\s*>`)
	titleRe   = regexp.MustCompile(
		`(?is)<h1 class="devsite-page-title"[^>]*>(.*?)(?:<devsite|</h1)`,
	)
	summaryRe = regexp.MustCompile(`(?is)<p>([^<]{40,})</p>`)
	versionRe = regexp.MustCompile(
		`(?is)<li>([^:<]+):\s*<code[^>]*>(.*?)</code>`,
	)
	hrefRe  = regexp.MustCompile(`(?is)<a href="([^"]+)"`)
	countRe = regexp.MustCompile(`^([\d,]+)`)
	// widthRe matches one width of the list an embedding model's field names.
	widthRe = regexp.MustCompile(`\d[\d,]*`)
)

// applyModelPage reads one model's page onto the model the pricing page
// established for it, and adds nothing when no such model exists.
func (b *builder) applyModelPage(doc catalog.Document, id string) {
	table := tableRe.FindStringSubmatch(string(doc.Body))
	if table == nil {
		return
	}
	m, ok := b.models[id]
	if !ok {
		return
	}
	m.AddSource(doc.URL)
	if m.Name == "" {
		m.Name = text(first(titleRe, string(doc.Body)))
	}
	m.SetAttr(AttrSummary, text(first(summaryRe, string(doc.Body))))
	for _, row := range pageRowRe.FindAllStringSubmatch(table[1], -1) {
		cells := pageCellRe.FindAllStringSubmatch(row[1], -1)
		if len(cells) < 2 {
			continue
		}
		applyProperty(m, label(cells[0][1]), cells[1][1])
	}
}

// applyProperty records one row of the property table.
func applyProperty(m *catalog.Model, name, cell string) {
	switch name {
	case rowModelCode:
		m.SetAttr(AttrModelCode, text(cell))
		m.AddList(ListAliases, text(cell))
	case rowUpdated:
		m.SetAttr(AttrLatestUpdte, text(cell))
	case rowModelCard:
		m.SetAttr(AttrModelCard, first(hrefRe, cell))
	case rowVersions:
		for _, version := range versionRe.FindAllStringSubmatch(cell, -1) {
			m.AddList(ListSnapshots, text(version[2]))
		}
	case rowDataTypes, rowTokens, rowLimits:
		applyFields(m, cell)
	case rowCaps, rowConsuming:
		applyCapabilities(m, cell)
	}
}

// applyFields records the modalities and the token bounds, which Google states
// as named fields inside one cell.
func applyFields(m *catalog.Model, cell string) {
	for _, field := range sectionRe.FindAllStringSubmatch(cell, -1) {
		value := text(field[2])
		switch strings.ToLower(text(field[1])) {
		case fieldInputs, fieldInput:
			addModalities(m, ListInputModalities, value)
		case fieldOutput:
			addModalities(m, ListOutputModalities, value)
		case fieldInLimit, fieldContext, fieldTextInput:
			m.SetLimit(LimitContextWindow, parseCount(value))
		case fieldOutLimit:
			m.SetLimit(LimitMaxOutputTokens, parseCount(value))
		case fieldDimension:
			m.AddList(ListDimensions, dimensionsOf(value)...)
		}
	}
}

// dimensionsOf reads the widths an embedding model can return.
//
// Google states them as a range and then names the ones it recommends, as
// "Flexible, supports: 128 - 3072, Recommended: 768, 1536, 3072". Only the
// named widths are recorded: the range says what the model will accept and the
// list says what a reader can pick from, and a range is not a list.
func dimensionsOf(value string) []string {
	_, named, ok := strings.Cut(strings.ToLower(value), recommendedMarker)
	if !ok {
		named = value
	}
	var out []string
	for _, width := range widthRe.FindAllString(named, -1) {
		out = append(out, strings.ReplaceAll(width, ",", ""))
	}
	return out
}

// applyCapabilities records the capabilities a model has, and drops the ones
// it is listed as lacking. A capability qualified as being in preview is still
// one the model has.
func applyCapabilities(m *catalog.Model, cell string) {
	for _, field := range sectionRe.FindAllStringSubmatch(cell, -1) {
		state := strings.ToLower(text(field[2]))
		if strings.Contains(state, notSupported) ||
			!strings.Contains(state, supported) {
			continue
		}
		m.AddList(ListFeatures, featureName(text(field[1])))
	}
}

// addModalities records every modality named in one prose list, such as "Text,
// Image, Video, Audio, and PDF" or "Audio (translated speech) and Text".
func addModalities(m *catalog.Model, key, value string) {
	for _, part := range splitProse(value) {
		name, ok := modalityNames[bareNoun(part)]
		if !ok || name == "" {
			continue
		}
		m.AddList(key, name)
	}
}

// proseSeparators divide a list written for a reader. Google writes one four
// ways and mixes them in a single line: "Text and Image / PDF". The fourth is
// the "with" of "Video with audio", which is how the video models state that
// they return a soundtrack alongside the picture; it names a second modality
// and not a quality of the first.
var proseSeparators = strings.NewReplacer(
	",",
	"\x00",
	" and ",
	"\x00",
	" with ",
	"\x00",
	"/",
	"\x00",
)

// splitProse divides a list written for a reader into its items.
func splitProse(value string) []string {
	var out []string
	for item := range strings.SplitSeq(proseSeparators.Replace(value), "\x00") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// parenRe matches the aside Google puts after a modality to say what it is
// carrying, which names the content rather than the modality.
var parenRe = regexp.MustCompile(`\([^)]*\)`)

// bareNoun reduces one item of a prose list to the noun it names, dropping a
// trailing plural and any aside.
func bareNoun(value string) string {
	s := strings.ToLower(strings.TrimSpace(parenRe.ReplaceAllString(value, "")))
	s = strings.TrimSpace(s)
	if _, ok := modalityNames[s]; ok {
		return s
	}
	return strings.TrimSuffix(s, "s")
}

// featureName rewrites a capability into the catalog's vocabulary.
func featureName(name string) string {
	key := strings.ToLower(strings.TrimSpace(name))
	if mapped, ok := featureNames[key]; ok {
		return mapped
	}
	return strings.ReplaceAll(key, " ", "_")
}

// label reads a row's heading, which follows a pictogram whose own text is not
// part of it.
func label(cell string) string {
	s := iconRe.ReplaceAllString(cell, "")
	return strings.ToLower(text(pageSupRe.ReplaceAllString(s, "")))
}

// parseCount reads a grouped quantity such as "1,048,576".
func parseCount(value string) int64 {
	match := countRe.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil {
		return 0
	}
	n, err := strconv.ParseInt(strings.ReplaceAll(match[1], ",", ""), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// first returns the first capture of re, or the empty string.
func first(re *regexp.Regexp, body string) string {
	if match := re.FindStringSubmatch(body); match != nil {
		return match[1]
	}
	return ""
}
