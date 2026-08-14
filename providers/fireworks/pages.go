package fireworks

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Scalar keys the model pages populate.
const (
	AttrSummary       = "summary"
	AttrHuggingFaceID = "hugging_face_id"
)

// LimitContextWindow is the bound a model page states, which the pricing page
// does not.
const LimitContextWindow = "context_window"

// Enumeration keys the model pages populate.
const (
	ListFeatures         = catalog.ListFeatures
	ListInputModalities  = "input_modalities"
	ListOutputModalities = "output_modalities"
)

// FeatureFunctionCalling is the one capability a model page states as a flag.
const FeatureFunctionCalling = catalog.CapabilityFunctionCalling

// FeatureReasoning is the capability the chat completion reference states,
// family by family, against the parameter that turns it on.
const FeatureReasoning = catalog.CapabilityReasoning

// everyModelRe matches the sentence the structured outputs guide states its
// scope in, which is every model Fireworks serves. It is matched rather than
// assumed: the guide worked through one model by name and then said the
// feature is not particular to it, and a guide rewritten to name a list stops
// yielding the capability for all of them.
var everyModelRe = regexp.MustCompile(
	`(?i)all fireworks models support this feature`,
)

// applyStructuredOutputs records the capability against every model that
// generates a response.
//
// This is the one capability Fireworks states outside the model record. That
// record carries a flag for tool use and one for image input and nothing for
// this, and the guide explains why: the answer is the same for every model, so
// there is nothing to flag per model.
//
// The guide says all of them, and it means all the models it is about. It
// constrains what a model writes, and an embedding model writes nothing: it
// returns a vector, which no schema describes and no grammar could constrain.
// Reading "all" past the models the sentence is about would state a capability
// that cannot be exercised.
func (b *builder) applyStructuredOutputs(doc catalog.Document) {
	if !everyModelRe.Match(doc.Body) {
		return
	}
	for _, id := range b.order {
		m := b.models[id]
		if m.Kind != KindChat {
			continue
		}
		m.AddList(ListFeatures, catalog.CapabilityStructuredOutputs)
		m.AddSource(doc.URL)
	}
}

// Fields of the model record a page carries.
//
// The record is JSON embedded in the page as a string, so its quotes arrive
// escaped, and how many times depends on how deeply the page nested it. The
// escaping is therefore matched rather than undone.
var (
	contextRe = fieldRe(`contextLength`, `(\d+)`)
	imageRe   = fieldRe(`supportsImageInput`, `(true|false)`)
	toolsRe   = fieldRe(`supportsTools`, `(true|false)`)
	nameRe    = fieldRe(`displayName`, `\\*"(.*?[^\\])\\*"`)
	summaryRe = fieldRe(`description`, `\\*"(.*?[^\\])\\*"`)
	huggingRe = fieldRe(`huggingFaceUrl`, `\\*"(.*?[^\\])\\*"`)
)

// fieldRe matches one field of the embedded record, whatever its escaping.
func fieldRe(name, value string) *regexp.Regexp {
	return regexp.MustCompile(`\\*"` + name + `\\*":\s*` + value)
}

// applyModelPage reads one model's page onto the model the pricing page
// established for it.
//
// The pricing page links every row to the page of the model it prices, and
// several rows link to the same page: a model served three ways is one model
// with three rates. The link is therefore the join, and needs no matching on
// names.
func (b *builder) applyModelPage(doc catalog.Document) {
	m, ok := b.byURL(doc.URL)
	if !ok {
		return
	}
	body := string(doc.Body)
	m.AddSource(doc.URL)
	if m.Name == "" {
		m.Name = unescape(first(nameRe, body))
	}
	m.SetAttr(AttrSummary, unescape(first(summaryRe, body)))
	m.SetAttr(
		AttrHuggingFaceID,
		huggingFaceID(unescape(first(huggingRe, body))),
	)
	if n, err := strconv.ParseInt(first(contextRe, body), 10, 64); err == nil {
		m.SetLimit(LimitContextWindow, n)
	}
	m.AddList(ListInputModalities, "text")
	m.AddList(ListOutputModalities, "text")
	if first(imageRe, body) == "true" {
		m.AddList(ListInputModalities, "image")
	}
	if first(toolsRe, body) == "true" {
		m.AddList(ListFeatures, FeatureFunctionCalling)
	}
}

// byURL returns the model whose page is at url.
func (b *builder) byURL(url string) (*catalog.Model, bool) {
	for _, id := range b.order {
		if b.models[id].Attrs[AttrModelURL] == url {
			return b.models[id], true
		}
	}
	return nil, false
}

// libraryPageURLs picks the models whose console record left something for the
// model library's page to state: a context window the record puts at none, and
// the width of the vector an embedding model returns, which the record has no
// field for at all. Every other model is already fully stated, so its library
// page is not fetched.
func libraryPageURLs(pages []catalog.Document, embedding []string) []string {
	var urls []string
	for _, doc := range pages {
		if !strings.HasPrefix(doc.URL, modelPagePre) {
			continue
		}
		context, _ := strconv.ParseInt(
			first(contextRe, string(doc.Body)),
			10,
			64,
		)
		if context > 0 && !slices.Contains(embedding, doc.URL) {
			continue
		}
		urls = append(urls, libraryURL(doc.URL))
	}
	slices.Sort(urls)
	return slices.Compact(urls)
}

// libraryURL is the model library's page for the model whose console page is
// at url. The two sites key a model the same way, so one address is the other
// with its host swapped.
func libraryURL(url string) string {
	rest, ok := strings.CutPrefix(url, modelPagePre)
	if !ok {
		return ""
	}
	return libraryPagePre + rest
}

