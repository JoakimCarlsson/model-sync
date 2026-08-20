package azure

import (
	"regexp"
	"slices"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// snapshotSuffixRe matches the dated version a meter names a model's release
// by, which Azure writes as the month and day with no separator: gpt-4o-0806
// is the August 6th release of gpt-4o.
var snapshotSuffixRe = regexp.MustCompile(`^([0-9]{4})$`)

// applySnapshots relates a family to the dated versions the price list meters
// it as, and gives the family the rates of the version it deploys by default.
//
// Azure sells a model under an undated name and bills it under a dated one.
// A deployment asks for gpt-4o and Azure serves the version the deployment
// names or the current one; the price list has no meter called gpt-4o at all,
// only gpt-4o-0513, gpt-4o-0806 and gpt-4o-1120. What reached the undated
// model was therefore only what Azure meters without a date, which for gpt-4o
// and gpt-4o-mini is the fine tuning grader alone: the two most deployed models
// on the service published no rate for an ordinary completion.
//
// So each version is recorded against the family it belongs to, and the
// version the documentation gives as the model's own is copied onto it, every
// rate carrying the snapshot it was read from. The dimension is what keeps the
// copy honest: it does not say that an undated deployment is billed at this
// rate whatever version it runs, it says this is what the version Azure
// currently ships costs. The versions keep their own entries, since each has
// its own retirement date and its own regions.
//
// Only the rates of ordinary inference are copied. A fine tuning rate is
// already stated against the family, which is where Azure states it.
func (b *builder) applySnapshots() {
	families := b.snapshotFamilies()
	for _, id := range b.order {
		versions, ok := families[id]
		if !ok {
			continue
		}
		m := b.models[id]
		m.AddList(ListSnapshots, versions...)
		current := b.currentSnapshot(m, versions)
		if current == "" {
			continue
		}
		suffix := strings.TrimPrefix(current, id+"-")
		for _, price := range b.models[current].Prices {
			if fineTuning(price) {
				continue
			}
			price.Dims = price.Dims.With(DimSnapshot, suffix)
			m.AddPrice(price)
		}
		for _, source := range b.models[current].Sources {
			m.AddSource(source)
		}
	}
}

// snapshotFamilies groups the dated identifiers under the undated one they
// extend, for the undated names the price list also carries a model for. A
// dated name whose family has no entry of its own is left alone: it is then the
// only name Azure uses and there is nothing to relate it to.
func (b *builder) snapshotFamilies() map[string][]string {
	out := map[string][]string{}
	for _, id := range b.order {
		at := strings.LastIndex(id, "-")
		if at < 0 {
			continue
		}
		family := id[:at]
		if !snapshotSuffixRe.MatchString(id[at+1:]) {
			continue
		}
		if _, ok := b.models[family]; !ok {
			continue
		}
		out[family] = append(out[family], id)
	}
	for family := range out {
		slices.Sort(out[family])
	}
	return out
}

// currentSnapshot returns the version whose date is the one the documentation
// gives the family as its own, and nothing where none of them matches.
//
// The match is the month and day of the version, which is the only part the
// meter states: the schedule dates gpt-4o at 2024-11-20 and the meter calls
// that version gpt-4o-1120. A family whose documented version has no meter is
// left without a copied rate rather than given the newest meter's, because
// which version an undated deployment serves is Azure's statement to make and
// not this parser's guess.
func (b *builder) currentSnapshot(m *catalog.Model, versions []string) string {
	date := m.Attrs[AttrVersion]
	if len(date) < len("2024-11-20") {
		return ""
	}
	want := m.ID + "-" + date[5:7] + date[8:10]
	if !slices.Contains(versions, want) {
		return ""
	}
	return want
}

// fineTuning reports whether a rate covers something other than an ordinary
// request: a fine tuned deployment, or the grader model a fine tuning run
// scores its output with.
func fineTuning(price catalog.Price) bool {
	if _, ok := price.Dims[DimFineTuned]; ok {
		return true
	}
	_, ok := price.Dims[DimModelGrader]
	return ok
}
