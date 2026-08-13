package azure

import (
	"regexp"
	"strconv"
	"strings"
)

// Columns of the capability tables, named as Azure heads them. The other model
// collections are documented in tables of this shape rather than the shape the
// OpenAI family is documented in: one row per model, with everything the model
// holds and can do written as bullets in a single cell.
const (
	colModel        = "model"
	colCapabilities = "capabilities"
)

// Numeric and enumeration keys only the capability tables populate.
const (
	ListLanguages  = "languages"
	ListDimensions = "embedding_dimensions"
)

// collectionModels map the name a capability table heads a row with onto the
// identifier the meters call the same model by.
//
// The two documents name a model differently and neither reduces to the other:
// a meter's name has already had the vendor stripped off it by the time it is
// an identifier, so "DeepSeek-V4-Pro" is metered as v4-pro and
// "FLUX.2-flex" as flex. Matching them by shape would join Grok 4 to Grok 4.20
// and FLUX.2-flex to a service tier, so the join is stated rather than
// inferred. A model Azure documents and does not name here keeps no
// capabilities, which is the same position it was in before.
//
// Several names may share an identifier, because Azure documents a reasoning
// and a non-reasoning variant of one metered model, and they agree on
// everything this reads.
var collectionModels = map[string][]string{
	"cohere-command-a":                       {"command-a"},
	"cohere-command-a-plus-05-2026":          {"command-a-plus"},
	"cohere-rerank-v4.0-fast":                {"rerank-v4-fast"},
	"cohere-rerank-v4.0-pro":                 {"rerank-v4-pro"},
	"embed-v-4-0":                            {"embed-v4"},
	"deepseek-v3.2":                          {"v3.2"},
	"deepseek-v3.2-speciale":                 {"v3.2-sp"},
	"deepseek-v4-flash":                      {"v4-flash"},
	"deepseek-v4-pro":                        {"v4-pro"},
	"flux-1.1-pro":                           {"flux-1.1-pro"},
	"flux.1-kontext-pro":                     {"kontext-pro"},
	"flux.2-flex":                            {"flex"},
	"flux.2-pro":                             {"flux-2-pro"},
	"grok-4":                                 {"grok-4"},
	"grok-4-20-non-reasoning":                {"grok-4.2"},
	"grok-4-20-reasoning":                    {"grok-4.2"},
	"grok-4.1-fast-non-reasoning":            {"grok-4.1"},
	"grok-4.1-fast-reasoning":                {"grok-4.1"},
	"grok-4.3":                               {"grok-4.3"},
	"grok-code-fast-1":                       {"grok-code-fast-1"},
	"kimi-k2.5":                              {"kimi-k2.5"},
	"kimi-k2.6":                              {"kimi-k2.6"},
	"kimi-k2.7-code":                         {"kimi-k2.7-code"},
	"llama-3.3-70b-instruct":                 {"llama-3.3-70b", "llama3.3"},
	"llama-4-maverick-17b-128e-instruct-fp8": {"llama-4-maverick-17b"},
	"mistral-document-ai-2512":               {"doc-ai-2512"},
	"mistral-large-3":                        {"large-3"},
	"mistral-medium-3-5":                     {"mm3.5"},
	"mistral-ocr-4-0":                        {"ocr-4"},
}

// bulletModalities map a word a capability bullet names a modality with onto
// the catalog's vocabulary.
//
// An embedding model writes "Output: Vector", which is its return value rather
// than a modality. The catalog has no word for a vector, so it is read as the
// text the model works in; leaving it out left embed-v4 stating what it takes
// and nothing about what it gives back, which a consumer cannot tell from a
// model that returns nothing. The widths in the same bullet are read separately.
var bulletModalities = map[string]string{
	"text":   "text",
	"image":  "image",
	"images": "image",
	"audio":  "audio",
	"video":  "video",
	"pdf":    "file",
	"pdfs":   "file",
	"vector": "text",
}

