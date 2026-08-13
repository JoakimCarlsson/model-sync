package elevenlabs

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// cardHeadingRe matches the heading of one flagship card, which is the display
// name of a model linked to its section further down the page.
var cardHeadingRe = regexp.MustCompile(`^####\s+\[([^\]]+)\]\([^)]*\)\s*$`)

// headingRe matches any heading, which is what ends a card.
var headingRe = regexp.MustCompile(`^#{1,6}\s`)

// applyCards reads the flagship cards at the top of the models page.
//
// They are the only place ElevenLabs states a display name or says what a model
// can do. A card heads a model by name and follows it with a paragraph per
// capability; the first of those describes the model and the rest are what it
// offers. Only the flagship models have one, so the rest keep no name.
func (b *builder) applyCards(doc catalog.Document) {
	for _, c := range scanCardSections(string(doc.Body)) {
		id, ok := b.identify(c.Name)
		if !ok {
			continue
		}
		m := b.models[id]
		m.AddSource(doc.URL)
		if m.Name == "" {
			m.Name = c.Name
		}
		m.AddList(ListCapabilities, c.Capabilities...)
	}
}

// identify reports which model a card's name belongs to.
//
// The name reduces to the identifier by punctuation alone — Eleven Flash v2.5
// is eleven_flash_v2_5 — except that ElevenLabs drops its own name from some
// identifiers and not others, so Eleven Music v2 is metered as music_v2. Both
// readings are tried and only one that names a model already listed is used.
func (b *builder) identify(name string) (string, bool) {
	slug := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		}
		return '_'
	}, name)
	for _, candidate := range []string{
		slug,
		strings.TrimPrefix(slug, "eleven_"),
	} {
		if _, ok := b.models[candidate]; ok {
			return candidate, true
		}
	}
	return "", false
}

// flagship is one flagship card.
type flagship struct {
	Name         string
	Capabilities []string
}

// scanCardSections walks the page and returns every flagship card, taking the
// paragraphs between one card's heading and the next as its own.
func scanCardSections(body string) []flagship {
	var (
		out     []flagship
		current *flagship
		first   bool
	)
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if match := cardHeadingRe.FindStringSubmatch(line); match != nil {
			out = append(out, flagship{Name: clean(match[1])})
			current, first = &out[len(out)-1], true
			continue
		}
		if headingRe.MatchString(line) {
			current = nil
			continue
		}
		if current == nil || line == "" || strings.HasPrefix(line, "<") {
			continue
		}
		if first {
			first = false
			continue
		}
		current.Capabilities = append(current.Capabilities, clean(line))
	}
	return out
}
