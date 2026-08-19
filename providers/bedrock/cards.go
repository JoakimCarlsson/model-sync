package bedrock

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Scalar keys the model cards populate.
const (
	AttrSummary          = "summary"
	AttrState            = "state"
	AttrReleaseDate      = "release_date"
	AttrRetirementDate   = "retirement_date"
	AttrKnowledgeCutoff  = "knowledge_cutoff"
	AttrModelID          = "model_id"
	AttrGlobalProfile    = "global_inference_id"
	AttrMarketplaceID    = "marketplace_product_id"
	AttrFineTuning       = "fine_tuning_supported"
	AttrMaxImagePayload  = "max_image_payload_size"
	AttrSamplingRule     = "sampling_parameters"
	AttrReasoningNote    = "reasoning_note"
	AttrDefaultDimension = "default_embedding_dimension"
)

// Numeric keys the model cards populate.
const (
	LimitContextWindow       = "context_window"
	LimitMaxOutputTokens     = "max_output_tokens"
	LimitMaxInputTokens      = "max_input_tokens"
	LimitCacheMinTokens      = "prompt_cache_min_tokens"
	LimitCacheMaxCheckpoints = "prompt_cache_max_checkpoints"
)

// Enumeration keys the model cards populate.
const (
	ListFeatures         = catalog.ListFeatures
	ListEndpoints        = "endpoints"
	ListAliases          = "aliases"
	ListInputModalities  = "input_modalities"
	ListOutputModalities = "output_modalities"
	ListRegions          = "regions"
	ListInRegion         = "in_region_regions"
	ListGeoRegions       = "geo_regions"
	ListGlobalRegions    = "global_regions"
	ListProfiles         = "inference_profile_ids"
	ListServiceTiers     = "service_tiers"
	ListCacheTTLs        = "prompt_cache_ttls"
	ListCacheFields      = "prompt_cache_fields"
	ListDimensions       = "embedding_dimensions"
	ListLanguages        = "languages"
	ListUseCases         = "use_cases"
)

// Fields of a card's detail list, named as AWS labels them.
const (
	fieldLaunch       = "model launch date"
	fieldEOL          = "model eol date"
	fieldLifecycle    = "model lifecycle"
	fieldContext      = "context window"
	fieldMaxOut       = "max output tokens"
	fieldReasoning    = "reasoning"
	fieldCutoff       = "knowledge cutoff"
	fieldMarketplace  = "marketplace product id"
	fieldFineTuning   = "fine-tuning supported"
	fieldLanguages    = "languages"
	fieldUseCases     = "supported use cases"
	fieldImagePayload = "max image payload size"
	fieldSampling     = "sampling parameters"
)

// supportedIcon is the image AWS marks a supported entry with. A card states
// support as a picture rather than as a word, so the picture is what is read.
const supportedIcon = "icon-yes.png"

// Capabilities a card names that the catalog has no canonical value for.
const (
	featurePromptCaching = "prompt_caching"
	featureStreaming     = "streaming"
	featureComputerUse   = "computer_use"
	featureBatch         = "batch_inference"
	featureLatency       = "latency_optimized"
)

// cardFeatures map a capability a card lists onto the catalog's vocabulary.
// Only the names that differ are listed; the rest keep AWS's own words with
// their spacing reduced to an identifier.
var cardFeatures = map[string]string{
	"response streaming":       featureStreaming,
	"tool use":                 catalog.CapabilityFunctionCalling,
	"client-side tool calling": catalog.CapabilityFunctionCalling,
	"prompt caching":           featurePromptCaching,
	"structured outputs":       catalog.CapabilityStructuredOutputs,
	"computer use":             "computer_use",
	// AWS distinguishes the tools a caller declares from the ones the service
	// runs for the model. Only the first is function calling, so the second
	// keeps AWS's own words, normalized like every other unmapped name.
	"server-side tool calling": "server_side_tool_calling",
}

