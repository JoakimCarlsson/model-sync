package perplexity

import (
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// LimitContextWindow is the only bound Perplexity states for a model, and it
// states it for its own models only.
const LimitContextWindow = "context_window"

// sonarModelPre prefixes the page each Sonar model has of its own.
const sonarModelPre = baseURL + "/docs/sonar/models/"

var (
	// sonarHrefRe matches a link from the Sonar index to one model's page.
	sonarHrefRe = regexp.MustCompile(`/docs/sonar/models/([a-z0-9-]+)`)
	// contextRe matches the context window, which a model page states in a
	// heading rather than in a table.
	contextRe = regexp.MustCompile(`(?i)(\d+)\s*([km])?\s*context length`)
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
	match := contextRe.FindStringSubmatch(string(doc.Body))
	if match == nil {
		return
	}
	m.AddSource(doc.URL)
	m.SetLimit(LimitContextWindow, parseTokens(match[1], match[2]))
}

// parseTokens reads a quantity written with a thousands or millions suffix.
func parseTokens(digits, suffix string) int64 {
	n, err := strconv.ParseInt(digits, 10, 64)
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
