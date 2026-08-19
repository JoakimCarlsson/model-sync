package assemblyai

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// GatewayModelsURL is the only document naming the models AssemblyAI resells
// through its LLM Gateway together with the identifier a request selects one
// by. The pricing page sells the same models under display names alone.
const GatewayModelsURL = "https://www.assemblyai.com/docs/llm-gateway/" +
	"available-models.md"

// GatewaySpecURL is the OpenAPI reference for the chat completions endpoint,
// read for the two things the model tables leave unsaid: what a message may
// carry, and where the endpoint lives.
const GatewaySpecURL = "https://www.assemblyai.com/docs/llm-gateway/" +
	"api-reference/create-chat-completion.md"

// The two shapes of table the models page carries, told apart by the heading
// of their second column: one states what a model is, the other what it costs.
const (
	gatewayHeadRoster  = "id"
	gatewayHeadPricing = "parameter"
)

// textContentPhrase is the sentence in the specification that bounds a message
// to text. It is matched rather than assumed, so that the day AssemblyAI
// admits an image content part this stops calling a model text only.
const textContentPhrase = "only supports text content parts"

// Column headings of the rate table, matched by prefix because each carries
// the denominator in parentheses after the name.
const (
	gatewayPrompt      = "prompt"
	gatewayCompletion  = "completion"
	gatewayCacheRead   = "cache read"
	gatewayCacheWrite  = "cache write"
	gatewayCacheWrite1 = "cache write 1h"
	gatewaySurcharge   = "regional surcharge"
)

// cacheTTLHour is the lifetime the second cache write column is quoted for.
// The first column states no lifetime and is recorded without one.
const cacheTTLHour = "1h"

// gatewayParameters map a request parameter the table lists onto the
// capability accepting it states. AssemblyAI publishes no capability list for
// these models: it publishes, per model, which parameters that model takes,
// and a model taking tools is a model that calls them.
var gatewayParameters = map[string]string{
	"tools":           FeatureFunctionCalling,
	"tool_choice":     FeatureFunctionCalling,
	"response_format": FeatureStructuredOutputs,
	"stream":          FeatureStreaming,
}

var (
	// serverRe matches the production server the specification declares.
	serverRe = regexp.MustCompile(`(?m)^\s*-\s*url:\s*(https://\S+)`)
	// pathRe matches the path the specification hangs the operation on.
	pathRe = regexp.MustCompile(`(?m)^\s{2,}(/[a-z/]+):\s*$`)
)

// applyGateway reads the LLM Gateway models page, which is two tables over the
// same models: one stating what each is, one stating what each costs. Both are
// keyed by the identifier rather than by the display name, so the two join
// without the name matching the transcription documents need.
func (b *builder) applyGateway(doc catalog.Document) {
	for _, table := range pipeTables(string(doc.Body)) {
		if len(table) < 2 {
			continue
		}
		heads := table[0]
		if !strings.EqualFold(clean(cellAt(heads, 0)), "model") {
			continue
		}
		switch strings.ToLower(clean(cellAt(heads, 1))) {
		case gatewayHeadRoster:
			for _, row := range table[1:] {
				b.applyGatewayModel(row, heads, doc.URL)
			}
		case gatewayHeadPricing:
			for _, row := range table[1:] {
				b.applyGatewayRates(row, heads, doc.URL)
			}
		}
	}
}

// applyGatewayModel records one row of the roster table.
func (b *builder) applyGatewayModel(row, heads []string, source string) {
	m, ok := b.gatewayModel(row, source)
	if !ok {
		return
	}
	if m.Name == "" {
		m.Name = clean(cellAt(row, 0))
	}
	for i, head := range heads {
		value := clean(cellAt(row, i))
		if value == "" {
			continue
		}
		switch strings.ToLower(clean(head)) {
		case "supported parameters":
			applyGatewayParameters(m, value)
		case "max context":
			m.SetLimit(LimitContextWindow, count(value))
		case "retirement date":
			m.SetAttr(AttrRetirementDate, value)
		}
	}
}

