package groq

import (
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Guides stating what the speech models take and return, which the models
// table has no column for and the model pages state only in part.
const (
	SpeechToTextURL = baseURL + "/docs/speech-to-text.md"
	OrpheusURL      = baseURL + "/docs/text-to-speech/orpheus.md"
)

// Column headings the audio guides write.
const (
	colParameter     = "parameter"
	colModel         = "model"
	colEndpoint      = "endpoint"
	colLanguages     = "supported language(s)"
	colLanguage      = "language"
	colTranscription = "transcription support"
	colTranslation   = "translation support"
	colSpeedFactor   = "real-time speed factor"
	colWordError     = "word error rate"
	colVoiceID       = "voice id"
)

// Labels the speech-to-text guide states its file bounds under, written as a
// label on one line and the value on the next.
const (
	labelFileSize   = "Max File Size"
	labelFileLength = "Minimum File Length"
	labelBilled     = "Minimum Billed Length"
	labelFileTypes  = "Supported File Types"
)

// endpointNames map the endpoint an audio guide names onto a word for it.
var endpointNames = map[string]string{
	"transcriptions": "transcriptions",
	"translations":   "translations",
	"speech":         "speech",
}

// applySpeechToText reads the speech-to-text guide.
//
// The guide states per model what the models table has no column for: which
// languages it handles, whether it transcribes, whether it translates, how
// much faster than real time it runs and how often it is wrong. It states for
// both models what file it accepts, how small and how large, from what length
// it is billed and what parameters the request takes.
//
// The word-level timestamps the guide documents are recorded as a capability,
// because the guide states that the transcription can be timed per word and
// not merely that a parameter exists.
func (b *builder) applySpeechToText(doc catalog.Document) {
	body := string(doc.Body)
	shared := sharedAudioFacts(body)
	shared.endpoints = guideEndpoints(body, doc.URL)
	shared.parameters = guideParameters(body, doc.URL)
	for _, t := range scanTables(body, doc.URL) {
		b.applyWhisperTable(t, doc.URL, shared)
	}
}

// audioFacts are the things a guide states once for every model it covers.
type audioFacts struct {
	fileSizes  string
	fileLength string
	billed     int64
	formats    []string
	endpoints  []string
	parameters []string
}

// applyWhisperTable reads one of the guide's per-model tables onto the models
// it names, whichever of the two it is: one states the languages and one the
// speed and the error rate, and both key their rows by the model.
func (b *builder) applyWhisperTable(
	t table,
	source string,
	shared audioFacts,
) {
	idCol := columnOf(t.Headers, colModelID)
	if idCol < 0 {
		idCol = columnOf(t.Headers, colModel)
	}
	if idCol < 0 || columnOf(t.Headers, colParameter) >= 0 {
		return
	}
	for _, row := range t.Rows {
		m, ok := b.models[clean(cellAt(row, idCol))]
		if !ok {
			continue
		}
		m.AddSource(source)
		m.SetAttr(
			AttrLanguages,
			cellValue(t, row, colLanguages, colLanguage),
		)
		m.SetAttr(AttrSpeedFactor, cellValue(t, row, colSpeedFactor))
		m.SetAttr(AttrWordErrorRate, cellValue(t, row, colWordError))
		m.SetAttr(AttrFileSizeTiers, shared.fileSizes)
		m.SetAttr(AttrMinFileLength, shared.fileLength)
		m.SetLimit(LimitMinBilledSeconds, shared.billed)
		m.AddList(ListAudioFormats, shared.formats...)
		m.AddList(ListParameters, shared.parameters...)
		addSupported(m, cellValue(t, row, colTranscription), "transcriptions")
		addSupported(m, cellValue(t, row, colTranslation), "translations")
		if len(shared.endpoints) > 0 && m.Lists[ListEndpoints] == nil {
			m.AddList(ListEndpoints, shared.endpoints...)
		}
		if hasParameter(shared.parameters, "timestamp_granularities") {
			m.AddList(ListFeatures, catalog.CapabilityWordTimestamps)
		}
	}
}

// addSupported records an endpoint the table says a model supports.
func addSupported(m *catalog.Model, answer, endpoint string) {
	if strings.EqualFold(strings.TrimSpace(answer), "yes") {
		m.AddList(ListEndpoints, endpoint)
	}
}

// hasParameter reports whether the guide's parameter table names one, ignoring
// the brackets Groq writes after a parameter taking a list.
func hasParameter(parameters []string, name string) bool {
	for _, p := range parameters {
		if strings.TrimSuffix(p, "[]") == name {
			return true
		}
	}
	return false
}

// cellValue returns a row's value under the first of the given headings the
// table carries.
func cellValue(t table, row []string, headings ...string) string {
	for _, heading := range headings {
		if col := columnOf(t.Headers, heading); col >= 0 {
			return valueOrEmpty(cellAt(row, col))
		}
	}
	return ""
}

// sharedAudioFacts reads the bounds the speech-to-text guide states once for
// both models, which it writes as a label and the value on the line below.
func sharedAudioFacts(body string) audioFacts {
	var (
		out   audioFacts
		label string
	)
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "|") {
			continue
		}
		switch label {
		case labelFileSize:
			out.fileSizes = clean(line)
		case labelFileLength:
			out.fileLength = clean(line)
		case labelBilled:
			out.billed = parseCount(line)
		case labelFileTypes:
			out.formats = lowerAll(backticked(line))
		}
		label = ""
		if line == labelFileSize ||
			line == labelFileLength ||
			line == labelBilled ||
			line == labelFileTypes {
			label = line
		}
	}
	return out
}

