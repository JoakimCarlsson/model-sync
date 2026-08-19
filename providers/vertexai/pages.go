package vertexai

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Scalar keys the model pages populate beside the lifecycle page's.
const (
	// AttrState is the launch stage Google publishes for a model, which every
	// page states twice: once as a row of its own and once against the release
	// the version block describes.
	AttrState = "state"
	// AttrReleaseDate is when the release the page describes was published.
	AttrReleaseDate = "release_date"
	// AttrKnowledgeCutoff is how recent the data a model was trained on is.
	// Google states it only for the models it serves for other labs and for
	// its own embedding model; a Gemini page states none.
	AttrKnowledgeCutoff = "knowledge_cutoff"
	// AttrRetirementQualifier holds the words a page hedges a retirement date
	// with, "or later" or "not sooner than". The date is what Google commits
	// to at the earliest, and dropping the hedge would state a withdrawal it
	// has not announced.
	AttrRetirementQualifier = "retirement_date_qualifier"
	// AttrInputSizeLimit is how large a request may be, which Google states in
	// bytes rather than in tokens and so has no place among the limits.
	AttrInputSizeLimit = "input_size_limit"
	// AttrLayers is the depth of an embedding model, which only the E5 pages
	// state.
	AttrLayers = "layers"
)

// Numeric keys the model pages populate beside the two bounds.
const (
	// LimitMaxInputTokens is the ceiling the models Vertex serves for other
	// labs state in place of a context window. It is recorded beside the
	// context window rather than as it, because a page stating both states
	// them as different rows.
	LimitMaxInputTokens = "max_input_tokens"
	// LimitConcurrentSessions is how many streams the Live API model holds at
	// once, which is a bound on the model rather than on a prompt.
	LimitConcurrentSessions = "max_concurrent_sessions"
	// The quota keys, which the pages state per endpoint. Only a figure the
	// page states the same for every endpoint is recorded; see readQuotaLimits.
	LimitRequestsPerMinute     = "requests_per_minute"
	LimitTokensPerMinute       = "tokens_per_minute"
	LimitInputTokensPerMinute  = "input_tokens_per_minute"
	LimitOutputTokensPerMinute = "output_tokens_per_minute"
)

// ListRegions holds the endpoints a model answers on. Google publishes these
// per model and no other catalog here carries them, so they are recorded under
// a key of this package's own.
const ListRegions = "regions"

// stateNames map the launch stages Google publishes onto the catalog's
// vocabulary. A page writes the same stage two ways, heading a row with "GA"
// and the version block with "Generally available", and Google's own scale
// runs Experimental, Preview and GA with Deprecated at the end of it.
var stateNames = map[string]string{
	"ga":                  "active",
	"generally available": "active",
	"preview":             "preview",
	"public preview":      "preview",
	"private preview":     "preview",
	"experimental":        "experimental",
	"deprecated":          "deprecated",
}

// stateOf reads a launch stage, returning the empty string for a wording this
// package has not seen rather than a guess at what it meant.
func stateOf(value string) string {
	return stateNames[strings.ToLower(strings.TrimSpace(value))]
}

