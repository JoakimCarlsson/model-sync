package vertexai

import (
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

const docsBase = "https://cloud.google.com"

// Documents Google publishes about the models Vertex serves, as distinct from
// the billing catalog, which describes none of them.
const (
	// ModelsURL indexes the page each model has of its own.
	ModelsURL = docsBase + "/vertex-ai/generative-ai/docs/models"
	// modelPagePre prefixes those pages. Google files them under the platform
	// the models are sold through rather than under the documentation set the
	// index belongs to.
	modelPagePre = docsBase + "/gemini-enterprise-agent-platform/models/"
)

// modelSections are the parts of the documentation that describe a model
// rather than a task. Everything else under the same path is a guide.
var modelSections = []string{
	"gemini/",
	"open-models/",
	"maas/",
	"partner-models/",
}

// Numeric keys the model pages populate.
const (
	LimitContextWindow   = "context_window"
	LimitMaxOutputTokens = "max_output_tokens"
)

// Enumeration keys the model pages populate.
const (
	ListFeatures         = catalog.ListFeatures
	ListInputModalities  = "input_modalities"
	ListOutputModalities = "output_modalities"
	// ListDimensions holds the widths an embedding model can return, which is
	// the key every other provider here states a vector width under.
	ListDimensions = "embedding_dimensions"
)

// featureNames map a capability a page lists onto the catalog's vocabulary.
// Only the names that differ are listed; the rest keep Google's own words with
// their spacing reduced to an identifier.
var featureNames = map[string]string{
	"thinking":          "reasoning",
	"structured output": "structured_outputs",
	"context caching":   "prompt_caching",
	"grounding":         "web_search",
	"tools":             "function_calling",
}

// modalityNames map a modality a page names onto the catalog's vocabulary.
//
// An embedding page marks "Embeddings" as an output, which is the model's
// return value rather than a modality. The catalog has no word for a vector, so
// it is read as the text the model works in; skipping it left those models
// stating what they take and nothing about what they give back, which a
// consumer cannot tell from a model that returns nothing.
var modalityNames = map[string]string{
	"text":       "text",
	"image":      "image",
	"audio":      "audio",
	"video":      "video",
	"embeddings": "text",
}

var (
	// specTableRe matches the specification table, which carries a class of
	// its own and is the only table stating what a model is.
	specTableRe = regexp.MustCompile(
		`(?is)<table class="geap-model-table">(.*?)</table>`,
	)
	specIDRe = regexp.MustCompile(
		`(?is)id="model-id".*?<code[^>]*>(.*?)</code>`,
	)
	// specModalityRe matches one modality and the direction it flows in.
	specModalityRe = regexp.MustCompile(
		`(?is)class="geap-modality-label">(.*?)</span>.*?` +
			`class="geap-supported-modality">(.*?)</span>`,
	)
	// specCapabilityRe matches the row listing what a model can do. The table
	// carries a second list in the same shape below it, of the ways the model
	// may be bought, and those are not capabilities.
	specCapabilityRe = regexp.MustCompile(
		`(?is)<th id="capabilities">.*?</tr>`,
	)
	// specFeatureRe matches one capability and whether the model has it. The
	// page states the answer in a class rather than only in the text.
	specFeatureRe = regexp.MustCompile(
		`(?is)<li class="geap-feature">(.*?)</li>`,
	)
	// specFeatureNameRe matches the capability's own name, which the page
	// links and follows with a break and the variants of it the model offers.
	// Those variants are the capability rather than more of them.
	specFeatureNameRe = regexp.MustCompile(`(?is)^(.*?)(?:<br|$)`)
	specSupportedRe   = regexp.MustCompile(`(?is)class="geap-supported"`)
	// specHeadRe matches a labelled row of the table. The heading may not run
	// across markup: a row stating a bound carries two headings, the group's
	// and the bound's, and it is the second that names the value beside it.
	specHeadRe = regexp.MustCompile(
		`(?is)<th[^>]*>([^<]*)</th>\s*<td[^>]*>(.*?)</td>`,
	)
	specTagRe   = regexp.MustCompile(`(?s)<[^>]*>`)
	specCountRe = regexp.MustCompile(`([\d][\d,]*)`)
	// specQuotaRe matches the bounds a page states as prose rather than as
	// rows. The models Vertex serves for other labs state theirs among the
	// per-region quotas, as "65,536 maximum output, 163,840 context length".
	specQuotaRe = regexp.MustCompile(
		`(?i)([\d][\d,]*)\s*(maximum output|context length)`,
	)
	// modelHrefRe matches a link from the index to a page under the model
	// documentation.
	modelHrefRe = regexp.MustCompile(
		`href="/gemini-enterprise-agent-platform/models/([a-z0-9./-]+)"`,
	)
)

// Rows of the specification table this parser reads, named as Google heads
// them.
const (
	rowContext = "context window"
	rowMaxOut  = "maximum output tokens"
	// rowSequence is what an embedding model's page calls the same bound the
	// generative pages call a context window: the longest input it accepts.
	rowSequence = "maximum sequence length"
	// rowDimensions is the width of the vector an embedding model returns,
	// which its page states as a ceiling, "Up to 1,024".
	rowDimensions = "output dimensions"
)

// modelPageURLs derives the model pages the index links to, keeping the
// sections that describe models and dropping the guides filed beside them.
func modelPageURLs(index catalog.Document) []string {
	var urls []string
	for _, match := range modelHrefRe.FindAllStringSubmatch(
		string(index.Body),
		-1,
	) {
		path := match[1]
		if !inModelSection(path) {
			continue
		}
		url := modelPagePre + path
		if !slices.Contains(urls, url) {
			urls = append(urls, url)
		}
	}
	slices.Sort(urls)
	return urls
}

// inModelSection reports whether a path describes a model. A page filed under
// a section's own capabilities is a guide to a feature, not a model.
func inModelSection(path string) bool {
	if strings.Contains(path, "capabilities/") {
		return false
	}
	for _, section := range modelSections {
		if strings.HasPrefix(path, section) {
			return true
		}
	}
	return false
}

// applyModelPages reads every model page onto the models the billing catalog
// established.
//
// A page states the identifier the API answers to, which is the identifier a
// SKU is read as, so the two need no reconciling for the models Vertex sells
// plainly. A SKU can be finer than a model, though: Vertex prices Gemini 2.5
// Flash differently with thinking on and again above a length, and names each
// rate for the model plus the condition. Those reach the model's page through
// the same longest-prefix rule the plain ones match exactly by.
func (b *builder) applyModelPages(docs []catalog.Document) {
	pages := map[string]documented{}
	sources := map[string]string{}
	for _, doc := range docs {
		id, page, ok := readModelPage(string(doc.Body))
		if !ok {
			continue
		}
		pages[servedName(id)] = page
		sources[servedName(id)] = doc.URL
	}
	names := slices.Sorted(maps.Keys(pages))
	for _, id := range b.order {
		name := longestPrefix(id, names)
		if name == "" {
			continue
		}
		m := b.models[id]
		m.AddSource(sources[name])
		m.SetLimit(LimitContextWindow, pages[name].Context)
		m.SetLimit(LimitMaxOutputTokens, pages[name].MaxOut)
		m.AddList(ListFeatures, pages[name].Features...)
		m.AddList(ListInputModalities, pages[name].InputMod...)
		m.AddList(ListOutputModalities, pages[name].OutMod...)
		if width := pages[name].Dimensions; width > 0 {
			m.AddList(ListDimensions, strconv.FormatInt(width, 10))
		}
	}
}

// documented is what a model page states about one model.
type documented struct {
	Context int64
	MaxOut  int64
	// Dimensions is the width of the vector an embedding model returns, and is
	// zero for every model that returns something else.
	Dimensions int64
	Features   []string
	InputMod   []string
	OutMod     []string
}

// readModelPage reads one page's specification table, reporting whether the
// page carries one at all. Many pages under the same path are guides.
func readModelPage(body string) (string, documented, bool) {
	table := specTableRe.FindStringSubmatch(body)
	if table == nil {
		return "", documented{}, false
	}
	id := specText(firstOf(specIDRe, table[1]))
	if id == "" {
		return "", documented{}, false
	}
	var page documented
	for _, row := range specHeadRe.FindAllStringSubmatch(table[1], -1) {
		switch strings.ToLower(specText(row[1])) {
		case rowContext:
			page.Context = parseCount(specText(row[2]))
		case rowMaxOut:
			page.MaxOut = parseCount(specText(row[2]))
		case rowSequence:
			page.Context = parseCount(specText(row[2]))
		case rowDimensions:
			page.Dimensions = parseCount(specText(row[2]))
		}
	}
	readQuotas(&page, specText(table[1]))
	readModalities(&page, table[1])
	readFeatures(&page, table[1])
	return id, page, true
}

// readQuotas reads the bounds a page states as prose. The models Vertex serves
// for other labs carry no token-limit rows and state the same two figures
// among their per-region quotas instead, once per region and alike in each, so
// the first statement is taken and the rest agree with it.
func readQuotas(page *documented, text string) {
	for _, match := range specQuotaRe.FindAllStringSubmatch(text, -1) {
		n := parseCount(match[1])
		if strings.EqualFold(match[2], "context length") && page.Context == 0 {
			page.Context = n
			continue
		}
		if page.MaxOut == 0 {
			page.MaxOut = n
		}
	}
}

// readModalities records what a model takes and returns. A page names every
// modality it knows of and says of each whether it flows in, out, or both.
func readModalities(page *documented, table string) {
	for _, match := range specModalityRe.FindAllStringSubmatch(table, -1) {
		name, ok := modalityNames[strings.ToLower(specText(match[1]))]
		if !ok {
			continue
		}
		direction := strings.ToLower(specText(match[2]))
		if strings.Contains(direction, "input") {
			page.InputMod = appendNew(page.InputMod, name)
		}
		if strings.Contains(direction, "output") {
			page.OutMod = appendNew(page.OutMod, name)
		}
	}
}

// readFeatures records the capabilities a model has.
//
// A page lists the ones it lacks as plainly as the ones it has, marking each
// in a class of its own, so only the supported ones are kept. Only the
// capabilities row is read: the table carries a second list in the same shape
// below it, of the ways the model may be bought, and provisioned throughput is
// a billing arrangement rather than something the model can do.
func readFeatures(page *documented, table string) {
	row := specCapabilityRe.FindString(table)
	for _, match := range specFeatureRe.FindAllStringSubmatch(row, -1) {
		if !specSupportedRe.MatchString(match[1]) {
			continue
		}
		name := specText(firstOf(specFeatureNameRe, match[1]))
		page.Features = appendNew(page.Features, featureName(name))
	}
}

// featureWordRe matches whatever in a capability's name is not part of an
// identifier, including the brackets Google writes an abbreviation in.
var featureWordRe = regexp.MustCompile(`[^a-z0-9]+`)

// featureName rewrites a capability into the catalog's vocabulary.
func featureName(name string) string {
	head := strings.ToLower(strings.TrimSpace(name))
	if mapped, ok := featureNames[head]; ok {
		return mapped
	}
	return strings.Trim(featureWordRe.ReplaceAllString(head, "_"), "_")
}

// servingSuffixes are what the documentation appends to a model's name that
// the billing catalog leaves off. They say how a model is served or which
// tuning of it is offered, not which model it is: the catalog's deepseek-v3.2
// is the page's deepseek-v3.2-maas, and its llama-3.3-70b the page's
// llama-3.3-70b-instruct-maas.
var servingSuffixes = []string{"-maas", "-preview", "-instruct", "-it"}

// servedName strips those suffixes, so that the two documents name a model the
// same way. Stripping is repeated because a page can carry two of them.
func servedName(id string) string {
	name := strings.ToLower(strings.TrimSpace(id))
	for changed := true; changed; {
		changed = false
		for _, suffix := range servingSuffixes {
			if trimmed, ok := strings.CutSuffix(name, suffix); ok {
				name, changed = trimmed, true
			}
		}
	}
	return name
}

// longestPrefix returns the longest name that id equals or extends, so a SKU
// naming a condition as well as a model still reaches the model.
func longestPrefix(id string, names []string) string {
	lower, best := strings.ToLower(id), ""
	for _, name := range names {
		if lower != name && !strings.HasPrefix(lower, name+"-") {
			continue
		}
		if len(name) > len(best) {
			best = name
		}
	}
	return best
}

// appendNew adds a value not already present.
func appendNew(items []string, value string) []string {
	if value == "" || slices.Contains(items, value) {
		return items
	}
	return append(items, value)
}

// parseCount reads a grouped quantity such as "1,048,576".
func parseCount(value string) int64 {
	match := specCountRe.FindStringSubmatch(value)
	if match == nil {
		return 0
	}
	n, err := strconv.ParseInt(strings.ReplaceAll(match[1], ",", ""), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// firstOf returns the first capture of re, or the empty string.
func firstOf(re *regexp.Regexp, body string) string {
	if match := re.FindStringSubmatch(body); match != nil {
		return match[1]
	}
	return ""
}

// specText strips markup and collapses whitespace.
func specText(html string) string {
	return strings.Join(
		strings.Fields(specTagRe.ReplaceAllString(html, " ")),
		" ",
	)
}
