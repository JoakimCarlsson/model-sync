package xai

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Dimension keys the variant grid populates.
const (
	DimResolution = "resolution"
	DimQuality    = "quality"
	DimSize       = "size"
)

// Metrics charged for what is supplied to a generation request, as distinct
// from what it produces.
const (
	MetricImageInput catalog.Metric = "image_input"
	MetricVideoInput catalog.Metric = "video_input"
	MetricAudioInput catalog.Metric = "audio_input"
)

// qualitySeparator divides the two axes of a variant label, as in "1K · Low".
const qualitySeparator = "·"

// inputRows are the labels naming something supplied to a request rather than
// a variant of its output. They are what separates the input rows of the
// pricing grid from the output rows, since both are the same markup and the
// page states them in one run.
var inputRows = map[string]struct {
	metric catalog.Metric
	unit   catalog.Unit
}{
	"image":            {MetricImageInput, UnitPerImage},
	"video":            {MetricVideoInput, UnitPerSecond},
	"video per second": {MetricVideoInput, UnitPerSecond},
	"audio":            {MetricAudioInput, UnitPerSecond},
}

var (
	// variantRowRe matches one row of the pricing grid, which is a styled pair
	// of divs rather than a table. The test id is the anchor: it is the only
	// marker on the page that exists to identify an amount, and it survives
	// restyling in a way class names do not.
	variantRowRe = regexp.MustCompile(
		`>([^<>]{1,40})</div><div[^>]*font-mono[^>]*>` +
			`<span data-testid="usd-amount">\$([\d.,]+)</span>`,
	)
	// outputSummaryRe matches the headline output rate, which is the only
	// place the grid's denominator is written.
	outputSummaryRe = regexp.MustCompile(
		`<p[^>]*>Output</p><p[^>]*>(?:<span[^>]*>)*` +
			`<span data-testid="usd-amount">\$([\d.,]+)</span>(?:</span>)*` +
			`<span[^>]*>\s*(?:per\s+)?([^<]*)</span>`,
	)
)

// applyVariantPage reads the rendered detail page of a generation model.
//
// It exists because the markdown xAI serves for these models states one
// headline rate where the page states a matrix. The markdown for
// grok-imagine-image-2.0 gives $0.04 per image; the page gives four rates from
// $0.04 to $0.08 by resolution and quality, and the markdown figure is only
// the cheapest of them. Reading the markdown alone understates every model
// charging more for a larger or better output.
//
// Only the rates are taken from here. Everything else about these models still
// comes from their markdown, which states it more plainly.
func (b *builder) applyVariantPage(doc catalog.Document) {
	body := string(doc.Body)
	summary := outputSummaryRe.FindStringSubmatch(body)
	if summary == nil {
		return
	}
	unit, ok := unitFor(summary[2])
	if !ok {
		return
	}
	m := b.model(variantPageID(doc.URL), kindForUnit(unit))
	m.AddSource(doc.URL)

	inputs, outputs := splitVariantRows(variantRows(body))
	if len(outputs) > 0 {
		metric := outputMetric(unit)
		dropFlatRate(m, metric)
		for _, row := range outputs {
			m.AddPrice(catalog.Price{
				Metric:   metric,
				Unit:     unit,
				Amount:   row.amount,
				Currency: currency,
				Dims:     variantDims(row.label),
			})
		}
	}
	for _, row := range inputs {
		billing := inputRows[strings.ToLower(row.label)]
		m.AddPrice(catalog.Price{
			Metric:   billing.metric,
			Unit:     billing.unit,
			Amount:   row.amount,
			Currency: currency,
		})
	}
}

// variantRow is one label and amount from the grid.
type variantRow struct {
	label  string
	amount float64
}

// variantRows returns every label and amount pair on a page.
func variantRows(body string) []variantRow {
	var out []variantRow
	for _, match := range variantRowRe.FindAllStringSubmatch(body, -1) {
		amount, err := strconv.ParseFloat(
			strings.ReplaceAll(match[2], ",", ""),
			64,
		)
		if err != nil {
			continue
		}
		out = append(out, variantRow{
			label:  strings.TrimSpace(match[1]),
			amount: amount,
		})
	}
	return out
}

// splitVariantRows divides the grid into what a request is charged for
// supplying and what it is charged for producing.
//
// The division is by what a row is called, not by where it sits. The page runs
// both groups together in one sequence of identical markup, and a model with
// no output variants at all still lists its inputs, so reading position would
// file an input rate as a variant of the output.
func splitVariantRows(rows []variantRow) (inputs, outputs []variantRow) {
	for _, row := range rows {
		if _, ok := inputRows[strings.ToLower(row.label)]; ok {
			inputs = append(inputs, row)
			continue
		}
		outputs = append(outputs, row)
	}
	return inputs, outputs
}

// variantDims reads a label into the axes it names. xAI writes three forms:
// "1K · Low" gives a resolution and a quality, "1K (1024x1024)" gives a
// resolution and the pixel size it means, and "480p" gives a resolution alone.
func variantDims(label string) catalog.Dims {
	dims := catalog.Dims{}
	resolution := label
	if before, after, ok := strings.Cut(label, qualitySeparator); ok {
		resolution = before
		dims = dims.With(DimQuality, normalizeAxis(after))
	}
	if before, after, ok := strings.Cut(resolution, "("); ok {
		resolution = before
		dims = dims.With(DimSize, normalizeAxis(strings.TrimSuffix(
			strings.TrimSpace(after), ")",
		)))
	}
	return dims.With(DimResolution, normalizeAxis(resolution))
}

// normalizeAxis renders one axis of a label as a dimension value.
func normalizeAxis(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// dropFlatRate removes an undimensioned rate the matrix supersedes, so the
// headline figure is not left looking like a rate that applies whatever you
// ask for.
func dropFlatRate(m *catalog.Model, metric catalog.Metric) {
	m.Prices = slices.DeleteFunc(m.Prices, func(p catalog.Price) bool {
		return p.Metric == metric && len(p.Dims) == 0
	})
}

// outputMetric reports what a generation model's output rate bills for, which
// its denominator settles: images are priced per image, video per second.
func outputMetric(unit catalog.Unit) catalog.Metric {
	if unit == UnitPerSecond {
		return MetricVideoOutput
	}
	return MetricImageOutput
}

// kindForUnit reports what a model producing output at this denominator is.
func kindForUnit(unit catalog.Unit) catalog.Kind {
	if unit == UnitPerSecond {
		return KindVideo
	}
	return KindImage
}

// variantPageID reads the model identifier out of the page URL, which for a
// rendered page carries no extension to strip.
func variantPageID(url string) string {
	return url[strings.LastIndex(url, "/")+1:]
}
