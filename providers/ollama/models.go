package ollama

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Kinds of model Ollama distributes.
const (
	KindChat      catalog.Kind = "chat"
	KindEmbedding catalog.Kind = "embedding"
	KindVision    catalog.Kind = "vision"
)

// AttrSummary is the description Ollama gives a model.
//
// The download count shown beside it is deliberately not recorded. It rises
// continuously, so committing it would rewrite every model's file on most
// syncs for a reason that has nothing to do with the model, burying real
// changes in churn.
const AttrSummary = "summary"

// Enumeration keys the library populates.
const (
	ListCapabilities   = "capabilities"
	ListParameterSizes = "parameter_sizes"
)

// capabilityKinds maps a capability tag onto the kind it implies. A model
// tagged as an embedder is not a chat model however it is otherwise
// described.
var capabilityKinds = map[string]catalog.Kind{
	"embedding": KindEmbedding,
	"vision":    KindVision,
}

var (
	entryRe = regexp.MustCompile(
		`(?is)<li[^>]*>\s*<a[^>]+href="/library/([^"]+)"(.*?)</li>`,
	)
	summaryRe = regexp.MustCompile(`(?is)<p[^>]*text-md[^>]*>(.*?)</p>`)
	tagRe     = regexp.MustCompile(
		`(?is)<span[^>]*(?:text-indigo-600|text-blue-600)[^>]*>(.*?)</span>`,
	)
	markupRe = regexp.MustCompile(`(?s)<[^>]*>`)
	// sizeRe matches a tag that states a parameter count rather than a
	// capability, as in "8b" or "1.5b".
	sizeRe = regexp.MustCompile(`(?i)^\d+(\.\d+)?[bm]$`)
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

// applyTag records one tag as either a size the model comes in or something it
// can do.
func (b *builder) applyTag(m *catalog.Model, tag string) {
	if tag == "" {
		return
	}
	if sizeRe.MatchString(tag) {
		m.AddList(ListParameterSizes, strings.ToLower(tag))
		return
	}
	capability := strings.ToLower(tag)
	m.AddList(ListCapabilities, capability)
	if kind, ok := capabilityKinds[capability]; ok {
		m.Kind = kind
	}
}