// What the library page states that the console record does not. The page is
// rendered rather than embedded, so these read the rendered text: a labelled
// row of the specification table, and a line of the model's own FAQ.
var (
	libraryContextRe = regexp.MustCompile(
		`(?s)Context Length</span>.{0,200}?>([\d.]+)\s*([kKmM]) tokens<`,
	)
	libraryDimensionRe = regexp.MustCompile(
		`(?i)embedding dimensions from [\d,]+ to ([\d,]+)`,
	)
)

// applyLibraryPage reads what the console record left unstated off the model
// library's page for the same model.
//
// It fills rather than overrides. The record states a context window exactly
// and this page rounds it for display, so the page is only believed where the
// record put the window at none, which is Fireworks' way of saying the console
// has no figure rather than that the model has no window.
func (b *builder) applyLibraryPage(doc catalog.Document) {
	m, ok := b.byURL(consoleURL(doc.URL))
	if !ok {
		return
	}
	body := string(doc.Body)
	if match := libraryContextRe.FindStringSubmatch(body); match != nil &&
		m.Limits[LimitContextWindow] == 0 {
		m.SetLimit(LimitContextWindow, tokenCount(match[1], match[2]))
		m.AddSource(doc.URL)
	}
	if match := libraryDimensionRe.FindStringSubmatch(body); match != nil {
		m.SetAttr(
			AttrDefaultDimension,
			strings.ReplaceAll(match[1], ",", ""),
		)
		m.AddSource(doc.URL)
	}
}

// consoleURL is the console page of the model whose library page is at url.
func consoleURL(url string) string {
	rest, ok := strings.CutPrefix(url, libraryPagePre)
	if !ok {
		return ""
	}
	return modelPagePre + rest
}

// tokenCount reads a count the library page abbreviated, which it writes as
// "262k tokens" or "1.05m tokens".
func tokenCount(amount, scale string) int64 {
	value, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		return 0
	}
	if strings.EqualFold(scale, "m") {
		return int64(value * 1_000_000)
	}
	return int64(value * 1_000)
}

// The chat completion reference documents reasoning_effort model family by
// model family, under a heading of its own. That list is the only place
// Fireworks says which models reason: the guide to reasoning works its
// examples through a placeholder model, and the record on a model's page
// carries no flag for it.
var (
	reasoningSectionRe = regexp.MustCompile(
		`(?s)reasoning_effort:.*?Model-specific behavior:?\*\*(.*?)\n\s*\w+:\n`,
	)
	reasoningFamilyRe = regexp.MustCompile(`(?m)^\s*-\s+\*\*(.+?)\*\*:`)
)

// applyReasoning records the capability against the models the chat completion
// reference documents a reasoning effort for.
//
// The reference states what each family does with the parameter, down to which
// efforts it accepts and whether reasoning is on when nothing is passed. A
// family named there is a family Fireworks says reasons; a family absent from
// it is not, which is why nothing is inferred for the models it leaves out.
func (b *builder) applyReasoning(doc catalog.Document) {
	families := reasoningFamilies(string(doc.Body))
	for _, id := range b.order {
		m := b.models[id]
		if m.Kind != KindChat {
			continue
		}
		for _, family := range families {
			if !namesModel(family, m.Name) {
				continue
			}
			m.AddList(ListFeatures, FeatureReasoning)
			m.AddSource(doc.URL)
			break
		}
	}
}

// reasoningFamilies returns the names the reference's model-specific paragraphs
// are written against. A paragraph heads several models where one behaviour
// covers them all, either as a comma separated list or as the models a chat
// template's name is followed by in brackets.
func reasoningFamilies(body string) []string {
	section := reasoningSectionRe.FindStringSubmatch(body)
	if section == nil {
		return nil
	}
	var out []string
	for _, match := range reasoningFamilyRe.FindAllStringSubmatch(
		section[1],
		-1,
	) {
		named := match[1]
		if _, inside, ok := strings.Cut(named, "("); ok {
			named = strings.TrimSuffix(inside, ")")
		}
		for _, name := range strings.Split(named, ",") {
			if name = strings.TrimSpace(name); name != "" {
				out = append(out, name)
			}
		}
	}
	return out
}

// namesModel reports whether family names the model: every word of the family
// inside the model's name, in order and adjacent. Adjacency is what keeps a
// paragraph about MiniMax M2 off MiniMax M2.7, which is a different model and
// documented, when it is documented at all, in a paragraph of its own.
func namesModel(family, model string) bool {
	want, have := nameWords(family), nameWords(model)
	if len(want) == 0 || len(want) > len(have) {
		return false
	}
	for at := 0; at+len(want) <= len(have); at++ {
		if slices.Equal(have[at:at+len(want)], want) {
			return true
		}
	}
	return false
}

// huggingFaceID reduces the address of a model's weights to the identifier
// they are published under, which is what other providers record.
func huggingFaceID(url string) string {
	_, id, ok := strings.Cut(url, "huggingface.co/")
	if !ok {
		return ""
	}
	return strings.Trim(id, "/")
}

// unescape undoes the backslashes the embedded record's own encoding added.
func unescape(value string) string {
	r := strings.NewReplacer(`\\"`, `"`, `\"`, `"`, `\\n`, " ", `\\`, `\`)
	return strings.TrimSpace(r.Replace(value))
}

// first returns the first capture of re, or the empty string.
func first(re *regexp.Regexp, body string) string {
	if match := re.FindStringSubmatch(body); match != nil {
		return match[1]
	}
	return ""
}