// cardModalities map a row of a card's modality matrix onto the catalog's
// vocabulary. A card lists every modality it knows of and marks each supported
// or not, so the ones it does not have are named too and are dropped.
var cardModalities = map[string]string{
	"text":      "text",
	"image":     "image",
	"audio":     "audio",
	"video":     "video",
	"speech":    "audio",
	"embedding": "embedding",
}

var (
	cardTitleRe  = regexp.MustCompile(`(?m)^#\s+(.+?)\s*$`)
	cardDetailRe = regexp.MustCompile(
		`(?m)^\+\s+\*\*([^*]+):\*\*\s*(.*?)\s*$`,
	)
	// cardSummaryRe matches the paragraph under the details heading, which is
	// the card's own description of the model.
	cardSummaryRe = regexp.MustCompile(
		`(?s)## Model Details\s*\n<a name="[^"]*"></a>\s*\n+(.*?)\n\+`,
	)
	// cardEntryRe matches one marked entry, which is an icon followed by the
	// name it reports on, optionally wrapped in a link.
	cardEntryRe = regexp.MustCompile(
		`!\[[^\]]*\]\(([^)]*)\)\s*(?:\[([^\]]+)\]\([^)]*\)|([^|+<]+))`,
	)
	// cardModelIDRe matches an identifier the model answers to, which a card
	// states bare in its access table and again with a routing prefix.
	cardModelIDRe = regexp.MustCompile(
		"`?((?:[a-z0-9-]+\\.)+[a-z0-9.-]+-v\\d+:\\d+)`?",
	)
	cardCountRe = regexp.MustCompile(`(?i)([\d][\d,.]*)\s*([km])?`)
	linkTextRe  = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	// cardDropRe matches the words one document writes and the other does
	// not. The last of them is the generation Amazon marks its own older
	// models with and marks them with inconsistently: the list's Titan Image
	// Generator V2 is the card's Titan Image Generator G1 v2.
	cardDropRe = regexp.MustCompile(
		`(?i)\b(instruct|latency|optimized|custom|g1)\b`,
	)
	// cardZeroRe matches the trailing zero of a version written to one decimal
	// place, which one document writes and the other leaves off: the price
	// list's Nova 2.0 Lite is the card's Nova 2 Lite. A version carrying two
	// decimals, as Pixtral Large 25.02 does, is not one of these.
	cardZeroRe = regexp.MustCompile(`(\d)\.0\b`)
	cardWordRe = regexp.MustCompile(`[^a-z0-9]+`)
)

// applyCards reads every model card onto the model it describes.
//
// The cards are collected before any is applied, because which card describes
// a model is decided by comparing it against all of them, and in two rounds.
// Every model naming a card outright takes it first, and only the cards left
// over are offered to a model that merely begins one or is begun by one. One
// card can still describe several models, since a latency-optimized variant is
// the same model on a faster path and is carded under the plain name.
//
// A card no model claims describes a model of its own, since the guide cards
// what the price list does not meter.
func (b *builder) applyCards(docs []catalog.Document) {
	cards := make([]card, 0, len(docs))
	for _, doc := range docs {
		title := first(cardTitleRe, string(doc.Body))
		if title == "" {
			continue
		}
		body := string(doc.Body)
		cards = append(cards, card{
			doc:    doc,
			title:  title,
			author: cardAuthor(body),
			tokens: compareTokens(title),
			sorted: slices.Sorted(slices.Values(compareTokens(title))),
			ids:    cardModelIDs(body),
			tables: parseTables(body),
		})
	}
	ids := slices.Clone(b.order)
	matched := map[string]card{}
	claimed := map[string]bool{}
	for _, id := range ids {
		if best, ok := exactCard(cards, b.models[id]); ok {
			matched[id] = best
			claimed[best.doc.URL] = true
		}
	}
	free := make([]card, 0, len(cards))
	for _, c := range cards {
		if !claimed[c.doc.URL] {
			free = append(free, c)
		}
	}
	for _, id := range ids {
		if _, done := matched[id]; done {
			continue
		}
		if best, ok := fuzzyCard(free, b.models[id]); ok {
			matched[id] = best
			claimed[best.doc.URL] = true
		}
	}
	for _, id := range ids {
		if best, ok := matched[id]; ok {
			applyCard(b.models[id], best)
		}
	}
	for _, c := range cards {
		if !claimed[c.doc.URL] {
			b.applyLoneCard(c)
		}
	}
}

