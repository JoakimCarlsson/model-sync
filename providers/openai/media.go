package openai

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Enumerations and bounds the image and video guides state. None of them are
// on a model page: an image or video model's page lists its modalities, its
// snapshot and its rate limits and stops, and what the model will actually
// render is written only in the guide for it.
const (
	ListSizes            = "sizes"
	ListResolutions      = "resolutions"
	ListQualities        = "qualities"
	ListOutputFormats    = "output_formats"
	ListDurationsSeconds = "durations_seconds"

	LimitMaxDurationSeconds = "max_duration_seconds"
	LimitMaxEdgePixels      = "max_edge_pixels"
	LimitMinOutputPixels    = "min_output_pixels"
	LimitMaxOutputPixels    = "max_output_pixels"
)

// ImageGuideURL and VideoGuideURL are where what a generated image or video
// may be is written.
const (
	ImageGuideURL = baseURL + "/api/docs/guides/image-generation.md"
	VideoGuideURL = baseURL + "/api/docs/guides/video-generation.md"
)

// Headings and labels the image guide writes its options under. The options
// live in a raw HTML table whose rows are label and list rather than header and
// data, so the label cell is what selects the list beside it.
const (
	sizeQualityHeading  = "### size and quality options"
	outputFormatHeading = "### output format"
	labelPopularSizes   = "popular sizes"
	labelConstraints    = "size constraints"
	labelQualities      = "quality options"
)

var (
	// gptImageFamilyRe matches the sentence enumerating the models the image
	// guide's limitations and format rules cover, written as "GPT Image models
	// (`gpt-image-2`, `gpt-image-1.5`, `gpt-image-1`, and `gpt-image-1-mini`)".
	gptImageFamilyRe = regexp.MustCompile(`GPT Image models \(([^)]*)\)`)
	// imageFormatRe matches the sentence naming the formats the Image API
	// returns.
	imageFormatRe = regexp.MustCompile(
		"The default format is `(\\w+)`, but you can also request " +
			"`(\\w+)` or `(\\w+)`",
	)
	// dimensionRe matches a width by height token, which is how both guides
	// write a size.
	dimensionRe = regexp.MustCompile(`^\d+x\d+$`)
	// maxEdgeRe, totalPixelsRe read the two constraints the image guide states
	// as prose inside its options table.
	maxEdgeRe = regexp.MustCompile(
		"(?i)maximum edge length must be less than or equal to\\s*`?([\\d,]+)px",
	)
	totalPixelsRe = regexp.MustCompile(
		"(?i)total pixels must be at least\\s*`?([\\d,]+)`?\\s*" +
			"and no more than\\s*`?([\\d,]+)",
	)
	// videoDurationRe matches the sentence stating how long a Sora generation
	// may run, written as "Both `sora-2` and `sora-2-pro` support `16`- and
	// `20`-second generations."
	videoDurationRe = regexp.MustCompile(
		"(?i)(.+?) support ([`\\d\\-, and]+)-second generations",
	)
	// htmlCellRe matches one cell of the image guide's options table.
	htmlCellRe = regexp.MustCompile(`(?is)<td[^>]*>(.*?)</td>`)
)

// applyImageGuide reads what a GPT Image model will render.
//
// Two things are read besides the per-image rates the JSX tables carry. The
// sizes, qualities and pixel bounds are stated for gpt-image-2 alone, in the
// paragraph that says it accepts any resolution satisfying the constraints
// beside it, so they are recorded against that model and no other. The output
// formats are stated for the Image API as a whole, and the models that reach
// it are the ones the guide's limitations paragraph enumerates by name.
func (b *builder) applyImageGuide(doc catalog.Document) {
	for _, t := range scanJSXTables(doc) {
		b.applyImageTable(t)
	}
	body := string(doc.Body)
	b.applyImageOptions(doc.URL, body)
	b.applyImageFormats(doc.URL, body)
}