// Labels the capability bullets are written under.
const (
	bulletInput     = "input"
	bulletOutput    = "output"
	bulletContext   = "context"
	bulletTools     = "tool calling"
	bulletFormats   = "response formats"
	bulletLanguages = "languages"
)

var (
	// annotationRe matches what Azure appends to a documented name that is not
	// part of it: a preview marker and the footnote numbers it carries. Both
	// are written as words after the name, which is what separates the
	// footnote on "model-router 1" from the version in "grok-code-fast-1".
	annotationRe = regexp.MustCompile(
		`(?i)(?:\s*\(preview\)|\s+preview|\s+\d+)+\s*$`,
	)
	// tokenCountRe matches the bound a bullet states, which is a count of
	// tokens and is the only number in a bullet said to be one: the others
	// count pages, megabytes or reference images.
	tokenCountRe = regexp.MustCompile(`(?i)([\d][\d,.]*)\s*([km])?\s*tokens`)
	// vectorRe matches the widths an embedding model can return, which is the
	// one thing an output bullet states that is not a modality.
	vectorRe = regexp.MustCompile(`(?i)vector\s*\(([^)]*)\)`)
	// countListRe matches one count of a list of them, where a comma inside a
	// number groups its digits and a comma followed by a space separates two.
	countListRe = regexp.MustCompile(`\d[\d,]*\d|\d`)
	// wordRe matches one word of a bullet's value.
	wordRe = regexp.MustCompile(`[a-zA-Z]+`)
	// codeRe matches a language code, which Azure writes in code style.
	codeRe = regexp.MustCompile(`(?is)<code[^>]*>(.*?)</code>`)
)

// readCollections reads the capability tables, keyed by the identifier the
// meters use, so that what they state lands on the same models the OpenAI
// tables' readings do.
func readCollections(body string) map[string]documented {
	out := map[string]documented{}
	for _, table := range docTableRe.FindAllStringSubmatch(body, -1) {
		rows := docRowRe.FindAllStringSubmatch(table[1], -1)
		if len(rows) < 2 {
			continue
		}
		at := collectionColumns(rows[0][1])
		if at[colModel] < 0 || at[colCapabilities] < 0 {
			continue
		}
		for _, row := range rows[1:] {
			readCollectionRow(
				out,
				at,
				docCellRe.FindAllStringSubmatch(row[1], -1),
			)
		}
	}
	return out
}

// collectionColumns locates the two columns this reads. The model column is
// matched exactly, because the OpenAI tables head theirs "Model ID" and are
// read elsewhere.
func collectionColumns(row string) map[string]int {
	at := map[string]int{colModel: -1, colCapabilities: -1}
	for i, cell := range docCellRe.FindAllStringSubmatch(row, -1) {
		header := strings.ToLower(docText(cell[1]))
		for name := range at {
			if at[name] < 0 && header == name {
				at[name] = i
			}
		}
	}
	return at
}

// readCollectionRow records one documented model against every identifier it
// is metered under.
func readCollectionRow(
	out map[string]documented,
	at map[string]int,
	cells [][]string,
) {
	cell := cellText(cells, at[colModel])
	ids, ok := collectionModels[documentedName(cell)]
	if !ok {
		return
	}
	for _, id := range ids {
		d := out[id]
		d.Name = soleName(d.Name, displayName(cell))
		readCapabilities(&d, cellAt(cells, at[colCapabilities]))
		out[id] = d
	}
}

// nameAmbiguous marks a model that more than one documented row names
// differently, which is not a display name for either.
const nameAmbiguous = "\x00ambiguous"

// displayName is the name a capability table heads a row with, kept as the
// vendor writes it and with only the preview marker and footnote number Azure
// appends stripped off.
//
// This is the one display name Azure publishes. Its OpenAI tables head a Model
// ID column, which is the identifier, but a capability table names a model the
// way the lab that made it does, and the meters name the same model with the
// vendor already stripped off: DeepSeek-V4-Pro is metered as v4-pro. Taking the
// heading restores the part the SKU dropped.
func displayName(cell string) string {
	return strings.TrimSpace(annotationRe.ReplaceAllString(cell, ""))
}

