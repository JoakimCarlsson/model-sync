package fireworks

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Metrics Fireworks bills on.
const (
	MetricInputTokens       catalog.Metric = "input_tokens"
	MetricCachedInputTokens catalog.Metric = "cached_input_tokens"
	MetricOutputTokens      catalog.Metric = "output_tokens"
)

// UnitPer1MTokens is the only denominator Fireworks quotes.
const UnitPer1MTokens catalog.Unit = "per_1m_tokens"

// Kinds of model Fireworks serves.
const (
	KindChat      catalog.Kind = "chat"
	KindEmbedding catalog.Kind = "embedding"
)

// Dimension keys Fireworks' prices vary along.
const (
	// DimTier is the serving path chosen per request.
	DimTier = "tier"
	// DimServing is the variant of the deployment, which Fireworks writes as
	// a suffix on the model's display name rather than as a parameter.
	DimServing = "serving"
)

// Serving paths Fireworks prices separately.
const (
	TierStandard = "standard"
	TierPriority = "priority"
)

// AttrModelURL is where the model can be inspected.
const AttrModelURL = "model_url"

// tripleOrder is what the three amounts in a cell mean, in the order the page
// writes them.
var tripleOrder = []catalog.Metric{
	MetricInputTokens,
	MetricCachedInputTokens,
	MetricOutputTokens,
}

// servingSuffixes are the words Fireworks appends to a display name to mark a
// deployment variant rather than a different model.
var servingSuffixes = []string{"fast us", "fast", "us"}

var (
	linkRe   = regexp.MustCompile(`\[([^\]]*)\]\(([^)]*)\)`)
	amountRe = regexp.MustCompile(`\$\s*([\d,]*\.?\d+)`)
)

// clean strips markdown decoration from a cell value.
func clean(cell string) string {
	s := strings.ReplaceAll(cell, `\$`, "$")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "`", "")
	return strings.Join(strings.Fields(s), " ")
}

// modelRef is what one model cell holds.
type modelRef struct {
	ID      string
	Name    string
	Serving string
	URL     string
}

// modelPathPrefix precedes the identifier in a model's console URL.
const modelPathPrefix = "/models/"

// splitModelCell reads a model cell, separating the deployment variant from
// the model it is a variant of.
func splitModelCell(cell string) (modelRef, bool) {
	match := linkRe.FindStringSubmatch(cell)
	if match == nil {
		return modelRef{}, false
	}
	name := clean(match[1])
	url := strings.TrimSpace(match[2])
	at := strings.Index(url, modelPathPrefix)
	if at < 0 {
		return modelRef{}, false
	}
	ref := modelRef{
		ID:   strings.Trim(url[at+len(modelPathPrefix):], "/"),
		Name: name,
		URL:  url,
	}
	lower := strings.ToLower(name)
	for _, suffix := range servingSuffixes {
		if base, ok := strings.CutSuffix(lower, " "+suffix); ok {
			ref.Serving = strings.ReplaceAll(suffix, " ", "-")
			ref.Name = strings.TrimSpace(name[:len(base)])
			break
		}
	}
	return ref, ref.ID != ""
}

// parseTriple reads the amounts in a rate cell, in the order the page states
// them. A cell holding a dash offers that serving path no rate at all.
func parseTriple(cell string) []float64 {
	var out []float64
	for _, match := range amountRe.FindAllStringSubmatch(clean(cell), -1) {
		value, err := strconv.ParseFloat(
			strings.ReplaceAll(match[1], ",", ""),
			64,
		)
		if err != nil {
			continue
		}
		out = append(out, value)
	}
	return out
}

// bandRe matches a row that prices by parameter count rather than naming a
// model, which is what the size-based rate cards are made of.
var bandRe = regexp.MustCompile(
	`(?i)parameters|^\s*(up to|less than|more than|\d[\d.]*[MB]?\s*[-–])`,
)

// isBand reports whether a row prices a size band rather than a model.
func isBand(cell string) bool {
	return bandRe.MatchString(clean(cell))
}

// slugID turns a display name into an identifier, for the rows that name a
// model without linking to it.
func slugID(name string) string {
	s := strings.ToLower(clean(name))
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' {
			return r
		}
		return '-'
	}, s)
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}
