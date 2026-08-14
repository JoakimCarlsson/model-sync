package together

import (
	"html"
	"regexp"
	"slices"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// LibraryIndexURL is the sitemap naming every page Together's site holds,
// which is how the model library is enumerated. The library's own index pages
// are paginated and their cards name a model without stating the identifier
// the API answers to, so a card cannot be matched to a catalog row; the page
// behind it can.
const LibraryIndexURL = "https://www.together.ai/sitemap.xml"

// LibraryPre prefixes one model's page in that library.
const LibraryPre = "https://www.together.ai/models/"

// libraryItemPre opens one item of the specification list a model's page
// carries, which is a label followed by the values recorded under it.
const libraryItemPre = `models-d-content_box-item-inner">`

// Labels of the specification list this package reads.
const (
	libraryEndpoint   = "Endpoint"
	libraryContext    = "Context length"
	libraryType       = "Type"
	libraryFeatures   = "Features"
	libraryInputMode  = "Input modalities"
	libraryOutputMode = "Output modalities"
)

// libraryReasoning is the value the Type label takes for a model that reasons.
// The label is a set of tags saying what a model is, and this is the one of
// them that names a capability rather than a use.
const libraryReasoning = "reasoning"

// libraryFeatureValues maps a tag of the Features label onto the capability it
// names. Together writes the weaker of the two structured-output strengths
// here, so a model tagged with it carries the narrow value and the general one
// both, the general one being what a consumer asking whether the answer can be
// constrained keys on.
var libraryFeatureValues = map[string][]string{
	"function calling": {catalog.CapabilityFunctionCalling},
	"json mode": {
		catalog.CapabilityStructuredOutputs,
		catalog.CapabilityJSONMode,
	},
}

// libraryModalities maps a tag of the two modality labels onto the medium it
// names.
var libraryModalities = map[string]string{
	"text":  ModalityText,
	"image": ModalityImage,
	"video": ModalityVideo,
	"audio": ModalityAudio,
}

// libraryPageRe matches one model's page in the sitemap.
var libraryPageRe = regexp.MustCompile(
	regexp.QuoteMeta(LibraryPre) + `[A-Za-z0-9._-]+`,
)

// libraryTagRe matches one HTML tag, which is all that separates a label from
// its values.
var libraryTagRe = regexp.MustCompile(`<[^>]*>`)

// libraryURLs derives the model pages the sitemap names.
func libraryURLs(sitemap catalog.Document) []string {
	var urls []string
	for _, url := range libraryPageRe.FindAllString(string(sitemap.Body), -1) {
		if !slices.Contains(urls, url) {
			urls = append(urls, url)
		}
	}
	return urls
}

// libraryFields reads the specification list, returning each label with the
// values under it. A label stated twice keeps its first reading, since the
// page repeats the list once per tab.
func libraryFields(body string) map[string][]string {
	fields := map[string][]string{}
	for _, chunk := range strings.Split(body, libraryItemPre)[1:] {
		item, _, _ := strings.Cut(chunk, "</li>")
		parts := libraryParts(item)
		if len(parts) < 2 {
			continue
		}
		if _, ok := fields[parts[0]]; !ok {
			fields[parts[0]] = parts[1:]
		}
	}
	return fields
}

// libraryParts splits one item into the text its markup holds.
func libraryParts(item string) []string {
	var out []string
	for _, part := range strings.Split(
		html.UnescapeString(libraryTagRe.ReplaceAllString(item, "\x00")),
		"\x00",
	) {
		if text := strings.Join(strings.Fields(part), " "); text != "" {
			out = append(out, text)
		}
	}
	return out
}

// applyLibrary reads one model's page in the library onto the model the
// catalog page established for it.
//
// The library states per model what the catalog page states per table and, for
// the models whose row is a dash, what the catalog page does not state at all:
// a context length, the two capabilities its columns report, and whether the
// model reasons. It states the modalities too, and it states them a model at a
// time rather than a table at a time, which is how a chat model that also
// takes video is distinguishable from one that does not.
//
// A page naming a model the catalog did not establish is skipped rather than
// creating one. The library covers every model Together has ever listed,
// including those it now serves only on dedicated inference, and the catalog
// page is what says which are still sold per token.
//
// The context length here is rounded to the nearest power-of-two label, 256K
// where the catalog page says 262144, so it is recorded only where the catalog
// page has none: the exact figure is the better one wherever it exists.
func (b *builder) applyLibrary(doc catalog.Document) {
	fields := libraryFields(string(doc.Body))
	id := ""
	if endpoint := fields[libraryEndpoint]; len(endpoint) > 0 {
		id = endpoint[0]
	}
	m, ok := b.models[id]
	if !ok {
		return
	}
	m.AddSource(doc.URL)
	if length := fields[libraryContext]; len(length) > 0 {
		m.SetLimit(LimitContextWindow, parseCount(length[0]))
	}
	for _, tag := range fields[libraryType] {
		if strings.EqualFold(tag, libraryReasoning) {
			m.AddList(ListFeatures, catalog.CapabilityReasoning)
		}
	}
	for _, tag := range fields[libraryFeatures] {
		m.AddList(ListFeatures, libraryFeatureValues[strings.ToLower(tag)]...)
	}
	for _, tag := range fields[libraryInputMode] {
		m.AddList(ListInputModalities, libraryModalities[strings.ToLower(tag)])
	}
	for _, tag := range fields[libraryOutputMode] {
		m.AddList(ListOutputModalities, libraryModalities[strings.ToLower(tag)])
	}
}
