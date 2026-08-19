package voyage

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Sections of the pricing page.
const (
	sectionText       = "text embeddings"
	sectionMultimodal = "multimodal embeddings"
	sectionRerankers  = "rerankers"
	sectionOlder      = "older models"
)

// filesAPI is the storage service, which is billed like a model but is not
// one. It is recorded so that the rate is not lost for want of somewhere to
// put it.
const (
	filesAPIID   = "files-api"
	filesAPIName = "Files API"
)

var (
	storageRe = regexp.MustCompile(
		`(?i)Storage is priced at \*\*\$([\d.]+) per GB per month\*\*`,
	)
	batchDiscountRe = regexp.MustCompile(`(?i)\*\*(\d+)% discount\*\*`)
	upscaleRe       = regexp.MustCompile(
		`(?i)Images with fewer than ([\d,]+ ?[a-z]*) pixels will be upscaled`,
	)
	downsampleRe = regexp.MustCompile(
		`(?i)Images containing over ([\d,.]+ ?[a-z]*) pixels will be ` +
			`downsampled`,
	)
	batchWindowRe = regexp.MustCompile(
		`(?i)\*\*(\d+)-hour completion window\*\*`,
	)
	retentionRe = regexp.MustCompile(
		`(?i)each retained for (\d+) days`,
	)
	batchModelsRe = regexp.MustCompile(
		`(?i)Batch API can currently be used to execute queries against the ` +
			`following models:([^\n]+)`,
	)
)

// rateColumns are the rate columns of a pricing table, paired with the
// denominator each states.
//
// Both are read. A row states the same rate twice, per thousand tokens and per
// million tokens, and the two have disagreed: voyage-context-4 was listed at
// $0.00018 per thousand and $0.12 per million, differing by half again, until
// Voyage corrected the thousand column. Recording both keeps such a
// disagreement visible instead of resolving it here on a guess about which
// column is maintained.
var rateColumns = []struct {
	header string
	metric catalog.Metric
	unit   catalog.Unit
}{
	{"price per thousand tokens", MetricInputTokens, UnitPer1KTokens},
	{"price per million tokens", MetricInputTokens, UnitPer1MTokens},
	{"price per billion pixels", MetricPixels, UnitPer1BPixels},
}

// applyPricing reads the pricing page.
func (b *builder) applyPricing(doc catalog.Document) {
	body := string(doc.Body)
	for _, t := range scanTables(body, doc.URL) {
		state, ok := stateFor(t.Section)
		if !ok {
			continue
		}
		b.applyRateTable(t, state)
	}
	b.applyStorage(body, doc.URL)
}

// stateFor reports whether a section holds rates, and what it says about the
// standing of the models in it.
func stateFor(section string) (string, bool) {
	switch section {
	case sectionText, sectionMultimodal, sectionRerankers:
		return StateActive, true
	case sectionOlder:
		return StateLegacy, true
	}
	return "", false
}

// applyRateTable reads one rate table. A row can name several models sharing a
// rate, in which case each gets its own entry.
func (b *builder) applyRateTable(t table, state string) {
	idCol := columnOf(t.Headers, "model")
	if idCol < 0 {
		return
	}
	freeCol := columnOf(
		t.Headers,
		"number of free tokens",
		"number of free tokens and pixels",
	)
	estCol := columnOf(t.Headers, "estimated price per request")
	for _, row := range t.Rows {
		for _, id := range splitModels(cellAt(row, idCol)) {
			m := b.model(id, kindFor(id))
			m.AddSource(t.Source)
			m.SetAttr(AttrState, state)
			b.applyFreeAllowance(m, cellAt(row, freeCol))
			m.SetAttr(AttrEstPerRequest, clean(cellAt(row, estCol)))
			for _, col := range rateColumns {
				at := columnOf(t.Headers, col.header)
				if at < 0 {
					continue
				}
				amount, ok := parseAmount(cellAt(row, at))
				if !ok {
					continue
				}
				m.AddPrice(catalog.Price{
					Metric:   col.metric,
					Unit:     col.unit,
					Amount:   amount,
					Currency: currency,
				})
			}
		}
	}
}

// applyFreeAllowance records the usage Voyage grants before charging.
//
// The allowance is kept verbatim as well as parsed, because a multimodal model
// is granted two of them at once, as "200M text tokens and 150B pixels", and
// reducing that to one number would silently drop the pixel half.
func (b *builder) applyFreeAllowance(m *catalog.Model, cell string) {
	allowance := clean(cell)
	if allowance == "" {
		return
	}
	m.SetAttr(AttrFreeAllowance, allowance)
	if !strings.Contains(strings.ToLower(allowance), " and ") {
		m.SetLimit(LimitFreeTokens, parseCount(allowance))
	}
}

// applyPixelBands records the band an image is billed in, which the pricing
// page states in prose beneath the multimodal rates: an image smaller than the
// floor is enlarged and billed as the floor, and one larger than the ceiling
// is reduced and billed as the ceiling.
//
// It is read after the model pages rather than with the rest of the pricing
// page, because it applies to the models the multimodal page lists and nothing
// on the pricing page says which those are.
func (b *builder) applyPixelBands(doc catalog.Document) {
	body := string(doc.Body)
	low := upscaleRe.FindStringSubmatch(body)
	high := downsampleRe.FindStringSubmatch(body)
	if low == nil || high == nil {
		return
	}
	for _, m := range b.servedBy(baseURL + "/multimodal-embeddings.md") {
		m.SetLimit(LimitMinBillPixels, parseCount(low[1]))
		m.SetLimit(LimitMaxBillPixels, parseCount(high[1]))
	}
}

// applyStorage records the file storage rate, which Voyage states in prose and
// which is the only rate it quotes per gigabyte per month.
func (b *builder) applyStorage(body, source string) {
	match := storageRe.FindStringSubmatch(body)
	if match == nil {
		return
	}
	amount, ok := parseAmount("$" + match[1])
	if !ok {
		return
	}
	m := b.model(filesAPIID, KindTool)
	m.Name = filesAPIName
	m.AddSource(source)
	m.SetLimit(LimitFileRetention, retentionDays(body))
	m.AddPrice(catalog.Price{
		Metric:   MetricStorage,
		Unit:     UnitPerGBMonth,
		Amount:   amount,
		Currency: currency,
	})
}

// applyBatch records the batch discount against the models it covers, which
// Voyage lists in a sentence on its own page rather than beside the rates.
func (b *builder) applyBatch(doc catalog.Document) {
	body := string(doc.Body)
	discount := batchDiscountRe.FindStringSubmatch(body)
	models := batchModelsRe.FindStringSubmatch(body)
	if discount == nil || models == nil {
		return
	}
	window := batchWindowRe.FindStringSubmatch(body)
	for _, id := range backtickedIDs(models[1]) {
		m := b.model(id, kindFor(id))
		m.AddSource(doc.URL)
		m.SetAttr(AttrBatchDiscount, discount[1]+"%")
		if window != nil {
			m.SetAttr(AttrBatchWindow, window[1]+"-hour")
		}
	}
}

// retentionDays reads how long the storage service keeps a file, which Voyage
// states in the same paragraph as the storage rate.
func retentionDays(body string) int64 {
	match := retentionRe.FindStringSubmatch(body)
	if match == nil {
		return 0
	}
	return parseCount(match[1])
}
