package voyage

import (
	"encoding/json"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// endpointPages pairs each API reference document with the guide page whose
// tables list the models that endpoint serves. The reference states a bound on
// a request, not on a model, and this is what says whose requests.
var endpointPages = map[string]string{
	refURL + "/embeddings-api.md": baseURL + "/embeddings.md",
	refURL + "/multimodal-embeddings-api.md": baseURL +
		"/multimodal-embeddings.md",
	refURL + "/contextualized-embeddings-api.md": baseURL +
		"/contextualized-chunk-embeddings.md",
	refURL + "/reranker-api.md": baseURL + "/reranker.md",
}

// openAPI is the part of Voyage's definition this package reads: one path, the
// properties of its request body, and the server they are addressed to.
type openAPI struct {
	Servers []struct {
		URL string `json:"url"`
	} `json:"servers"`
	Paths map[string]struct {
		Post struct {
			RequestBody struct {
				Content map[string]struct {
					Schema struct {
						Properties map[string]struct {
							Description string `json:"description"`
							Enum        []any  `json:"enum"`
						} `json:"properties"`
					} `json:"schema"`
				} `json:"content"`
			} `json:"requestBody"`
		} `json:"post"`
	} `json:"paths"`
}

var (
	// jsonBlockRe matches the fenced OpenAPI definition each reference page
	// consists of.
	jsonBlockRe = regexp.MustCompile("(?s)```json\\n(.*?)\\n```")
	// countForRe matches a bound stated once per model, as in "1M for
	// `voyage-4-lite`, `voyage-3.5-lite`; 320K for `voyage-4`". The clause
	// runs to the next semicolon because the models sharing one bound are
	// separated by commas.
	countForRe = regexp.MustCompile(
		"([\\d,.]+ ?[KM]?) (?:tokens )?for ((?:[^;]*?`[^`]+`)+)",
	)
	// listLimitRe matches the cap on how many items one request may carry,
	// which each endpoint states in its own words.
	listLimitRe = regexp.MustCompile(
		`(?i)(?:maximum length of the list is|list must not contain more ` +
			`than|number of documents cannot exceed) ([\d,]+)`,
	)
	// totalTokensRe matches a token budget stated for the endpoint rather
	// than per model.
	totalTokensRe = regexp.MustCompile(
		`(?i)total number of tokens across all inputs must not exceed ` +
			`([\d,]+ ?[KM]?)`,
	)
	// perInputTokensRe matches the ceiling on a single input, which the
	// multimodal endpoint states separately from the budget for the request.
	perInputTokensRe = regexp.MustCompile(
		`(?i)each input in the list must not exceed ([\d,]+ ?[KM]?) tokens`,
	)
	// totalChunksRe matches the cap on chunks in one contextualized request.
	totalChunksRe = regexp.MustCompile(
		`(?i)total number of chunks across all inputs must not exceed ` +
			`([\d,]+ ?[KM]?)`,
	)
	// imageLimitRe matches the size an image may not exceed, in pixels and in
	// megabytes.
	imageLimitRe = regexp.MustCompile(
		`(?i)each image must not contain more than ([\d,]+ ?[a-z]*) pixels ` +
			`or be larger than ([\d,]+) MB`,
	)
	// videoLimitRe matches the size a video may not exceed.
	videoLimitRe = regexp.MustCompile(
		`(?i)each video must not be larger than ([\d,]+) MB`,
	)
	// pixelRateRe matches how many pixels Voyage counts as one token, which
	// differs between a still and a moving picture.
	pixelRateRe = regexp.MustCompile(
		`(?i)every ([\d,]+) pixels of an image and every ([\d,]+) pixels ` +
			`of a video being counted as a token`,
	)
	// dtypeSupportRe matches the sentence naming the models that can return
	// something other than a float, and the types they can return. The list of
	// models runs to the end of the sentence.
	dtypeSupportRe = regexp.MustCompile(
		"(?i)`ubinary` are supported by ([^.]+)",
	)
)

// applyReference reads one endpoint's OpenAPI definition.
//
// Everything here is stated for the endpoint. The parameters it accepts and
// the ceilings on one call belong to every model reachable through it, and the
// per-model exceptions inside those descriptions name their models themselves.
func (b *builder) applyReference(doc catalog.Document) {
	block := jsonBlockRe.FindStringSubmatch(string(doc.Body))
	if block == nil {
		return
	}
	var spec openAPI
	if err := json.Unmarshal([]byte(block[1]), &spec); err != nil {
		return
	}
	served := b.servedBy(endpointPages[doc.URL])
	if len(served) == 0 {
		return
	}
	for _, path := range slices.Sorted(maps.Keys(spec.Paths)) {
		props := requestProperties(spec, path)
		for _, m := range served {
			m.AddSource(doc.URL)
			m.AddList(ListEndpoints, endpointURL(spec, path))
			m.AddList(ListParameters, slices.Sorted(maps.Keys(props))...)
		}
		for _, name := range slices.Sorted(maps.Keys(props)) {
			b.applyParameter(served, name, props[name].Description,
				props[name].Enum)
		}
	}
}

// requestProperties returns the request body properties of one path.
func requestProperties(spec openAPI, path string) map[string]struct {
	Description string `json:"description"`
	Enum        []any  `json:"enum"`
} {
	content := spec.Paths[path].Post.RequestBody.Content
	for _, media := range slices.Sorted(maps.Keys(content)) {
		return content[media].Schema.Properties
	}
	return nil
}

// endpointURL returns the address a path is served at.
func endpointURL(spec openAPI, path string) string {
	if len(spec.Servers) == 0 {
		return path
	}
	return spec.Servers[0].URL + path
}

// applyParameter records what one request parameter says about the models the
// endpoint serves: the bounds in its description, the capability accepting it
// amounts to, and, for the output type, the set of vector encodings on offer.
func (b *builder) applyParameter(
	served []*catalog.Model,
	name, description string,
	enum []any,
) {
	switch name {
	case "input", "inputs", "texts", "documents":
		applyRequestBounds(served, description)
		b.applyPerModel(budgetClause(description), LimitTokensPerReq)
	case "query":
		b.applyPerModel(description, LimitQueryTokens)
	case "input_type":
		addFeature(served, FeatureInputTypes)
	case "truncation":
		addFeature(served, FeatureTruncation)
	case "enable_auto_chunking":
		addFeature(served, FeatureAutoChunking)
	case "output_dtype":
		applyDtypes(served, description, enum)
	}
}

// addFeature records a capability against every model an endpoint serves.
func addFeature(served []*catalog.Model, feature string) {
	for _, m := range served {
		m.AddList(ListFeatures, feature)
	}
}

// applyRequestBounds records the ceilings one request may not exceed, which
// the input parameter of every endpoint states in a list beneath its own
// description.
func applyRequestBounds(served []*catalog.Model, description string) {
	bounds := []struct {
		re  *regexp.Regexp
		key string
	}{
		{listLimitRe, LimitInputsPerReq},
		{totalTokensRe, LimitTokensPerReq},
		{perInputTokensRe, LimitTokensPerInput},
		{totalChunksRe, LimitChunksPerReq},
		{videoLimitRe, LimitVideoMB},
	}
	for _, bound := range bounds {
		match := bound.re.FindStringSubmatch(description)
		if match == nil {
			continue
		}
		for _, m := range served {
			m.SetLimit(bound.key, parseCount(match[1]))
		}
	}
	applyPairedBounds(served, description, imageLimitRe,
		LimitImagePixels, LimitImageMB)
	applyPairedBounds(served, description, pixelRateRe,
		LimitPixelsPerToken, LimitVideoPixels)
}

// applyPairedBounds records a sentence stating two numbers at once.
func applyPairedBounds(
	served []*catalog.Model,
	description string,
	re *regexp.Regexp,
	first, second string,
) {
	match := re.FindStringSubmatch(description)
	if match == nil {
		return
	}
	for _, m := range served {
		m.SetLimit(first, parseCount(match[1]))
		m.SetLimit(second, parseCount(match[2]))
	}
}

// budgetClause returns the part of an input parameter's description that
// states the budget for a whole request.
//
// Two bounds in the same description are stated in the same shape, a number
// followed by the models it holds for, and the other one is the ceiling on a
// query paired with a single document. Reading the description whole would
// take whichever came first, so only the clause naming the total is read.
func budgetClause(description string) string {
	for _, item := range strings.Split(description, "<li>") {
		if strings.Contains(
			strings.ToLower(item),
			"total number of tokens",
		) {
			return item
		}
	}
	return ""
}

// applyPerModel records a bound that varies by model, which Voyage states as a
// number followed by the models it holds for.
func (b *builder) applyPerModel(description, key string) {
	for _, match := range countForRe.FindAllStringSubmatch(description, -1) {
		for _, id := range modelIDs(match[2]) {
			m, ok := b.models[id]
			if !ok {
				continue
			}
			m.SetLimit(key, parseCount(match[1]))
		}
	}
}

// applyDtypes records the encodings a returned vector may take.
//
// The parameter's enumeration is the full set the endpoint offers, and its
// description says which models the narrow ones are available on. Where it
// names them, only those models carry them and the rest carry the float that
// the same sentence says every model supports; where it names none, the whole
// set belongs to every model the endpoint serves.
func applyDtypes(served []*catalog.Model, description string, enum []any) {
	var all []string
	for _, value := range enum {
		if s, ok := value.(string); ok {
			all = append(all, s)
		}
	}
	if len(all) == 0 {
		return
	}
	match := dtypeSupportRe.FindStringSubmatch(description)
	if match == nil {
		for _, m := range served {
			m.AddList(ListOutputDtypes, all...)
			m.AddList(ListFeatures, FeatureQuantizedOutput)
		}
		return
	}
	quantized := modelIDs(match[1])
	for _, m := range served {
		if slices.Contains(quantized, m.ID) {
			m.AddList(ListOutputDtypes, all...)
			m.AddList(ListFeatures, FeatureQuantizedOutput)
			continue
		}
		m.AddList(ListOutputDtypes, dtypeFloat)
	}
}

// dtypeFloat is the encoding Voyage states every model returns.
const dtypeFloat = "float"

// instructionsRe matches the sentence granting a reranker an instruction in
// its query, which Voyage states on the reranker guide rather than in the
// model table.
var instructionsRe = regexp.MustCompile(
	`(?i)for ((?:[^.]*?` + "`" + `[^` + "`" + `]+` + "`" + `)+), ` +
		`optional instructions can be`,
)

// applyInstructions records the rerankers whose relevance can be steered by an
// instruction written into the query.
func (b *builder) applyInstructions(doc catalog.Document) {
	for _, match := range instructionsRe.FindAllStringSubmatch(
		string(doc.Body),
		-1,
	) {
		for _, id := range modelIDs(match[1]) {
			m, ok := b.models[id]
			if !ok {
				continue
			}
			m.AddList(ListFeatures, FeatureInstructions)
			m.AddSource(doc.URL)
		}
	}
}
