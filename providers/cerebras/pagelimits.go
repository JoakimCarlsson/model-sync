package cerebras

import (
	"html"
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// pageRateFields map a field of a model page's rate limit entry onto the key
// it fills, before the plan's suffix is appended.
//
// The page calls the token rate an input token rate where the rate limit page
// heads the same number TPM. The canonical key is used, because it is one
// number stated twice and not two numbers.
var pageRateFields = map[string]string{
	"requestsPerMin":    LimitRequestsPerMinute,
	"inputTokensPerMin": LimitTokensPerMinute,
	"dailyTokens":       LimitTokensPerDay,
	"imagesPerRequest":  LimitImagesPerRequest,
}

// entryRe matches one entry of a braced list of objects, which is how a model
// page states its limits, one object per plan.
var entryRe = regexp.MustCompile(`(?s)\{(.*?)\}`)

// applyPageRateLimits reads the rate limits a model page states for itself.
//
// The page repeats the rate limit page for the two published plans and adds
// the one bound only it states, how many images a single request may carry.
func applyPageRateLimits(m *catalog.Model, value string) {
	for _, entry := range entryRe.FindAllStringSubmatch(value, -1) {
		fields := map[string]string{}
		for _, f := range fieldRe.FindAllStringSubmatch(entry[1], -1) {
			fields[f[1]] = f[2]
		}
		suffix, ok := tierSuffixes[strings.ToLower(fields["tier"])]
		if !ok {
			continue
		}
		for name, key := range pageRateFields {
			m.SetLimit(key+suffix, parseCount(fields[name]))
		}
	}
}

// applyKnownLimitations records what a model page says a model does that no
// capability of it states.
//
// These are the caveats Cerebras attaches to a model rather than to the API:
// which endpoint an input works on, which reasoning formats are refused, which
// parameter is unsafe on this model, whether constrained decoding is honoured.
// None of them reduces to a capability, so each is kept as the sentence
// Cerebras wrote it as.
func applyKnownLimitations(m *catalog.Model, value string) {
	for _, span := range spanRe.FindAllStringSubmatch(value, -1) {
		if note := plainText(span[1]); note != "" {
			m.AddNote(note)
		}
	}
}

// spanRe matches one caveat, which a page writes as an element so it may carry
// a link or a marked-up parameter name inside it.
var spanRe = regexp.MustCompile(`(?s)<span[^>]*>(.*?)</span>`)

// plainText reduces marked-up prose to the sentence it reads as.
func plainText(value string) string {
	s := tagRe.ReplaceAllString(value, "")
	s = html.UnescapeString(s)
	return strings.Join(strings.Fields(s), " ")
}
