package openrouter

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// categories are the subject listings OpenRouter publishes. The set is not
// prose anywhere: the models endpoint rejects an unknown category with the
// list of the ones it accepts, and this is that list.
var categories = []string{
	"academia",
	"finance",
	"health",
	"legal",
	"marketing",
	"marketing/seo",
	"programming",
	"roleplay",
	"science",
	"technology",
	"translation",
	"trivia",
}

// categoryURL addresses one subject listing.
func categoryURL(category string) string {
	return modelsBase + "?category=" +
		strings.ReplaceAll(category, "/", "%2F")
}

// categoryOf recovers the category a listing URL asked for.
func categoryOf(url string) string {
	_, query, ok := strings.Cut(url, "?category=")
	if !ok {
		return ""
	}
	return strings.ReplaceAll(query, "%2F", "/")
}

// applyCategory records that a model is one of the models OpenRouter lists
// under a subject.
//
// The listing is short, twenty models for each of the twelve subjects, and
// OpenRouter states nothing about how it is chosen or what the order means. So
// the membership is recorded and the position is not: that a model appears
// under "programming" is what the document says, and that it appears fourth is
// a fact about the ranking rather than about the model.
func (b *builder) applyCategory(doc catalog.Document) error {
	category := categoryOf(doc.URL)
	if category == "" {
		return nil
	}
	var list listing
	if err := json.Unmarshal(doc.Body, &list); err != nil {
		return fmt.Errorf("decode %s: %w", doc.URL, err)
	}
	for _, e := range list.Data {
		m, ok := b.models[e.ID]
		if !ok {
			continue
		}
		m.AddList(ListCategories, category)
		m.AddSource(doc.URL)
	}
	return nil
}

// isCategoryURL reports whether a document is a subject listing.
func isCategoryURL(url string) bool {
	return strings.HasPrefix(url, modelsBase+"?category=")
}
