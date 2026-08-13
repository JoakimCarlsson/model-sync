package berget

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// LimitContextWindow is the only numeric bound Berget publishes beyond a
// model's size, and it is on the documentation site rather than the endpoint.
const LimitContextWindow = "context_window"

// Enumeration keys the overview and the endpoint's capabilities populate.
const (
	ListInputModalities  = "input_modalities"
	ListOutputModalities = "output_modalities"
)

// FeatureVision is the capability Berget marks a model that reads images by.
// It names a modality rather than an API feature, so it is recorded as an
// input modality instead of joining the capability list, where it would be the
// one provider spelling image input that way.
const FeatureVision = "vision"

// typeModalities map Berget's model type onto what a model of that type takes
// and returns. The endpoint states the type and nothing else about modality,
// so a model reading images is found by its vision capability instead.
var typeModalities = map[string]struct{ in, out []string }{
	"text":           {[]string{"text"}, []string{"text"}},
	"embedding":      {[]string{"text"}, nil},
	"rerank":         {[]string{"text"}, nil},
	"speech-to-text": {[]string{"audio"}, []string{"text"}},
}

// cardRe matches one model card on the overview, which states the identifier
// the API answers to and the model's context window in one line.
var cardRe = regexp.MustCompile(
	`(?i)<code>([^<]+)</code>\s*·\s*([\d,]+)\s*(k?)\s*tokens`,
)

// applyOverview reads the context window off the documentation site.
//
// The endpoint publishes no bound on a request's length. The overview does, on
// a card per model keyed by the same identifier the endpoint uses, which is
// why the two documents need no reconciling.
func (b *builder) applyOverview(doc catalog.Document) {
	for _, card := range cardRe.FindAllStringSubmatch(string(doc.Body), -1) {
		m, ok := b.models[strings.TrimSpace(card[1])]
		if !ok {
			continue
		}
		m.AddSource(doc.URL)
		m.SetLimit(LimitContextWindow, parseCount(card[2], card[3]))
	}
}

// parseCount reads a quantity written either in full, as "8192", or with a
// thousands suffix, as "256k".
func parseCount(digits, suffix string) int64 {
	n, err := strconv.ParseInt(strings.ReplaceAll(digits, ",", ""), 10, 64)
	if err != nil {
		return 0
	}
	if strings.EqualFold(suffix, "k") {
		n *= 1_000
	}
	return n
}

// applyModalities records what a model takes and returns, read from its type
// and from whether Berget marks it as reading images.
func applyModalities(m *catalog.Model, e entry) {
	flow, ok := typeModalities[e.ModelType]
	if !ok {
		return
	}
	m.AddList(ListInputModalities, flow.in...)
	m.AddList(ListOutputModalities, flow.out...)
	if e.Capabilities[FeatureVision] {
		m.AddList(ListInputModalities, "image")
	}
}
