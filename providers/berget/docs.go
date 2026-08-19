package berget

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// LimitContextWindow is the only numeric bound Berget publishes beyond a
// model's size, and it is on the documentation site rather than the endpoint.
const LimitContextWindow = "context_window"

// Scalar keys the documentation populates.
const (
	// AttrDocsTitle is the heading Berget writes above a model's card. It is
	// kept because the heading is where a card states a lifecycle the endpoint
	// does not, writing "GLM 5.2 (maintenance)" over a model the endpoint
	// still reports as stable.
	AttrDocsTitle = "docs_title"
	// AttrModelCard is the page the card links to, which for every model
	// Berget cards is the weights on Hugging Face.
	AttrModelCard = "model_card_url"
	// AttrHuggingFaceID is that link with the host removed.
	AttrHuggingFaceID = "hugging_face_id"
	// AttrOpenWeights records that Berget serves open models, which it states
	// of its catalogue as a whole rather than of any one model.
	AttrOpenWeights = "open_weights"
	// AttrDataResidency and AttrTrainsOnData record the two guarantees Berget
	// attaches to every model it serves.
	AttrDataResidency = "data_residency"
	AttrTrainsOnData  = "trains_on_customer_data"
	// AttrReasoningMandatory marks a model Berget says cannot be asked to stop
	// reasoning.
	AttrReasoningMandatory = "reasoning_mandatory"
)

// Enumeration keys the documentation and the endpoint's capabilities populate.
const (
	ListInputModalities  = "input_modalities"
	ListOutputModalities = "output_modalities"
	ListLanguages        = "languages"
)

// residency is what Berget states of every model it serves, on
// https://docs.berget.ai/what-is-berget: that all inference, storage and
// processing happens on European infrastructure, that data never crosses
// borders, and that it is never used to train models. The statement is made of
// the platform rather than of a model, and it is recorded on every model
// because there is no model it excludes.
var residency = map[string]string{
	AttrOpenWeights:   "true",
	AttrDataResidency: "EU",
	AttrTrainsOnData:  "false",
}

// FeatureVision is the capability Berget marks a model that reads images by.
// It names a modality rather than an API feature, so it is recorded as an
// input modality instead of joining the capability list, where it would be the
// one provider spelling image input that way.
const FeatureVision = "vision"

// typeModalities map Berget's model type onto what a model of that type takes
// and returns. The endpoint states the type and nothing else about modality,
// so a model reading images is found by its vision capability instead.
//
// An embedding and a rerank model work in text on both sides, which is the
// medium and not the return value: one answers with a vector and the other with
// a set of scores, and the catalog has a word for neither. Recording the input
// alone would leave a consumer unable to tell an unstated output from a model
// that returns nothing.
var typeModalities = map[string]struct{ in, out []string }{
	"text":           {[]string{"text"}, []string{"text"}},
	"embedding":      {[]string{"text"}, []string{"text"}},
	"rerank":         {[]string{"text"}, []string{"text"}},
	"speech-to-text": {[]string{"audio"}, []string{"text"}},
}

// docsReasoning names the model the documentation states reasons, in prose
// rather than in any field: the guide at
// https://docs.berget.ai/models/choosing-a-model sends analytical reasoning to
// "GLM 4.7 with reasoning mode for lower cost", which is Berget naming a
// reasoning mode on one of its models. No page names one on any other.
var docsReasoning = []string{"zai-org/GLM-4.7-FP8"}

// docsLanguages names the model the documentation ties to a language. Berget
// writes on https://docs.berget.ai/models/model-selection-philosophy that
// KB-Whisper is a Swedish speech recognition model from the National Library
// of Sweden. The overview says its three speech models cover Swedish and
// Norwegian alongside a multilingual option without saying which is which, so
// only the model named outright is recorded.
var docsLanguages = map[string][]string{
	"KBLab/kb-whisper-large": {"sv"},
}

// cardRe matches one model card in the documentation's markdown mirror. A card
// names the model's weights in its link and, for every kind but speech,
// carries the model's context window on its first line.
var cardRe = regexp.MustCompile(
	`(?s)<Card title="([^"]*)" href="([^"]*)">([^<]*)</Card>`,
)

// cardModelRe reads a card's body for the identifier the API answers to and,
// where the card states one, the context window beside it.
var cardModelRe = regexp.MustCompile(
	"(?i)`([^`]+)`(?:\\s*·\\s*([\\d,]+)\\s*(k?)\\s*tokens)?",
)

// huggingFacePrefix is the host every model card links into.
const huggingFacePrefix = "https://huggingface.co/"

