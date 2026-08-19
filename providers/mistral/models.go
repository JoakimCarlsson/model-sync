package mistral

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Patterns over the flight payload a model page serves. React renders each
// fact into a fixed shape, so the anchor for a value is the presentation class
// its own element carries rather than the label beside it, which the page
// writes two different ways depending on whether a tooltip sits between them.
var (
	// namesRe matches the list of identifiers the model answers to. The first
	// is the dated identifier and the rest are aliases pointing at it.
	namesRe = regexp.MustCompile(`"names":\[([^\]]*)\]`)
	// titleRe matches the display name, which is the page's heading.
	titleRe = regexp.MustCompile(`"as":"h1","size":"h3","children":"([^"]+)"`)
	// badgeRe matches the lifecycle badge. Its inline style is a colour for
	// every standing but the current one, which carries none.
	badgeRe = regexp.MustCompile(
		`font-mono uppercase text-\[11px\]","style":(?:"\$undefined"|\{[^}]*\})` +
			`,"children":"([^"]+)"`,
	)
	versionRe  = regexp.MustCompile(`"children":\["v","([^"]+)"\]`)
	releasedRe = regexp.MustCompile(
		`"className":"text-sm text-foreground/50","children":` +
			`\["([A-Z][a-z]+ \d{1,2}, \d{4})"`,
	)
	summaryRe = regexp.MustCompile(
		`"className":"text-secondary-foreground/93","children":"([^"]+)"`,
	)
	weightsRe = regexp.MustCompile(
		`\{"type":"([A-Za-z ]+)","licenses":\[([^\]]*)\]\}`,
	)
	// pricingRe matches the rate card. The lazy body stops at the brace the
	// retirement flag follows, which is the object's own closing brace.
	pricingRe = regexp.MustCompile(`(?s)"pricing":(\{.*?\}),"isRetired":`)
	// featureRe matches one capability tile, keyed by the capability it names.
	featureRe = regexp.MustCompile(
		`\["\$","div","([a-z0-9-]+)",\{"className":"flex flex-col",` +
			`"aria-disabled"`,
	)
	// modalityRe matches a modality tooltip, which Mistral writes as the
	// modality followed by the direction it flows in.
	modalityRe = regexp.MustCompile(`"children":"([A-Z][a-z]+) (input|output)"`)
	// lifecycleRe matches the one lifecycle date a page states, which is the
	// deprecation date until the model retires and the retirement date after.
	lifecycleRe = regexp.MustCompile(
		`"children":"(Deprecation|Retirement) date"\}\],\["\$","span",null,` +
			`\{"className":"[^"]*","children":"([^"]+)"`,
	)
	replacementRe = regexp.MustCompile(
		`"children":"Replacement"\}\],\["\$","\$L\d+",null,\{"href":` +
			`"/models/[a-z0-9.-]+","className":"[^"]*","children":"([^"]+)"`,
	)
	quotedRe = regexp.MustCompile(`"([^"]*)"`)
)

// statRe matches the value of one headline statistic. Mistral renders every
// one of them into the same element, so the label is matched first and the
// value taken from the next such element within the tile.
func statRe(label string) *regexp.Regexp {
	return regexp.MustCompile(
		`(?s)"` + regexp.QuoteMeta(label) + `"(?:.{0,160}?)` +
			`"text-lg font-bold font-mono text-primary-soft","children":` +
			`"([^"]+)"`,
	)
}

var (
	contextRe = statRe("Context")
	maxOutRe  = statRe("Max output")
)

// rateCard is the rate a model page states, before it is split into prices.
// Mistral groups rates by direction and names the exception within a group,
// leaving the ordinary input rate unlabelled.
//
// A model billed the same whichever direction a token travels in is written
// flat instead, with one amount and no groups, and the embedded rate is the
// card itself.
type rateCard struct {
	Free   bool   `json:"free"`
	Input  []rate `json:"input"`
	Output []rate `json:"output"`
	rate
}

// rate is one amount together with what it is quoted against.
type rate struct {
	Price       float64 `json:"price"`
	Denominator string  `json:"denominator"`
	Label       string  `json:"label"`
}

// applyModelPage reads one model's page.
//
// A page that names no API identifier is skipped. Mistral publishes pages for
// weights it has released without serving, and those describe a download
// rather than a model the catalog can key or price.
func (b *builder) applyModelPage(doc catalog.Document) {
	body := flight(doc.Body)
	names := quotedList(namesRe, body)
	if len(names) == 0 {
		return
	}
	features := featureRe.FindAllStringSubmatch(body, -1)
	list := make([]string, 0, len(features))
	for _, match := range features {
		list = append(list, match[1])
	}

	m := b.model(names[0], kindFor(names[0], list))
	b.slugs[strings.TrimPrefix(doc.URL, modelPagePre)] = names[0]
	m.AddSource(doc.URL)
	if m.Name == "" {
		m.Name = first(titleRe, body)
	}
	m.AddList(ListAliases, names[1:]...)
	for _, id := range list {
		m.AddList(ListFeatures, featureName(id))
	}
	applyState(m, body)
	applyWeights(m, body)
	applyEndpoints(m, body)
	applyLimits(m, body)
	applyModalities(m, body)
	applyPrices(m, body)
}

