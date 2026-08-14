package perplexity

import (
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Bounds Perplexity states for a model.
const (
	// LimitContextWindow is what a model may be given.
	LimitContextWindow = "context_window"
	// LimitMaxOutputTokens is what it may answer with.
	LimitMaxOutputTokens = "max_output_tokens"
)

// SonarAPIURL is the reference for the Sonar chat completions endpoint. It is
// the only document bounding what a Sonar model may answer with, which it
// states as the ceiling on the request's max_tokens, and it enumerates the
// models the endpoint takes, which is what the ceiling applies to.
const SonarAPIURL = baseURL + "/api-reference/sonar-post.md"

// sonarModelPre prefixes the page each Sonar model has of its own.
const sonarModelPre = baseURL + "/docs/sonar/models/"

var (
	// sonarHrefRe matches a link from the Sonar index to one model's page.
	sonarHrefRe = regexp.MustCompile(`/docs/sonar/models/([a-z0-9-]+)`)
	// contextRe matches the context window, which is stated in prose rather
	// than in a table: a model page heads a card with it, and the Agent API's
	// model page writes it into a sentence.
	contextRe = regexp.MustCompile(
		`(?i)(\d+)\s*([km])?(?:-token)?\s*context\s+(?:length|window)`,
	)
	// maxTokensRe matches the ceiling the request schema puts on a completion.
	maxTokensRe = regexp.MustCompile(
		`(?s)max_tokens:\s*\n\s*anyOf:.*?maximum:\s*(\d+)`,
	)
	// modelEnumRe matches the identifiers the request schema accepts, which is
	// how the reference says which models it is describing.
	modelEnumRe = regexp.MustCompile(
		`(?s)\n\s+model:\n.*?\n\s+enum:\n((?:\s+-\s+[^\n]+\n)+)`,
	)
)

// sonarModelURLs derives the Sonar model pages the index links to.
func sonarModelURLs(index catalog.Document) []string {
	var urls []string
	for _, match := range sonarHrefRe.FindAllStringSubmatch(
		string(index.Body),
		-1,
	) {
		url := sonarModelPre + match[1] + ".md"
		if !slices.Contains(urls, url) {
			urls = append(urls, url)
		}
	}
	return urls
}

// applySonarPage reads the context window off one Sonar model's page.
//
// The page addresses the model by the identifier the API answers to, which is
// also the page's own address, so the two need no matching. Only Perplexity's
// own models have such a page: for the models it brokers it links out to the
// lab that made them rather than restating what they hold.
func (b *builder) applySonarPage(doc catalog.Document) {
	id := strings.TrimSuffix(path.Base(doc.URL), ".md")
	m, ok := b.models[id]
	if !ok {
		return
	}
	m.AddSource(doc.URL)
	if !slices.Contains(b.sonar, id) {
		b.sonar = append(b.sonar, id)
	}
	if readsReasoning(string(doc.Body)) {
		m.AddList(ListFeatures, featureReasoning)
	}
	match := contextRe.FindStringSubmatch(string(doc.Body))
	if match == nil {
		return
	}
	m.SetLimit(LimitContextWindow, parseTokens(match[1], match[2]))
}

// applySonarReference reads the output ceiling off the Sonar API reference.
//
// The ceiling is a property of the endpoint rather than of any one model, so
// it is recorded against every model the endpoint's schema will accept, which
// the schema itself enumerates.
func (b *builder) applySonarReference(doc catalog.Document) {
	body := string(doc.Body)
	match := maxTokensRe.FindStringSubmatch(body)
	if match == nil {
		return
	}
	ceiling, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil {
		return
	}
	for _, id := range referenceModels(body) {
		m, ok := b.models[id]
		if !ok {
			continue
		}
		m.AddSource(doc.URL)
		m.SetLimit(LimitMaxOutputTokens, ceiling)
	}
}

// referenceModels returns the identifiers a request schema enumerates.
func referenceModels(body string) []string {
	match := modelEnumRe.FindStringSubmatch(body)
	if match == nil {
		return nil
	}
	var ids []string
	for _, line := range strings.Split(match[1], "\n") {
		id := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// parseTokens reads a quantity written with a thousands or millions suffix.
func parseTokens(digits, suffix string) int64 {
	n, err := strconv.ParseInt(strings.ReplaceAll(digits, ",", ""), 10, 64)
	if err != nil {
		return 0
	}
	switch strings.ToLower(suffix) {
	case "k":
		n *= 1_000
	case "m":
		n *= 1_000_000
	}
	return n
}
