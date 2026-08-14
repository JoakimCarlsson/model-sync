package ollama

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Kinds of model Ollama distributes. A model that reads images is a chat model
// that takes an image, which is what its modalities say, rather than a kind of
// its own.
const (
	KindChat      catalog.Kind = "chat"
	KindEmbedding catalog.Kind = "embedding"
)

// AttrCloud marks a model Ollama also runs on its own hardware. The library
// tags it in a colour of its own, beside the capabilities, and it is the one
// tag that says where a model runs rather than what it can do, which is why it
// is an attribute and not a feature. It is also what tells the priced models
// apart from the free ones: a rate exists only where Ollama does the running.
const AttrCloud = "cloud"

// AttrUsageLevel is how much of a plan's allowance a cloud model draws, which
// Ollama states in words on the model's page, from "low" to "extra high". It
// is the only thing most cloud models say about what they cost.
const AttrUsageLevel = "usage_level"

// AttrUsageRank is the same statement as a number. The page draws it as four
// bars with as many filled as the level, and Ollama's pricing page numbers
// them one to four, so both the word and the number are recorded.
const AttrUsageRank = "usage_level_rank"

// AttrSummary is the description Ollama gives a model.
//
// The download count shown beside it is deliberately not recorded. It rises
// continuously, so committing it would rewrite every model's file on most
// syncs for a reason that has nothing to do with the model, burying real
// changes in churn.
const AttrSummary = "summary"

// Enumeration keys the library and the tag listings populate.
const (
	ListFeatures         = catalog.ListFeatures
	ListParameterSizes   = "parameter_sizes"
	ListInputModalities  = "input_modalities"
	ListOutputModalities = "output_modalities"
)

// capabilityKinds maps a capability tag onto the kind it implies. A model
// tagged as an embedder is not a chat model however it is otherwise
// described.
var capabilityKinds = map[string]catalog.Kind{
	"embedding": KindEmbedding,
}

// capabilityFeatures map a capability tag onto the catalog's vocabulary.
// Ollama's own words for these are shared with no other provider.
var capabilityFeatures = map[string]string{
	"tools":    catalog.CapabilityFunctionCalling,
	"thinking": catalog.CapabilityReasoning,
	"insert":   "fill_in_the_middle",
}

// schemaRe matches the sentence stating what structured outputs do, which is
// the whole of what Ollama says about which models have them: the capability
// belongs to the runtime, so the sentence names none. It is matched rather
// than assumed, so a page rewritten to name particular models stops yielding
// the capability for all of them.
var schemaRe = regexp.MustCompile(
	`(?i)Structured outputs let you enforce a JSON schema on model responses`,
)

// applyStructuredOutputs records the capability against every model that
// generates a response.
//
// The library tags each model with what it can do and has no tag for this, and
// the reason is that it would be the same tag on every one of them: Ollama
// constrains the decoding itself, so a schema holds for whatever model is
// loaded. That makes its scope the models Ollama generates with. An embedding
// model returns a vector, which no schema describes, and is left alone.
func (b *builder) applyStructuredOutputs(doc catalog.Document) {
	if !schemaRe.Match(doc.Body) {
		return
	}
	for _, id := range b.order {
		m := b.models[id]
		if m.Kind == KindEmbedding {
			continue
		}
		m.AddList(ListFeatures, catalog.CapabilityStructuredOutputs)
		m.AddSource(doc.URL)
	}
}

// capabilityModalities map a capability tag onto the modality it really names.
// A model tagged for vision does not have a vision feature; it takes an image,
// which is what every provider stating modalities says instead.
var capabilityModalities = map[string]string{
	"vision": "image",
	"audio":  "audio",
}

// modalityNames map the wording of a tag listing's input column onto the
// catalog's vocabulary.
var modalityNames = map[string]string{
	"text":  "text",
	"image": "image",
	"audio": "audio",
	"video": "video",
}

// addModality records one modality under key, ignoring a word the listing uses
// that names no modality.
func addModality(m *catalog.Model, key, value string) {
	if name, ok := modalityNames[strings.ToLower(strings.TrimSpace(value))]; ok {
		m.AddList(key, name)
	}
}

var (
	entryRe = regexp.MustCompile(
		`(?is)<li[^>]*>\s*<a[^>]+href="/library/([^"]+)"(.*?)</li>`,
	)
	summaryRe = regexp.MustCompile(`(?is)<p[^>]*text-md[^>]*>(.*?)</p>`)
	tagRe     = regexp.MustCompile(
		`(?is)<span[^>]*(?:text-indigo-600|text-blue-600|text-cyan-500)` +
			`[^>]*>(.*?)</span>`,
	)
	markupRe = regexp.MustCompile(`(?s)<[^>]*>`)
	// sizeRe matches a tag that states a parameter count rather than a
	// capability, as in "8b" or "1.5b". A mixture of experts states its count
	// as a product, "8x7b", and a model shipped at an effective size prefixes
	// it, "e4b"; both are sizes and neither is anything a model can do.
	sizeRe = regexp.MustCompile(`(?i)^(e|\d+(\.\d+)?x)?\d+(\.\d+)?[bm]$`)
)

// text strips markup and collapses whitespace.
func text(html string) string {
	return strings.Join(
		strings.Fields(markupRe.ReplaceAllString(html, " ")),
		" ",
	)
}

// applyLibrary reads the model library page.
func (b *builder) applyLibrary(doc catalog.Document) {
	for _, entry := range entryRe.FindAllStringSubmatch(string(doc.Body), -1) {
		id := strings.TrimSpace(entry[1])
		if id == "" {
			continue
		}
		m := b.model(id, KindChat)
		m.AddSource(doc.URL)
		if match := summaryRe.FindStringSubmatch(entry[2]); match != nil {
			m.SetAttr(AttrSummary, text(match[1]))
		}
		for _, tag := range tagRe.FindAllStringSubmatch(entry[2], -1) {
			b.applyTag(m, text(tag[1]))
		}
	}
}

// applyTag records one tag as a size the model comes in, where it runs, or
// something it can do.
//
// A capability is translated rather than kept: Ollama's words for these are
// shared with no other provider, and two of them name a modality rather than a
// feature. Anything unrecognized keeps Ollama's own word, since inventing a
// translation would lose which capability it was.
func (b *builder) applyTag(m *catalog.Model, tag string) {
	if tag == "" {
		return
	}
	if sizeRe.MatchString(tag) {
		m.AddList(ListParameterSizes, strings.ToLower(tag))
		return
	}
	capability := strings.ToLower(tag)
	if capability == AttrCloud {
		m.SetAttr(AttrCloud, "true")
		return
	}
	if kind, ok := capabilityKinds[capability]; ok {
		m.Kind = kind
		return
	}
	if modality, ok := capabilityModalities[capability]; ok {
		m.AddList(ListInputModalities, modality)
		return
	}
	if feature, ok := capabilityFeatures[capability]; ok {
		m.AddList(ListFeatures, feature)
		return
	}
	m.AddList(ListFeatures, capability)
}