// soleName keeps a name only while one documented row states it.
//
// Azure documents a reasoning and a non-reasoning variant of one metered model,
// so two rows reach grok-4.2 naming it grok-4-20-reasoning and
// grok-4-20-non-reasoning. Neither is the name of the model they are metered
// as, so a model named two ways keeps no name, the same rule the row-to-meter
// table already follows for capabilities.
func soleName(existing, found string) string {
	if existing == "" {
		return found
	}
	if existing == found {
		return existing
	}
	return nameAmbiguous
}

// documentedName reduces the name heading a row to what identifies the model,
// dropping the preview marker and footnote number Azure appends to it.
func documentedName(cell string) string {
	return annotationRe.ReplaceAllString(strings.ToLower(cell), "")
}

// readCapabilities reads the bullets of one capability cell. Everything a
// capability table states about a model is written here.
func readCapabilities(d *documented, cell string) {
	for _, part := range docBreakRe.Split(cell, -1) {
		label, value, ok := strings.Cut(docText(part), ":")
		if !ok {
			continue
		}
		label = strings.ToLower(strings.Trim(label, " -*"))
		switch {
		case label == bulletInput:
			d.InputMod = appendNew(d.InputMod, modalitiesOf(value)...)
			if d.Context == 0 {
				d.Context = tokenCount(value)
			}
		case label == bulletOutput:
			d.OutMod = appendNew(d.OutMod, modalitiesOf(value)...)
			d.Dimensions = appendNew(d.Dimensions, vectorWidths(value)...)
			if d.MaxOut == 0 {
				d.MaxOut = tokenCount(value)
			}
		case strings.HasPrefix(label, bulletContext):
			d.Context = tokenCount(value)
		case label == bulletTools && affirmative(value):
			d.Features = appendNew(d.Features, "function_calling")
		case label == bulletFormats && mentionsJSON(value):
			d.Features = appendNew(d.Features, "json_mode")
		case label == bulletLanguages:
			d.Languages = appendNew(d.Languages, languagesOf(part)...)
		}
	}
}

// modalitiesOf reads what a bullet names, which Azure writes as prose: "text
// and image", "image or PDF pages", "One Image".
func modalitiesOf(value string) []string {
	var out []string
	for _, word := range wordRe.FindAllString(strings.ToLower(value), -1) {
		if name, ok := bulletModalities[word]; ok {
			out = appendNew(out, name)
		}
	}
	return out
}

// tokenCount reads the bound a bullet states, which is the first count in it.
// A bullet that states a second is stating a limit on something else, as an
// image count or a page count.
func tokenCount(value string) int64 {
	match := tokenCountRe.FindStringSubmatch(value)
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

// vectorWidths reads the widths an embedding model can be asked for.
func vectorWidths(value string) []string {
	match := vectorRe.FindStringSubmatch(value)
	if match == nil {
		return nil
	}
	return splitCounts(match[1])
}

// splitCounts reads the counts a cell states. It states either one width, or
// the several a model can be asked for with a unit written after the last of
// them. A grouped number is one count and not two, which is why the separator
// is not the comma alone.
func splitCounts(value string) []string {
	var out []string
	for _, match := range countListRe.FindAllString(value, -1) {
		out = appendNew(out, strings.ReplaceAll(match, ",", ""))
	}
	return out
}

// languagesOf reads the codes a languages bullet names, which Azure writes in
// code style and joins with a final "and".
func languagesOf(part string) []string {
	var out []string
	for _, match := range codeRe.FindAllStringSubmatch(part, -1) {
		if code := strings.TrimSpace(docText(match[1])); code != "" {
			out = appendNew(out, code)
		}
	}
	return out
}

// affirmative reports whether a yes or no bullet says yes.
func affirmative(value string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "yes")
}

// mentionsJSON reports whether a response format bullet names JSON.
func mentionsJSON(value string) bool {
	return strings.Contains(strings.ToLower(value), "json")
}
