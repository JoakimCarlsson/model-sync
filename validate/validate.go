package validate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Rule names the check a problem came from, so a report can be grouped and a
// consumer of this package can act on one class without parsing prose.
type Rule string

// The rules. Every one but ChatUnpriced is an error.
const (
	RuleNegativeAmount Rule = "negative_amount"
	RuleMissingField   Rule = "missing_price_field"
	RuleOutputAboveCtx Rule = "output_above_context"
	RuleAmbiguousPrice Rule = "ambiguous_price"
	RuleDimsCase       Rule = "dims_case"
	RuleMissingAPIID   Rule = "missing_api_id"
	RuleChatUnpriced   Rule = "chat_unpriced"
)

// Problem is one rule failing for one model.
type Problem struct {
	Provider string
	Model    string
	Rule     Rule
	Detail   string
	Warning  bool
}

// String renders a problem as one line.
func (p Problem) String() string {
	severity := "error"
	if p.Warning {
		severity = "warning"
	}
	return fmt.Sprintf(
		"%s: %s/%s: %s: %s",
		severity,
		p.Provider,
		p.Model,
		p.Rule,
		p.Detail,
	)
}

// kindChat is the kind whose models are expected to carry an ordinary
// inference rate. It is the one kind every provider spells the same way.
const kindChat = "chat"

// fineTuningDims are the dimension keys that qualify a rate as something other
// than ordinary inference. A model priced only under one of them has a rate a
// consumer cannot quote for a plain completion request.
var fineTuningDims = []string{"fine_tuned", "model_grader"}

// Catalog checks every model and returns what it found, in catalog order so a
// report is stable between runs.
func Catalog(cat *catalog.Catalog) []Problem {
	var problems []Problem
	for _, provider := range cat.Providers {
		for _, model := range provider.Models {
			problems = append(problems, check(provider.ID, model)...)
		}
	}
	return problems
}

// Errors reports whether any problem stops publication.
func Errors(problems []Problem) bool {
	for _, p := range problems {
		if !p.Warning {
			return true
		}
	}
	return false
}

// Summary counts problems by rule, so a run that found a hundred of one thing
// says so in one line rather than a hundred.
func Summary(problems []Problem) string {
	counts := map[Rule]int{}
	for _, p := range problems {
		counts[p.Rule]++
	}
	rules := make([]string, 0, len(counts))
	for rule, n := range counts {
		rules = append(rules, fmt.Sprintf("%s=%d", rule, n))
	}
	sort.Strings(rules)
	return strings.Join(rules, " ")
}

// reporter records one problem. It exists so each rule reads as a statement
// about the data rather than as an append.
type reporter func(rule Rule, warning bool, format string, args ...any)

// check runs every rule against one model.
func check(provider string, m catalog.Model) []Problem {
	var problems []Problem
	report := func(rule Rule, warning bool, format string, args ...any) {
		problems = append(problems, Problem{
			Provider: provider,
			Model:    m.ID,
			Rule:     rule,
			Detail:   fmt.Sprintf(format, args...),
			Warning:  warning,
		})
	}
	for _, price := range m.Prices {
		checkPrice(price, report)
	}
	checkAmbiguity(m.Prices, report)
	checkLimits(m.Limits, report)
	if m.Attrs[catalog.APIID] == "" {
		report(RuleMissingAPIID, false, "no %s", catalog.APIID)
	}
	if m.Kind == kindChat && len(m.Prices) > 0 && !hasPlainRate(m.Prices) {
		report(
			RuleChatUnpriced,
			true,
			"every rate is qualified by %s",
			strings.Join(fineTuningDims, " or "),
		)
	}
	return problems
}

// checkLimits covers the bounds that contradict each other. An output ceiling
// above the context window cannot be asked for, since the request carrying it
// fails.
func checkLimits(limits map[string]int64, report reporter) {
	window := limits["context_window"]
	output := limits["max_output_tokens"]
	if window <= 0 || output <= window {
		return
	}
	report(
		RuleOutputAboveCtx,
		false,
		"max_output_tokens %d exceeds context_window %d",
		output,
		window,
	)
}

// checkPrice covers the rules that look at one rate alone.
func checkPrice(price catalog.Price, report reporter) {
	label := describe(price)
	if price.Amount < 0 {
		report(
			RuleNegativeAmount,
			false,
			"%s: amount %v; a rate that is not a number is variable",
			label,
			price.Amount,
		)
	}
	if price.Metric == "" {
		report(RuleMissingField, false, "%s: no metric", label)
	}
	if price.Unit == "" {
		report(RuleMissingField, false, "%s: no unit", label)
	}
	if price.Currency == "" {
		report(RuleMissingField, false, "%s: no currency", label)
	}
	for key, value := range price.Dims {
		if value == strings.ToLower(value) {
			continue
		}
		report(
			RuleDimsCase,
			false,
			"%s: dimension %s=%q is not lowercase",
			label,
			key,
			value,
		)
	}
}

// checkAmbiguity finds one (metric, unit, dims) key carrying more than one
// amount, which is two documents disagreeing rather than two rates.
func checkAmbiguity(prices []catalog.Price, report reporter) {
	seen := map[string][]catalog.Price{}
	var order []string
	for _, price := range prices {
		key := string(price.Metric) + "|" + string(price.Unit) + "|" +
			price.Dims.Key()
		if _, ok := seen[key]; !ok {
			order = append(order, key)
		}
		seen[key] = append(seen[key], price)
	}
	for _, key := range order {
		group := seen[key]
		amounts := map[float64]bool{}
		for _, price := range group {
			amounts[price.Amount] = true
		}
		if len(amounts) < 2 {
			continue
		}
		values := make([]string, 0, len(amounts))
		for amount := range amounts {
			values = append(values, fmt.Sprint(amount))
		}
		sort.Strings(values)
		report(
			RuleAmbiguousPrice,
			false,
			"%s: %s; add the dimension that tells them apart",
			describe(group[0]),
			strings.Join(values, " and "),
		)
	}
}

// hasPlainRate reports whether any rate applies to an ordinary request.
func hasPlainRate(prices []catalog.Price) bool {
	for _, price := range prices {
		qualified := false
		for _, dim := range fineTuningDims {
			if _, ok := price.Dims[dim]; ok {
				qualified = true
			}
		}
		if !qualified {
			return true
		}
	}
	return false
}

// describe names a rate the way the data does, so a report line can be found
// in api.json by searching for what it names.
func describe(price catalog.Price) string {
	out := string(price.Metric)
	if price.Unit != "" {
		out += " " + string(price.Unit)
	}
	if key := price.Dims.Key(); key != "" {
		out += " {" + key + "}"
	}
	return out
}
