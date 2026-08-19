package perplexity

import (
	"regexp"
	"slices"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// RouterModelsURL is the Router API's model catalog. It is the second rate
// card Perplexity publishes for the models it hosts itself, and the only
// document saying that it hosts them: the prefix those models are filed under
// says who serves them and not who made them, which this page states outright.
const RouterModelsURL = baseURL + "/docs/gateway/models.md"

// Router endpoint references. Neither enumerates the models it takes, so the
// paths they document are recorded against the models the Router catalog
// listed, which is the set the catalog page calls the Router's allowlist.
const (
	RouterChatURL     = baseURL + "/api-reference/gateway-chat-completions-post.md"
	RouterMessagesURL = baseURL + "/api-reference/gateway-messages-post.md"
)

// routerColumns are the rate columns of the Router catalog, whose headings
// state the denominator and whose cells hold an amount and nothing else.
var routerColumns = []struct {
	header string
	metric catalog.Metric
}{
	{"input", MetricInputTokens},
	{"output", MetricOutputTokens},
	{"cache read", MetricCachedInputTokens},
}

// openWeightRe matches the Router catalog's claim about the models it lists,
// which it makes of the table as a whole rather than of any row.
var openWeightRe = regexp.MustCompile(
	`(?i)open-(?:source|weight) models hosted by Perplexity`,
)

// applyRouterCatalog reads the Router API's rate card.
//
// Its models are the ones already named by the Agent API's model page, so
// nothing new is created here. What the page adds is a second set of amounts
// for the same models, which is why every rate it states carries the API it
// bills: the two cards agree on most rows and disagree on at least one cache
// read, and an amount without that dimension would not say which of the two it
// is. The page also states, of the whole table, that Perplexity hosts these
// models and that they are open weight, which is the only place either is
// said.
func (b *builder) applyRouterCatalog(doc catalog.Document) {
	hosted := openWeightRe.MatchString(string(doc.Body))
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		if columnOf(t.Headers, "cache read") < 0 {
			continue
		}
		for _, row := range t.Rows {
			m, ok := b.rowModel(t, row, KindChat)
			if !ok {
				continue
			}
			b.router = appendUnique(b.router, m.ID)
			if hosted {
				m.SetAttr(AttrHostedBy, providerID)
				m.SetAttr(AttrOpenWeights, "true")
			}
			b.applyRouterRates(m, t, row)
		}
	}
}

// applyRouterRates reads one row of the Router catalog.
func (b *builder) applyRouterRates(
	m *catalog.Model,
	t table,
	row []string,
) {
	for _, col := range routerColumns {
		at := columnOf(t.Headers, col.header)
		if at < 0 {
			continue
		}
		amount, ok := parseAmount(cellAt(row, at))
		if !ok {
			continue
		}
		m.AddPrice(catalog.Price{
			Metric:   col.metric,
			Unit:     UnitPer1MTokens,
			Amount:   amount,
			Currency: currency,
			Dims:     catalog.Dims{}.With(DimAPI, APIRouter),
		})
	}
}

// applyRouterReference records a Router endpoint against every model the
// Router catalog listed.
func (b *builder) applyRouterReference(doc catalog.Document) {
	path, ok := referencePath(string(doc.Body))
	if !ok {
		return
	}
	for _, id := range b.router {
		m := b.models[id]
		m.AddSource(doc.URL)
		m.AddList(ListEndpoints, path)
	}
}

// appendUnique adds id to ids unless it is already there.
func appendUnique(ids []string, id string) []string {
	if slices.Contains(ids, id) {
		return ids
	}
	return append(ids, id)
}

// operationRe matches the line an API reference opens its schema with, which
// names the method and the path the schema describes.
var operationRe = regexp.MustCompile(
	"(?m)^" + "`{3,}" + `yaml\s+(?:get|post|put|delete)\s+(\S+)\s*$`,
)

// referencePath returns the endpoint path an API reference documents.
func referencePath(body string) (string, bool) {
	match := operationRe.FindStringSubmatch(body)
	if match == nil {
		return "", false
	}
	return strings.TrimSpace(match[1]), true
}
