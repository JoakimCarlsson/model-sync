package perplexity

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// The API references this parser reads besides the two chat endpoints. Each
// documents one endpoint and, where the endpoint serves a fixed set of models,
// enumerates them, which is what says who its schema is about.
const (
	AsyncSonarURL     = baseURL + "/api-reference/async-sonar-post.md"
	EmbeddingsPostURL = baseURL + "/api-reference/embeddings-post.md"
	ContextEmbedURL   = baseURL + "/api-reference/contextualized-embeddings-post.md"
	SearchPostURL     = baseURL + "/api-reference/search-post.md"
)

var (
	// requestSchemaRe matches one request schema of an OpenAPI document.
	requestSchemaRe = regexp.MustCompile(
		`(?s)\n {4}(\w*Request):\n(.*?)\n {4}\w`,
	)
	// propertyRe matches the name of one property of a request schema.
	propertyRe = regexp.MustCompile(`(?m)^ {8}([a-z][a-z0-9_]*):$`)
	// minimumRe matches the floor a schema puts on a numeric property. Only
	// the embedding width states one, and states it because the vector may be
	// truncated to anything down to that width rather than only to a listed
	// set of widths.
	minimumRe = regexp.MustCompile(
		`(?s)\n\s+dimensions:\n(?:[^\n]*\n){0,12}?\s+minimum:\s*(\d+)`,
	)
	// batchRe, tokenBatchRe and chunkRe match the bounds an embedding request
	// schema states in the prose describing its input rather than as numbers
	// of their own.
	batchRe      = regexp.MustCompile(`(?i)maximum (\d+) (?:texts|documents)`)
	tokenBatchRe = regexp.MustCompile(
		`(?i)must not exceed ([\d,]+) tokens combined`,
	)
	chunkRe = regexp.MustCompile(
		`(?i)chunks across all documents must not exceed ([\d,]+)`,
	)
	// spaceRe collapses the line wrapping a YAML block scalar imposes on the
	// prose inside it.
	spaceRe = regexp.MustCompile(`\s+`)
)

// enumeratedLists are the request properties whose enumerated values say
// something about the model rather than about the request: what a search may
// draw on, how hard the model may be asked to think, and how a vector may be
// encoded.
var enumeratedLists = []struct {
	property string
	list     string
}{
	{"search_mode", ListSearchModes},
	{"reasoning_effort", ListReasoningEfforts},
	{"encoding_format", ListEncodingFormats},
}

// applyReference reads one API reference onto the models it serves.
//
// A reference documents an endpoint rather than a model, so everything it
// states is recorded against every model the endpoint takes. Where the schema
// enumerates those models it is believed over the caller, since the schema is
// the endpoint's own account of what it will accept; where it does not, the
// caller passes the set it established elsewhere.
func (b *builder) applyReference(doc catalog.Document, ids []string) {
	body := string(doc.Body)
	targets := ids
	if enumerated := referenceModels(body); len(enumerated) > 0 {
		targets = enumerated
	}
	path, hasPath := referencePath(body)
	params := requestParameters(body)
	ceiling := outputCeiling(body)
	flat := spaceRe.ReplaceAllString(body, " ")
	for _, id := range targets {
		m, ok := b.models[id]
		if !ok {
			continue
		}
		m.AddSource(doc.URL)
		if hasPath {
			m.AddList(ListEndpoints, path)
		}
		m.AddList(ListParameters, params...)
		m.SetLimit(LimitMaxOutputTokens, ceiling)
		for _, list := range enumeratedLists {
			m.AddList(list.list, enumValues(body, list.property)...)
		}
		m.SetLimit(LimitMinEmbeddingDimension, firstNumber(minimumRe, body))
		m.SetLimit(LimitMaxInputsPerRequest, firstNumber(batchRe, flat))
		m.SetLimit(LimitMaxTokensPerRequest, firstNumber(tokenBatchRe, flat))
		m.SetLimit(LimitMaxChunksPerRequest, firstNumber(chunkRe, flat))
	}
}

// addEndpoint records an endpoint against every model given, for the case
// where a reference names no model and the endpoint is the only thing it has
// to say about the set that answers on it.
func (b *builder) addEndpoint(ids []string, path, source string) {
	for _, id := range ids {
		m, ok := b.models[id]
		if !ok {
			continue
		}
		m.AddSource(source)
		m.AddList(ListEndpoints, path)
	}
}

// requestParameters returns the properties an endpoint's request body takes.
// A schema wrapping another one, as the asynchronous endpoint wraps the
// synchronous one, is skipped: its two properties describe the envelope and
// not the request the caller is making.
func requestParameters(body string) []string {
	var out []string
	for _, match := range requestSchemaRe.FindAllStringSubmatch(body, -1) {
		if strings.HasPrefix(match[1], "Async") {
			continue
		}
		schema := "\n" + match[2]
		at := strings.Index(schema, "\n      properties:\n")
		if at < 0 {
			continue
		}
		for _, prop := range propertyRe.FindAllStringSubmatch(
			schema[at:],
			-1,
		) {
			out = append(out, prop[1])
		}
	}
	return out
}

// enumValues returns the values a request schema allows for one property.
func enumValues(body, property string) []string {
	re, err := regexp.Compile(
		`(?s)\n\s+` + property +
			`:\n(?:[^\n]*\n){0,12}?\s+enum:\n((?:\s+-\s+[^\n]+\n)+)`,
	)
	if err != nil {
		return nil
	}
	match := re.FindStringSubmatch(body)
	if match == nil {
		return nil
	}
	return enumList(match[1])
}

// enumList splits the block of a YAML enumeration into its values. A value
// that is not a bare token is dropped: Perplexity writes an optional
// enumeration as one branch of an alternation, and the branch admitting no
// value at all sits at the same indent as the values themselves.
func enumList(block string) []string {
	var out []string
	for _, line := range strings.Split(block, "\n") {
		value := strings.TrimSpace(
			strings.TrimPrefix(strings.TrimSpace(line), "-"),
		)
		if enumValueRe.MatchString(value) {
			out = append(out, value)
		}
	}
	return out
}

// enumValueRe matches a bare enumerated value.
var enumValueRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)

// firstNumber returns the quantity a regexp captures, or zero.
func firstNumber(re *regexp.Regexp, body string) int64 {
	match := re.FindStringSubmatch(body)
	if match == nil {
		return 0
	}
	value, err := strconv.ParseInt(
		strings.ReplaceAll(match[1], ",", ""),
		10,
		64,
	)
	if err != nil {
		return 0
	}
	return value
}
