package anthropic

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Guides that open with a compatibility block, which is the most precise thing
// Anthropic publishes about a capability: a list of API identifiers, the beta
// header the capability needs if it needs one, and the platforms it reaches.
// The comparison table has a row for thinking and nothing else, so every other
// capability a consumer derives a boolean from is stated here or nowhere.
const (
	ComputerUseURL = baseURL +
		"/agents-and-tools/tool-use/computer-use-tool.md"
	CompactionURL  = baseURL + "/build-with-claude/compaction.md"
	TaskBudgetsURL = baseURL + "/build-with-claude/task-budgets.md"
	EffortURL      = baseURL + "/build-with-claude/effort.md"
	// FastModeURL states its models in a bullet list rather than a
	// compatibility block, naming each model and its identifier together.
	FastModeURL = baseURL + "/build-with-claude/fast-mode.md"
	// ServiceTiersURL states Priority Tier by exclusion, naming the models it
	// is not available on rather than the ones it is.
	ServiceTiersURL = baseURL + "/api/service-tiers.md"
)

var (
	// betaHeaderRe matches the header named in a compatibility block.
	betaHeaderRe = regexp.MustCompile(
		"(?m)^-[^\\n]*Beta header[^\\n]*?`([^`]+)`",
	)
	// availableOnRe matches the sentence an effort level names its models in,
	// which is how Anthropic states that two of the five levels reach fewer
	// models than the parameter does.
	availableOnRe = regexp.MustCompile(`Available on (.+?)\.(?:\s|$)`)
	// effortLevelRe matches the level a row of the effort table describes.
	effortLevelRe = regexp.MustCompile("^`([a-z]+)`$")
	// fastModelRe matches one model of the fast mode list, which writes the
	// display name and the identifier together.
	fastModelRe = regexp.MustCompile(
		`(?m)^\*\s+(Claude [^(]+?)\s*\(([a-z0-9-]+)\)`,
	)
	// priorityExceptRe matches the sentence stating Priority Tier's scope,
	// which Anthropic writes as every model less a named few.
	priorityExceptRe = regexp.MustCompile(
		`(?i)Priority Tier is supported on all available Claude ` +
			`models except (.+?)\.(?:\s|$)`,
	)
	// defaultEffortRe matches the sentence stating what a request that sets no
	// effort gets.
	defaultEffortRe = regexp.MustCompile(
		"(?i)By default, Claude uses ([a-z]+) effort",
	)
)

// applyCompatibility records a capability against every model the guide's
// compatibility block lists, together with the beta header the capability
// needs.
//
// The block states identifiers rather than display names, so nothing has to be
// resolved, and an identifier no document has established is skipped rather
// than created: these guides list Claude Mythos Preview, which Anthropic
// publishes no rate, bound or cutoff for.
//
// The header is kept as a note rather than as a capability of its own. It is
// not a second thing the model can do, it is the condition on the first, and a
// consumer reading the feature list would otherwise see two entries where
// Anthropic states one.
func (b *builder) applyCompatibility(
	doc catalog.Document,
	feature string,
) {
	header := ""
	if match := betaHeaderRe.FindSubmatch(doc.Body); match != nil {
		header = string(match[1])
	}
	for _, id := range supportedIDs(string(doc.Body)) {
		m, ok := b.models[id]
		if !ok {
			continue
		}
		m.AddList(ListFeatures, feature)
		m.AddSource(doc.URL)
		if header != "" {
			m.AddNote(feature + " requires beta header " + header)
		}
	}
}

// supportedIDs reads the identifiers a compatibility block lists.
func supportedIDs(body string) []string {
	line := supportedModelsRe.FindStringSubmatch(body)
	if line == nil {
		return nil
	}
	var out []string
	for _, match := range backtickedRe.FindAllStringSubmatch(line[1], -1) {
		out = append(out, strings.TrimSpace(match[1]))
	}
	return out
}

// applyEffort records the effort parameter and the levels each model accepts.
//
// Anthropic states this in two registers and both are read. The parameter
// itself carries a compatibility block, so the models it works on are listed
// outright. The levels are a table, and two of its five rows narrow their own
// availability to a named list of models while the other three state no
// restriction at all: those three are levels of the parameter, and their scope
// is the parameter's.
func (b *builder) applyEffort(doc catalog.Document) {
	body := string(doc.Body)
	ids := supportedIDs(body)
	unrestricted, restricted := effortLevels(body)
	fallback := defaultEffortRe.FindStringSubmatch(body)
	for _, id := range ids {
		m, ok := b.models[id]
		if !ok {
			continue
		}
		m.AddList(ListParameters, "effort")
		m.AddList(ListEffortLevels, unrestricted...)
		m.AddSource(doc.URL)
		if fallback != nil {
			m.SetAttr(AttrDefaultEffort, strings.ToLower(fallback[1]))
		}
	}
	for level, names := range restricted {
		b.addEffortLevel(doc, level, names)
	}
}

// addEffortLevel records one level against each model its row names.
func (b *builder) addEffortLevel(
	doc catalog.Document,
	level string,
	names []string,
) {
	for _, name := range names {
		m, ok := b.models[b.resolve(name)]
		if !ok {
			continue
		}
		m.AddList(ListEffortLevels, level)
		m.AddSource(doc.URL)
	}
}

// effortLevels reads the table of levels, separating the ones stated of the
// parameter from the ones stated of named models.
func effortLevels(
	body string,
) (unrestricted []string, restricted map[string][]string) {
	restricted = map[string][]string{}
	for _, t := range scanTables(body, "") {
		for _, row := range t.Rows {
			match := effortLevelRe.FindStringSubmatch(cellAt(row, 0))
			if match == nil {
				continue
			}
			if models := availableOnRe.FindStringSubmatch(
				clean(cellAt(row, 1)),
			); models != nil {
				restricted[match[1]] = splitNames(models[1])
				continue
			}
			unrestricted = append(unrestricted, match[1])
		}
	}
	return unrestricted, restricted
}

// applyFastMode records the models Anthropic offers a faster generation speed
// on, which is a capability and a rate: the pricing page carries the premium
// rate under its own tier, and this page carries the list of models the rate
// exists for.
func (b *builder) applyFastMode(doc catalog.Document) {
	for _, match := range fastModelRe.FindAllStringSubmatch(
		string(doc.Body),
		-1,
	) {
		m, ok := b.models[match[2]]
		if !ok {
			continue
		}
		m.AddList(ListFeatures, FeatureFastMode)
		m.AddSource(doc.URL)
	}
}

// applyPriorityTier records the tier Anthropic prioritizes requests in, whose
// scope it states by subtraction: every available model less the few it names.
//
// The subtraction is read rather than assumed. A page rewritten to name the
// models the tier is available on stops matching and records nothing, which is
// the failure worth having: a stale list of exclusions would mark a model as
// carrying a tier Anthropic had removed it from.
func (b *builder) applyPriorityTier(doc catalog.Document) {
	match := priorityExceptRe.FindStringSubmatch(clean(string(doc.Body)))
	if match == nil {
		return
	}
	excluded := map[string]bool{}
	for _, name := range splitNames(match[1]) {
		excluded[b.resolve(name)] = true
	}
	for _, id := range b.order {
		m := b.models[id]
		if m.Kind != KindChat || withdrawn(m) || excluded[id] {
			continue
		}
		m.AddList(ListFeatures, FeaturePriorityTier)
		m.AddSource(doc.URL)
	}
}
