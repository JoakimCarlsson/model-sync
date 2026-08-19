package fireworks

import (
	"regexp"
	"strconv"
	"strings"
)

// Fireworks quotes several of its rates against a band of parameter count
// rather than against a model: "4B - 16B parameters", "Models >300B". A band
// is a rate card row, and which band a model falls in is decided by the
// parameter count its page in the library states.
type band struct {
	// Label is the row as Fireworks wrote it, which is what the rate is
	// recorded as varying along.
	Label string
	// Min and Max bound the count in parameters. A zero Max is unbounded
	// above.
	Min, Max float64
	// MinExclusive and MaxExclusive say whether a count equal to the bound
	// falls inside the band.
	MinExclusive, MaxExclusive bool
	// MoE marks the bands Fireworks writes for mixture-of-experts models,
	// which it prices apart from dense models of the same size.
	MoE bool
	// Amounts are the rates on the row, in column order.
	Amounts []float64
}

// contains reports whether a model of this many parameters falls in the band.
func (b band) contains(params float64) bool {
	if b.Min > 0 {
		if params < b.Min || (b.MinExclusive && params == b.Min) {
			return false
		}
	}
	if b.Max > 0 {
		if params > b.Max || (b.MaxExclusive && params == b.Max) {
			return false
		}
	}
	return true
}

// scaleRe matches one count written with the suffix Fireworks scales it by.
var scaleRe = regexp.MustCompile(`([\d.]+)\s*([MBT])\b`)

// parenRe matches the examples a band row names, which are models rather than
// bounds and would otherwise be read as bounds.
var parenRe = regexp.MustCompile(`\([^)]*\)`)

// dashRe matches the typographic dashes a band row separates its bounds with,
// which differ between the rate cards and are all read as a range.
var dashRe = regexp.MustCompile("[‐-―]")

// parseBand reads a rate card row, returning whether it prices a band at all.
// A row naming a model rather than a size is not a band and yields nothing.
func parseBand(label string, amounts []float64) (band, bool) {
	spec := parenRe.ReplaceAllString(label, " ")
	spec = dashRe.ReplaceAllString(spec, "-")
	lower := strings.ToLower(spec)
	counts := scaleRe.FindAllStringSubmatch(spec, -1)
	if len(counts) == 0 {
		return band{}, false
	}
	b := band{
		Label:   strings.Join(strings.Fields(label), " "),
		MoE:     strings.Contains(lower, "moe"),
		Amounts: amounts,
	}
	switch {
	case len(counts) >= 2:
		b.Min, b.Max = countOf(counts[0]), countOf(counts[1])
	case strings.Contains(lower, "less than"):
		b.Max, b.MaxExclusive = countOf(counts[0]), true
	case strings.Contains(lower, "up to"):
		b.Max = countOf(counts[0])
	case strings.Contains(lower, "more than"), strings.Contains(spec, ">"):
		b.Min, b.MinExclusive = countOf(counts[0]), true
	default:
		return band{}, false
	}
	return b, true
}

// countOf turns one matched count into a number of parameters.
func countOf(match []string) float64 {
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0
	}
	switch match[2] {
	case "T":
		return value * 1e12
	case "B":
		return value * 1e9
	}
	return value * 1e6
}

// parameterCount reads the count a model's page states, which it writes as
// "2.81T" or "8.18B".
func parameterCount(value string) float64 {
	match := scaleRe.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil {
		return 0
	}
	return countOf(match)
}

// bandFor returns the row of a rate card that prices a model of this many
// parameters.
//
// Fireworks writes separate rows for mixture-of-experts models where it prices
// them apart, so a model is only ever matched against rows of its own
// architecture: a dense model never falls in a MoE row, and a MoE model whose
// count is outside every MoE row is left unpriced rather than moved onto a row
// written for dense models.
func bandFor(bands []band, params float64, moe bool) (band, bool) {
	if params <= 0 {
		return band{}, false
	}
	labelled := false
	for _, b := range bands {
		labelled = labelled || b.MoE
	}
	for _, b := range bands {
		if labelled && b.MoE != moe {
			continue
		}
		if b.contains(params) {
			return b, true
		}
	}
	return band{}, false
}