// guideEndpoints reads the endpoints a guide's endpoint table names.
func guideEndpoints(body, source string) []string {
	var out []string
	for _, t := range scanTables(body, source) {
		col := columnOf(t.Headers, colEndpoint)
		if col < 0 {
			continue
		}
		for _, row := range t.Rows {
			name := strings.ToLower(clean(cellAt(row, col)))
			if mapped, ok := endpointNames[name]; ok {
				out = append(out, mapped)
			}
		}
	}
	return out
}

// guideParameters reads the request parameters a guide's parameter table
// names, which is what the API accepts rather than what the model can do.
func guideParameters(body, source string) []string {
	var out []string
	for _, t := range scanTables(body, source) {
		col := columnOf(t.Headers, colParameter)
		if col < 0 {
			continue
		}
		for _, row := range t.Rows {
			if name := clean(cellAt(row, col)); name != "" {
				out = append(out, name)
			}
		}
	}
	return out
}

// lowerAll lower-cases every value.
func lowerAll(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, strings.ToLower(v))
	}
	return out
}

// applyOrpheus reads the text-to-speech guide.
//
// The guide states what the models table cannot: which language each Orpheus
// model speaks, which voices it offers, how long an input may be, what format
// the audio comes back in and what the request takes. The voices are listed
// per language rather than per model, so they are matched to the model the
// guide's own table gives that language to.
func (b *builder) applyOrpheus(doc catalog.Document) {
	body := string(doc.Body)
	tables := scanTables(body, doc.URL)
	byLanguage := map[string]*catalog.Model{}
	parameters := guideParameters(body, doc.URL)
	endpoints := guideEndpoints(body, doc.URL)
	for _, t := range tables {
		idCol := columnOf(t.Headers, colModelID)
		langCol := columnOf(t.Headers, colLanguage)
		if idCol < 0 || langCol < 0 {
			continue
		}
		for _, row := range t.Rows {
			m, ok := b.models[clean(cellAt(row, idCol))]
			if !ok {
				continue
			}
			language := clean(cellAt(row, langCol))
			m.AddSource(doc.URL)
			m.SetAttr(AttrLanguage, language)
			m.AddList(ListParameters, parameters...)
			m.AddList(ListEndpoints, endpoints...)
			m.SetLimit(LimitMaxInputCharacters, inputCharacters(parameters, t))
			byLanguage[languageKey(language)] = m
		}
	}
	addVoices(tables, byLanguage)
}

// addVoices records the voices under each heading against the model that
// speaks that heading's language.
func addVoices(tables []table, byLanguage map[string]*catalog.Model) {
	for _, t := range tables {
		col := columnOf(t.Headers, colVoiceID)
		if col < 0 {
			continue
		}
		m := voicesFor(t.Section, byLanguage)
		if m == nil {
			continue
		}
		for _, row := range t.Rows {
			m.AddList(ListVoices, clean(cellAt(row, col)))
		}
	}
}

// voicesFor returns the model a voices heading belongs to, which is the one
// whose language the heading names.
func voicesFor(
	section string,
	byLanguage map[string]*catalog.Model,
) *catalog.Model {
	for language, m := range byLanguage {
		if strings.Contains(section, language) {
			return m
		}
	}
	return nil
}

// languageKey reduces a language as the table writes it to the word a heading
// names it by, which is the first of the two where the table qualifies the
// language with its dialect.
func languageKey(language string) string {
	fields := strings.Fields(strings.ToLower(language))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// inputCharacters reads the ceiling the guide states on the text a request may
// carry, which it writes in the description of the parameter carrying it
// rather than as a limit of its own.
func inputCharacters(parameters []string, t table) int64 {
	col := columnOf(t.Headers, colParameter)
	descCol := columnOf(t.Headers, "description")
	if col < 0 || descCol < 0 {
		return 0
	}
	for _, row := range t.Rows {
		if clean(cellAt(row, col)) != "input" {
			continue
		}
		_, after, ok := strings.Cut(clean(cellAt(row, descCol)), "max ")
		if !ok {
			continue
		}
		return parseCount(after)
	}
	return 0
}
