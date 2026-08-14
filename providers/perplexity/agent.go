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

// reasoningClaimRe matches prose calling a model a reasoning model or saying
// what reasoning effort it takes.
var reasoningClaimRe = regexp.MustCompile(`(?i)reasoning (?:model|effort)`)

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
	}
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
