package cohere

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Reference pages for the four endpoints the overview's endpoint column names.
// They are the only documents stating what a request may carry, and for the
// embedding and rerank families they are the only documents bounding a call at
// all: the overview's tables for those two families have no column for a
// quantity beyond the context length.
const (
	ChatReferenceURL      = "https://docs.cohere.com/reference/chat.md"
	EmbedReferenceURL     = "https://docs.cohere.com/reference/embed.md"
	RerankReferenceURL    = "https://docs.cohere.com/reference/rerank.md"
	AudioReferenceURL     = "https://docs.cohere.com/reference/create-audio-transcription.md"
	referenceEndpointChat = endpointChat
)

// referenceEndpoints map a reference page onto the endpoint it documents, as
// the overview's endpoint column writes it. A parameter reaches a model
// because the model answers on that endpoint and for no other reason, so a
// model the overview states no endpoint for is reached by none of them.
var referenceEndpoints = map[string]string{
	ChatReferenceURL:   referenceEndpointChat,
	EmbedReferenceURL:  "Embed",
	RerankReferenceURL: "Rerank",
	AudioReferenceURL:  "Audio Transcriptions",
}

// Keys the reference pages populate.
const (
	// LimitMaxTextsPerCall bounds how many texts one embedding call may carry.
	LimitMaxTextsPerCall = "max_texts_per_call"
	// LimitMaxImagesPerCall bounds how many images one embedding call may
	// carry, which only the third generation embedders are given.
	LimitMaxImagesPerCall = "max_images_per_call"
	// LimitMaxDocuments is the ceiling the rerank reference recommends rather
	// than enforces, which is why it is not named a maximum outright.
	LimitMaxDocuments = "recommended_max_documents"
	// LimitMaxTokensPerDocument is the length a rerank document is truncated
	// to unless the caller asks for another.
	LimitMaxTokensPerDocument = "default_max_tokens_per_document"
	// AttrMaxImageSize is the ceiling on one image of an embedding call.
	AttrMaxImageSize = "max_image_size"
	// AttrMaxTotalImageSize is the ceiling on all of them together.
	AttrMaxTotalImageSize = "max_total_image_size"
	// ListParameters is the enumeration of request parameters an endpoint
	// accepts.
	ListParameters = catalog.ListParameters
	// ListInputTypes enumerates what an embedding call may declare its input
	// is for, which changes the vector the same text is turned into.
	ListInputTypes = "input_types"
	// ListEmbeddingTypes enumerates the number formats a vector is returned
	// in.
	ListEmbeddingTypes = "embedding_types"
	// ListFileFormats enumerates the containers a transcription call accepts.
	ListFileFormats = "file_formats"
)

// Version substrings the embed reference states an image rule against. It
// states two rules, one for the third generation and one for the fourth and
// anything after it, and an identifier carries its generation.
const (
	embedV3 = "v3."
	embedV4 = "v4."
)

var (
	// bodyRe matches the request body section of a reference page, which is
	// where the parameters are listed. The section heading names the encoding
	// rather than the endpoint, so it is matched loosely and ends at the next
	// heading of the same level.
	bodyRe = regexp.MustCompile(`(?s)### Body \([^)]*\)\n(.*?)\n## `)
	// paramRe matches one top level parameter. A nested field of an object
	// parameter is indented and is deliberately not matched: it is a field of
	// a value, not a parameter of the request.
	paramRe = regexp.MustCompile("(?m)^- `([a-z0-9_]+)`")
	// maxTextsRe matches the ceiling on how many texts one embedding call may
	// carry.
	maxTextsRe = regexp.MustCompile(
		"Maximum number of texts per call is `(\\d+)`",
	)
	// inputTypesRe matches the enumeration of what an embedding call may
	// declare its input is for. It is anchored on the first value so that a
	// reference rewritten to enumerate something else yields nothing.
	inputTypesRe = regexp.MustCompile(
		"(?m)^\\s*- Allowed values: (`search_document`[^\n]*)$",
	)
	// embeddingTypesRe matches the enumeration of number formats a vector may
	// be returned in, anchored the same way.
	embeddingTypesRe = regexp.MustCompile(
		"(?m)^\\s*- Allowed values: (`float`[^\n]*)$",
	)
	// imagesV3Re matches the sentence bounding an embedding call's images for
	// the third generation embedders.
	imagesV3Re = regexp.MustCompile(
		"For \\*\\*Embed v3.x\\*\\* models, the maximum number of images per " +
			"call is `(\\d+)`, and each image has a maximum size of `([^`]+)`",
	)
	// imagesV4Re matches the sentence bounding them for the fourth, which
	// bounds the request rather than the image.
	imagesV4Re = regexp.MustCompile(
		"For \\*\\*Embed v4.0 and newer\\*\\* models, there is no limit on " +
			"the number of images per call. The combined size of all images " +
			"in the request must be at most `([^`]+)`",
	)
	// documentsRe matches the ceiling the rerank reference recommends.
	documentsRe = regexp.MustCompile(
		`we recommend against sending more than ([\d,]+) documents in a ` +
			`single request`,
	)
	// tokensPerDocRe matches the length a rerank document is truncated to.
	tokensPerDocRe = regexp.MustCompile(
		"`max_tokens_per_doc` \\(integer, optional\\)[^\n]*?Defaults to " +
			"`(\\d+)`",
	)
	// fileFormatsRe matches the containers a transcription call accepts.
	fileFormatsRe = regexp.MustCompile(
		`Supported file extensions are ([^.]+)\.`,
	)
	// valueRe matches one value of an enumeration, which the reference writes
	// in code style.
	valueRe = regexp.MustCompile("`([a-z0-9_]+)`")
)

