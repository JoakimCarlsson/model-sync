package elevenlabs

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// currency is the only currency ElevenLabs quotes.
const currency = "USD"

// Metrics ElevenLabs bills on. Audio submitted and audio generated are not one
// metric, because transcribing an hour of recording and composing an hour of
// music count opposite ends of the request.
const (
	// MetricSpeech counts the characters of text synthesised.
	MetricSpeech catalog.Metric = "speech"
	// MetricAudio counts the audio a request carries in.
	MetricAudio catalog.Metric = "audio"
	// MetricAudioOutput counts the audio a request produces.
	MetricAudioOutput catalog.Metric = "audio_output"
)

// Units ElevenLabs quotes amounts against.
const (
	UnitPer1KChars catalog.Unit = "per_1k_characters"
	UnitPerHour    catalog.Unit = "per_hour"
	UnitPerMinute  catalog.Unit = "per_minute"
)

// family is one card on the API pricing page and the models it prices.
type family struct {
	// Card is the heading, lowercased, that the rate is quoted under.
	Card string
	// Metric is what the rate counts, which the card states only in prose.
	Metric catalog.Metric
	// Fragments are the parts of an identifier the card's models share.
	Fragments []string
}

// families maps the cards onto identifiers. ElevenLabs quotes a rate for a
// family and names the family the way it markets it, never by identifier, so a
// card has to be matched to the models it covers. A model belongs to the family
// matching the longest fragment of its identifier, which is what keeps Scribe
// v2 Realtime out of the Scribe v2 card.
//
// The cards this leaves out — Speech Engine, Voice Isolator and both Dubbing
// versions — are products built on models rather than models, and the models
// page names none of them.
var families = []family{
	{
		Card:      "flash / turbo",
		Metric:    MetricSpeech,
		Fragments: []string{"eleven_flash", "eleven_turbo"},
	},
	{
		Card:      "multilingual v2 / v3",
		Metric:    MetricSpeech,
		Fragments: []string{"eleven_multilingual_v2", "eleven_v3"},
	},
	{
		Card:      "scribe v2",
		Metric:    MetricAudio,
		Fragments: []string{"scribe_v2"},
	},
	{
		Card:      "scribe v2 realtime",
		Metric:    MetricAudio,
		Fragments: []string{"scribe_v2_realtime"},
	},
	{
		Card:      "music",
		Metric:    MetricAudioOutput,
		Fragments: []string{"music_v"},
	},
	{
		Card:      "voice changer",
		Metric:    MetricAudio,
		Fragments: []string{"_sts_"},
	},
	{
		Card:      "sound effects",
		Metric:    MetricAudioOutput,
		Fragments: []string{"text_to_sound"},
	},
}

// cardRe matches one pricing card: its title, the product line under it, and
// the amount with the denominator it is quoted against.
var cardRe = regexp.MustCompile(
	`(?is)<h3[^>]*>([^<]{1,60})</h3>\s*<p[^>]*>([^<]{1,40})</p>` +
		`(.{0,600}?)<p[^>]*>\s*Price per ([^<]{1,40})</p>`,
)

// cardAmountRe matches the amount inside a card.
var cardAmountRe = regexp.MustCompile(`>\s*\$\s*([\d,]*\.?\d+)\s*<`)

// card is one rate read from the pricing page.
type card struct {
	Title       string
	Product     string
	Amount      float64
	Denominator string
}

// scanCards reads every priced card on the page.
func scanCards(body string) []card {
	var out []card
	for _, match := range cardRe.FindAllStringSubmatch(body, -1) {
		amount, ok := parseAmount(match[3])
		if !ok {
			continue
		}
		out = append(out, card{
			Title:       normalize(match[1]),
			Product:     normalize(match[2]),
			Amount:      amount,
			Denominator: normalize(match[4]),
		})
	}
	return out
}

// parseAmount reads the first amount in a fragment of markup.
func parseAmount(fragment string) (float64, bool) {
	match := cardAmountRe.FindStringSubmatch(fragment)
	if match == nil {
		return 0, false
	}
	value, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", ""), 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

// normalize lowercases a cell and collapses its whitespace.
func normalize(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(clean(text)), " "))
}

// unitFor maps a card's denominator onto a unit.
func unitFor(denominator string) (catalog.Unit, bool) {
	switch {
	case strings.HasPrefix(denominator, "1k character"):
		return UnitPer1KChars, true
	case strings.HasPrefix(denominator, "hour"):
		return UnitPerHour, true
	case strings.HasPrefix(denominator, "minute"):
		return UnitPerMinute, true
	}
	return "", false
}

// familyOf returns the family pricing a model, which is the one matching the
// longest fragment of its identifier.
func familyOf(id string) (family, bool) {
	var (
		best  family
		found bool
		match int
	)
	for _, f := range families {
		for _, fragment := range f.Fragments {
			if !strings.Contains(id, fragment) || len(fragment) <= match {
				continue
			}
			best, found, match = f, true, len(fragment)
		}
	}
	return best, found
}

// noteNoCard records that no card on the API pricing page covers a model, which
// is the whole of why it carries no rate. Without it a served model with no
// amount reads as a free one.
const noteNoCard = "no card on the API pricing page covers this model"

// noteUnpriced marks the served models the cards leave out.
//
// Two kinds fall outside every card. Voice design has no card at all: the
// pricing page describes the product without quoting an amount for it. The
// first generation speech model has none either, because the card covering the
// multilingual line is headed for its second and third versions and this parser
// will not read a rate quoted for one version onto another.
//
// A deprecated model is skipped. ElevenLabs drops a model from its cards when it
// withdraws it, so the absence there is the withdrawal and not an omission.
func (b *builder) noteUnpriced() {
	for _, id := range b.order {
		m := b.models[id]
		if len(m.Prices) > 0 || m.Attrs[AttrState] == StateDeprecated {
			continue
		}
		m.AddNote(noteNoCard)
	}
}

// applyPricing reads the API pricing page onto the models the models page
// established, because a card names a family and never an identifier.
func (b *builder) applyPricing(doc catalog.Document) {
	for _, c := range scanCards(string(doc.Body)) {
		unit, ok := unitFor(c.Denominator)
		if !ok {
			continue
		}
		for _, id := range b.order {
			f, ok := familyOf(id)
			if !ok || f.Card != c.Title {
				continue
			}
			m := b.models[id]
			m.AddSource(doc.URL)
			m.AddPrice(catalog.Price{
				Metric:   f.Metric,
				Unit:     unit,
				Amount:   c.Amount,
				Currency: currency,
			})
		}
	}
}
