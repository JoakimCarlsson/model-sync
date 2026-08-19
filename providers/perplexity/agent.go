package perplexity

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Documents describing the Agent API. The model page is the only one naming a
// model; the other two state what the API takes and does, and are read onto
// every model that page listed, because that is exactly the set the API
// serves.
const (
	AgentModelsURL  = baseURL + "/docs/agent-api/models.md"
	AgentOutputURL  = baseURL + "/docs/agent-api/output-control.md"
	AgentRequestURL = baseURL + "/api-reference/agent-post.md"
)

// agentFeatures map a claim the Agent API's output guide makes of every model
// onto the capability it states. Only a claim made of every model is read: the
// model page warns outright that a brokered model may support neither
// reasoning nor tools, so the guide's structured-output section, which says
// which shapes the API accepts and not which models honour them, is left to
// the models that state it for themselves.
var agentFeatures = []struct {
	claim   *regexp.Regexp
	feature string
}{
	{
		regexp.MustCompile(`(?i)streaming is supported across all models`),
		"streaming",
	},
}

// agentContentParts map a content part of the Agent API's request and response
// schema onto the modality it carries. The schema admits one kind of output
// part and two kinds of input part, of which only text is read: an image part
// the endpoint accepts says what the endpoint takes and not what the model
// behind it can see, and the model page is explicit that the models differ.
var agentContentParts = map[string]struct{ in, out string }{
	"input_text":  {in: ModalityText},
	"output_text": {out: ModalityText},
}

var (
	// reasoningClaimRe matches prose calling a model a reasoning model or
	// saying what reasoning effort it takes.
	reasoningClaimRe = regexp.MustCompile(`(?i)reasoning (?:model|effort)`)
	// effortClaimRe matches prose enumerating the reasoning efforts one model
	// takes, which the model page states for one model and no other.
	effortClaimRe = regexp.MustCompile(`(?i)accepts (.+?) reasoning effort`)
	// backtickRe matches one backticked value of such an enumeration.
	backtickRe = regexp.MustCompile("`([a-z]+)`")
	// fastModeRe matches the page's statement that one model is also sold at a
	// multiple of its listed rates on a faster tier. The multiple is not an
	// amount and the page states no amount for it, so it is recorded as the
	// sentence it is stated in.
	fastModeRe = regexp.MustCompile(
		`(?i)supports Fast mode at \S+ the listed token prices`,
	)
)

// applyAgentCards reads the prose of the Agent API's model page. Its tables
// carry rates and a documentation link and nothing else, but the card heading
// a tab, and the notes under one, occasionally say more about a model than the
// row does.
//
// A line is read only where it names exactly one of the models the tables
// listed, so a card summarising a whole family, or one introducing two models
// and describing each differently, states nothing about either.
func (b *builder) applyAgentCards(doc catalog.Document) {
	for _, raw := range strings.Split(string(doc.Body), "\n") {
		line := clean(raw)
		m, ok := b.lineModel(line)
		if !ok {
			continue
		}
		if reasoningClaimRe.MatchString(line) {
			m.AddSource(doc.URL)
			m.AddList(ListFeatures, featureReasoning)
		}
		if match := contextRe.FindStringSubmatch(line); match != nil {
			m.AddSource(doc.URL)
			m.SetLimit(LimitContextWindow, parseTokens(match[1], match[2]))
		}
		if match := effortClaimRe.FindStringSubmatch(raw); match != nil {
			m.AddSource(doc.URL)
			m.AddList(ListReasoningEfforts, backtickValues(match[1])...)
		}
	}
	b.applyFastMode(doc)
}

// backtickValues returns the backticked values of an enumeration written in
// prose.
func backtickValues(text string) []string {
	var out []string
	for _, match := range backtickRe.FindAllStringSubmatch(text, -1) {
		out = append(out, match[1])
	}
	return out
}

// applyFastMode records the model the page says is also sold on a faster tier.
// The claim names one model, and names it in full, so the model it is about is
// the one with the longest identifier the sentence contains rather than the
// only one it contains: every shorter identifier of the same family is
// contained in the longer one.
func (b *builder) applyFastMode(doc catalog.Document) {
	for _, raw := range strings.Split(string(doc.Body), "\n") {
		line := clean(raw)
		if !fastModeRe.MatchString(line) {
			continue
		}
		m, ok := b.longestAgentModel(line)
		if !ok {
			continue
		}
		m.AddSource(doc.URL)
		m.SetAttr(AttrPriorityPricing, line)
	}
}

// longestAgentModel returns the brokered model a line names, which is the one
// whose identifier is the longest the line contains.
func (b *builder) longestAgentModel(line string) (*catalog.Model, bool) {
	slug := slugID(line)
	var found string
	for _, id := range b.agent {
		bare := bareID(id)
		if strings.Contains(slug, bare) && len(bare) > len(found) {
			found = bare
			continue
		}
	}
	if found == "" {
		return nil, false
	}
	for _, id := range b.agent {
		if bareID(id) == found {
			return b.models[id], true
		}
	}
	return nil, false
}

// lineModel returns the model a line of prose is about, and reports false
// unless the line names exactly one. A model is named by the part of its
// identifier after the namespace, since the prose writes the model's name and
// not the identifier the API answers to.
func (b *builder) lineModel(line string) (*catalog.Model, bool) {
	slug := slugID(line)
	var found *catalog.Model
	for _, id := range b.agent {
		if !strings.Contains(slug, bareID(id)) {
			continue
		}
		if found != nil {
			return nil, false
		}
		found = b.models[id]
	}
	return found, found != nil
}

// applyAgentGuide reads a guide stating what the Agent API does onto every
// model it serves.
func (b *builder) applyAgentGuide(doc catalog.Document) {
	body := string(doc.Body)
	for _, claim := range agentFeatures {
		if !claim.claim.MatchString(body) {
			continue
		}
		for _, id := range b.agent {
			m := b.models[id]
			m.AddSource(doc.URL)
			m.AddList(ListFeatures, claim.feature)
		}
	}
}

// applyAgentSchema reads what the Agent API takes and returns off its request
// reference, which states it as the content parts a message may hold.
func (b *builder) applyAgentSchema(doc catalog.Document) {
	body := string(doc.Body)
	for part, media := range agentContentParts {
		if !strings.Contains(body, part) {
			continue
		}
		for _, id := range b.agent {
			m := b.models[id]
			m.AddSource(doc.URL)
			m.AddList(ListInputModalities, media.in)
			m.AddList(ListOutputModalities, media.out)
		}
	}
}

// bareID returns an identifier without the namespace Perplexity files it
// under.
func bareID(id string) string {
	_, bare, ok := strings.Cut(id, "/")
	if !ok {
		return id
	}
	return bare
}
