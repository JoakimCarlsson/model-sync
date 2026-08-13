package fireworks

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Scalar keys the model pages populate.
const (
	AttrSummary       = "summary"
	AttrHuggingFaceID = "hugging_face_id"
)

// LimitContextWindow is the bound a model page states, which the pricing page
// does not.
const LimitContextWindow = "context_window"

// Enumeration keys the model pages populate.
const (
	ListFeatures         = catalog.ListFeatures
	ListInputModalities  = "input_modalities"
	ListOutputModalities = "output_modalities"
)

// FeatureFunctionCalling is the one capability a model page states as a flag.
const FeatureFunctionCalling = "function_calling"

// Fields of the model record a page carries.
//
// The record is JSON embedded in the page as a string, so its quotes arrive
// escaped, and how many times depends on how deeply the page nested it. The
// escaping is therefore matched rather than undone.
var (
	contextRe = fieldRe(`contextLength`, `(\d+)`)
	imageRe   = fieldRe(`supportsImageInput`, `(true|false)`)
	toolsRe   = fieldRe(`supportsTools`, `(true|false)`)
	nameRe    = fieldRe(`displayName`, `\\*"(.*?[^\\])\\*"`)
	summaryRe = fieldRe(`description`, `\\*"(.*?[^\\])\\*"`)
	huggingRe = fieldRe(`huggingFaceUrl`, `\\*"(.*?[^\\])\\*"`)
)

// fieldRe matches one field of the embedded record, whatever its escaping.
func fieldRe(name, value string) *regexp.Regexp {
	return regexp.MustCompile(`\\*"` + name + `\\*":\s*` + value)
}

// applyModelPage reads one model's page onto the model the pricing page
// established for it.
//
// The pricing page links every row to the page of the model it prices, and
// several rows link to the same page: a model served three ways is one model
// with three rates. The link is therefore the join, and needs no matching on
// names.
func (b *builder) applyModelPage(doc catalog.Document) {
	m, ok := b.byURL(doc.URL)
	if !ok {
		return
	}
	body := string(doc.Body)
	m.AddSource(doc.URL)
	if m.Name == "" {
		m.Name = unescape(first(nameRe, body))
	}
	m.SetAttr(AttrSummary, unescape(first(summaryRe, body)))
	m.SetAttr(
		AttrHuggingFaceID,
		huggingFaceID(unescape(first(huggingRe, body))),
	)
	if n, err := strconv.ParseInt(first(contextRe, body), 10, 64); err == nil {
		m.SetLimit(LimitContextWindow, n)
	}
	m.AddList(ListInputModalities, "text")
	m.AddList(ListOutputModalities, "text")
	if first(imageRe, body) == "true" {
		m.AddList(ListInputModalities, "image")
	}
	if first(toolsRe, body) == "true" {
		m.AddList(ListFeatures, FeatureFunctionCalling)
	}
}

// byURL returns the model whose page is at url.
func (b *builder) byURL(url string) (*catalog.Model, bool) {
	for _, id := range b.order {
		if b.models[id].Attrs[AttrModelURL] == url {
			return b.models[id], true
		}
	}
	return nil, false
}

// huggingFaceID reduces the address of a model's weights to the identifier
// they are published under, which is what other providers record.
func huggingFaceID(url string) string {
	_, id, ok := strings.Cut(url, "huggingface.co/")
	if !ok {
		return ""
	}
	return strings.Trim(id, "/")
}

// unescape undoes the backslashes the embedded record's own encoding added.
func unescape(value string) string {
	r := strings.NewReplacer(`\\"`, `"`, `\"`, `"`, `\\n`, " ", `\\`, `\`)
	return strings.TrimSpace(r.Replace(value))
}

// first returns the first capture of re, or the empty string.
func first(re *regexp.Regexp, body string) string {
	if match := re.FindStringSubmatch(body); match != nil {
		return match[1]
	}
	return ""
}
