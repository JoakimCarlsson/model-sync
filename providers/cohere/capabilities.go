package cohere

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Guides Cohere states a capability in. Neither the overview nor the pricing
// page has a capability column, so what a model can do is stated only in the
// guide to the capability itself.
const (
	// StructuredOutputsURL names the models that can be constrained to a
	// shape, which it lists outright.
	StructuredOutputsURL = "https://docs.cohere.com/v2/docs/structured-outputs.md"
	// ToolUseURL states which models can call tools, which it does by naming a
	// family rather than by listing its members.
	ToolUseURL = "https://docs.cohere.com/v2/docs/tool-use-overview.md"
)

// familyCommand is the family the tool use guide names.
const familyCommand = "command"

// endpointChat is the endpoint the tool use guide states tools are called
// through, as the overview's endpoint column writes it.
const endpointChat = "Chat"

var (
	// compatibleRe matches the list of models a guide opens by declaring
	// itself compatible with, which is a run of bullets under a heading of its
	// own.
	compatibleRe = regexp.MustCompile(
		`(?i)compatible models:\s*\n\n((?:\*[^\n]*\n)+)`,
	)
	// bulletRe matches one model of that list.
	bulletRe = regexp.MustCompile(`(?m)^\*\s*(.+?)\s*$`)
	// commandFamilyRe matches the sentence the tool use guide states its own
	// scope in. Cohere names the family rather than its members, so this is
	// matched rather than assumed: a guide rewritten to name something
	// narrower stops yielding the capability instead of going on claiming it
	// for nine models.
	commandFamilyRe = regexp.MustCompile(
		`(?i)tool use is a technique which allows developers to connect ` +
			`Cohere.s Command family of models to external tools`,
	)
)

// applyStructuredOutputs records the capability against each model the guide
// lists.
//
// The guide names products, the same way the rate cards do, and the same table
// resolves them. It documents two strengths of the capability, a mode that only
// requires JSON and a mode that holds the answer to a caller's schema, and
// states one list of models for both, so every model on it carries both values.
func (b *builder) applyStructuredOutputs(doc catalog.Document) {
	list := compatibleRe.FindStringSubmatch(string(doc.Body))
	if list == nil {
		return
	}
	for _, bullet := range bulletRe.FindAllStringSubmatch(list[1], -1) {
		for _, id := range b.identify(clean(bullet[1])) {
			m := b.models[id]
			m.AddList(
				ListFeatures,
				catalog.CapabilityStructuredOutputs,
				catalog.CapabilityJSONMode,
			)
			m.AddSource(doc.URL)
		}
	}
}

// applyToolUse records tool calling against the family the guide names.
//
// This is the one capability Cohere states by family rather than by model. The
// guide says tool use connects the Command family to external tools and that
// the Chat endpoint is what carries it, so both halves are required: a Command
// model the overview gives a Chat endpoint gets the capability, and a Command
// model listed only in the table of platform identifiers, which states no
// endpoint, does not.
func (b *builder) applyToolUse(doc catalog.Document) {
	if !commandFamilyRe.Match(doc.Body) {
		return
	}
	for _, m := range b.models {
		if m.Attrs[AttrFamily] != familyCommand {
			continue
		}
		if !strings.Contains(
			strings.Join(m.Lists[ListEndpoints], " "),
			endpointChat,
		) {
			continue
		}
		m.AddList(ListFeatures, catalog.CapabilityFunctionCalling)
		m.AddSource(doc.URL)
	}
}
