package assemblyai

import (
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// sectionModes maps a heading onto the mode of the models listed under it.
var sectionModes = map[string]string{
	"pre-recorded models": ModePrerecorded,
	"streaming models":    ModeStreaming,
	"add-on models":       ModeAddOn,
	"pre-recorded":        ModePrerecorded,
	"streaming":           ModeStreaming,
}

// sectionMetrics maps the heading above a rate table onto what that rate
// counts. The pricing page states the distinction in prose beside the
// streaming table rather than in the table itself.
var sectionMetrics = map[string]catalog.Metric{
	ModePrerecorded: MetricAudio,
	ModeStreaming:   MetricSession,
}

// applyModels reads the models page: first the cards describing each model,
// then the rate tables, which name models the same way the cards do.
func (b *builder) applyModels(doc catalog.Document) {
	body := string(doc.Body)
	b.applyCards(body, doc.URL)
	b.applyRateTables(body, doc.URL)
}

// applyCards reads the MDX cards, each of which is one model and its
// capabilities.
func (b *builder) applyCards(body, source string) {
	for _, block := range sectionBlocks(body) {
		mode, ok := sectionModes[block.heading]
		if !ok {
			continue
		}
		for _, card := range cardRe.FindAllStringSubmatch(block.body, -1) {
			name := strings.TrimSpace(card[1])
			if name == "" {
				continue
			}
			m := b.model(slugID(name), KindTranscription)
			m.AddSource(source)
			if m.Name == "" {
				m.Name = name
			}
			m.SetAttr(AttrMode, mode)
			m.SetAttr(AttrDocumentationURL, strings.TrimSpace(card[2]))
			for _, item := range listItemRe.FindAllStringSubmatch(card[3], -1) {
				m.AddList(ListCapabilities, clean(item[1]))
			}
		}
	}
}

// applyRateTables reads the rate tables under the pricing heading.
func (b *builder) applyRateTables(body, source string) {
	for _, block := range sectionBlocks(body) {
		mode, ok := sectionModes[block.heading]
		if !ok {
			continue
		}
		metric, ok := sectionMetrics[mode]
		if !ok {
			continue
		}
		for _, row := range tableRows(block.body) {
			b.applyRateRow(row, mode, metric, source)
		}
	}
}

// applyRateRow records one model's rate.
func (b *builder) applyRateRow(
	row []string,
	mode string,
	metric catalog.Metric,
	source string,
) {
	name := clean(cellAt(row, 0))
	if name == "" || strings.EqualFold(name, "model") {
		return
	}
	amount, ok := parseAmount(cellAt(row, 1))
	if !ok {
		return
	}
	m := b.model(slugID(name), KindTranscription)
	m.AddSource(source)
	if m.Name == "" {
		m.Name = name
	}
	m.SetAttr(AttrMode, mode)
	m.SetAttr(AttrVolumeDiscounts, strings.ToLower(clean(cellAt(row, 2))))
	m.AddPrice(catalog.Price{
		Metric:   metric,
		Unit:     UnitPerHour,
		Amount:   amount,
		Currency: currency,
		Dims:     catalog.Dims{DimMode: mode},
	})
}

// block is one heading and everything under it up to the next heading.
type block struct {
	heading string
	body    string
}

// sectionBlocks divides a document by heading, so that a card or a table can
// be attributed to the section it appears in.
func sectionBlocks(body string) []block {
	var (
		out     []block
		heading string
		lines   []string
	)
	flush := func() {
		if heading != "" {
			out = append(out, block{heading, strings.Join(lines, "\n")})
		}
		lines = nil
	}
	for _, raw := range strings.Split(body, "\n") {
		if after, ok := strings.CutPrefix(strings.TrimSpace(raw), "#"); ok {
			flush()
			heading = strings.ToLower(
				clean(strings.TrimSpace(strings.TrimLeft(after, "# "))),
			)
			continue
		}
		lines = append(lines, raw)
	}
	flush()
	return out
}

// tableRows returns the data rows of every pipe table in a passage.
func tableRows(body string) [][]string {
	var out [][]string
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := splitRow(line)
		if isSeparator(cells) {
			continue
		}
		out = append(out, cells)
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
