package together

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// ListImageSizes holds the width-by-height combinations a model accepts, which
// Together states for the one image model whose rate depends on them.
const ListImageSizes = "image_sizes"

// pricingSuffix ends the heading of a section stating one model's rate card in
// prose rather than in the image table's single per-megapixel column.
const pricingSuffix = " pricing"

// imageRateRe matches one bullet of such a section: a resolution, then the
// amount charged for one image at it.
var imageRateRe = regexp.MustCompile(
	`(?m)^\s*\*\s*(.+?):\s*\\?\$([\d.]+)\s*/\s*image`,
)

// imageSizeRe matches one width-by-height pair. Together writes them with a
// multiplication sign rather than a letter.
var imageSizeRe = regexp.MustCompile(`(\d{3,4})\s*[x×]\s*(\d{3,4})`)

// headingRe matches a markdown heading, which is what ends the section before
// it.
var headingRe = regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`)

// applyImageRates reads the prose rate cards the catalog page carries beside
// its image table.
//
// The image table has one price column and it is per megapixel, which is the
// wrong shape for a model Together charges a flat amount per image for and
// varies that amount by resolution. Where that is the case it says so under a
// heading naming the model, and states in the same section which exact
// width-by-height combinations the model accepts. Both are read here and both
// are kept beside the table's own figure rather than replacing it, since the
// table states a rate for the default and this states the rest of the card.
//
// A section is tied to a model by its heading, which opens with the name the
// image table gives the model and ends with the word pricing. Nothing else on
// the page is headed that way.
func (b *builder) applyImageRates(doc catalog.Document) {
	for _, section := range proseSections(string(doc.Body)) {
		subject, ok := strings.CutSuffix(section.Heading, pricingSuffix)
		if !ok {
			continue
		}
		m := b.byName(subject)
		if m == nil {
			continue
		}
		applyImageSection(m, section.Text, doc.URL)
	}
}

// applyImageSection records what one such section states.
func applyImageSection(m *catalog.Model, section, source string) {
	for _, match := range imageRateRe.FindAllStringSubmatch(section, -1) {
		amount := parseAmount("$" + match[2])
		if !amount.Found {
			continue
		}
		m.AddPrice(catalog.Price{
			Metric:   MetricImageOutput,
			Unit:     UnitPerImage,
			Amount:   amount.Value,
			Currency: currency,
			Dims: catalog.Dims{
				DimResolution: imageResolution(match[1]),
			},
		})
		m.AddSource(source)
	}
	for _, match := range imageSizeRe.FindAllStringSubmatch(section, -1) {
		m.AddList(ListImageSizes, match[1]+"x"+match[2])
		m.AddSource(source)
	}
}

// imageResolution reduces a bullet's label to the resolution it names.
// Together writes one of them as a bare tier and one with the word resolution
// after it, and the two mean the same thing.
func imageResolution(label string) string {
	return strings.TrimSuffix(strings.ToLower(clean(label)), " resolution")
}

// byName returns the model whose name contains subject, or nil where none or
// more than one does. Matching on a name is only safe where it is
// unambiguous, and the page has one row per model name.
func (b *builder) byName(subject string) *catalog.Model {
	var found *catalog.Model
	for _, id := range b.order {
		m := b.models[id]
		if !strings.Contains(strings.ToLower(m.Name), subject) {
			continue
		}
		if found != nil {
			return nil
		}
		found = m
	}
	return found
}

// proseSection is one heading and the text under it.
type proseSection struct {
	Heading string
	Text    string
}

// proseSections splits a document into its headings and the text under each,
// in the order they are written so that a run is reproducible. A heading is
// lowercased and stripped of the decoration Together writes it with.
func proseSections(body string) []proseSection {
	headings := headingRe.FindAllStringSubmatchIndex(body, -1)
	out := make([]proseSection, 0, len(headings))
	for i, heading := range headings {
		end := len(body)
		if i+1 < len(headings) {
			end = headings[i+1][0]
		}
		out = append(out, proseSection{
			Heading: strings.ToLower(clean(body[heading[2]:heading[3]])),
			Text:    body[heading[1]:end],
		})
	}
	return out
}
