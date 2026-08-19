package perplexity

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// RateLimitsURL is the usage tier page. Perplexity rate limits each of its
// four APIs on its own terms and states every one of them by tier, which is
// the only document putting a number on what a model will serve.
const RateLimitsURL = baseURL + "/docs/admin/rate-limits-usage-tiers.md"

var (
	// tierRe matches the tier or tiers one row of a limit table applies to,
	// which Perplexity writes either singly or as a range.
	tierRe = regexp.MustCompile(`(?i)tiers?\s*(\d)(?:\s*[^\d]{1,3}\s*(\d))?`)
	// rateRe matches the leading quantity of a limit cell, which Perplexity
	// writes with the unit after it.
	rateRe = regexp.MustCompile(`^([\d,]+)`)
)

// applyRateLimits reads the usage tier page.
//
// The page has one section per API and never names a model outside the Sonar
// section, so which models a table binds is read from the section it sits
// under: the Agent API's limits go to every model that API serves, the
// Embeddings API's to every embedding model, and the Search API's to the
// product the pricing page created for it. The Sonar section is the exception
// and tabulates its models by name, one table per tier.
func (b *builder) applyRateLimits(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		section := strings.ToLower(t.Section)
		switch {
		case strings.HasPrefix(section, "agent api rate limits"):
			b.applyTierLimits(t, b.agent)
		case strings.HasPrefix(section, "sonar api rate limits"):
			b.applySonarTierTable(t)
		case strings.HasPrefix(section, "search api rate limits"):
			b.applySearchLimits(t)
		case strings.HasPrefix(section, "embeddings api rate limits"):
			b.applyTierLimits(t, b.embeddingsFor(t))
		}
	}
}

// applyTierLimits reads a table whose rows are usage tiers and whose columns
// are the ceilings that tier buys, onto every model given.
func (b *builder) applyTierLimits(t table, ids []string) {
	minute := columnOf(t.Headers, "requests per minute")
	second := columnOf(t.Headers, "qps (queries per second)")
	if columnOf(t.Headers, "tier") < 0 || (minute < 0 && second < 0) {
		return
	}
	for _, row := range t.Rows {
		for _, tier := range rowTiers(cellAt(row, 0)) {
			b.setTierLimit(ids, t, LimitRPMTierPrefix, tier, row, minute)
			b.setTierLimit(ids, t, LimitRPSTierPrefix, tier, row, second)
		}
	}
}

// setTierLimit records one cell of a tier table against every model given.
func (b *builder) setTierLimit(
	ids []string,
	t table,
	prefix string,
	tier int,
	row []string,
	at int,
) {
	if at < 0 {
		return
	}
	value, ok := parseRate(cellAt(row, at))
	if !ok {
		return
	}
	key := prefix + strconv.Itoa(tier)
	for _, id := range ids {
		m, ok := b.models[id]
		if !ok {
			continue
		}
		m.AddSource(t.Source)
		m.SetLimit(key, value)
	}
}

// applySonarTierTable reads one tier's worth of Sonar limits. The tier is the
// title of the tab holding the table, and the models are its rows: rows naming
// an endpoint rather than a model resolve to nothing and are skipped.
func (b *builder) applySonarTierTable(t table) {
	tiers := rowTiers(t.Tab)
	at := columnOf(t.Headers, "requests per minute (rpm)")
	if len(tiers) != 1 || at < 0 {
		return
	}
	key := LimitRPMTierPrefix + strconv.Itoa(tiers[0])
	for _, row := range t.Rows {
		id := slugID(cellAt(row, 0))
		m, ok := b.models[id]
		if !ok || !slices.Contains(b.sonar, id) {
			continue
		}
		value, ok := parseRate(cellAt(row, at))
		if !ok {
			continue
		}
		m.AddSource(t.Source)
		m.SetLimit(key, value)
	}
}

// applySearchLimits reads the Search API's limits, which are stated in query
// units rather than in requests: the page says outright that one request may
// spend five of them, so recording them as requests would misstate the limit
// by whatever a caller batches into a call.
func (b *builder) applySearchLimits(t table) {
	limit := columnOf(t.Headers, "rate limit")
	burst := columnOf(t.Headers, "burst capacity")
	if limit < 0 && burst < 0 {
		return
	}
	m, ok := b.sectionTool(t.Section)
	if !ok {
		return
	}
	m.AddSource(t.Source)
	for _, row := range t.Rows {
		if value, ok := parseRate(cellAt(row, limit)); ok {
			m.SetLimit(LimitQueryUnitsPerSecond, value)
		}
		if value, ok := parseRate(cellAt(row, burst)); ok {
			m.SetLimit(LimitBurstQueryUnits, value)
		}
	}
}

// sectionTool returns the tool product a section heading is about, which is
// the longest tool identifier the heading contains.
func (b *builder) sectionTool(section string) (*catalog.Model, bool) {
	slug := slugID(section)
	var found string
	for _, id := range append(slices.Clone(b.tools), b.searchAPI...) {
		if strings.Contains(slug, id) && len(id) > len(found) {
			found = id
		}
	}
	if found == "" {
		return nil, false
	}
	return b.models[found], true
}

// embeddingsFor returns the embedding models a limit table binds. The page
// gives the contextualized models a table of their own and says their limits
// are separate, so the general table is read onto the others only.
func (b *builder) embeddingsFor(t table) []string {
	contextualized := strings.EqualFold(t.Heading, "Contextualized Embeddings")
	var ids []string
	for _, id := range b.embedding {
		m, ok := b.models[id]
		if !ok {
			continue
		}
		if (m.Attrs[AttrContextualized] == "true") != contextualized {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

// rowTiers returns the usage tiers a cell names, expanding a range into the
// tiers it covers.
func rowTiers(cell string) []int {
	match := tierRe.FindStringSubmatch(clean(cell))
	if match == nil {
		return nil
	}
	from, err := strconv.Atoi(match[1])
	if err != nil {
		return nil
	}
	to := from
	if match[2] != "" {
		if parsed, err := strconv.Atoi(match[2]); err == nil {
			to = parsed
		}
	}
	if to < from {
		return nil
	}
	tiers := make([]int, 0, to-from+1)
	for tier := from; tier <= to; tier++ {
		tiers = append(tiers, tier)
	}
	return tiers
}

// parseRate reads the quantity a limit cell opens with.
func parseRate(cell string) (int64, bool) {
	match := rateRe.FindStringSubmatch(clean(cell))
	if match == nil {
		return 0, false
	}
	value, err := strconv.ParseInt(
		strings.ReplaceAll(match[1], ",", ""),
		10,
		64,
	)
	if err != nil {
		return 0, false
	}
	return value, true
}
