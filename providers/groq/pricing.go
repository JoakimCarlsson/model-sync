package groq

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Dimensions the rates on a system's page vary along: which of the models the
// system reaches for is answering, and which built-in tool it called.
const (
	DimUnderlyingModel = "underlying_model"
	DimTool            = "tool"
)

// headPricing opens the section a page states its rates under, and toolsHeading
// the half of a system's section that leaves the models for the tools.
const (
	headPricing  = "### PRICING"
	toolsHeading = "built-in tool pricing"
)

var (
	// underlyingRe matches the heading over the rates of the models a system
	// reaches for, which is where the denominator of those rates is stated
	// rather than beside each amount.
	underlyingRe = regexp.MustCompile(`(?i)^Underlying Model Pricing \((.+)\)$`)
	// componentRe matches the heading naming which of those models the rates
	// under it belong to.
	componentRe = regexp.MustCompile(`(?i)^Pricing \((.+)\)$`)
	// amountRe matches an amount and the denominator written after it, which a
	// tool rate carries and a token rate leaves to the heading above it.
	amountRe = regexp.MustCompile(`^\$([\d,]*\.?\d+)\s*(?:/\s*(.+))?$`)
)

// sectionUnits maps a denominator the pricing section states onto a unit.
var sectionUnits = map[string]catalog.Unit{
	"per 1m tokens": UnitPer1MTokens,
	"1000 requests": UnitPer1KRequests,
	"hour":          UnitPerHour,
}

// tokenMetrics maps the label over a token rate onto the metric the amount is
// charged against. A page states the cached rate under a label of its own,
// which is the one rate the table where the model is listed leaves out: its
// price column holds the two amounts and nothing else.
var tokenMetrics = map[string]catalog.Metric{
	"input":        MetricInputTokens,
	"cached input": MetricCachedInputTokens,
	"output":       MetricOutputTokens,
}

// toolMetrics maps a built-in tool onto the metric its rate is charged
// against. A tool absent here is billed per call like the rest.
var toolMetrics = map[string]catalog.Metric{
	"code execution": MetricRuntime,
}

// addPagePricing reads the rates a model's own page states.
//
// A model page states the same two token rates as the table and a cached input
// rate besides, which the table has no column for. The two that agree are
// dropped as duplicates and the cached one is the model's third rate.
//
// A system's page goes further. A system has no rate of its own, but the page
// is not silent about what a query costs: it states the rate of each model the
// system reaches for and of each built-in tool it can call, which is every
// amount Groq will bill for it. Those are recorded with the model or the tool
// as a dimension, so a reader sees the rates rather than only the sentence
// saying they exist.
//
// The section is written as a label with the value on the line below it, so
// the two are read as a pair and the value is not read again as a label of its
// own. What follows a token rate is Groq restating it as tokens to the dollar,
// which states nothing the amount does not and is passed over.
func addPagePricing(m *catalog.Model, body string) {
	var (
		lines     = pricingSection(body)
		unit      catalog.Unit
		component string
		inTools   bool
	)
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if match := underlyingRe.FindStringSubmatch(line); match != nil {
			unit = sectionUnits[strings.ToLower(strings.TrimSpace(match[1]))]
			continue
		}
		if match := componentRe.FindStringSubmatch(line); match != nil {
			component = strings.TrimSpace(match[1])
			continue
		}
		if strings.EqualFold(line, toolsHeading) {
			inTools = true
			continue
		}
		label := strings.ToLower(line)
		if metric, ok := tokenMetrics[label]; ok && !inTools {
			addSectionRate(m, sectionRate{
				metric:    metric,
				unit:      tokenUnit(unit),
				dims:      dimsFor(DimUnderlyingModel, modelSlug(component)),
				label:     strings.TrimSpace(component + " " + line),
				value:     valueAfter(lines, i),
				keepValue: component != "",
			})
			i++
			continue
		}
		if inTools {
			metric, ok := toolMetrics[label]
			if !ok {
				metric = MetricToolCall
			}
			addSectionRate(m, sectionRate{
				metric:    metric,
				dims:      dimsFor(DimTool, toolKey(line)),
				label:     line,
				value:     valueAfter(lines, i),
				keepValue: true,
			})
			i++
		}
	}
}

