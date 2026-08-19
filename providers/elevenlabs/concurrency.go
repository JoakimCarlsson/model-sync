package elevenlabs

import (
	"slices"
	"strings"
)

// concurrencyColumns map a column of the concurrency table onto the models it
// covers.
//
// The table heads each column with the name of a group of models rather than
// with an identifier, so a column has to be matched to the models under it. The
// fragments are tried in order, because a heading can contain more than one and
// the earlier ones are the more specific: the realtime transcription column is
// matched before the batch one, which would otherwise take it.
var concurrencyColumns = []struct {
	// Heading is the fragment of the column heading, lowercased, naming the
	// group.
	Heading string
	// Prefixes are the identifier prefixes of the models in it.
	Prefixes []string
	// Exclude drops an identifier a prefix would otherwise take.
	Exclude []string
}{
	{
		Heading:  "(multilingual v2)",
		Prefixes: []string{"eleven_multilingual_v2"},
	},
	{Heading: "(flash)", Prefixes: []string{"eleven_flash"}},
	{Heading: "realtime stt", Prefixes: []string{"scribe_v2_realtime"}},
	{
		Heading:  "stt",
		Prefixes: []string{"scribe"},
		Exclude:  []string{"scribe_v2_realtime"},
	},
	{Heading: "music", Prefixes: []string{"music_"}},
}

// concurrencyHeading is the word every column of the table carries and nothing
// else on the page does. A column without it states a plan's priority in the
// queue rather than a bound on a model.
const concurrencyHeading = "concurrency"

// applyConcurrency reads the table pairing a plan with how many requests of a
// model may be in flight at once.
//
// The bound is a fact about a model and not only about a plan: the table gives
// the same plan a different figure for different models, which is ElevenLabs
// saying what a model costs it to run. Enterprise is written as "Elevated"
// rather than as a figure, so it yields no number and no key.
func (b *builder) applyConcurrency(t table) {
	planCol := columnOf(t.Headers, "plan")
	if planCol < 0 {
		return
	}
	for _, row := range t.Rows {
		plan := strings.ToLower(clean(cellAt(row, planCol)))
		if plan == "" {
			continue
		}
		for i, header := range t.Headers {
			limit := parseCount(cellAt(row, i))
			if limit == 0 {
				continue
			}
			for _, id := range b.concurrencyModels(clean(header)) {
				m := b.models[id]
				m.AddSource(t.Source)
				m.SetLimit(limitConcurrency+plan, limit)
			}
		}
	}
}

// concurrencyModels returns the models one column heading covers. It names only
// models the models table already established, so a column for something this
// catalog does not hold as a model contributes nothing.
func (b *builder) concurrencyModels(header string) []string {
	lower := strings.ToLower(header)
	if !strings.Contains(lower, concurrencyHeading) {
		return nil
	}
	for _, column := range concurrencyColumns {
		if !strings.Contains(lower, column.Heading) {
			continue
		}
		var out []string
		for _, id := range b.order {
			if !hasAnyPrefix(id, column.Prefixes) ||
				slices.Contains(column.Exclude, id) {
				continue
			}
			out = append(out, id)
		}
		return out
	}
	return nil
}

// hasAnyPrefix reports whether an identifier starts with any of the prefixes.
func hasAnyPrefix(id string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(id, prefix) {
			return true
		}
	}
	return false
}
