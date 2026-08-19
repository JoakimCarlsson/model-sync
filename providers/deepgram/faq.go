package deepgram

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// noteLegacyRate says where a rate that no table holds came from, and carries
// the qualifications the answer states: that the rate is what an existing
// deployment keeps paying, and that a growth plan pays less by an amount
// Deepgram describes rather than quotes.
const noteLegacyRate = "stated in the pricing page's questions as the " +
	"unchanged rate for existing deployments; growth plan rates are " +
	"described there as about 12.5% lower without being quoted"

var (
	// ldRe matches the structured description of the page Deepgram publishes
	// beside the rendered one.
	ldRe = regexp.MustCompile(
		`(?is)<script[^>]*application/ld\+json[^>]*>(.*?)</script>`,
	)
	// legacyRateRe matches a model and its hourly rate as the answer about the
	// older models writes them, "Nova-2 streaming at $0.35/hour".
	legacyRateRe = regexp.MustCompile(
		`(?i)\b([A-Z][A-Za-z0-9-]*)\s+(streaming\s+)?at\s+` +
			`<strong>\$([\d.]+)/hour</strong>`,
	)
)

// faqPage is the shape Deepgram publishes its questions in.
type faqPage struct {
	Type string `json:"@type"`
	// MainEntity is a list of questions, which Deepgram nests one level
	// deeper than the schema does.
	MainEntity []json.RawMessage `json:"mainEntity"`
}

// faqQuestion is one question and the answer under it.
type faqQuestion struct {
	Name           string `json:"name"`
	AcceptedAnswer struct {
		Text string `json:"text"`
	} `json:"acceptedAnswer"`
}

// applyFAQ reads the questions published beside the pricing tables. They are
// where Deepgram states the rates of the models it no longer lists in a table:
// the tables price what it recommends, and the answer about older models
// prices what an existing deployment still runs on. The rates are recorded
// only against models the documentation names as options, so that a sentence
// mentioning a name is never enough to invent one.
func (b *builder) applyFAQ(doc catalog.Document) {
	for _, answer := range faqAnswers(string(doc.Body)) {
		b.applyLegacyRates(answer, doc.URL)
	}
}

// faqAnswers returns every answer the page publishes.
func faqAnswers(body string) []string {
	var out []string
	for _, match := range ldRe.FindAllStringSubmatch(body, -1) {
		var page faqPage
		if err := json.Unmarshal([]byte(match[1]), &page); err != nil {
			continue
		}
		if page.Type != "FAQPage" {
			continue
		}
		for _, entry := range page.MainEntity {
			out = append(out, questions(entry)...)
		}
	}
	return out
}

// questions reads one entry of the list, which Deepgram writes either as a
// question or as a list of them.
func questions(entry json.RawMessage) []string {
	var one faqQuestion
	if err := json.Unmarshal(entry, &one); err == nil {
		if one.AcceptedAnswer.Text != "" {
			return []string{one.AcceptedAnswer.Text}
		}
		return nil
	}
	var many []faqQuestion
	if err := json.Unmarshal(entry, &many); err != nil {
		return nil
	}
	out := make([]string, 0, len(many))
	for _, q := range many {
		out = append(out, q.AcceptedAnswer.Text)
	}
	return out
}

// applyLegacyRates records the hourly rates an answer states against the
// models it names. The answer names no plan, so the rate carries none: it
// says only that these are the rates that have not changed.
func (b *builder) applyLegacyRates(answer, source string) {
	for _, match := range legacyRateRe.FindAllStringSubmatch(answer, -1) {
		m, ok := b.models[slugID(match[1])]
		if !ok {
			continue
		}
		amount, err := strconv.ParseFloat(match[3], 64)
		if err != nil {
			continue
		}
		m.AddPrice(catalog.Price{
			Metric:   MetricAudio,
			Unit:     UnitPerHour,
			Amount:   amount,
			Currency: currency,
			Dims: catalog.Dims{}.With(
				DimDelivery,
				legacyDelivery(match[2]),
			),
			Note: noteLegacyRate,
		})
		m.AddSource(source)
	}
}

// legacyDelivery reads whether the answer said the rate is for a live
// connection. It says so for one model and leaves it unsaid for the others,
// which are then left without the dimension rather than given one.
func legacyDelivery(word string) string {
	if strings.TrimSpace(strings.ToLower(word)) == DeliveryStreaming {
		return DeliveryStreaming
	}
	return ""
}
