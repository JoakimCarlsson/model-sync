package anthropic

import (
	"regexp"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Pages stating a capability the comparison table has no row for. That table
// covers thinking and nothing else, so every other capability a consumer
// derives a boolean from is stated only in the guide to the capability.
const (
	// StructuredOutputsURL opens with a compatibility block listing the
	// identifiers it supports, which is the shape four of these guides share
	// and which applyCompatibility reads.
	StructuredOutputsURL = baseURL + "/build-with-claude/structured-outputs.md"
	// ToolUseURL states tool calling for Claude rather than for a list of
	// models.
	ToolUseURL = baseURL + "/agents-and-tools/tool-use/overview.md"
)

var (
	// supportedModelsRe matches the compatibility line a guide opens with,
	// which names every identifier the capability is available on. It is the
	// one place Anthropic states a capability per model outside the comparison
	// table, and it states it precisely: API identifiers rather than display
	// names, so nothing has to be resolved.
	supportedModelsRe = regexp.MustCompile(`(?m)^-\s*Supported models:\s*(.+)$`)
	// backtickedRe matches one identifier of that line.
	backtickedRe = regexp.MustCompile("`([^`]+)`")
	// toolUseRe matches the sentence the tool use page opens by stating its
	// scope in. Anthropic states this of Claude rather than of a list of
	// models, so it is matched rather than assumed: a page rewritten to name
	// particular models stops yielding the capability for all of them.
	toolUseRe = regexp.MustCompile(
		`(?i)Tool use lets Claude call functions that you define`,
	)
)

// applyToolUse records tool calling against every model still served.
//
// Anthropic publishes no per-model list for it. What it publishes is a
// statement about Claude: tool use lets Claude call the functions a caller
// defines, and the page carries no exception for any model. Its scope is
// therefore the same as the modality sentence's, every chat model the
// documents name that has not retired, and the server-side tools are not
// models and are left alone.
func (b *builder) applyToolUse(doc catalog.Document) {
	if !toolUseRe.Match(doc.Body) {
		return
	}
	for _, id := range b.order {
		m := b.models[id]
		if m.Kind != KindChat || withdrawn(m) {
			continue
		}
		m.AddList(ListFeatures, catalog.CapabilityFunctionCalling)
		m.AddSource(doc.URL)
	}
}
