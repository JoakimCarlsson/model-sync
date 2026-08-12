package xai

import (
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Sections of the pricing page, each holding a table of a different shape.
const (
	sectionText    = "text api pricing"
	sectionImagine = "imagine pricing"
	sectionVoice   = "voice pricing"
	sectionTools   = "tool invocation costs"
)

// applyPricing reads the pricing page.
func (b *builder) applyPricing(doc catalog.Document) {
	body := string(doc.Body)
	for _, t := range scanTables(body, doc.URL) {
		switch t.Section {
		case sectionText:
			b.applyTextTable(t)
		case sectionImagine:
			b.applyImagineTable(t)
		case sectionVoice:
			b.applyVoiceTable(t)
		case sectionTools:
			b.applyToolTable(t)
		}
	}
	b.applyBatchDiscounts(body, doc.URL)
	b.applyCutoff(body, doc.URL)
}

// textColumns are the rate columns of the text table, in the order xAI writes
// them, paired with what each bills for.
var textColumns = []struct {
	header string
	metric catalog.Metric
}{
	{"input / 1m tokens", MetricInputTokens},
	{"cached input / 1m tokens", MetricCachedInputTokens},
	{"output / 1m tokens", MetricOutputTokens},
}

// applyTextTable reads the per-token rates. Each model occupies two rows, one
// per prompt band, which becomes a dimension rather than two models.
func (b *builder) applyTextTable(t mdTable) {
	idCol := columnOf(t.Headers, "model")
	contextCol := columnOf(t.Headers, "context")
	if idCol < 0 {
		return
	}
	for _, row := range t.Rows {
		ref := splitModelCell(cellAt(row, idCol))
		if ref.ID == "" {
			continue
		}
		m := b.model(ref.ID, KindChat)
		m.AddSource(t.Source)
		m.SetLimit(LimitContextWindow, parseCount(cellAt(row, contextCol)))
		dims := catalog.Dims{}.With(DimPromptBand, ref.Band)
		for _, col := range textColumns {
			at := columnOf(t.Headers, col.header)
			if at < 0 {
				continue
			}
			a := parseAmount(cellAt(row, at))
			if !a.Found {
				continue
			}
			m.AddPrice(catalog.Price{
				Metric:   col.metric,
				Unit:     UnitPer1MTokens,
				Amount:   a.Value,
				Currency: currency,
				Dims:     dims,
			})
		}
	}
}

// applyImagineTable reads the image and video rates, which state their unit in
// the cell and are told apart by it.
func (b *builder) applyImagineTable(t mdTable) {
	idCol := columnOf(t.Headers, "model")
	costCol := columnOf(t.Headers, "cost")
	if idCol < 0 || costCol < 0 {
		return
	}
	for _, row := range t.Rows {
		id := clean(cellAt(row, idCol))
		a := parseAmount(cellAt(row, costCol))
		if id == "" || !a.Found {
			continue
		}
		metric, kind := MetricImageOutput, KindImage
		if a.Unit == UnitPerSecond {
			metric, kind = MetricVideoOutput, KindVideo
		}
		m := b.model(id, kind)
		m.AddSource(t.Source)
		m.AddPrice(catalog.Price{
			Metric:   metric,
			Unit:     a.Unit,
			Amount:   a.Value,
			Currency: currency,
			Note:     a.Note,
		})
	}
}

// applyVoiceTable reads the voice rates.
//
// This table is the untidiest xAI publishes. Its first column is a mode rather
// than a model, sometimes naming the model in parentheses and sometimes not; a
// cell can hold two rates separated by a line break, one for audio and one for
// text; and a rate can be followed by the same rate expressed per hour, or by
// two rates distinguished only by a parenthesised transport.
func (b *builder) applyVoiceTable(t mdTable) {
	modeCol := columnOf(t.Headers, "mode")
	costCol := columnOf(t.Headers, "cost")
	if modeCol < 0 || costCol < 0 {
		return
	}
	for _, row := range t.Rows {
		mode := clean(cellAt(row, modeCol))
		if mode == "" {
			continue
		}
		id, label := voiceID(mode)
		m := b.model(id, KindVoice)
		m.AddSource(t.Source)
		if m.Name == "" {
			m.Name = label
		}
		dims := catalog.Dims{}.With(DimMode, slugID(label))
		for _, fragment := range splitFragments(cellAt(row, costCol)) {
			for _, clause := range splitRates(fragment) {
				rest, transport := rateQualifier(clause)
				a := parseAmount(rest)
				if !a.Found {
					m.AddNote(clause)
					continue
				}
				m.AddPrice(catalog.Price{
					Metric:   MetricAudio,
					Unit:     a.Unit,
					Amount:   a.Value,
					Currency: currency,
					Dims:     dims.With(DimTransport, transport),
					Note:     a.Note,
				})
			}
		}
	}
}

// voiceID resolves the mode cell to an identifier, preferring the model named
// in parentheses over the mode's own name.
func voiceID(mode string) (id, label string) {
	open := strings.Index(mode, "(")
	if open < 0 || !strings.HasSuffix(mode, ")") {
		return slugID(mode), strings.TrimSpace(mode)
	}
	inner := strings.TrimSpace(mode[open+1 : len(mode)-1])
	return inner, strings.TrimSpace(mode[:open])
}

// applyToolTable reads the server-side tool rates. Tools whose cost is
// token-based or a cross-reference carry no rate of their own and are recorded
// without one rather than skipped.
func (b *builder) applyToolTable(t mdTable) {
	nameCol := columnOf(t.Headers, "tool name")
	labelCol := columnOf(t.Headers, "tool")
	costCol := columnOf(t.Headers, "cost / 1k calls")
	descCol := columnOf(t.Headers, "description")
	if nameCol < 0 || costCol < 0 {
		return
	}
	for _, row := range t.Rows {
		id, _, _ := strings.Cut(clean(cellAt(row, nameCol)), ",")
		id = strings.TrimSpace(strings.Trim(id, "†* "))
		if id == "" || strings.Contains(id, " ") {
			continue
		}
		m := b.model(id, KindTool)
		m.AddSource(t.Source)
		if m.Name == "" {
			m.Name = clean(cellAt(row, labelCol))
		}
		m.SetAttr(AttrSummary, clean(cellAt(row, descCol)))
		a := parseAmount(cellAt(row, costCol))
		if !a.Found {
			m.AddNote(clean(cellAt(row, costCol)))
			continue
		}
		m.AddPrice(catalog.Price{
			Metric:   MetricToolCall,
			Unit:     UnitPer1KCalls,
			Amount:   a.Value,
			Currency: currency,
		})
	}
}

// applyBatchDiscounts records the batch discount, which xAI states as a
// percentage followed by the models it applies to rather than as rates. The
// percentage is recorded rather than multiplied out, because a derived rate
// would be indistinguishable from one xAI published.
func (b *builder) applyBatchDiscounts(body, source string) {
	at := discountRe.FindStringSubmatchIndex(body)
	if at == nil {
		return
	}
	discount := body[at[2]:at[3]] + "%"
	for _, raw := range strings.Split(body[at[1]:], "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		id, ok := strings.CutPrefix(line, "- ")
		if !ok {
			break
		}
		m := b.model(clean(id), KindChat)
		m.AddSource(source)
		m.SetAttr(AttrBatchDiscount, discount)
	}
}

// applyCutoff records the knowledge cutoff, which xAI states in a sentence
// naming one model rather than in any model's own listing.
func (b *builder) applyCutoff(body, source string) {
	match := cutoffRe.FindStringSubmatch(body)
	if match == nil {
		return
	}
	id := slugID(match[1])
	if _, known := b.models[id]; !known {
		return
	}
	m := b.model(id, KindChat)
	m.AddSource(source)
	m.SetAttr(AttrKnowledgeCutoff, isoDate(match[2]))
}