var (
	// versionEntryRe matches one release the version block describes, which is
	// its identifier followed by the list of what Google states about it.
	versionEntryRe = regexp.MustCompile(
		`(?is)<li><code[^>]*>(.*?)</code></li>\s*<ul>(.*?)</ul>`,
	)
	// versionFieldRe matches one such statement, "Release date: May 19, 2026".
	versionFieldRe = regexp.MustCompile(`(?is)<li>(.*?)</li>`)
	// listedLimitRe matches the token bounds a page states as a list rather
	// than as rows of the table. The models Vertex serves for other labs write
	// "Maximum input tokens: 1,000,000" where Google's own head a row with the
	// bound and put the figure beside it.
	//
	// The match runs to the end of the item, because one such item may state
	// two bounds rather than one: Claude Sonnet 4.5 accepts "1M (Preview)" and
	// "200,000 (GA)", and reading the first figure alone recorded a ceiling of
	// one token. An item stating two is left out, the page not saying which
	// holds for the model as sold.
	listedLimitRe = regexp.MustCompile(
		`(?i)^maximum (input|output) tokens\s*:\s*([\d][\d,]*)$`,
	)
	// listedItemRe matches one item of such a list.
	listedItemRe = regexp.MustCompile(`(?is)<li>(.*?)</li>`)
	// quotaFieldRe matches one quota a page states per endpoint. The labels
	// are matched longest first, so that an input quota is not read as the
	// undivided one.
	quotaFieldRe = regexp.MustCompile(
		`(?i)\b(input tpm|output tpm|tpm|qpm)\s*:\s*([\d][\d,]*)`,
	)
	// cellRe matches one cell of a row, which never holds another.
	cellRe = regexp.MustCompile(`(?is)<td[^>]*>(.*?)</td>`)
	// codeRe matches a value the page sets in code font.
	codeRe = regexp.MustCompile(`(?is)<code[^>]*>(.*?)</code>`)
	// regionRe matches what an endpoint identifier looks like. A page may name
	// an area rather than an endpoint, setting "Multi-region" or "global
	// endpoint" in the same font, and those are not identifiers a caller can
	// address.
	regionRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	// dateRe matches a date as Google writes one, with or without the day and
	// with or without the comma it separates the year by.
	dateRe = regexp.MustCompile(
		`(?i)\b(January|February|March|April|May|June|July|August|September|` +
			`October|November|December),?\s*(\d{1,2})?,?\s*(\d{4})\b`,
	)
	// qualifierRe matches the words a page hedges a retirement date with.
	qualifierRe = regexp.MustCompile(
		`(?i)(or later|no sooner than|not sooner than)`,
	)
)

// months number the names Google writes a date with.
var months = map[string]string{
	"january": "01", "february": "02", "march": "03", "april": "04",
	"may": "05", "june": "06", "july": "07", "august": "08",
	"september": "09", "october": "10", "november": "11", "december": "12",
}

// isoDate rewrites a date into ISO form, keeping whatever precision Google
// published: a date without a day becomes a month, and a value naming no month
// at all becomes nothing rather than an invented one.
func isoDate(value string) string {
	match := dateRe.FindStringSubmatch(value)
	if match == nil {
		return ""
	}
	month := months[strings.ToLower(match[1])]
	if month == "" {
		return ""
	}
	if match[2] == "" {
		return match[3] + "-" + month
	}
	day, err := strconv.Atoi(match[2])
	if err != nil || day < 1 || day > 31 {
		return match[3] + "-" + month
	}
	return fmt.Sprintf("%s-%s-%02d", match[3], month, day)
}

// Rows of the specification table read as a whole block rather than as a
// heading and a value, because Google heads the group once and leaves the rows
// spanned under it unlabelled.
const (
	blockVersions   = "versions"
	blockRegions    = "supported-regions"
	blockQuotas     = "quota-limits"
	blockTokens     = "token-limits"
	blockTools      = "tools"
	blockCapability = "capabilities"
)

// rowBlocks match each of those blocks, from the label that opens it to the
// label that opens the next.
var rowBlocks = map[string]*regexp.Regexp{
	blockVersions:   blockRe(blockVersions),
	blockRegions:    blockRe(blockRegions),
	blockQuotas:     blockRe(blockQuotas),
	blockTokens:     blockRe(blockTokens),
	blockTools:      blockRe(blockTools),
	blockCapability: blockRe(blockCapability),
}

// blockRe compiles the match for one such block.
func blockRe(id string) *regexp.Regexp {
	return regexp.MustCompile(
		`(?is)<th[^>]*id="` + regexp.QuoteMeta(id) + `".*?(?:<th|\z)`,
	)
}

// rowBlock returns the part of the table one labelled row occupies.
func rowBlock(table, id string) string {
	re, ok := rowBlocks[id]
	if !ok {
		return ""
	}
	return re.FindString(table)
}

