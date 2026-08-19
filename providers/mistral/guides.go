package mistral

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Patterns over the three capability guides.
var (
	// ocrLimitsRe matches the answer the OCR guide's FAQ gives to the question
	// of what the API will accept, which is the only place either bound is
	// stated.
	ocrLimitsRe = regexp.MustCompile(
		`Uploaded document files must be ([\d,]+) MB or less and contain ` +
			`([\d,]+) pages or fewer`,
	)
	// ocrFormatRe matches one row of the table of formats the OCR processor
	// reads. Mistral names the format and then lists its extensions.
	ocrFormatRe = regexp.MustCompile(
		`"font-semibold text-foreground","children":"[A-Za-z0-9/ ]+"\}\]," ` +
			`\(([^"]+)\)"`,
	)
	// languageCellRe matches the cell of a supported-language table holding
	// the languages, which is the one of the two the region does not wrap in
	// an emphasis.
	languageCellRe = regexp.MustCompile(
		`"px-4 py-4 align-middle[^"]*","children":"([A-Z][^"]*)"`,
	)
	// ocrHeadingRe matches the heading dividing the two language tables.
	ocrHeadingRe = regexp.MustCompile(`"sectionId":"ocr","children":"OCR"`)
	// reasoningModelRe matches one model the reasoning guide names as taking a
	// reasoning effort.
	reasoningModelRe = regexp.MustCompile(
		`"children":"([a-z0-9.-]+)"\}\],": Supports adjustable reasoning ` +
			`via the "`,
	)
)

// paramReasoningEffort is the request parameter the reasoning guide says asks
// a model for more or less thinking.
const paramReasoningEffort = "reasoning_effort"

// applyOCRGuide records what the OCR guide states about a document.
//
// The bounds and the list of formats are stated once, of the OCR processor
// rather than of a version of it, so they are recorded on every OCR model.
// Mistral versions the processor behind one endpoint and states no bound that
// differs between versions.
func (b *builder) applyOCRGuide(doc catalog.Document) {
	body := flight(doc.Body)
	match := ocrLimitsRe.FindStringSubmatch(body)
	formats := ocrFormats(body)
	if match == nil && len(formats) == 0 {
		return
	}
	for _, m := range b.byKind(KindOCR) {
		m.AddSource(doc.URL)
		if match != nil {
			m.SetLimit(LimitMaxFileSizeMB, parseCount(match[1]))
			m.SetLimit(LimitMaxDocumentPages, parseCount(match[2]))
		}
		m.AddList(ListFileFormats, formats...)
	}
}

// ocrFormats returns the file extensions the OCR guide lists, in the order it
// lists them.
func ocrFormats(body string) []string {
	var out []string
	for _, row := range ocrFormatRe.FindAllStringSubmatch(body, -1) {
		for _, ext := range strings.Split(row[1], ",") {
			out = append(out, strings.TrimPrefix(strings.TrimSpace(ext), "."))
		}
	}
	return sortedUnique(out)
}

// applyLanguages records the languages Mistral states strong performance in.
//
// The page carries two tables and says of each which family of models it
// describes: one for the language models and one for OCR. Neither names a
// model, so each table is recorded on the models of the kind its heading
// names, and nothing is recorded for the kinds no table covers. Mistral adds
// that a model can do well in languages the tables leave out, so a list here
// is what Mistral vouches for rather than the whole of what a model
// understands.
func (b *builder) applyLanguages(doc catalog.Document) {
	body := flight(doc.Body)
	split := len(body)
	if at := ocrHeadingRe.FindStringIndex(body); at != nil {
		split = at[0]
	}
	var models, ocr []string
	for _, cell := range languageCellRe.FindAllStringSubmatchIndex(body, -1) {
		names := splitLanguages(body[cell[2]:cell[3]])
		if cell[0] < split {
			models = append(models, names...)
			continue
		}
		ocr = append(ocr, names...)
	}
	b.addLanguages(doc, KindChat, models)
	b.addLanguages(doc, KindOCR, ocr)
}

// addLanguages records one table against every model of a kind.
func (b *builder) addLanguages(
	doc catalog.Document,
	kind catalog.Kind,
	names []string,
) {
	if len(names) == 0 {
		return
	}
	for _, m := range b.byKind(kind) {
		m.AddSource(doc.URL)
		m.AddList(ListLanguages, names...)
	}
}

// splitLanguages separates the languages a cell lists.
func splitLanguages(cell string) []string {
	parts := strings.Split(cell, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, strings.TrimSpace(part))
	}
	return out
}

// applyReasoning records the models the reasoning guide names.
//
// A model page marks a reasoning model by giving it a reasoning output, and
// that is where most of them are read from. The guide is read as well because
// it names two models whose pages do not: Mistral put the thinking behind a
// request parameter for them, so the model answers either way and the page
// states only the ordinary output.
func (b *builder) applyReasoning(doc catalog.Document) {
	body := flight(doc.Body)
	for _, match := range reasoningModelRe.FindAllStringSubmatch(body, -1) {
		m := b.byName(match[1])
		if m == nil {
			continue
		}
		m.AddSource(doc.URL)
		m.AddList(ListFeatures, catalog.CapabilityReasoning)
		m.AddList(ListParameters, paramReasoningEffort)
	}
}

// byKind returns the models of one kind, in identifier order.
func (b *builder) byKind(kind catalog.Kind) []*catalog.Model {
	out := make([]*catalog.Model, 0, len(b.order))
	for _, id := range b.sortedIDs() {
		if b.models[id].Kind == kind {
			out = append(out, b.models[id])
		}
	}
	return out
}