// applyLoneCard records a model the price list does not meter.
//
// The guide cards every model Bedrock serves and the price list meters only
// some of them, so reading the list alone loses whole families: every Stability
// image model, every Cohere and Titan embedding model, the rerankers, and the
// Anthropic models past Claude 3, none of which the list names. A card carries
// no rate for most of them, and a model AWS documents and does not publish a
// price for is still a model AWS serves.
func (b *builder) applyLoneCard(c card) {
	id := cardID(c)
	if id == "" {
		return
	}
	if _, taken := b.models[id]; taken {
		return
	}
	m := b.model(id, "")
	applyCard(m, c)
	m.Kind = cardKind(c)
}

// cardID names a model by the lab a card credits it to and the name the card
// titles it with, which is the same shape the price list's names reduce to.
func cardID(c card) string {
	name := slug(c.title)
	if name == "" {
		return ""
	}
	if author := slug(c.author); author != "" {
		return author + "/" + name
	}
	return name
}

// outputKinds map the output a card marks a model as producing onto what the
// model is. A model returning a vector is an embedding model however it is
// billed, which is what a name alone cannot always say: the price list bills
// Marengo by the token like any chat model.
var outputKinds = map[string]catalog.Kind{
	"embedding": KindEmbedding,
	"image":     KindImage,
	"video":     KindVideo,
}

// cardKind reports what a card says a model does, from what it produces where
// that settles it and from its name where it does not. A card marking text out
// and nothing else describes a model that answers, and whether it answers by
// transcribing or by reranking is only in the name.
func cardKind(c card) catalog.Kind {
	for _, t := range c.tables {
		if !t.hasHeading(headingOutputModalities) {
			continue
		}
		for _, row := range t.rows {
			for i, value := range row {
				if t.heading(i) != headingOutputModalities {
					continue
				}
				for _, name := range markedNames(value) {
					if kind, ok := outputKinds[name]; ok {
						return kind
					}
				}
			}
		}
	}
	return kindFor(MetricUsage, c.title)
}

// card is one model card with its title reduced to the form the two documents
// can be compared in.
type card struct {
	doc    catalog.Document
	title  string
	author string
	tokens []string
	sorted []string
	ids    []string
	tables []table
}

// cardAuthorRe matches the lab a card names above its title, which is the
// only place a card states one. AWS writes it and the model's name on one
// line divided by an em dash, so the line is split on the rune rather than
// matched word by word.
var cardAuthorRe = regexp.MustCompile(`(?m)^##\s+!\[[^\]]*\]\([^)]*\)\s*(.+)$`)

// authorDivider is the rune AWS divides the lab from the model with.
const authorDivider = "\u2014"

// cardAuthor reads the lab a card credits the model to.
func cardAuthor(body string) string {
	line := first(cardAuthorRe, body)
	author, _, ok := strings.Cut(line, authorDivider)
	if !ok {
		return ""
	}
	return strings.TrimSpace(author)
}

// cardEndpointIDRe matches the identifier a card gives in its table of the
// endpoints a model answers on, which is the one identifier both documents
// state.
var cardEndpointIDRe = regexp.MustCompile(
	`(?m)^\|\s*bedrock-[a-z]+\s*\|\s*` +
		"`?((?:[a-z0-9-]+\\.)+[a-z0-9.:-]+)`?" + `\s*\|`,
)

// cardVersionRe matches the release a card appends to an identifier where the
// price list leaves it off.
var cardVersionRe = regexp.MustCompile(`-v\d+:\d+$`)