// applyDocs reads the documentation site's markdown mirror.
//
// The endpoint publishes no bound on a request's length. The overview does, on
// a card per model keyed by the same identifier the endpoint uses, which is
// why the two documents need no reconciling. The same card links the model to
// its weights, and the pages around it state in prose the few things no field
// on the endpoint carries.
func (b *builder) applyDocs(doc catalog.Document) {
	body := string(doc.Body)
	for _, card := range cardRe.FindAllStringSubmatch(body, -1) {
		b.applyCard(card[1], card[2], card[3], doc.URL)
	}
	b.applyMatrix(body)
	for _, id := range docsReasoning {
		if m, ok := b.models[id]; ok {
			m.AddSource(doc.URL)
			m.AddList(ListFeatures, catalog.CapabilityReasoning)
		}
	}
	for id, languages := range docsLanguages {
		if m, ok := b.models[id]; ok {
			m.AddSource(doc.URL)
			m.AddList(ListLanguages, languages...)
		}
	}
	for _, m := range b.models {
		m.AddSource(doc.URL)
		for _, key := range []string{
			AttrOpenWeights,
			AttrDataResidency,
			AttrTrainsOnData,
		} {
			m.SetAttr(key, residency[key])
		}
	}
}

// matrixHeading is the first heading of the capability matrix, which is how
// its table is told apart from the two other tables on the same page.
const matrixHeading = "Model"

// matrixColumn is the heading of the one column in the capability matrix that
// says something the endpoint's flags do not agree with.
const matrixColumn = "Multimodal"

// matrixMark is what the matrix writes in a cell for a capability a model has.
const matrixMark = "✓"

// applyMatrix records where the capability matrix at
// https://docs.berget.ai/models/capabilities contradicts the listing.
//
// For every column but one the matrix restates the endpoint's own capability
// flags, and for those it agrees. Its multimodal column does not: it marks
// both Mistral Small 3.2 and GPT-OSS 120B as taking images where the listing
// reports the vision flag false on both. The modality lists follow the
// listing, because a flag the API answers with is the narrower claim and
// because trusting the matrix would assert image input for a model the
// endpoint says has none. The disagreement is recorded as a note so that a
// reader is not left to discover it by opening both documents.
func (b *builder) applyMatrix(body string) {
	column := -1
	for _, line := range strings.Split(body, "\n") {
		cells := tableCells(line)
		if cells == nil {
			continue
		}
		if len(cells) > 0 && cells[0] == matrixHeading {
			column = slices.Index(cells, matrixColumn)
			continue
		}
		if column >= 0 {
			b.applyMatrixRow(cells, column)
		}
	}
}

// applyMatrixRow records one row of the matrix against the model it names.
func (b *builder) applyMatrixRow(cells []string, column int) {
	if column >= len(cells) || cells[column] != matrixMark {
		return
	}
	m, ok := b.models[strings.Trim(cells[0], "`")]
	if !ok || slices.Contains(m.Lists[ListInputModalities], "image") {
		return
	}
	m.AddNote(
		"the capability matrix marks this model multimodal, " +
			"which the listing's vision flag denies",
	)
}

// tableCells splits one markdown table row into its cells, returning nil for a
// line that is not one. A rule row, whose cells are all dashes, is not one
// either: it separates a heading from its body and names nothing.
func tableCells(line string) []string {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
		return nil
	}
	fields := strings.Split(strings.Trim(line, "|"), "|")
	cells := make([]string, 0, len(fields))
	rule := true
	for _, field := range fields {
		cell := strings.TrimSpace(field)
		if strings.Trim(cell, "-: ") != "" {
			rule = false
		}
		cells = append(cells, cell)
	}
	if rule {
		return nil
	}
	return cells
}

// applyCard records one card onto the model it names.
func (b *builder) applyCard(title, href, body, source string) {
	fields := cardModelRe.FindStringSubmatch(body)
	if fields == nil {
		return
	}
	m, ok := b.models[strings.TrimSpace(fields[1])]
	if !ok {
		return
	}
	m.AddSource(source)
	m.SetAttr(AttrDocsTitle, strings.TrimSpace(title))
	if window := parseCount(fields[2], fields[3]); window > 0 {
		m.SetLimit(LimitContextWindow, window)
	}
	if !strings.HasPrefix(href, huggingFacePrefix) {
		return
	}
	m.SetAttr(AttrModelCard, href)
	m.SetAttr(AttrHuggingFaceID, strings.TrimPrefix(href, huggingFacePrefix))
}

// parseCount reads a quantity written either in full, as "8192", or with a
// thousands suffix, as "256k".
func parseCount(digits, suffix string) int64 {
	if digits == "" {
		return 0
	}
	n, err := strconv.ParseInt(strings.ReplaceAll(digits, ",", ""), 10, 64)
	if err != nil {
		return 0
	}
	if strings.EqualFold(suffix, "k") {
		n *= 1_000
	}
	return n
}

// applyModalities records what a model takes and returns, read from its type
// and from whether Berget marks it as reading images.
func applyModalities(m *catalog.Model, e entry) {
	flow, ok := typeModalities[e.ModelType]
	if !ok {
		return
	}
	m.AddList(ListInputModalities, flow.in...)
	m.AddList(ListOutputModalities, flow.out...)
	if e.Capabilities[FeatureVision] {
		m.AddList(ListInputModalities, "image")
	}
}