// applyImageOptions records the sizes, qualities and pixel bounds gpt-image-2
// accepts.
func (b *builder) applyImageOptions(source, body string) {
	m := b.models[imageOptionsModel]
	if m == nil {
		return
	}
	section := sectionAfterPrefix(body, sizeQualityHeading)
	if section == "" {
		return
	}
	m.AddSource(source)
	for label, cell := range htmlLabelled(section) {
		switch label {
		case labelPopularSizes:
			for _, token := range quotedTokens(cell) {
				if dimensionRe.MatchString(token) {
					m.AddList(ListSizes, token)
				}
			}
		case labelQualities:
			m.AddList(ListQualities, quotedTokens(cell)...)
		case labelConstraints:
			applyPixelBounds(m, cell)
		}
	}
}

// imageOptionsModel is the model the image guide's options table describes.
// The paragraph above it names it, and the sizes it lists are introduced as
// the ones that model accepts rather than as the API's.
const imageOptionsModel = "gpt-image-2"

// applyPixelBounds records the two constraints on a generated image's size.
func applyPixelBounds(m *catalog.Model, text string) {
	if match := maxEdgeRe.FindStringSubmatch(text); match != nil {
		m.SetLimit(LimitMaxEdgePixels, parseCount(match[1]))
	}
	if match := totalPixelsRe.FindStringSubmatch(text); match != nil {
		m.SetLimit(LimitMinOutputPixels, parseCount(match[1]))
		m.SetLimit(LimitMaxOutputPixels, parseCount(match[2]))
	}
}

// applyImageFormats records the file formats the Image API returns, against
// each model the guide names as reaching it.
func (b *builder) applyImageFormats(source, body string) {
	match := imageFormatRe.FindStringSubmatch(
		sectionAfterPrefix(body, outputFormatHeading),
	)
	family := gptImageFamilyRe.FindStringSubmatch(body)
	if match == nil || family == nil {
		return
	}
	for _, id := range quotedTokens(family[1]) {
		m := b.models[id]
		if m == nil {
			continue
		}
		m.AddSource(source)
		m.AddList(ListOutputFormats, match[1], match[2], match[3])
	}
}

// applyVideoGuide records how long a Sora generation may run.
//
// The guide states the two lengths in one sentence naming both models, which
// is the only place either is written; the pricing page states a rate per
// second and no bound on the seconds.
func (b *builder) applyVideoGuide(doc catalog.Document) {
	match := videoDurationRe.FindStringSubmatch(string(doc.Body))
	if match == nil {
		return
	}
	var seconds []string
	for _, token := range quotedTokens(match[2]) {
		if parseCount(token) > 0 {
			seconds = append(seconds, token)
		}
	}
	for _, id := range quotedTokens(match[1]) {
		m := b.models[id]
		if m == nil || len(seconds) == 0 {
			continue
		}
		m.AddSource(doc.URL)
		m.AddList(ListDurationsSeconds, seconds...)
		m.SetLimit(
			LimitMaxDurationSeconds,
			parseCount(seconds[len(seconds)-1]),
		)
	}
}

// htmlLabelled reads a two column HTML table whose left cell labels the list in
// its right cell, which is how the image guide states its options.
func htmlLabelled(section string) map[string]string {
	out := map[string]string{}
	cells := htmlCellRe.FindAllStringSubmatch(section, -1)
	for i := 0; i+1 < len(cells); i += 2 {
		label := strings.ToLower(
			strings.Join(strings.Fields(cellText(cells[i][1])), " "),
		)
		out[label] = cells[i+1][1]
	}
	return out
}

// quotedTokens returns every backticked token in a fragment, which is how
// OpenAI writes an identifier, a size, a format and a duration alike.
func quotedTokens(text string) []string {
	var out []string
	for _, match := range quotedRe.FindAllStringSubmatch(text, -1) {
		if token := strings.TrimSpace(match[1]); token != "" {
			out = append(out, token)
		}
	}
	return out
}

// sectionAfterPrefix returns the body of the section whose heading begins with
// prefix, matched without regard to case, up to the next heading of the same
// or a higher level.
func sectionAfterPrefix(body, prefix string) string {
	depth := len(prefix) - len(strings.TrimLeft(prefix, "#"))
	var out []string
	inside := false
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if !inside {
			if strings.HasPrefix(strings.ToLower(line), prefix) {
				inside = true
			}
			continue
		}
		if hashes := len(line) - len(strings.TrimLeft(line, "#")); hashes > 0 &&
			hashes <= depth {
			break
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
