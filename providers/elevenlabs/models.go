package elevenlabs

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Kinds of model ElevenLabs publishes.
const (
	KindSpeech        catalog.Kind = "speech"
	KindTranscription catalog.Kind = "transcription"
	KindVoiceChanger  catalog.Kind = "voice-changer"
	KindVoiceDesign   catalog.Kind = "voice-design"
	KindMusic         catalog.Kind = "music"
	KindSoundEffects  catalog.Kind = "sound-effects"
)

// States ElevenLabs distinguishes by which table a model appears in.
const (
	StateCurrent    = "current"
	StateDeprecated = "deprecated"
)

// Scalar keys the models page populates.
const (
	AttrSummary  = "summary"
	AttrState    = "state"
	AttrDuration = "approximate_audio_duration"
)

// Numeric keys the models page populates.
const LimitCharacterLimit = "character_limit"

// ListLanguages holds the languages a model supports, where ElevenLabs names
// them rather than giving a count.
const ListLanguages = "languages"

// sectionDeprecated is the heading above the models being withdrawn.
const sectionDeprecated = "deprecated models"

// idKinds maps a fragment of an identifier onto what the model does. The
// fragments are checked in order, because a name can contain more than one and
// the earlier ones are more specific.
var idKinds = []struct {
	fragment string
	kind     catalog.Kind
}{
	{"_sts_", KindVoiceChanger},
	{"_ttv_", KindVoiceDesign},
	{"scribe", KindTranscription},
	{"music", KindMusic},
	{"text_to_sound", KindSoundEffects},
}

var (
	linkRe  = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	tagRe   = regexp.MustCompile(`(?s)<[^>]*>`)
	codeRe  = regexp.MustCompile("`([^`]+)`")
	countRe = regexp.MustCompile(`([\d,]+)`)
)

// clean strips markdown decoration from a cell value.
func clean(cell string) string {
	s := linkRe.ReplaceAllString(cell, "$1")
	s = tagRe.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, `\~`, "~")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "`", "")
	return strings.Join(strings.Fields(s), " ")
}

// kindFor reports what a model does, read from its identifier.
func kindFor(id string) catalog.Kind {
	lower := strings.ToLower(id)
	for _, entry := range idKinds {
		if strings.Contains(lower, entry.fragment) {
			return entry.kind
		}
	}
	return KindSpeech
}

// parseCount reads a grouped decimal such as "40,000".
func parseCount(cell string) int64 {
	match := countRe.FindStringSubmatch(clean(cell))
	if match == nil {
		return 0
	}
	n, err := strconv.ParseInt(strings.ReplaceAll(match[1], ",", ""), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// applyModels reads the models page.
func (b *builder) applyModels(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		idCol := columnOf(t.Headers, "model id")
		if idCol < 0 {
			continue
		}
		var (
			descCol  = columnOf(t.Headers, "description")
			langCol  = columnOf(t.Headers, "languages")
			limitCol = columnOf(t.Headers, "character limit")
			durCol   = columnOf(t.Headers, "approximate audio duration")
		)
		for _, row := range t.Rows {
			id := clean(cellAt(row, idCol))
			if id == "" {
				continue
			}
			m := b.model(id, kindFor(id))
			m.AddSource(t.Source)
			m.SetAttr(AttrSummary, clean(cellAt(row, descCol)))
			m.SetAttr(AttrDuration, clean(cellAt(row, durCol)))
			m.SetLimit(LimitCharacterLimit, parseCount(cellAt(row, limitCol)))
			m.AddList(ListLanguages, languagesOf(cellAt(row, langCol))...)
			if t.Section == sectionDeprecated {
				m.SetAttr(AttrState, StateDeprecated)
			} else {
				m.SetAttr(AttrState, StateCurrent)
			}
		}
	}
}

// languagesOf reads the languages cell, which names them as codes when they
// are few and links to a list when they are many. Only named codes are
// recorded, since a link says nothing a reader can use.
func languagesOf(cell string) []string {
	var out []string
	for _, match := range codeRe.FindAllStringSubmatch(cell, -1) {
		if code := strings.TrimSpace(match[1]); code != "" {
			out = append(out, code)
		}
	}
	return out
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