// applyGatewayParameters records the parameters one model accepts and the
// capabilities accepting them states.
func applyGatewayParameters(m *catalog.Model, cell string) {
	for _, part := range strings.Split(cell, ",") {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		m.AddList(ListParameters, name)
		m.AddList(ListFeatures, gatewayParameters[name])
	}
}

// applyGatewayRates records one row of the rate table. Every column is a rate
// against the same million tokens, so what separates them is the metric, and
// the two cache write columns differ only in the lifetime they buy.
func (b *builder) applyGatewayRates(row, heads []string, source string) {
	m, ok := b.gatewayModel(row, source)
	if !ok {
		return
	}
	for i, head := range heads {
		cell := clean(cellAt(row, i))
		if cell == "" {
			continue
		}
		label := strings.ToLower(clean(head))
		if strings.HasPrefix(label, gatewaySurcharge) {
			m.SetAttr(AttrRegionalSurcharge, cell)
			continue
		}
		metric, dims, ok := gatewayMetric(label)
		if !ok {
			continue
		}
		amount, ok := parseAmount(cell)
		if !ok {
			continue
		}
		m.AddPrice(catalog.Price{
			Metric:   metric,
			Unit:     UnitPer1MTokens,
			Amount:   amount,
			Currency: currency,
			Dims:     dims,
		})
	}
}

// gatewayMetric reads a rate column heading. The cache write columns are
// tested longest first, since one heading is a prefix of the other.
func gatewayMetric(label string) (catalog.Metric, catalog.Dims, bool) {
	switch {
	case strings.HasPrefix(label, gatewayCacheWrite1):
		return MetricCacheWriteTokens,
			catalog.Dims{DimCacheTTL: cacheTTLHour}, true
	case strings.HasPrefix(label, gatewayCacheWrite):
		return MetricCacheWriteTokens, nil, true
	case strings.HasPrefix(label, gatewayCacheRead):
		return MetricCachedInputTokens, nil, true
	case strings.HasPrefix(label, gatewayPrompt):
		return MetricInputTokens, nil, true
	case strings.HasPrefix(label, gatewayCompletion):
		return MetricOutputTokens, nil, true
	}
	return "", nil, false
}

// gatewayModel returns the entry for the identifier a row states, which both
// tables carry in their second column. A row without one is not a model.
func (b *builder) gatewayModel(
	row []string,
	source string,
) (*catalog.Model, bool) {
	id := clean(cellAt(row, 1))
	if id == "" || strings.Contains(id, " ") {
		return nil, false
	}
	m := b.model(id, KindChat)
	m.AddSource(source)
	m.SetAttr(AttrAPIIdentifier, id)
	return m, true
}

// applyGatewaySpec records what the chat completions specification states of
// every model reached through it: that a message carries text and comes back
// as text, and the endpoint that carries it.
func (b *builder) applyGatewaySpec(doc catalog.Document) {
	body := string(doc.Body)
	if !strings.Contains(body, textContentPhrase) {
		return
	}
	endpoint := gatewayEndpoint(body)
	for _, id := range b.order {
		m := b.models[id]
		if m.Kind != KindChat {
			continue
		}
		m.AddSource(doc.URL)
		m.AddList(ListInputModalities, ModalityText)
		m.AddList(ListOutputModalities, ModalityText)
		m.AddList(ListEndpoints, endpoint)
	}
}

// gatewayEndpoint joins the server the specification declares to the path it
// hangs the operation on, which is how the specification states an endpoint.
func gatewayEndpoint(body string) string {
	server := serverRe.FindStringSubmatch(body)
	path := pathRe.FindStringSubmatch(body)
	if server == nil || path == nil {
		return ""
	}
	return strings.TrimSuffix(server[1], "/") + path[1]
}