// cardModelIDs reads the identifiers a card claims, each also without the
// release, so that an identifier written one way matches the other.
func cardModelIDs(body string) []string {
	var ids []string
	for _, match := range cardEndpointIDRe.FindAllStringSubmatch(body, -1) {
		for _, id := range []string{
			match[1],
			cardVersionRe.ReplaceAllString(match[1], ""),
		} {
			if !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
	}
	return ids
}

// cardByID returns the card claiming an identifier the price list states too.
//
// It is tried before any reading of the name, because it is the only join the
// two documents make themselves: a meter reaching a model through the
// bedrock-mantle endpoint names it in its usage type, and every card names the
// same identifier in the table of endpoints the model answers on. It is what
// settles that the list's "NVIDIA Nemotron Nano 2" is the card's "NVIDIA
// Nemotron Nano 9B v2", which no comparison of the two names would.
func cardByID(cards []card, ids []string) (card, bool) {
	for _, c := range cards {
		for _, id := range c.ids {
			if slices.Contains(ids, id) {
				return c, true
			}
		}
	}
	return card{}, false
}

// exactCard finds the card a model is named by outright, on the identifier
// where both documents state one and on the whole of the name where only one
// of them does.
//
// The name is a poor key and is only the fallback: the price list gives it in
// prose and so does a card, and the two disagree in small ways.
//
// The list names a model as the card does not: it writes "R1" where the card
// writes "DeepSeek-R1", "Writer Palmyra Vision 7B" where the card writes
// "Palmyra Vision 7B", and the bare "google.gemma-4-31b" where the card writes
// "Gemma 4 31B". The author the list records beside the name settles all
// three, so a name matching no card is tried again without its author's words
// and then again with them.
func exactCard(cards []card, m *catalog.Model) (card, bool) {
	if c, ok := cardByID(cards, m.Lists[ListAliases]); ok {
		return c, true
	}
	for _, want := range compareNames(m) {
		if c, ok := sameCard(cards, want); ok {
			return c, true
		}
	}
	return card{}, false
}

// fuzzyCard finds the card a model's name begins or is begun by, and is only
// offered the cards no model was named by outright.
//
// An exact match is the surer reading and takes its card first: there is a
// Nova Sonic card and a Nova 2 Sonic card, and Nova Sonic 2.0 begins the first
// while naming the second. Holding the exact matches back also keeps a model
// from taking its successor's card, which is the price list's Titan Image
// Generator G1 against the card for the G1 v2 that the list meters separately.
func fuzzyCard(cards []card, m *catalog.Model) (card, bool) {
	names := compareNames(m)
	for _, want := range names {
		if c, ok := bestCard(cards, want); ok {
			return c, true
		}
	}
	return soleCard(cards, withoutVersion(names[0]))
}

// compareNames reduces the ways a model may be named to the words they are
// compared by.
func compareNames(m *catalog.Model) [][]string {
	names := []string{
		m.Name,
		withoutAuthor(m.Name, m.Attrs[AttrAuthor]),
		m.Attrs[AttrAuthor] + " " + m.Name,
	}
	wants := make([][]string, 0, len(names))
	for _, name := range names {
		wants = append(wants, compareTokens(name))
	}
	return wants
}

// cardVendorRe matches the vendor an identifier-shaped name opens with, which
// the price list writes and a card does not: openai.gpt-5.4 is carded as
// GPT-5.4.
var cardVendorRe = regexp.MustCompile(`(?i)^[a-z]+\.`)

// withoutAuthor drops the lab from a name the price list either wrote as an
// identifier or opened with the lab's own name.
func withoutAuthor(name, author string) string {
	if !strings.Contains(name, " ") && cardVendorRe.MatchString(name) {
		return cardVendorRe.ReplaceAllString(name, "")
	}
	tokens, prefix := compareTokens(name), compareTokens(author)
	if len(tokens) > len(prefix) && beginsWith(tokens, prefix) {
		return strings.Join(tokens[len(prefix):], " ")
	}
	return name
}

// sameCard returns the one card naming exactly the words a model is named by,
// in whatever order it puts them. Amazon writes the generation before the
// family where the price list writes it after, calling Nova Sonic 2.0 the Nova
// 2 Sonic, and Mistral does the same to Ministral 8B 3.0.
func sameCard(cards []card, want []string) (card, bool) {
	if len(want) == 0 {
		return card{}, false
	}
	sorted := slices.Sorted(slices.Values(want))
	var found card
	matches := 0
	for _, c := range cards {
		if slices.Equal(c.sorted, sorted) {
			found, matches = c, matches+1
		}
	}
	return found, matches == 1
}

// soleCard returns the one card whose name opens with want, and nothing where
// several do, since a name cut this short no longer tells them apart.
func soleCard(cards []card, want []string) (card, bool) {
	var found card
	matches := 0
	for _, c := range cards {
		if beginsWith(c.tokens, want) {
			found, matches = c, matches+1
		}
	}
	return found, matches == 1
}

// withoutVersion drops the release a name ends in, which the two documents
// date differently: the list's Voxtral Mini 1.0 is the card's Voxtral Mini 3B
// 2507, and its Magistral Small 1.2 is the card's Magistral Small 2509. This
// is the last thing tried, and only where one card is left, because a name
// shortened to its family alone would otherwise take a sibling's card.
func withoutVersion(tokens []string) []string {
	for len(tokens) > 2 && isNumber(tokens[len(tokens)-1]) {
		tokens = tokens[:len(tokens)-1]
	}
	return tokens
}

// isNumber reports whether a word is a bare number rather than a size or a
// name, so that the 1.0 of Voxtral Mini 1.0 is droppable and the 7B of Palmyra
// Vision 7B is not.
func isNumber(word string) bool {
	return word != "" && strings.IndexFunc(word, func(r rune) bool {
		return r < '0' || r > '9'
	}) < 0
}

// bestCard returns the most specific card naming a model.
//
// Neither document carries the other's identifier, so the comparison is on the
// prose name, and the two disagree in small ways: the price list writes "Llama
// 3.1 70B" where the card writes "Llama 3.1 70B Instruct", and "Nova 2.0 Lite"
// where the card writes "Nova 2 Lite". Both names are reduced to their words,
// with the serving words dropped and a one-place version's trailing zero taken
// off, and either word list may then begin the other.
//
// Comparing words rather than the letters run together is what keeps Claude
// Opus 4 from claiming Claude Opus 4.1, whose name it would otherwise begin.
func bestCard(cards []card, want []string) (card, bool) {
	var best card
	found := false
	for _, c := range cards {
		if !beginsWith(c.tokens, want) && !beginsWith(want, c.tokens) {
			continue
		}
		if !found || len(c.tokens) > len(best.tokens) ||
			(len(c.tokens) == len(best.tokens) && c.title < best.title) {
			best, found = c, true
		}
	}
	return best, found
}

// beginsWith reports whether the first word list opens with the second, and is
// false for an empty second list so that an unnamed model matches nothing.
func beginsWith(items, prefix []string) bool {
	if len(prefix) == 0 || len(prefix) > len(items) {
		return false
	}
	return slices.Equal(items[:len(prefix)], prefix)
}

// applyCard records one card against one model.
func applyCard(m *catalog.Model, c card) {
	body := string(c.doc.Body)
	m.AddSource(c.doc.URL)
	if !prose(m.Name) {
		m.Name = c.title
	}
	m.SetAttr(AttrAuthor, c.author)
	m.SetAttr(AttrSummary, linkText(first(cardSummaryRe, body)))
	applyDetails(m, body)
	for _, t := range c.tables {
		applyTable(m, t, c)
	}
	for _, id := range cardModelIDRe.FindAllStringSubmatch(body, -1) {
		m.AddList(ListAliases, id[1])
	}
}

// applyDetails records the labelled facts a card opens with.
func applyDetails(m *catalog.Model, body string) {
	for _, field := range cardDetailRe.FindAllStringSubmatch(body, -1) {
		value := linkText(field[2])
		switch strings.ToLower(strings.TrimSpace(field[1])) {
		case fieldLaunch:
			m.SetAttr(AttrReleaseDate, isoDate(value))
		case fieldEOL:
			m.SetAttr(AttrRetirementDate, isoDate(value))
		case fieldLifecycle:
			m.SetAttr(AttrState, strings.ToLower(value))
		case fieldCutoff:
			m.SetAttr(AttrKnowledgeCutoff, isoDate(value))
		case fieldContext:
			m.SetLimit(LimitContextWindow, parseCount(value))
		case fieldMaxOut:
			m.SetLimit(LimitMaxOutputTokens, parseCount(value))
		case fieldMarketplace:
			m.SetAttr(AttrMarketplaceID, value)
		case fieldFineTuning:
			m.SetAttr(AttrFineTuning, strings.ToLower(value))
		case fieldImagePayload:
			m.SetAttr(AttrMaxImagePayload, value)
		case fieldSampling:
			m.SetAttr(AttrSamplingRule, value)
		case fieldLanguages:
			for _, language := range entries(value) {
				m.AddList(ListLanguages, strings.TrimSuffix(language, "."))
			}
		case fieldUseCases:
			for _, use := range entries(value) {
				m.AddList(ListUseCases, strings.TrimSuffix(use, "."))
			}
		case fieldReasoning:
			applyReasoning(m, value)
		}
	}
}

// applyReasoning records the capability and, where AWS qualifies it, the
// qualification. A card states reasoning as a word rather than as a mark, and
// half the cards stating it add in what shape the model reasons, which the
// capability alone does not carry.
func applyReasoning(m *catalog.Model, value string) {
	if !strings.HasPrefix(strings.ToLower(value), "supported") {
		return
	}
	m.AddList(ListFeatures, catalog.CapabilityReasoning)
	note := strings.TrimSpace(strings.TrimPrefix(value, "Supported"))
	m.SetAttr(AttrReasoningNote, strings.TrimSpace(strings.Trim(note, "().")))
}

// Headings a card's tables carry, lowercased as headerRow reduces them.
const (
	headingInputModalities  = "input modalities"
	headingOutputModalities = "output modalities"
	headingSupported        = "supported"
	headingRegion           = "region"
	headingEndpoint         = "endpoint"
	headingModelID          = "model id"
	headingGeoProfile       = "geo inference id"
	headingGlobalProfile    = "global inference id"
	headingStandard         = "standard"
	headingCaching          = "prompt caching supported"
	headingInferenceOption  = "inference option"
)

// applyTable records one of a card's tables, which says which it is in its
// headings rather than in a heading of its own.
//
// A table headed by none of these reports on something that is not a fact
// about the model: the geo tables say which Region a request lands in when it
// is made from another, and the computer use table names the beta header the
// tool is declared with.
func applyTable(m *catalog.Model, t table, c card) {
	switch {
	case t.hasHeading(headingInputModalities), t.hasHeading(headingSupported):
		applyMarked(m, t)
	case t.headed(headingRegion):
		applyRegions(m, t)
	case t.headed(headingEndpoint) && t.hasHeading(headingModelID):
		applyAccess(m, t)
	case t.headed(headingStandard):
		applyTiers(m, t)
	case t.headed(headingCaching):
		applyCaching(m, t)
	case t.headed(headingInferenceOption):
		applyCardPrices(m, t, c)
	}
}

// applyMarked records what a table of ticks and crosses marks as supported,
// under the list its column is headed for.
func applyMarked(m *catalog.Model, t table) {
	for _, row := range t.rows {
		for i, value := range row {
			applyCell(m, t.heading(i), value)
		}
	}
}

// regionCodeRe matches the Region a row of the availability table reports on,
// which AWS writes as the code followed by the place in brackets.
var regionCodeRe = regexp.MustCompile(`^([a-z]{2}(?:-[a-z]+)+-\d+)\b`)

// regionColumns map a column of the availability table onto the list of
// Regions reachable that way. A model is offered in a Region marked under any
// of the three, and the three are kept apart as well, because a model reached
// there only across Regions is not one a caller bound to a single Region can
// use.
var regionColumns = map[string]string{
	"in-region": ListInRegion,
	"geo":       ListGeoRegions,
	"global":    ListGlobalRegions,
}

// applyRegions records where a model is offered.
func applyRegions(m *catalog.Model, t table) {
	for _, row := range t.rows {
		match := regionCodeRe.FindStringSubmatch(strings.TrimSpace(
			cell(row, 0),
		))
		if match == nil {
			continue
		}
		for i, value := range row {
			key, ok := regionColumns[t.heading(i)]
			if !ok || !marked(value) {
				continue
			}
			m.AddList(key, match[1])
			m.AddList(ListRegions, match[1])
		}
	}
}

// marked reports whether a cell carries the icon AWS marks support with.
func marked(value string) bool {
	return strings.Contains(value, supportedIcon)
}

// identifierRe matches an identifier a model answers to, either the model's
// own or an inference profile's, which prefixes it with the geography the
// profile routes within.
var identifierRe = regexp.MustCompile(
	`^(?:[a-z0-9-]+\.)+[a-z0-9][a-z0-9.:-]*$`,
)

// identifiers reads the identifiers out of a cell, which states several
// separated by line breaks and states none as "N/A" or "Not supported".
func identifiers(value string) []string {
	var out []string
	for _, entry := range entries(value) {
		if identifierRe.MatchString(entry) {
			out = append(out, entry)
		}
	}
	return out
}

// applyAccess records the identifiers a card's programmatic access table
// states, which are the model's own on each endpoint it answers on and the
// inference profiles routing to it across Regions.
func applyAccess(m *catalog.Model, t table) {
	for _, row := range t.rows {
		m.AddList(ListEndpoints, strings.ToLower(strings.TrimSpace(
			cell(row, 0),
		)))
		for i := range row {
			value := cell(row, i)
			switch t.heading(i) {
			case headingModelID:
				for _, id := range identifiers(value) {
					m.AddList(ListAliases, id)
					m.SetAttr(AttrModelID, id)
				}
			case headingGeoProfile:
				m.AddList(ListProfiles, identifiers(value)...)
			case headingGlobalProfile:
				for _, id := range identifiers(value) {
					m.AddList(ListProfiles, id)
					m.SetAttr(AttrGlobalProfile, id)
				}
			}
		}
	}
}

// applyTiers records the service tiers a model is offered on, which AWS marks
// the same way it marks a capability.
func applyTiers(m *catalog.Model, t table) {
	for _, row := range t.rows {
		for i, value := range row {
			if marked(value) {
				m.AddList(ListServiceTiers, t.heading(i))
			}
		}
	}
}

// cacheColumns map a column of the prompt caching table onto what it states.
// The last is matched on its opening words because AWS writes the heading in
// the singular on two cards and in the plural on the rest.
var cacheColumns = []struct {
	heading string
	key     string
}{
	{"min tokens per cache checkpoint", LimitCacheMinTokens},
	{"max cache checkpoints per request", LimitCacheMaxCheckpoints},
	{"supported ttl", ListCacheTTLs},
	{"fields that accept prompt cache checkpoint", ListCacheFields},
}

// cacheColumn names what a column of the prompt caching table states.
func cacheColumn(heading string) string {
	for _, entry := range cacheColumns {
		if strings.HasPrefix(heading, entry.heading) {
			return entry.key
		}
	}
	return ""
}

// applyCaching records what a card states about prompt caching, which is the
// one capability AWS quantifies: how large a prompt has to be to be cached,
// how many checkpoints a request may carry, how long they live and which
// fields accept them.
func applyCaching(m *catalog.Model, t table) {
	for _, row := range t.rows {
		if !strings.EqualFold(strings.TrimSpace(cell(row, 0)), "yes") {
			continue
		}
		m.AddList(ListFeatures, featurePromptCaching)
		for i := range row {
			text := cell(row, i)
			switch cacheColumn(t.heading(i)) {
			case LimitCacheMinTokens:
				m.SetLimit(LimitCacheMinTokens, parseCount(text))
			case LimitCacheMaxCheckpoints:
				m.SetLimit(LimitCacheMaxCheckpoints, parseCount(text))
			case ListCacheTTLs:
				for _, ttl := range entries(text) {
					m.AddList(ListCacheTTLs, shortTTL(ttl))
				}
			case ListCacheFields:
				m.AddList(ListCacheFields, entries(text)...)
			}
		}
	}
}

// ttlRe matches a cache lifetime written as a number and a unit of time.
var ttlRe = regexp.MustCompile(`(?i)^(\d+)\s*(second|minute|hour|day)`)

// shortTTL reduces a lifetime AWS writes out to the form its meters use, so
// that the five minutes a card states and the 5m a usage type names read the
// same.
func shortTTL(value string) string {
	match := ttlRe.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil {
		return strings.ToLower(strings.TrimSpace(value))
	}
	return match[1] + strings.ToLower(match[2][:1])
}

// columnLists map a card's column heading onto the enumeration its marked
// entries belong to. A heading absent from this map introduces something that
// is not a list of what the model supports, and its cells are skipped: the
// same word stands under Input Modalities and under Output Modalities, and the
// Not Supported column names exactly what a model lacks.
var columnLists = map[string]string{
	"input modalities":    ListInputModalities,
	"output modalities":   ListOutputModalities,
	"apis supported":      ListEndpoints,
	"endpoints supported": ListEndpoints,
	"supported":           ListFeatures,
}

// applyCell records the marked entries of one cell, under the list its column
// is headed for.
func applyCell(m *catalog.Model, heading, cell string) {
	key, ok := columnLists[heading]
	if !ok {
		return
	}
	for _, name := range markedNames(cell) {
		if key != ListFeatures {
			addNamed(m, key, name)
			continue
		}
		m.AddList(ListFeatures, featureName(name))
	}
}

// markedNames reads what a cell marks as supported. A cell names one entry on
// the modality tables and a bulleted run of them on the capability tables, and
// each entry carries its own mark, so the cross beside one of a run is not a
// reason to drop the ticks beside the others.
func markedNames(cell string) []string {
	var out []string
	for _, entry := range cardEntryRe.FindAllStringSubmatch(cell, -1) {
		if !strings.Contains(entry[1], supportedIcon) {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(entry[2] + entry[3]))
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

// addNamed records a modality or an endpoint under key, translating a modality
// into the catalog's vocabulary and leaving an endpoint as the card writes it.
func addNamed(m *catalog.Model, key, name string) {
	if key == ListEndpoints {
		m.AddList(key, name)
		return
	}
	if modality, ok := cardModalities[name]; ok && modality != "" {
		m.AddList(key, modality)
	}
}

// featureName rewrites a capability into the catalog's vocabulary.
func featureName(name string) string {
	if mapped, ok := cardFeatures[name]; ok {
		return mapped
	}
	return strings.ReplaceAll(name, " ", "_")
}

// compareTokens reduces a model's prose name to the words the two documents
// can be compared by.
func compareTokens(name string) []string {
	s := cardZeroRe.ReplaceAllString(strings.ToLower(name), "$1")
	s = cardDropRe.ReplaceAllString(s, " ")
	return strings.Fields(cardWordRe.ReplaceAllString(s, " "))
}

// parseCount reads a quantity such as "200K tokens" or "64K".
func parseCount(value string) int64 {
	match := cardCountRe.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil {
		return 0
	}
	n, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", ""), 64)
	if err != nil {
		return 0
	}
	switch strings.ToLower(match[2]) {
	case "k":
		n *= 1_000
	case "m":
		n *= 1_000_000
	}
	return int64(n)
}

// linkText strips the links out of a value, keeping what they read as.
func linkText(value string) string {
	s := linkTextRe.ReplaceAllString(value, "$1")
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "`", "")), " ")
}

// first returns the first capture of re, or the empty string.
func first(re *regexp.Regexp, body string) string {
	if match := re.FindStringSubmatch(body); match != nil {
		return match[1]
	}
	return ""
}
