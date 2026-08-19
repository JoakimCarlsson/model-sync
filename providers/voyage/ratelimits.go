package voyage

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/joakimcarlsson/model-sync/catalog"
)

var (
	// seriesRowRe matches the one row of the rate limit table that names a
	// generation of models rather than the models in it.
	seriesRowRe = regexp.MustCompile(
		`(?i)^voyage 1 & 2 series embedding models$`,
	)
	// seriesMemberRe matches an identifier belonging to that generation.
	// Voyage numbered its first two generations 01 and 02 before dropping the
	// leading zero, and suffixed the instruction-tuned variants.
	seriesMemberRe = regexp.MustCompile(
		`^voyage-(?:[a-z]+-)?(?:01|02|1|2)(?:-instruct)?$`,
	)
	// tierMultiplierRe matches what a usage tier multiplies the first tier's
	// limits by.
	tierMultiplierRe = regexp.MustCompile(`(?i)^(\d+)x tier 1$`)
)

// applyRateLimits reads the rate limit page.
//
// Voyage states one pair of limits per model for the first usage tier and
// expresses the higher tiers as multiples of it, which is also how its own
// worked example states them, so the multiples are recorded as their own keys
// rather than left for a consumer to work out.
func (b *builder) applyRateLimits(doc catalog.Document) {
	body := string(doc.Body)
	tiers := usageTiers(body)
	for _, t := range scanTables(body, doc.URL) {
		idCol := columnOf(t.Headers, "models")
		tpmCol := columnOf(t.Headers, "basic tpm")
		rpmCol := columnOf(t.Headers, "basic rpm")
		if idCol < 0 || tpmCol < 0 || rpmCol < 0 {
			continue
		}
		for _, row := range t.Rows {
			tpm := parseCount(cellAt(row, tpmCol))
			rpm := parseCount(cellAt(row, rpmCol))
			for _, m := range b.rateLimited(cellAt(row, idCol)) {
				m.AddSource(t.Source)
				applyTiers(m, tiers, LimitTPM, tpm)
				applyTiers(m, tiers, LimitRPM, rpm)
			}
		}
	}
}

// rateLimited returns the models one row of the rate limit table applies to.
//
// Every row but one names its models outright. The remaining row names a
// generation, "voyage 1 & 2 Series embedding models", and is expanded to the
// embedding models whose identifiers carry those generation numbers, which is
// the only reading of the phrase the identifiers admit.
func (b *builder) rateLimited(cell string) []*catalog.Model {
	var out []*catalog.Model
	for _, name := range splitModels(cell) {
		if !seriesRowRe.MatchString(name) {
			if m, ok := b.models[name]; ok {
				out = append(out, m)
			}
			continue
		}
		for _, id := range b.order {
			m := b.models[id]
			if m.Kind == KindEmbedding && seriesMemberRe.MatchString(id) {
				out = append(out, m)
			}
		}
	}
	return out
}

// usageTiers returns the multiplier each tier above the first applies, keyed
// by tier number.
func usageTiers(body string) map[int]int64 {
	out := map[int]int64{}
	for _, t := range scanTables(body, "") {
		tierCol := columnOf(t.Headers, "usage tier")
		limitCol := columnOf(t.Headers, "rate limits")
		if tierCol < 0 || limitCol < 0 {
			continue
		}
		for _, row := range t.Rows {
			tier, err := strconv.Atoi(clean(cellAt(row, tierCol)))
			if err != nil {
				continue
			}
			match := tierMultiplierRe.FindStringSubmatch(
				clean(cellAt(row, limitCol)),
			)
			if match == nil {
				continue
			}
			out[tier] = parseCount(match[1])
		}
	}
	return out
}

// applyTiers records a limit for the first usage tier and for every tier
// Voyage states a multiple of it for.
func applyTiers(
	m *catalog.Model,
	tiers map[int]int64,
	key string,
	value int64,
) {
	if value == 0 {
		return
	}
	m.SetLimit(key, value)
	for tier, multiplier := range tiers {
		m.SetLimit(tierKey(key, tier), value*multiplier)
	}
}

// tierKey names the limit a model has at one usage tier.
func tierKey(key string, tier int) string {
	return fmt.Sprintf("%s_tier_%d", key, tier)
}
