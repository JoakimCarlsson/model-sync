package google

import (
	"regexp"
	"slices"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// cardPre prefixes the model card each Gemini model page links to, which
// DeepMind publishes rather than the Gemini API documentation.
const cardPre = "https://deepmind.google/"

var (
	// cardHrefRe matches the link a model page's model card row carries.
	cardHrefRe = regexp.MustCompile(
		`href="(https://deepmind\.google/models/model-cards/[^"]+)"`,
	)
	// cutoffRe matches the one sentence a model card states a knowledge cutoff
	// in. It is written among the model's known limitations rather than as a
	// field, and the sentence goes on to name a second, earlier date for the
	// domains the model was not refreshed in, so only the date the cutoff is
	// stated as is read.
	cutoffRe = regexp.MustCompile(
		`(?is)knowledge cutoff(?: date)? for [^<]{1,80}? (?:is|was) ` +
			`([A-Z][a-z]+ \d{4})`,
	)
)

// applyModelCard reads one model card for the knowledge cutoff, which is the
// only document Google states one in and the only field of a card that is not
// prose about the model's evaluation or its safety testing.
//
// A card is attached by the address the model page linked it at, since a card
// names its model in a title rather than by endpoint and one card covers a
// whole family.
func (b *builder) applyModelCard(doc catalog.Document) {
	cutoff := isoDate(first(cutoffRe, string(doc.Body)))
	if cutoff == "" {
		return
	}
	for _, id := range b.order {
		m := b.models[id]
		if m.Attrs[AttrModelCard] != doc.URL {
			continue
		}
		m.AddSource(doc.URL)
		m.SetAttr(AttrKnowledgeCutoff, cutoff)
	}
}

// cardURLs returns the model cards a set of model pages link to.
func cardURLs(pages []catalog.Document) []string {
	var urls []string
	for _, page := range pages {
		for _, match := range cardHrefRe.FindAllStringSubmatch(
			string(page.Body),
			-1,
		) {
			if !slices.Contains(urls, match[1]) {
				urls = append(urls, match[1])
			}
		}
	}
	return urls
}
