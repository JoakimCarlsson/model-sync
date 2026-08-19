package openai

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// What the moderation guide states. The moderation model's page lists one
// modality and a snapshot; what the model actually detects, and how large an
// image it will read, are written only here.
const (
	ListModerationCategories   = "moderation_categories"
	LimitMaxImageFileMegabytes = "max_image_file_megabytes"
)

// ModerationGuideURL is where the categories are enumerated.
const ModerationGuideURL = baseURL + "/api/docs/guides/moderation.md"

// categoriesHeading heads the section listing what the model detects.
const categoriesHeading = "## review supported categories"

var (
	// moderationCellRe matches a category cell, which the guide writes as the
	// first cell of a raw HTML row with the category name in backticks.
	moderationCellRe = regexp.MustCompile("(?is)<td>`([a-z/_-]+)`</td>")
	// moderationModelRe matches the sentence naming the model the section
	// describes, which also states the size bound below.
	moderationModelRe = regexp.MustCompile(
		"(?s)send only\\s+images[^.]*?to the `([\\w.-]+)` model",
	)
	// moderationImageSizeRe matches the bound on an image sent for moderation.
	moderationImageSizeRe = regexp.MustCompile(
		`(?i)image files are limited to (\d+)\s*MB`,
	)
)

// applyModerationGuide records what the moderation model detects.
//
// OpenAI's model page for it lists image_input under supported features and
// nothing else, which the catalog reads as a modality rather than a
// capability, leaving the model with no features at all. The categories are
// the answer to what it does, and they are here, in an HTML table the section
// introduces by naming the model it holds for. That name is what the
// categories are recorded against, so a second moderation model would not
// silently inherit them.
func (b *builder) applyModerationGuide(doc catalog.Document) {
	body := string(doc.Body)
	named := moderationModelRe.FindStringSubmatch(body)
	if named == nil {
		return
	}
	m := b.models[named[1]]
	if m == nil {
		return
	}
	m.AddSource(doc.URL)
	if match := moderationImageSizeRe.FindStringSubmatch(body); match != nil {
		m.SetLimit(LimitMaxImageFileMegabytes, parseCount(match[1]))
	}
	section := sectionAfterPrefix(body, categoriesHeading)
	for _, match := range moderationCellRe.FindAllStringSubmatch(section, -1) {
		m.AddList(ListModerationCategories, strings.TrimSpace(match[1]))
	}
}
