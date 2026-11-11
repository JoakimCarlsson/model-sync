package fireworks

import (
	"slices"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// card is what the index states about one model.
type card struct {
	URL     string
	Name    string
	Context int64
}

// indexCards reads the library index, returning one card per model it lists.
//
// The rendered index shows only the first handful of cards and loads the rest
// as the reader scrolls, so the markup no longer names the whole library. The
// page carries every entry in the data it renders from, one object per model
// with the path of its page, its title and its exact context window, and that
// data is what is read. An entry without a path or a title is skipped.
func indexCards(body string) []card {
	var out []card
	for _, entry := range flightObjects(body, isIndexEntry) {
		slug, _ := entry["slug"].(string)
		name, _ := entry["modelName"].(string)
		slug, name = strings.Trim(slug, "/"), strings.TrimSpace(name)
		if slug == "" || name == "" {
			continue
		}
		c := card{URL: libraryHost + modelPathPrefix + slug, Name: name}
		if window, ok := entry["contextLength"].(string); ok {
			if n, err := strconv.ParseInt(window, 10, 64); err == nil {
				c.Context = n
			}
		}
		out = append(out, c)
	}
	return out
}

// isIndexEntry reports whether an object of the index's data is the entry of
// one model, which is the object naming both the page and the title.
func isIndexEntry(obj map[string]any) bool {
	_, hasSlug := obj["slug"].(string)
	_, hasName := obj["modelName"].(string)
	return hasSlug && hasName
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
