package assemblyai

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// The two pages telling a caller which model to ask for. They are the only
// documents stating the identifier an API takes, and the streaming one answers
// a set of capabilities per model in a table the models page states nowhere.
const (
	PrerecordedModelsURL = "https://www.assemblyai.com/docs/" +
		"pre-recorded-audio/select-the-speech-model.md"
	StreamingModelsURL = "https://www.assemblyai.com/docs/" +
		"streaming/select-the-speech-model.md"
)

// The two shapes of table these pages carry, told apart by what their first
// column is headed with: one model per row, or one capability per row and one
// model per column.
const (
	headModel      = "name"
	headCapability = "feature"
)

var (
	// identifierRe matches the identifier out of the parameter a row quotes,
	// which each page writes in the syntax of the language its tab is for.
	identifierRe = regexp.MustCompile(`['"]([a-z0-9][a-z0-9.-]*)['"]`)
	// pillRe matches the badge marking the recommended model, which sits
	// inside the cell holding that model's name.
	pillRe = regexp.MustCompile(
		`(?is)<span[^>]*recommended-pill[^>]*>.*?</span>`,
	)
)

// matrixCapabilities read the capability matrix, whose cells answer in a word
// rather than in a yes: what says a model follows a speaker changing language
// is the word "native code switching" in its column, and a model answering
// "per turn" is saying it does not.
var matrixCapabilities = []struct {
	row     string
	answer  string
	feature string
}{
	{"language detection", "yes", FeatureLanguageDetection},
	{"partial transcripts", "yes", FeaturePartialTranscripts},
	{"multilingual", "code switching", FeatureCodeSwitching},
	{"customization", "keyterms prompting", FeatureKeyterms},
	{"customization", "native prompting", FeaturePrompting},
}

// applySelection reads one of the model-selection pages onto the models the
// models page named.
func (b *builder) applySelection(doc catalog.Document) {
	for _, table := range pipeTables(string(doc.Body)) {
		if len(table) < 2 {
			continue
		}
		switch strings.ToLower(clean(cellAt(table[0], 0))) {
		case headModel:
			for _, row := range table[1:] {
				b.applyIdentifierRow(row, doc.URL)
			}
		case headCapability:
			b.applyMatrix(table, doc.URL)
		}
	}
}

// applyIdentifierRow records the identifier one row states for one model.
func (b *builder) applyIdentifierRow(row []string, source string) {
	m, ok := b.lookup(modelName(cellAt(row, 0)))
	if !ok {
		return
	}
	match := identifierRe.FindStringSubmatch(clean(cellAt(row, 1)))
	if match == nil {
		return
	}
	m.AddSource(source)
	m.SetAttr(AttrAPIIdentifier, match[1])
	m.SetAttr(catalog.APIID, match[1])
}

// applyMatrix records what the capability matrix answers for each model it has
// a column for.
func (b *builder) applyMatrix(table [][]string, source string) {
	for _, row := range table[1:] {
		label := strings.ToLower(clean(cellAt(row, 0)))
		for i := 1; i < len(table[0]); i++ {
			m, ok := b.lookup(modelName(cellAt(table[0], i)))
			if !ok {
				continue
			}
			m.AddSource(source)
			applyMatrixCell(m, label, clean(cellAt(row, i)))
		}
	}
}

// applyMatrixCell records the capabilities one cell of the matrix states.
func applyMatrixCell(m *catalog.Model, label, cell string) {
	answer := strings.ToLower(cell)
	for _, c := range matrixCapabilities {
		if c.row == label && strings.Contains(answer, c.answer) {
			m.AddList(ListFeatures, c.feature)
		}
	}
}

// modelName reads a model's name out of a cell, dropping the badge marking the
// recommended one so that the name is the name.
func modelName(cell string) string {
	return clean(pillRe.ReplaceAllString(cell, " "))
}

// pipeTables returns every markdown table in a document as its rows, a table
// being a run of adjacent pipe lines. The rows of one table have to stay
// together here, since the matrix is read by column and a column means nothing
// without the header row above it.
func pipeTables(body string) [][][]string {
	var (
		out   [][][]string
		table [][]string
	)
	flush := func() {
		if len(table) > 0 {
			out = append(out, table)
		}
		table = nil
	}
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "|") {
			flush()
			continue
		}
		cells := splitRow(line)
		if isSeparator(cells) {
			continue
		}
		table = append(table, cells)
	}
	flush()
	return out
}
