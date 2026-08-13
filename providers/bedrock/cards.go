package bedrock

import (
	"regexp"
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
	ListFeatures         = "features"
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
	"tool use":                 "function_calling",
	"client-side tool calling": "function_calling",
	"prompt caching":           "prompt_caching",
	"structured outputs":       "structured_outputs",
	"computer use":             "computer_use",
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

// applyCard reads one model card onto the model the price list established.
func (b *builder) applyCard(doc catalog.Document) {
	body := string(doc.Body)
	title := first(cardTitleRe, body)
	m, ok := b.byName(title)
	if !ok {
		return
	}
	m.AddSource(doc.URL)
	if m.Name == "" {
		m.Name = title
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
			if strings.EqualFold(value, "supported") {
				m.AddList(ListFeatures, "reasoning")
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

// byName returns the model a card's title names.
//
// The price list names a model in prose and so does a card, and the two
// disagree in small ways: the list writes "Llama 3.1 70B" where the card
// writes "Llama 3.1 70B Instruct", and "Nova 2.0 Lite" where the card writes
// "Nova 2 Lite". Both names are reduced to letters and digits with the
// serving words dropped and a trailing zero taken off each version, and either
// may then be a prefix of the other, since the list names a serving variant
// the card does not have and the card names a tuning the list does not.
func (b *builder) byName(title string) (*catalog.Model, bool) {
	want := compareName(title)
	if want == "" {
		return nil, false
	}
	var found *catalog.Model
	for _, id := range b.order {
		m := b.models[id]
		got := compareName(m.Attrs[AttrModel])
		if got == "" {
			continue
		}
		if got != want &&
			!strings.HasPrefix(got, want) &&
			!strings.HasPrefix(want, got) {
			continue
		}
		if found == nil || len(compareName(found.Attrs[AttrModel])) < len(got) {
			found = m
		}
	}
	return found, found != nil
}

// compareName reduces a model's prose name to the form the two documents can
// be compared in.
func compareName(name string) string {
	s := cardZeroRe.ReplaceAllString(strings.ToLower(name), "$1")
	s = cardDropRe.ReplaceAllString(s, " ")
	return cardWordRe.ReplaceAllString(s, "")
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