// readVersionBlock reads the release the page describes: the stage Google
// serves it at, when it was published and when it is due to go.
//
// A page may describe more than one release, listing a preview beside the
// version that replaced it, so the entry read is the one whose identifier is
// the model the page is about, and failing that the first.
func readVersionBlock(page *documented, table, id string) {
	entries := versionEntryRe.FindAllStringSubmatch(
		rowBlock(table, blockVersions),
		-1,
	)
	if len(entries) == 0 {
		return
	}
	chosen := entries[0]
	for _, entry := range entries {
		if strings.EqualFold(specText(entry[1]), id) {
			chosen = entry
			break
		}
	}
	for _, field := range versionFieldRe.FindAllStringSubmatch(chosen[2], -1) {
		label, value, ok := strings.Cut(specText(field[1]), ":")
		if !ok {
			continue
		}
		readVersionField(page, strings.TrimSpace(label), value)
	}
}

// readVersionField records one statement of the version block.
func readVersionField(page *documented, label, value string) {
	switch {
	case strings.EqualFold(label, "launch stage"):
		page.State = stateOf(value)
	case strings.EqualFold(label, "release date"):
		page.ReleaseDate = isoDate(value)
	case strings.HasPrefix(strings.ToLower(label), "retirement date"):
		page.Retirement = isoDate(value)
		page.RetireQualifier = qualifierOf(label + " " + value)
	}
}

// qualifierOf reports the words a page hedges a retirement date with.
func qualifierOf(value string) string {
	return strings.ToLower(qualifierRe.FindString(value))
}

// readRegions records the endpoints a model answers on.
//
// The block states the same list several ways over, for availability, for
// where the request is processed and for each way the model may be bought, so
// only the first is read, that being the one saying where the model can be
// called at all. A page may name an area rather than an endpoint, setting
// "Multi-region" in the same font an identifier is set in, and an area is not
// something a caller can address.
func readRegions(page *documented, table string) {
	for _, cell := range cellRe.FindAllStringSubmatch(
		rowBlock(table, blockRegions),
		-1,
	) {
		codes := codeRe.FindAllStringSubmatch(cell[1], -1)
		if len(codes) == 0 {
			continue
		}
		for _, code := range codes {
			value := specText(code[1])
			if regionRe.MatchString(value) {
				page.Regions = appendNew(page.Regions, value)
			}
		}
		return
	}
}

// readListedLimits records the token bounds a page states as a list. Google's
// own models head a row with each bound; the models it serves for other labs
// write both into one cell.
func readListedLimits(page *documented, table string) {
	for _, item := range listedItemRe.FindAllStringSubmatch(
		rowBlock(table, blockTokens),
		-1,
	) {
		match := listedLimitRe.FindStringSubmatch(specText(item[1]))
		if match == nil {
			continue
		}
		if strings.EqualFold(match[1], "input") {
			page.MaxInput = parseCount(match[2])
			continue
		}
		page.MaxOut = parseCount(match[2])
	}
}

// readQuotaLimits records the rate limits a page publishes.
//
// Google states them once per endpoint and the endpoints differ, Claude Opus 5
// answering twice as many requests a minute on the global endpoint as on a
// multi-region one, and the block labels two different endpoints "Multi-region"
// alike, so there is no name to key the figures by. A figure the page states
// the same for every endpoint is a fact about the model and is recorded; one
// that varies is a fact about an endpoint the block does not identify, and is
// left out rather than reported as the model's.
func readQuotaLimits(page *documented, table string) {
	seen := map[string][]int64{}
	for _, match := range quotaFieldRe.FindAllStringSubmatch(
		specText(rowBlock(table, blockQuotas)),
		-1,
	) {
		label := strings.ToLower(match[1])
		seen[label] = append(seen[label], parseCount(match[2]))
	}
	page.Quotas = map[string]int64{}
	for label, values := range seen {
		key := quotaKeys[label]
		if key == "" || !alike(values) {
			continue
		}
		page.Quotas[key] = values[0]
	}
}

// quotaKeys name the quotas a page states.
var quotaKeys = map[string]string{
	"qpm":        LimitRequestsPerMinute,
	"tpm":        LimitTokensPerMinute,
	"input tpm":  LimitInputTokensPerMinute,
	"output tpm": LimitOutputTokensPerMinute,
}

// alike reports values that are all the same.
func alike(values []int64) bool {
	for _, value := range values {
		if value != values[0] {
			return false
		}
	}
	return len(values) > 0
}
