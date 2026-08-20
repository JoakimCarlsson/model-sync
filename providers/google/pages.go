package google

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Scalar keys the model pages populate.
const (
	AttrSummary   = "summary"
	AttrModelCode = "model_code"
	// AttrLastUpdated holds what a model page heads "Latest update", which is
	// the month Google last shipped a change to the model.
	AttrLastUpdated = "last_updated"
	AttrModelCard   = "model_card_url"
)

// Numeric keys the model pages populate.
const (
	LimitContextWindow   = "context_window"
	LimitMaxOutputTokens = "max_output_tokens"
	// LimitMaxAudioOutputTokens is the ceiling on what a speech model
	// generates, which Google heads "Output token limit" in the same table it
	// heads the input bound with, though the two count different things. A
	// speech model's input is text and its output is audio, and an audio token
	// is a slice of sound rather than a word: the 16,384 the tts models state
	// is not a length a text response could reach, and it stands above the
	// 8,192 token window that carries the prompt. Recording it as
	// max_output_tokens made every one of them publish a ceiling no request
	// could ask for.
	LimitMaxAudioOutputTokens = "max_audio_output_tokens"
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
	// fieldOutImages and fieldOutVideo are what the image and video pages
	// state in place of an output token limit, being models that answer in
	// neither tokens nor a vector. Both are written as a count and one of them
	// as a range, "1 to 4" images, whose upper end is the bound.
	fieldOutImages = "output images"
	fieldOutVideo  = "output video"
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
	applyTable(m, table[1])
}

// pageCodes returns the endpoints a model page states it describes. A page
// standing for a family lists every one of them, which is how the sizes of
// Imagen and the fast build of Veo 3.1 reach a page of their own.
func pageCodes(doc catalog.Document) []string {
	table := tableRe.FindStringSubmatch(string(doc.Body))
	if table == nil {
		return nil
	}
	for _, row := range pageRowRe.FindAllStringSubmatch(table[1], -1) {
		cells := pageCellRe.FindAllStringSubmatch(row[1], -1)
		if len(cells) < 2 || label(cells[0][1]) != rowModelCode {
			continue
		}
		return codesIn(cells[1][1])
	}
	return nil
}

// modelCode is the identifier a page states for the model it is read onto,
// which is nothing where the page names another model instead. A page standing
// for a family lists every endpoint in it, so the model's own is there to find;
// where it is not, Google has copied a sibling's row onto the page, as the
// streaming robotics page and the Lyria Pro page both do, and recording it
// would give the model its sibling's identifier.
func modelCode(id string, codes []string) string {
	if slices.Contains(codes, id) {
		return id
	}
	return ""
}

// applyProperty records one row of the property table.
func applyProperty(m *catalog.Model, name, cell string) {
	switch name {
	case rowModelCode:
		code := modelCode(m.ID, codesIn(cell))
		m.SetAttr(AttrModelCode, code)
		m.AddList(ListAliases, code)
	case rowUpdated:
		m.SetAttr(AttrLastUpdated, text(cell))
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
		case fieldOutImages:
			m.SetLimit(LimitMaxOutputImages, largestCount(value))
		case fieldOutVideo:
			m.SetLimit(LimitMaxOutputVideos, largestCount(value))
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
		name := featureName(text(field[1]))
		m.AddList(ListFeatures, name)
		if name == catalog.CapabilityReasoning {
			addThinkingLevels(m, strings.Trim(parenRe.FindString(state), "()"))
		}
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

// applyGuideTables reads every property table a capability guide carries.
//
// A guide devoted to one family repeats the model page of each member, and
// carries one for the members Google publishes no model page for at all: both
// builds of Veo 3 are priced, have no page of their own and are described only
// here. A table names the endpoints it covers in its own model code row, so it
// is attached by what it says rather than by where it sits, and a table
// describing a model the pricing page does not price adds nothing.
func (b *builder) applyGuideTables(doc catalog.Document) {
	for _, table := range tableRe.FindAllStringSubmatch(string(doc.Body), -1) {
		for _, id := range codesIn(tableCodes(table[1])) {
			m, ok := b.models[id]
			if !ok {
				continue
			}
			m.AddSource(doc.URL)
			applyTable(m, table[1])
		}
	}
}

// tableCodes returns the markup of a property table's model code row, which is
// where a table says which endpoints it describes.
func tableCodes(table string) string {
	for _, row := range pageRowRe.FindAllStringSubmatch(table, -1) {
		cells := pageCellRe.FindAllStringSubmatch(row[1], -1)
		if len(cells) < 2 || label(cells[0][1]) != rowModelCode {
			continue
		}
		return cells[1][1]
	}
	return ""
}

// applyTable records every row of one property table onto one model.
func applyTable(m *catalog.Model, table string) {
	for _, row := range pageRowRe.FindAllStringSubmatch(table, -1) {
		cells := pageCellRe.FindAllStringSubmatch(row[1], -1)
		if len(cells) < 2 {
			continue
		}
		applyProperty(m, label(cells[0][1]), cells[1][1])
	}
}
