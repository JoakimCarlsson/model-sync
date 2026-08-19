package deepgram

import (
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Keys the voice catalogs populate. Deepgram sells one text-to-speech model
// per generation of voice and then publishes the voices themselves, each with
// a model string of its own, so what the catalog says about a voice is
// recorded against the model that speaks it.
const (
	// ListVoices holds the model string of every voice a generation offers,
	// which is what a request names to be answered in that voice.
	ListVoices = "voices"
	// ListAccents holds the accents those voices speak in.
	ListAccents = "accents"
	// ListVoiceGenders holds the genders the catalog expresses them as.
	ListVoiceGenders = "voice_genders"
	// ListVoiceAges holds the ages it gives them.
	ListVoiceAges = "voice_ages"
)

// voiceColumns are the headings of the catalog's columns, which differ between
// the Aura catalog and the Flux one: the first calls a voice's gender its
// expressed gender and gives each voice a language, and the second calls the
// same column gender and states the language once for the whole catalog.
var voiceColumns = map[string]string{
	"model":            ListVoices,
	"accent":           ListAccents,
	"expressed gender": ListVoiceGenders,
	"gender":           ListVoiceGenders,
	"age":              ListVoiceAges,
	"language":         ListLanguages,
}

// voiceHeadings say which model a section of the Aura catalog describes.
// Deepgram divides that catalog by generation and language, writing the
// generation into every heading, and gives the first generation the name the
// pricing page also sells it under.
var voiceHeadings = map[string]string{
	"aura-2": "aura-2",
	"aura 1": "aura-1",
	"aura-1": "aura-1",
}

// fluxVoiceModel is the model the Flux voice catalog describes. That catalog
// is one page for one model and says so in its title rather than in a heading
// per section.
const fluxVoiceModel = "flux-tts"

// applyVoices reads a voice catalog onto the model that speaks its voices.
func (b *builder) applyVoices(doc catalog.Document) {
	if doc.URL == FluxVoicesURL {
		b.applyVoiceTables(string(doc.Body), fluxVoiceModel, doc.URL)
		return
	}
	for _, s := range mdSections(string(doc.Body)) {
		id := voiceModel(s.heading)
		if id == "" {
			continue
		}
		b.applyVoiceTables(s.body, id, doc.URL)
	}
}

// voiceModel returns the model a heading of the Aura catalog describes.
func voiceModel(heading string) string {
	lower := strings.ToLower(text(heading))
	for prefix, id := range voiceHeadings {
		if strings.HasPrefix(lower, prefix) {
			return id
		}
	}
	return ""
}

// applyVoiceTables records every voice a stretch of the catalog lists.
func (b *builder) applyVoiceTables(body, id, source string) {
	m, ok := b.models[id]
	if !ok {
		return
	}
	var columns []string
	for _, match := range tableRowRe.FindAllStringSubmatch(body, -1) {
		cells := strings.Split(match[1], "|")
		if separator(cells[0]) {
			continue
		}
		if header, isHeader := voiceHeader(cells); isHeader {
			columns = header
			continue
		}
		b.applyVoiceRow(m, cells, columns, source)
	}
}

// voiceHeader reports whether a row names the catalog's columns, and which of
// them say something about the voice rather than how it sounds in a sample.
func voiceHeader(cells []string) ([]string, bool) {
	out := make([]string, 0, len(cells))
	named := false
	for _, c := range cells {
		key := voiceColumns[strings.ToLower(plain(c))]
		if key != "" {
			named = true
		}
		out = append(out, key)
	}
	if !named {
		return nil, false
	}
	return out, true
}

// applyVoiceRow records one voice.
func (b *builder) applyVoiceRow(
	m *catalog.Model,
	cells, columns []string,
	source string,
) {
	if len(columns) == 0 {
		return
	}
	recorded := false
	for i, c := range cells {
		if i >= len(columns) || columns[i] == "" {
			continue
		}
		value := voiceValue(columns[i], c)
		if value == "" {
			continue
		}
		m.AddList(columns[i], value)
		recorded = true
	}
	if recorded {
		m.AddSource(source)
	}
}

// voiceValue reads one cell of the catalog. A voice's model string and the
// language it speaks are recorded as written, and what it sounds like is
// lowered because the two catalogs capitalise the same answer differently.
func voiceValue(column, cell string) string {
	if column == ListVoices {
		match := codeRe.FindStringSubmatch(cell)
		if match == nil {
			return ""
		}
		return match[1]
	}
	if column == ListLanguages {
		return plain(cell)
	}
	return strings.ToLower(plain(cell))
}

// applySpeechOptions reads the text-to-speech models and languages overview,
// which states which languages each generation of voice covers. The voice
// catalog gives a language per voice for Aura and none at all for Flux, so
// this is the only document answering for a model as a whole.
func (b *builder) applySpeechOptions(doc catalog.Document) {
	body := string(doc.Body)
	summaries := familySummaries(body)
	for _, s := range mdSections(body) {
		id := speechModel(s.heading)
		m, ok := b.models[id]
		if !ok {
			continue
		}
		for _, r := range optionRows(s.body) {
			m.AddList(ListLanguages, r.languages...)
		}
		m.SetAttr(AttrSummary, summaries[slugID(s.heading)])
		m.AddSource(doc.URL)
	}
}

// speechModel returns the model a heading of the text-to-speech overview
// describes, under the name the pricing page sells it as.
func speechModel(heading string) string {
	id := slugID(heading)
	if tts, ok := ttsModels[id]; ok {
		return tts
	}
	return id
}