// tokenUnit returns the denominator a token rate is quoted against, which a
// model's page leaves unstated. Groq quotes token rates one way throughout: the
// price column of the table where the model is listed is headed per 1M tokens,
// and so is the heading over the rates on a system's page.
func tokenUnit(stated catalog.Unit) catalog.Unit {
	if stated != "" {
		return stated
	}
	return UnitPer1MTokens
}

// sectionRate is one line of the pricing section and what the lines above it
// established about the amount below it.
type sectionRate struct {
	metric catalog.Metric
	unit   catalog.Unit
	dims   catalog.Dims
	label  string
	value  string
	// keepValue asks for what the page wrote to be kept as a note where it
	// wrote something other than an amount. It is set where the label belongs
	// to a section stating one value per label, so that anything unread is
	// Groq declining to state a rate rather than the parser losing one.
	keepValue bool
}

// addSectionRate records one rate, or, where the page writes something other
// than an amount, keeps what it wrote. Groq states a rate it has not settled as
// pending and a tool it does not bill for as billed elsewhere, and dropping
// either would leave a system reading as though the page named a shorter list
// of things that cost money than it does.
func addSectionRate(m *catalog.Model, r sectionRate) {
	match := amountRe.FindStringSubmatch(r.value)
	if match == nil {
		addSectionNote(m, r)
		return
	}
	unit := r.unit
	if match[2] != "" {
		unit = sectionUnits[strings.ToLower(strings.TrimSpace(match[2]))]
	}
	amount, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", ""), 64)
	if err != nil || unit == "" {
		addSectionNote(m, r)
		return
	}
	m.AddPrice(catalog.Price{
		Metric:   r.metric,
		Unit:     unit,
		Amount:   amount,
		Currency: currency,
		Dims:     r.dims,
	})
}

// addSectionNote keeps what a page wrote where an amount would go.
func addSectionNote(m *catalog.Model, r sectionRate) {
	if !r.keepValue || r.label == "" || r.value == "" {
		return
	}
	m.AddNote(fmt.Sprintf("%s: %s", r.label, r.value))
}

// pricingSection returns the non-empty lines of a page's pricing section,
// which runs from its heading to the rule closing it or to the next heading.
func pricingSection(body string) []string {
	var (
		out    []string
		inside bool
	)
	for _, raw := range strings.Split(body, "\n") {
		line := clean(strings.TrimSpace(raw))
		if strings.EqualFold(line, headPricing) {
			inside = true
			continue
		}
		if !inside {
			continue
		}
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "---") {
			break
		}
		if composedRe.MatchString(line) {
			break
		}
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// dimsFor returns the one dimension a rate varies along, or none where the
// page states a rate that varies along nothing.
func dimsFor(key, value string) catalog.Dims {
	if value == "" {
		return nil
	}
	return catalog.Dims{key: value}
}

// valueAfter returns the line following the one at i, which is where the page
// writes the amount belonging to the label on it.
func valueAfter(lines []string, i int) string {
	if i+1 >= len(lines) {
		return ""
	}
	return lines[i+1]
}

// modelSlug reduces the name of an underlying model to the shape Groq keys its
// models by, which is lower case with hyphens.
func modelSlug(name string) string {
	return joinLower(name, "-")
}

// toolKey reduces the name of a built-in tool to the word the capability lists
// already use for it, so the dimension and the feature read the same.
func toolKey(name string) string {
	return joinLower(name, "_")
}

// joinLower lower-cases a name and joins its words with sep.
func joinLower(name, sep string) string {
	return strings.Join(strings.Fields(strings.ToLower(name)), sep)
}
