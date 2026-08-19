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
	libraryProvider   = "Model provider"
	libraryUseCases   = "Main use cases"
	libraryDeployment = "Deployment"
	libraryParameters = "Parameters"
	libraryActive     = "Activated parameters"
	libraryReleased   = "Released"
	libraryUpdated    = "Last updated"
	libraryQuant      = "Quantization level"
	librarySpeed      = "Speed"
	libraryIntellect  = "Intelligence"
	libraryCategory   = "Category"
	libraryExternal   = "External link"
)

// libraryReasoning is the value the Type label takes for a model that reasons.
// The label is a set of tags saying what a model is, and this is the one of
// them that names a capability rather than a use.
const libraryReasoning = "reasoning"

// huggingFacePre prefixes a Hugging Face repository, which is where the
// External link points for a model whose weights are published there.
const huggingFacePre = "https://huggingface.co/"

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
// names. Together writes Structured Data for what an embedding model returns,
// which names a shape rather than a medium and has no counterpart here.
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

// libraryDescRe matches the description Together writes into the head of a
// model's page, which is the one sentence it summarizes the model in.
var libraryDescRe = regexp.MustCompile(
	`<meta content="([^"]*)" name="description"`,
)

// libraryHrefRe matches the target of the only link a specification item
// holds.
var libraryHrefRe = regexp.MustCompile(`href="([^"]+)"`)

// libraryOutputRes match the two ways a model's prose states a ceiling on how
// much it may generate. Neither the specification list nor any table has a
// field for it, and the prose is where Together states it when the ceiling is
// lower than the context window.
var libraryOutputRes = []*regexp.Regexp{
	regexp.MustCompile(`([\d,]+)[\s-]token output cap`),
	regexp.MustCompile(`([\d,]+)\s+max(?:imum)? output tokens`),
}

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

// libraryItem is one entry of the specification list: a label, the values
// recorded under it, and the markup they were read from, which is what still
// holds the target of a link.
type libraryItem struct {
	Label  string
	Values []string
	Markup string
}

// libraryFields reads the specification list, returning each label with the
// values under it. A label stated twice keeps its first reading, since the
// page repeats the list once per tab.
func libraryFields(body string) map[string]libraryItem {
	fields := map[string]libraryItem{}
	for _, chunk := range strings.Split(body, libraryItemPre)[1:] {
		markup, _, _ := strings.Cut(chunk, "</li>")
		parts := libraryParts(markup)
		if len(parts) < 2 {
			continue
		}
		if _, ok := fields[parts[0]]; !ok {
			fields[parts[0]] = libraryItem{
				Label:  parts[0],
				Values: parts[1:],
				Markup: markup,
			}
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

// libraryFirst returns the first value recorded under a label.
func libraryFirst(fields map[string]libraryItem, label string) string {
	if item, ok := fields[label]; ok && len(item.Values) > 0 {
		return item.Values[0]
	}
	return ""
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
// It is also the only document stating when a model was released, how many
// parameters it has, how many of them a token activates, what Together
// summarizes it as, and where its weights are published. Those hold for every
// modality, which is what makes the library the widest of the documents read
// here: an image or video model has a row on the catalog page and nothing
// else, and a page here.
//
// A page naming a model the catalog did not establish is skipped rather than
// creating one. The library covers every model Together has ever listed,
// including those it now serves only on dedicated inference, and the catalog
// page is what says which are still sold per token.
//
// The context length here is rounded to the nearest power-of-two label, 256K
// where the catalog page says 262144, so it is recorded only where the catalog
// page has none: the exact figure is the better one wherever it exists. The
// prices it states are not read at all, for the same reason and worse: they
// are rounded, they omit the dimensions a video or image rate varies along,
// and the catalog page states all of them.
func (b *builder) applyLibrary(doc catalog.Document) {
	body := string(doc.Body)
	fields := libraryFields(body)
	m, ok := b.models[libraryFirst(fields, libraryEndpoint)]
	if !ok {
		return
	}
	m.AddSource(doc.URL)
	if desc := libraryDescRe.FindStringSubmatch(body); desc != nil {
		m.SetAttr(AttrSummary, html.UnescapeString(desc[1]))
	}
	m.SetLimit(LimitContextWindow, parseCount(
		libraryFirst(fields, libraryContext),
	))
	m.SetLimit(LimitMaxOutputTokens, libraryOutputCap(body))
	m.SetAttr(AttrAuthor, libraryFirst(fields, libraryProvider))
	m.SetAttr(AttrQuantization, libraryFirst(fields, libraryQuant))
	m.SetAttr(AttrParameterCount, libraryFirst(fields, libraryParameters))
	m.SetAttr(AttrActiveParameters, libraryFirst(fields, libraryActive))
	m.SetAttr(AttrSpeed, libraryFirst(fields, librarySpeed))
	m.SetAttr(AttrIntelligence, libraryFirst(fields, libraryIntellect))
	m.SetAttr(AttrCategory, libraryFirst(fields, libraryCategory))
	m.SetAttr(AttrReleaseDate, parseDate(
		libraryFirst(fields, libraryReleased),
	))
	m.SetAttr(AttrLastUpdated, parseDate(libraryFirst(fields, libraryUpdated)))
	b.applyLibraryLink(m, fields)
	m.AddList(ListTags, fields[libraryType].Values...)
	m.AddList(ListUseCases, fields[libraryUseCases].Values...)
	m.AddList(ListDeployments, fields[libraryDeployment].Values...)
	for _, tag := range fields[libraryType].Values {
		if strings.EqualFold(tag, libraryReasoning) {
			m.AddList(ListFeatures, catalog.CapabilityReasoning)
		}
	}
	for _, tag := range fields[libraryFeatures].Values {
		m.AddList(ListFeatures, libraryFeatureValues[strings.ToLower(tag)]...)
	}
	for _, tag := range fields[libraryInputMode].Values {
		m.AddList(ListInputModalities, libraryModalities[strings.ToLower(tag)])
	}
	for _, tag := range fields[libraryOutputMode].Values {
		m.AddList(ListOutputModalities, libraryModalities[strings.ToLower(tag)])
	}
}

// applyLibraryLink records where the model is documented outside Together. The
// specification list calls the link provider docs and nothing more, and for a
// model whose weights are published it points at the Hugging Face repository,
// which is the only place Together states that identifier.
func (b *builder) applyLibraryLink(
	m *catalog.Model,
	fields map[string]libraryItem,
) {
	href := libraryHrefRe.FindStringSubmatch(fields[libraryExternal].Markup)
	if href == nil {
		return
	}
	m.SetAttr(AttrModelCardURL, href[1])
	if repo, ok := strings.CutPrefix(href[1], huggingFacePre); ok {
		m.SetAttr(AttrHuggingFaceID, strings.TrimSuffix(repo, "/"))
	}
}

// libraryOutputCap reads the ceiling on generated tokens out of a page's
// prose. Together states it in the model card and in the paragraph opening the
// page, in two wordings and never in a field of its own, so both wordings are
// tried and the first figure found is taken.
func libraryOutputCap(body string) int64 {
	text := strings.Join(libraryParts(body), " ")
	for _, re := range libraryOutputRes {
		if match := re.FindStringSubmatch(text); match != nil {
			return parseCount(match[1])
		}
	}
	return 0
}