// applyReference reads one endpoint's reference page.
//
// Everything here belongs to an endpoint rather than to a model, which is how
// Cohere documents it: the reference states what a call may carry and the
// overview states which models answer the call. The two together reach a
// model, and a model the overview states no endpoint for, which is the two
// nightly builds, is reached by neither.
func (b *builder) applyReference(doc catalog.Document) {
	endpoint, ok := referenceEndpoints[doc.URL]
	if !ok {
		return
	}
	body := string(doc.Body)
	params := parameters(body)
	for _, id := range b.order {
		m := b.models[id]
		if !onEndpoint(m, endpoint) {
			continue
		}
		m.AddList(ListParameters, params...)
		m.AddSource(doc.URL)
		b.applyEndpointLimits(doc.URL, body, m)
	}
}

// applyEndpointLimits records what one reference page bounds a call by.
func (b *builder) applyEndpointLimits(
	url, body string,
	m *catalog.Model,
) {
	switch url {
	case EmbedReferenceURL:
		applyEmbedReference(body, m)
	case RerankReferenceURL:
		applyRerankReference(body, m)
	case AudioReferenceURL:
		applyAudioReference(body, m)
	}
}

// applyEmbedReference records what the embed reference bounds a call by.
//
// The two image rules are stated per generation rather than per model, and an
// identifier carries its generation, so each rule reaches the models whose
// version it names. They bound different things: the third generation bounds
// the count and the single image, the fourth bounds only the request's total.
func applyEmbedReference(body string, m *catalog.Model) {
	if match := maxTextsRe.FindStringSubmatch(body); match != nil {
		m.SetLimit(LimitMaxTextsPerCall, parseCount(match[1]))
	}
	if match := inputTypesRe.FindStringSubmatch(body); match != nil {
		m.AddList(ListInputTypes, enumeration(match[1])...)
	}
	if match := embeddingTypesRe.FindStringSubmatch(body); match != nil {
		m.AddList(ListEmbeddingTypes, enumeration(match[1])...)
	}
	if match := imagesV3Re.FindStringSubmatch(body); match != nil &&
		strings.Contains(m.ID, embedV3) {
		m.SetLimit(LimitMaxImagesPerCall, parseCount(match[1]))
		m.SetAttr(AttrMaxImageSize, match[2])
	}
	if match := imagesV4Re.FindStringSubmatch(body); match != nil &&
		strings.Contains(m.ID, embedV4) {
		m.SetAttr(AttrMaxTotalImageSize, match[1])
	}
}

// applyRerankReference records what the rerank reference bounds a call by.
func applyRerankReference(body string, m *catalog.Model) {
	if match := documentsRe.FindStringSubmatch(body); match != nil {
		m.SetLimit(LimitMaxDocuments, parseCount(match[1]))
	}
	if match := tokensPerDocRe.FindStringSubmatch(body); match != nil {
		m.SetLimit(LimitMaxTokensPerDocument, parseCount(match[1]))
	}
}

// applyAudioReference records the containers a transcription call accepts.
func applyAudioReference(body string, m *catalog.Model) {
	match := fileFormatsRe.FindStringSubmatch(body)
	if match == nil {
		return
	}
	for _, format := range strings.FieldsFunc(match[1], func(r rune) bool {
		return r == ',' || r == ' '
	}) {
		if format == "and" {
			continue
		}
		m.AddList(ListFileFormats, format)
	}
}

// parameters reads the names of the request parameters a reference page lists.
func parameters(body string) []string {
	section := bodyRe.FindStringSubmatch(body)
	if section == nil {
		return nil
	}
	var out []string
	for _, match := range paramRe.FindAllStringSubmatch(section[1], -1) {
		out = append(out, match[1])
	}
	return out
}

// enumeration reads the values of an allowed value list.
func enumeration(list string) []string {
	var out []string
	for _, match := range valueRe.FindAllStringSubmatch(list, -1) {
		out = append(out, match[1])
	}
	return out
}

// onEndpoint reports whether the overview gives a model an endpoint.
func onEndpoint(m *catalog.Model, endpoint string) bool {
	return strings.Contains(
		strings.Join(m.Lists[ListEndpoints], " "),
		endpoint,
	)
}
