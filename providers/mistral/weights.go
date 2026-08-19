package mistral

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Patterns over the weights tab a model page carries.
//
// The tab is a table whose numeric cells are keyed by what they hold, so each
// number is anchored on its own key rather than on the column heading above
// it. A row leads with two links, the repository holding the weights and the
// licence governing them, and names the flavour of the weights in the text of
// the first. The licence link is optional in the pattern because a page whose
// row is served in pieces defers that cell, and the repository and its label
// are worth reading without it.
var (
	weightsRowRe = regexp.MustCompile(
		`(?s)"href":"https://huggingface\.co/([A-Za-z0-9._-]+/[A-Za-z0-9._-]+)"` +
			`.{0,400}?"children":\["([A-Za-z ]*Weights)"` +
			`(?:.{0,600}?"href":"([^"]+)")?`,
	)
	// gpuRAMRe matches one line of the tooltip stating how much memory the
	// weights need at a quantization, which is the only place the figures are
	// separated from each other.
	gpuRAMRe = regexp.MustCompile(
		`"children":" - (BF16 & Full Context|FP8 & 1/2 Context|` +
			`FP4 & 1/4 context):"\}\]," ","([0-9.]+)"`,
	)
	// endpointRe matches an endpoint a capability tile says the model answers
	// on.
	endpointRe = regexp.MustCompile(`"value":"(/v1/[a-z0-9/._-]+)"`)
)

// weightsCellRe matches the value of one cell of the weights row.
func weightsCellRe(key string) *regexp.Regexp {
	return regexp.MustCompile(
		`"` + regexp.QuoteMeta(key) + `",\{"className":"text-end","children":` +
			`\["\$","span",null,\{"className":"[^"]*","children":"([^"]+)"`,
	)
}

var (
	parameterCellRe = weightsCellRe("parameters")
	activeCellRe    = weightsCellRe("active")
)

// weightsKeys map the label Mistral gives a repository onto the key it is
// recorded under. A model whose weights come in one flavour is labelled
// plainly and is the one the API serves, as is the instruction-tuned flavour
// of a model published in several.
var weightsKeys = map[string]string{
	"Weights":           AttrHuggingFaceID,
	"Instruct Weights":  AttrHuggingFaceID,
	"Base Weights":      AttrHuggingFaceIDBase,
	"Reasoning Weights": AttrHuggingFaceIDReasoning,
}

// gpuRAMLimits map the quantization a tooltip line names onto the key its
// figure is recorded under.
var gpuRAMLimits = map[string]string{
	"BF16 & Full Context": LimitGPURAMBF16,
	"FP8 & 1/2 Context":   LimitGPURAMFP8,
	"FP4 & 1/4 context":   LimitGPURAMFP4,
}

// applyWeights records what a model page says about the weights behind a
// model.
//
// Every page carries the tab, and a page whose model Mistral does not publish
// the weights of carries it empty: the shelf is named and no row follows. That
// is a statement rather than a silence, so open_weights is set either way,
// which is what lets a consumer tell a model nobody can download from one
// this parser has not read.
func applyWeights(m *catalog.Model, body string) {
	match := weightsRe.FindStringSubmatch(body)
	if match == nil {
		return
	}
	shelf := match[1]
	m.SetAttr(AttrDistribution, strings.ToLower(shelf))
	m.SetAttr(AttrLicense, strings.Join(quoted(match[2]), ", "))
	var served string
	for _, row := range weightsRowRe.FindAllStringSubmatch(body, -1) {
		key, ok := weightsKeys[row[2]]
		if !ok {
			continue
		}
		m.SetAttr(key, row[1])
		if key != AttrHuggingFaceID {
			continue
		}
		served = row[1]
		m.SetAttr(AttrLicenseURL, row[3])
	}
	m.SetAttr(AttrOpenWeights, openWeights(shelf, served))
	m.SetLimit(LimitParameterCount, billions(first(parameterCellRe, body)))
	m.SetLimit(LimitActiveParameterCount, billions(first(activeCellRe, body)))
	for _, line := range gpuRAMRe.FindAllStringSubmatch(body, -1) {
		m.SetLimit(gpuRAMLimits[line[1]], parseCount(line[2]))
	}
}

// openWeights reports what the weights tab says about downloading a model.
// Mistral files a model on an open shelf or names a repository holding its
// weights, and a model it does neither for is one it serves and does not
// hand over.
func openWeights(shelf, repo string) string {
	if shelf == "Open" || repo != "" {
		return "true"
	}
	return "false"
}

// billions reads a parameter count, which Mistral writes in billions under a
// heading saying so.
func billions(value string) int64 {
	n, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0
	}
	return int64(n * 1_000_000_000)
}

// applyEndpoints records the paths a model answers on, which each capability
// tile names beneath the capability it describes.
func applyEndpoints(m *catalog.Model, body string) {
	paths := make([]string, 0, 4)
	for _, match := range endpointRe.FindAllStringSubmatch(body, -1) {
		paths = append(paths, match[1])
	}
	m.AddList(ListEndpoints, sortedUnique(paths)...)
}