// applyState records the standing, version and lifecycle dates. Licensing is
// read from the weights tab instead, which states it alongside the repository
// the licence governs.
func applyState(m *catalog.Model, body string) {
	m.SetAttr(AttrState, badgeStates[first(badgeRe, body)])
	m.SetAttr(AttrVersion, first(versionRe, body))
	m.SetAttr(AttrSummary, first(summaryRe, body))
	m.SetAttr(AttrReleased, isoDate(first(releasedRe, body)))
	m.SetAttr(AttrReplacement, first(replacementRe, body))
	for _, match := range lifecycleRe.FindAllStringSubmatch(body, -1) {
		key := AttrDeprecatedOn
		if match[1] == "Retirement" {
			key = AttrRetirementDate
		}
		m.SetAttr(key, isoDate(match[2]))
	}
}

// applyLimits records the context window and the output bound.
func applyLimits(m *catalog.Model, body string) {
	m.SetLimit(LimitContextWindow, parseCount(first(contextRe, body)))
	m.SetLimit(LimitMaxOutputToken, parseCount(first(maxOutRe, body)))
}

// applyModalities records what a model takes and what it emits. Mistral marks
// a reasoning model by giving it a reasoning output, and that is recorded as a
// capability, because no other provider treats reasoning as a modality.
func applyModalities(m *catalog.Model, body string) {
	for _, match := range modalityRe.FindAllStringSubmatch(body, -1) {
		name := strings.ToLower(match[1])
		if name == "max" {
			continue
		}
		if name == "reasoning" {
			m.AddList(ListFeatures, "reasoning")
			continue
		}
		key := ListInputModalities
		if match[2] == "output" {
			key = ListOutputModalities
		}
		if mapped, ok := modalityNames[name]; ok {
			m.AddList(key, mapped)
			continue
		}
		m.AddList(key, name)
	}
	closeModalities(m)
}

// unstatedOutputs name what a model gives back for the kinds whose pages state
// an input and stop. An embedding model answers with a vector and a moderation
// model with a set of category scores; neither is a modality, and what both
// work in is text.
var unstatedOutputs = map[catalog.Kind][]string{
	KindEmbedding:  {"text"},
	KindModeration: {"text"},
}

// closeModalities records the side a model's page leaves out.
//
// Mistral labels the modalities on a page as inputs and outputs, so what it
// states is read directly and this fills only what no page states. Recording
// one side alone leaves a consumer unable to tell an unstated output from a
// model that returns nothing, which is why the two go together.
func closeModalities(m *catalog.Model) {
	if len(m.Lists[ListInputModalities]) == 0 ||
		len(m.Lists[ListOutputModalities]) > 0 {
		return
	}
	m.AddList(ListOutputModalities, unstatedOutputs[m.Kind]...)
}

// applyPrices records the rate card.
//
// Mistral states a rate only while a model is current: a deprecated model's
// page drops its price card even though the model still serves. What is
// absent is therefore left absent rather than carried over from the version
// that replaced it.
func applyPrices(m *catalog.Model, body string) {
	match := pricingRe.FindStringSubmatch(body)
	if match == nil {
		return
	}
	var card rateCard
	if err := json.Unmarshal([]byte(match[1]), &card); err != nil {
		return
	}
	if len(card.Input) == 0 && len(card.Output) == 0 {
		addRate(m, card.rate, false)
		return
	}
	for _, r := range card.Input {
		addRate(m, r, false)
	}
	for _, r := range card.Output {
		addRate(m, r, true)
	}
}

// addRate records one amount, choosing the metric from the direction it flows
// in and from what the denominator counts.
func addRate(m *catalog.Model, r rate, output bool) {
	key := strings.ToLower(strings.TrimSpace(r.Denominator))
	mapped, ok := denominators[key]
	if !ok {
		return
	}
	price := catalog.Price{
		Metric:   metricFor(mapped.Metric, r.Label, output),
		Unit:     mapped.Unit,
		Amount:   r.Price,
		Currency: currency,
	}
	if kind, ok := pageKinds[key]; ok {
		price.Dims = catalog.Dims{DimPageKind: kind}
	}
	m.AddPrice(price)
}

// metricFor reports what an amount counts. A denominator that names its own
// subject wins, since a per-minute rate is audio however Mistral grouped it.
// The exception is the text a speech model is given: it is counted in the same
// characters as the audio it returns, so the denominator alone would name the
// two sides the same thing.
func metricFor(
	fixed catalog.Metric,
	label string,
	output bool,
) catalog.Metric {
	if fixed == MetricSpeech && !output {
		return MetricInputCharacters
	}
	if fixed != "" {
		return fixed
	}
	if strings.EqualFold(label, "cached input") {
		return MetricCachedInputTokens
	}
	if output {
		return MetricOutputTokens
	}
	return MetricInputTokens
}

// first returns the first capture of re, or the empty string.
func first(re *regexp.Regexp, body string) string {
	if match := re.FindStringSubmatch(body); match != nil {
		return match[1]
	}
	return ""
}

// quotedList returns the quoted strings inside re's first capture.
func quotedList(re *regexp.Regexp, body string) []string {
	match := re.FindStringSubmatch(body)
	if match == nil {
		return nil
	}
	return quoted(match[1])
}

// quoted returns the quoted strings in a JSON array body.
func quoted(list string) []string {
	matches := quotedRe.FindAllStringSubmatch(list, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if m[1] != "" {
			out = append(out, m[1])
		}
	}
	return out
}
