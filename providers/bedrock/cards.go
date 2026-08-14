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
	AttrSummary         = "summary"
	AttrState           = "lifecycle_state"
	AttrReleased        = "released"
	AttrRetirementDate  = "retirement_date"
	AttrKnowledgeCutoff = "knowledge_cutoff"
)

// Numeric keys the model cards populate.
const (
	LimitContextWindow   = "context_window"
	LimitMaxOutputTokens = "max_output_tokens"
)

// Enumeration keys the model cards populate.
const (
	ListFeatures         = catalog.ListFeatures
	ListEndpoints        = "endpoints"
	ListAliases          = "aliases"
	ListInputModalities  = "input_modalities"
	ListOutputModalities = "output_modalities"
)

// Fields of a card's detail list, named as AWS labels them.
const (
	fieldLaunch    = "model launch date"
	fieldEOL       = "model eol date"
	fieldLifecycle = "model lifecycle"
	fieldContext   = "context window"
	fieldMaxOut    = "max output tokens"
	fieldReasoning = "reasoning"
	fieldCutoff    = "knowledge cutoff"
)

// supportedIcon is the image AWS marks a supported entry with. A card states
// support as a picture rather than as a word, so the picture is what is read.
const supportedIcon = "icon-yes.png"

// cardFeatures map a capability a card lists onto the catalog's vocabulary.
// Only the names that differ are listed; the rest keep AWS's own words with
// their spacing reduced to an identifier.
var cardFeatures = map[string]string{
	"response streaming":       "streaming",
	"tool use":                 catalog.CapabilityFunctionCalling,
	"client-side tool calling": catalog.CapabilityFunctionCalling,
	"prompt caching":           "prompt_caching",
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
	"embedding": "",
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
	// cardRowRe matches one row of a card's tables.
	cardRowRe = regexp.MustCompile(`(?m)^\|(.*)\|\s*$`)
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
	cardDropRe  = regexp.MustCompile(
		`(?i)\b(instruct|latency|optimized|custom)\b`,
	)
	// cardZeroRe matches the trailing zero of a version written to one decimal
	// place, which one document writes and the other leaves off: the price
	// list's Nova 2.0 Lite is the card's Nova 2 Lite. A version carrying two
	// decimals, as Pixtral Large 25.02 does, is not one of these.
	cardZeroRe = regexp.MustCompile(`(\d)\.0\b`)
	cardWordRe = regexp.MustCompile(`[^a-z0-9]+`)
)

// applyCards reads every model card onto the models the price list
// established.
//
// The cards are collected before any is applied, because which card describes
// a model is decided by comparing it against all of them: a model takes the
// most specific card that names it, and one card can describe several models,
// since a latency-optimized variant is the same model on a faster path and is
// carded under the plain name.
func (b *builder) applyCards(docs []catalog.Document) {
	cards := make([]card, 0, len(docs))
	for _, doc := range docs {
		title := first(cardTitleRe, string(doc.Body))
		if title == "" {
			continue
		}
		cards = append(cards, card{
			doc:    doc,
			title:  title,
			tokens: compareTokens(title),
			sorted: slices.Sorted(slices.Values(compareTokens(title))),
			ids:    cardModelIDs(string(doc.Body)),
		})
	}
	for _, id := range b.order {
		m := b.models[id]
		best, ok := matchCard(cards, m)
		if !ok {
			continue
		}
		applyCard(m, best)
	}
}

// card is one model card with its title reduced to the form the two documents
// can be compared in.
type card struct {
	doc    catalog.Document
	title  string
	tokens []string
	sorted []string
	ids    []string
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

// matchCard finds the card describing a model, on the identifier where both
// documents state one and on the name where only one of them does.
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
//
// A name matching one card exactly wins over one that merely begins it,
// whichever rewriting found it, because the exact match is the surer reading:
// there is a Nova Sonic card and a Nova 2 Sonic card, and Nova Sonic 2.0
// begins the first while naming the second.
func matchCard(cards []card, m *catalog.Model) (card, bool) {
	if c, ok := cardByID(cards, m.Lists[ListAliases]); ok {
		return c, true
	}
	names := []string{
		m.Name,
		withoutAuthor(m.Name, m.Attrs[AttrAuthor]),
		m.Attrs[AttrAuthor] + " " + m.Name,
	}
	wants := make([][]string, 0, len(names))
	for _, name := range names {
		wants = append(wants, compareTokens(name))
	}
	for _, want := range wants {
		if c, ok := sameCard(cards, want); ok {
			return c, true
		}
	}
	for _, want := range wants {
		if c, ok := bestCard(cards, want); ok {
			return c, true
		}
	}
	return soleCard(cards, withoutVersion(wants[0]))
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
	m.SetAttr(AttrSummary, linkText(first(cardSummaryRe, body)))
	applyDetails(m, body)
	applyMatrices(m, body)
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
			m.SetAttr(AttrReleased, value)
		case fieldEOL:
			m.SetAttr(AttrRetirementDate, value)
		case fieldLifecycle:
			m.SetAttr(AttrState, strings.ToLower(value))
		case fieldCutoff:
			m.SetAttr(AttrKnowledgeCutoff, value)
		case fieldContext:
			m.SetLimit(LimitContextWindow, parseCount(value))
		case fieldMaxOut:
			m.SetLimit(LimitMaxOutputTokens, parseCount(value))
		case fieldReasoning:
			if strings.HasPrefix(strings.ToLower(value), "supported") {
				m.AddList(ListFeatures, catalog.CapabilityReasoning)
			}
		}
	}
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

// applyMatrices records what a card's tables mark as supported.
//
// A card states support as a picture: every modality, capability and endpoint
// it knows of is listed and each carries a tick or a cross, so what a model
// lacks is named as plainly as what it has. The tick is what is read, and the
// heading above a marked entry says which list it belongs to.
func applyMatrices(m *catalog.Model, body string) {
	var columns []string
	for _, row := range cardRowRe.FindAllStringSubmatch(body, -1) {
		cells := strings.Split(row[1], "|")
		if headings, ok := headerRow(cells); ok {
			columns = headings
			continue
		}
		for i, cell := range cells {
			if i < len(columns) {
				applyCell(m, columns[i], cell)
			}
		}
	}
}

// headerRow reports whether a row heads its table, and what it heads each
// column with. A card writes a heading in bold and nothing else that way.
func headerRow(cells []string) ([]string, bool) {
	headings := make([]string, len(cells))
	bold := false
	for i, cell := range cells {
		text := strings.TrimSpace(cell)
		if !strings.HasPrefix(text, "**") || !strings.HasSuffix(text, "**") {
			continue
		}
		bold = true
		headings[i] = strings.ToLower(linkText(strings.Trim(text, "* ")))
	}
	return headings, bold
}

// applyCell records the marked entries of one cell, under the list its column
// is headed for.
func applyCell(m *catalog.Model, heading, cell string) {
	key, ok := columnLists[heading]
	if !ok {
		return
	}
	for _, entry := range cardEntryRe.FindAllStringSubmatch(cell, -1) {
		if !strings.Contains(entry[1], supportedIcon) {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(entry[2] + entry[3]))
		if name == "" {
			continue
		}
		if key != ListFeatures {
			addNamed(m, key, name)
			continue
		}
		m.AddList(ListFeatures, featureName(name))
	}
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
