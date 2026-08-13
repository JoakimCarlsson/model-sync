package ollama

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Kinds of model Ollama distributes.
const (
	KindChat      catalog.Kind = "chat"
	KindEmbedding catalog.Kind = "embedding"
	KindVision    catalog.Kind = "vision"
)

// Scalar keys the library populates.
const (
	AttrSummary = "summary"
	AttrPulls   = "pulls"
)

// LimitPulls is the download count as a number.
const LimitPulls = "pulls"

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
	// pullsRe matches the download count, which sits in a bare span ahead of
	// a separate span holding the word itself.
	pullsRe = regexp.MustCompile(
		`(?is)<span\s*>\s*([\d.,]+[KMB]?)\s*</span>\s*<span[^>]*>\s*(?:&nbsp;|\s)*Pulls`,
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
		if match := pullsRe.FindStringSubmatch(entry[2]); match != nil {
			m.SetAttr(AttrPulls, strings.TrimSpace(match[1]))
			m.SetLimit(LimitPulls, parsePulls(match[1]))
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

// parsePulls expands Ollama's abbreviated download count, which it writes as
// "1.2M", into a number a reader can sort on.
func parsePulls(value string) int64 {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0
	}
	multiplier := 1.0
	switch strings.ToUpper(trimmed[len(trimmed)-1:]) {
	case "K":
		multiplier, trimmed = 1_000, trimmed[:len(trimmed)-1]
	case "M":
		multiplier, trimmed = 1_000_000, trimmed[:len(trimmed)-1]
	case "B":
		multiplier, trimmed = 1_000_000_000, trimmed[:len(trimmed)-1]
	}
	n, err := strconv.ParseFloat(strings.ReplaceAll(trimmed, ",", ""), 64)
	if err != nil {
		return 0
	}
	return int64(n * multiplier)
}
