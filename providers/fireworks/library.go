package fireworks

import (
	"html"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// The model library's index is the only document naming every model Fireworks
// serves. Each card links the model's page, titles it, and states its context
// window as an exact figure, which the model's own page then rounds for
// display. These read one card.
var (
	cardHrefRe = regexp.MustCompile(
		`href="(/models/[A-Za-z0-9._-]+/[A-Za-z0-9._-]+)"`,
	)
	cardTitleRe = regexp.MustCompile(
		`<h3 class="min-h-\[3em\][^"]*">([^<]*)</h3>`,
	)
	cardContextRe = regexp.MustCompile(`([\d,]+) Context<`)
)

// card is what the index states about one model.
type card struct {
	URL     string
	Name    string
	Context int64
}

// indexCards reads the library index, returning one card per model page it
// links. A link the page carries for another reason, such as the banner
// announcing a release, has no title under it and is skipped.
func indexCards(body string) []card {
	spans := cardHrefRe.FindAllStringSubmatchIndex(body, -1)
	out := make([]card, 0, len(spans))
	for i, span := range spans {
		end := len(body)
		if i+1 < len(spans) {
			end = spans[i+1][0]
		}
		segment := body[span[1]:end]
		title := cardTitleRe.FindStringSubmatch(segment)
		if title == nil {
			continue
		}
		c := card{
			URL:  libraryHost + body[span[2]:span[3]],
			Name: strings.TrimSpace(html.UnescapeString(title[1])),
		}
		if m := cardContextRe.FindStringSubmatch(segment); m != nil {
			n, err := strconv.ParseInt(
				strings.ReplaceAll(m[1], ",", ""),
				10,
				64,
			)
			if err == nil {
				c.Context = n
			}
		}
		out = append(out, c)
	}
	return out
}

// applyIndex records what the library index states, which is a set of pages to
// read and, for most models, the exact context window their own page will only
// round.
//
// The index cannot key a model on its own. The identifier a model is called by
// is the path it is served under, and the index states only the address of its
// page, whose account segment is the label of the organization that published
// the model rather than the account the model is served from. So the cards are
// held against their addresses and claimed when the page behind each one is
// read.
func (b *builder) applyIndex(doc catalog.Document) {
	for _, c := range indexCards(string(doc.Body)) {
		if _, seen := b.cards[c.URL]; seen {
			continue
		}
		b.cards[c.URL] = c
	}
}

// libraryPageURLs returns the page of every model the index links, in a stable
// order.
func libraryPageURLs(index catalog.Document) []string {
	var urls []string
	for _, c := range indexCards(string(index.Body)) {
		if !slices.Contains(urls, c.URL) {
			urls = append(urls, c.URL)
		}
	}
	slices.Sort(urls)
	return urls
}
