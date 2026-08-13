package groq

import (
	"path"
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// modelPagePre prefixes the page each model in the table links to.
const modelPagePre = baseURL + "/docs/model/"

// Enumeration keys the model pages populate.
const (
	ListFeatures         = "features"
	ListInputModalities  = "input_modalities"
	ListOutputModalities = "output_modalities"
)

// AttrModelCard records where the weights a model serves are published.
const AttrModelCard = "model_card_url"

// Headings a model page states its facts under, which it writes in capitals
// with the value on the line below.
const (
	headInput        = "INPUT"
	headOutput       = "OUTPUT"
	headCapabilities = "CAPABILITIES"
)

// featureNames map a capability a page names onto the catalog's vocabulary.
// Only the names that differ are listed; the rest keep Groq's own words with
// their spacing reduced to an identifier.
var featureNames = map[string]string{
	"tool use":           "function_calling",
	"json object mode":   "json_mode",
	"json schema mode":   "structured_outputs",
	"structured outputs": "structured_outputs",
	"reasoning":          "reasoning",
	"speech to text":     "transcription",
	"text to speech":     "speech",
	"prompt guard":       "moderation",
	"browser search":     "web_search",
	"code execution":     "code_execution",
}

// capabilityModalities map a capability that names a modality onto the
// modality it names. Groq marks a model that reads images with a vision
// capability, which every provider stating modalities calls an image input.
var capabilityModalities = map[string]string{
	"vision": "image",
}

// modalityNames map a modality a page names onto the catalog's vocabulary.
var modalityNames = map[string]string{
	"text":  "text",
	"image": "image",
	"audio": "audio",
	"video": "video",
}

var (
	// pageLinkRe matches a link from the table to one model's page.
	pageLinkRe = regexp.MustCompile(`\(/docs/model/([a-z0-9./-]+)\)`)
	// pageIDRe matches the identifier the page names under its heading, which
	// Groq writes alone in backticks.
	pageIDRe = regexp.MustCompile("(?m)^`([^`]+)`\\s*$")
	// pageLinkTextRe matches a linked capability, which is a name and the
	// guide that explains it.
	pageLinkTextRe = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)
	// pageCardRe matches the link to where the weights are published.
	pageCardRe = regexp.MustCompile(`\[Model card\]\(([^)]+)\)`)
)

// modelPageURLs derives the model pages the table links to.
func modelPageURLs(doc catalog.Document) []string {
	var urls []string
	for _, match := range pageLinkRe.FindAllStringSubmatch(
		string(doc.Body),
		-1,
	) {
		url := modelPagePre + match[1] + ".md"
		if !contains(urls, url) {
			urls = append(urls, url)
		}
	}
	return urls
}

// applyModelPage reads one model's page onto the model the table established.
//
// The page names the identifier the API answers to on a line of its own, which
// is the identifier the table keys the model by, so the two need no matching.
func (b *builder) applyModelPage(doc catalog.Document) {
	body := string(doc.Body)
	id := strings.TrimSpace(firstOf(pageIDRe, body))
	if id == "" {
		id = strings.TrimSuffix(path.Base(doc.URL), ".md")
	}
	m, ok := b.models[id]
	if !ok {
		return
	}
	m.AddSource(doc.URL)
	m.SetAttr(AttrModelCard, firstOf(pageCardRe, body))
	sections := readSections(body)
	addModalities(m, ListInputModalities, sections[headInput])
	addModalities(m, ListOutputModalities, sections[headOutput])
	for _, name := range splitList(sections[headCapabilities]) {
		key := strings.ToLower(strings.TrimSpace(name))
		if modality, ok := capabilityModalities[key]; ok {
			m.AddList(ListInputModalities, modality)
			continue
		}
		m.AddList(ListFeatures, featureName(name))
	}
}

// readSections returns the value under each heading the page states.
//
// A page is written as a heading in capitals, a blank line, and the value on
// one line of its own. Only that line is the value: what follows it is the
// next thing the page says, and the prose a page ends with carries no heading
// to close the last section off.
func readSections(body string) map[string]string {
	out := map[string]string{}
	heading := ""
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if isHeading(line) {
			heading = line
			continue
		}
		if heading == "" || line == "" {
			continue
		}
		if _, seen := out[heading]; !seen {
			out[heading] = line
		}
		heading = ""
	}
	return out
}

// isHeading reports whether a line is one of the page's capitalized headings.
// A heading carries no lower case letter and no markup.
func isHeading(line string) bool {
	if line == "" || strings.ContainsAny(line, "[]()`#*") {
		return false
	}
	return line == strings.ToUpper(line) && strings.ContainsAny(line, "ABC"+
		"DEFGHIJKLMNOPQRSTUVWXYZ")
}

// addModalities records every modality a section names.
func addModalities(m *catalog.Model, key, value string) {
	for _, name := range splitList(value) {
		if mapped, ok := modalityNames[strings.ToLower(name)]; ok {
			m.AddList(key, mapped)
		}
	}
}

// splitList divides a section that names several things, which Groq separates
// with commas and writes each of as a link when a guide explains it.
func splitList(value string) []string {
	var out []string
	for _, part := range strings.Split(
		pageLinkTextRe.ReplaceAllString(value, "$1"),
		",",
	) {
		if item := strings.TrimSpace(part); item != "" {
			out = append(out, item)
		}
	}
	return out
}

// featureName rewrites a capability into the catalog's vocabulary.
func featureName(name string) string {
	key := strings.ToLower(strings.TrimSpace(name))
	if mapped, ok := featureNames[key]; ok {
		return mapped
	}
	return strings.ReplaceAll(key, " ", "_")
}

// firstOf returns the first capture of re, or the empty string.
func firstOf(re *regexp.Regexp, body string) string {
	if match := re.FindStringSubmatch(body); match != nil {
		return match[1]
	}
	return ""
}

// contains reports whether items already holds value.
func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
