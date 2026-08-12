package anthropic

import (
	"regexp"
	"slices"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Lifecycle keys the deprecations page populates.
const (
	AttrState          = "state"
	AttrDeprecatedOn   = "deprecated_on"
	AttrRetirementDate = "retirement_date"
	AttrReplacement    = "recommended_replacement"
)

// notApplicable is what Anthropic writes in the deprecated column of a model
// that has not been deprecated.
const notApplicable = "n/a"

// stateRetired is the state of a model no longer served.
const stateRetired = "retired"

// dateSuffixRe matches the snapshot date Anthropic appends to a model
// identifier but never to a display name.
var dateSuffixRe = regexp.MustCompile(`^\d{8}$`)

// applyDeprecations reads the model status table and the deprecation history.
//
// This page is the only place every model's real API identifier is written.
// The pricing page names models by display name and the overview lists only
// current ones, so without this page a retired model is filed under a guessed
// identifier and looks current: "Claude Haiku 3.5" is really
// claude-3-5-haiku-20241022 and was retired in February 2026, and no amount of
// slugging the display name produces either fact. It is parsed before the
// other documents so these identifiers are what everything else resolves to.
func (b *builder) applyDeprecations(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		switch {
		case headerIs(t, 0, "api model name"):
			for _, row := range t.Rows {
				b.applyStatusRow(t, row)
			}
		case hasHeader(t, "deprecated model"):
			for _, row := range t.Rows {
				b.applyReplacementRow(t, row)
			}
		}
	}
}

// applyStatusRow records one model's lifecycle and registers its identifier so
// the other documents can resolve display names onto it.
func (b *builder) applyStatusRow(t mdTable, row []string) {
	id := clean(cellAt(row, 0))
	if id == "" {
		return
	}
	m := b.model(id, KindChat)
	m.AddSource(t.Source)
	m.SetAttr(AttrState, strings.ToLower(clean(cellAt(row, 1))))
	deprecated := clean(cellAt(row, 2))
	if !strings.EqualFold(deprecated, notApplicable) {
		m.SetAttr(AttrDeprecatedOn, deprecated)
	}
	m.SetAttr(AttrRetirementDate, clean(cellAt(row, 3)))
	b.register(id)
}

// applyReplacementRow records what Anthropic recommends migrating to, and the
// retirement the announcement was about.
//
// A model reachable only from the history is retired: the status table above
// covers everything current or recently retired, so anything appearing solely
// down here went years ago. The state is inferred rather than read, but only
// where the status table said nothing, which is why it is set last and why
// SetAttr does not overwrite.
func (b *builder) applyReplacementRow(t mdTable, row []string) {
	deprecated := clean(cellAt(row, columnOf(t, "deprecated model")))
	replacement := clean(cellAt(row, columnOf(t, "recommended replacement")))
	if deprecated == "" || replacement == "" {
		return
	}
	m := b.model(deprecated, KindChat)
	m.AddSource(t.Source)
	m.SetAttr(AttrReplacement, replacement)
	m.SetAttr(
		AttrRetirementDate,
		clean(cellAt(row, columnOf(t, "retirement date"))),
	)
	m.SetAttr(AttrState, stateRetired)
	b.register(deprecated)
}

// headerIs reports whether a table's column at index i is the given header.
func headerIs(t mdTable, i int, header string) bool {
	return strings.EqualFold(clean(cellAt(t.Headers, i)), header)
}

// hasHeader reports whether a table has the given column.
func hasHeader(t mdTable, header string) bool {
	return columnOf(t, header) >= 0
}

// columnOf returns the index of a named column, or -1.
func columnOf(t mdTable, header string) int {
	for i, h := range t.Headers {
		if strings.EqualFold(clean(h), header) {
			return i
		}
	}
	return -1
}

// register indexes an identifier by its version tokens so a display name can
// be resolved onto it. Anthropic orders those tokens differently in the two
// places it writes them, calling one model both "Claude Haiku 3.5" and
// claude-3-5-haiku-20241022, so the index is keyed on the sorted token set
// rather than on the order. Two identifiers reducing to the same set make it
// ambiguous, and an ambiguous key resolves to nothing rather than to a guess.
func (b *builder) register(id string) {
	key := tokenKey(id)
	if key == "" {
		return
	}
	if existing, ok := b.byTokens[key]; ok && existing != id {
		b.ambiguous[key] = true
		return
	}
	b.byTokens[key] = id
}

// lookup returns the identifier a display name refers to, if exactly one model
// matches its tokens.
func (b *builder) lookup(name string) (string, bool) {
	key := tokenKey(name)
	if key == "" || b.ambiguous[key] {
		return "", false
	}
	id, ok := b.byTokens[key]
	return id, ok
}

// tokenKey reduces a display name or an identifier to the sorted set of tokens
// that distinguish the model, dropping the vendor prefix every name carries
// and the snapshot date only identifiers carry.
func tokenKey(s string) string {
	var tokens []string
	fields := strings.FieldsFunc(
		strings.ToLower(clean(s)),
		func(r rune) bool {
			return (r < 'a' || r > 'z') && (r < '0' || r > '9')
		},
	)
	for _, token := range fields {
		if token == "claude" || dateSuffixRe.MatchString(token) {
			continue
		}
		tokens = append(tokens, token)
	}
	slices.Sort(tokens)
	return strings.Join(tokens, "-")
}
