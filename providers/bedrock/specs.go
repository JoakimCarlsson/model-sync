package bedrock

import (
	"regexp"
	"slices"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Numeric keys the guide's quota page populates.
const (
	LimitInputTPM  = "input_tokens_per_minute"
	LimitOutputTPM = "output_tokens_per_minute"
)

// specFieldRe matches one labelled fact of a model's specification, which the
// guide writes as a bulleted list with the label in bold and the value after
// a dash.
var specFieldRe = regexp.MustCompile(
	`(?m)^\+\s+\*\*([^*]+)\*\*\s*(.*?)\s*$`,
)

// specDividers are the runes AWS divides a label from its value with.
const specDividers = " -:\u2013\u2014"

// Fields of a specification, named as AWS labels them.
const (
	fieldSpecID     = "model id"
	fieldVectorSize = "output vector size"
	fieldMaxTokens  = "max input text tokens"
)

// vectorRe matches one of the widths a specification offers, and the note
// marking which of them is returned when none is asked for.
var vectorRe = regexp.MustCompile(`(\d[\d,]*)\s*(\(default\))?`)

// applySpec records a model's specification, which the guide states for the
// models Amazon built itself and for no others.
//
// It is the only place AWS states how wide a vector an embedding model
// returns. A card states a model's modalities and marks embedding as one of
// its outputs without ever saying how many numbers the output has, and the
// price list bills the model by the token like any other. Where the widths
// are a set the caller chooses from, all of them are recorded and the one AWS
// marks as the default is recorded again on its own.
func (b *builder) applySpec(doc catalog.Document) {
	fields := map[string]string{}
	for _, field := range specFieldRe.FindAllStringSubmatch(
		string(doc.Body),
		-1,
	) {
		label := strings.ToLower(strings.TrimSpace(field[1]))
		value := strings.Trim(linkText(field[2]), specDividers)
		fields[label] = value
	}
	id := strings.TrimSpace(fields[fieldSpecID])
	if !identifierRe.MatchString(id) {
		return
	}
	for _, m := range b.byIdentifier()[id] {
		m.AddSource(doc.URL)
		m.SetLimit(LimitMaxInputTokens, parseCount(fields[fieldMaxTokens]))
		applyVectorSizes(m, fields[fieldVectorSize])
	}
}

// applyVectorSizes records the widths an embedding model returns. The widths
// are read whole rather than split on the comma between them, because AWS
// writes a thousand with one too.
func applyVectorSizes(m *catalog.Model, value string) {
	for _, match := range vectorRe.FindAllStringSubmatch(value, -1) {
		width := strings.ReplaceAll(match[1], ",", "")
		m.AddList(ListDimensions, width)
		if match[2] != "" {
			m.SetAttr(AttrDefaultDimension, width)
		}
	}
}

// Headings of the quota page's table of default quotas.
const (
	headingQuotaModel  = "model"
	headingQuotaInput  = "default input tpm"
	headingQuotaOutput = "default output tpm"
)

// applyQuotas records the throughput AWS publishes a default for.
//
// AWS states a model's quotas in the Service Quotas console rather than in the
// guide, and the guide's quota pages describe the quotas without giving their
// values. The one exception is the table of defaults for the models reached
// through the newest endpoint, which names a model and both its per-minute
// token quotas, and which AWS says it will extend as more models launch there.
//
// The table names a model in prose, so it is joined on the name and only
// where exactly one model is named by those words.
func (b *builder) applyQuotas(doc catalog.Document) {
	for _, t := range parseTables(string(doc.Body)) {
		if !t.headed(headingQuotaModel) ||
			!t.hasHeading(headingQuotaInput) {
			continue
		}
		for _, row := range t.rows {
			m, ok := b.byName(cell(row, 0))
			if !ok {
				continue
			}
			m.AddSource(doc.URL)
			for i := range row {
				switch t.heading(i) {
				case headingQuotaInput:
					m.SetLimit(LimitInputTPM, parseCount(cell(row, i)))
				case headingQuotaOutput:
					m.SetLimit(LimitOutputTPM, parseCount(cell(row, i)))
				}
			}
		}
	}
}

// byName returns the one model a prose name names, comparing the words of the
// name as the cards are compared and counting the lab's own words as part of
// it, since the quota table writes the lab and the card does not.
func (b *builder) byName(name string) (*catalog.Model, bool) {
	want := slices.Sorted(slices.Values(compareTokens(name)))
	if len(want) == 0 {
		return nil, false
	}
	var found *catalog.Model
	matches := 0
	for _, id := range b.order {
		m := b.models[id]
		for _, candidate := range []string{
			m.Name,
			m.Attrs[AttrAuthor] + " " + m.Name,
		} {
			tokens := slices.Sorted(slices.Values(compareTokens(candidate)))
			if slices.Equal(tokens, want) {
				found, matches = m, matches+1
				break
			}
		}
	}
	return found, matches == 1
}
