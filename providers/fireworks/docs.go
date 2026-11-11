package fireworks

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// everyModelRe matches the sentence the grammar guide states its scope in,
// which is every model Fireworks serves. It is matched rather than assumed:
// the guide worked through one model by name and then said the feature is not
// particular to it, and a guide rewritten to name a list stops yielding the
// capability for all of them.
var everyModelRe = regexp.MustCompile(
	`(?i)all fireworks models support this feature`,
)

// applyStructuredOutputs records the capability against every model that
// generates a response.
//
// This is the one capability Fireworks states outside a model's own page. That
// page carries a row for tool use and one for image input and nothing for
// this, and the guide explains why: the answer is the same for every model, so
// there is nothing to state per model.
//
// The guide says all of them, and it means all the models it is about. It
// constrains what a model writes, and an embedding model writes nothing: it
// returns a vector, which no schema describes and no grammar could constrain.
// Reading "all" past the models the sentence is about would state a capability
// that cannot be exercised.
func (b *builder) applyStructuredOutputs(doc catalog.Document) {
	if !everyModelRe.Match(doc.Body) {
		return
	}
	for _, id := range b.order {
		m := b.models[id]
		if m.Kind != KindChat {
			continue
		}
		m.AddList(ListFeatures, FeatureStructured, FeatureGrammarMode)
		m.AddSource(doc.URL)
	}
}

// The chat completion reference documents reasoning_effort model family by
// model family, under a heading of its own. That list is the only place
// Fireworks says which models reason: the guide to reasoning works its
// examples through a placeholder model, and no page in the library carries a
// row for it.
var (
	reasoningSectionRe = regexp.MustCompile(
		`(?s)reasoning_effort:.*?Model-specific behavior:?\*\*(.*?)\n\s*\w+:\n`,
	)
	reasoningFamilyRe = regexp.MustCompile(`(?m)^\s*-\s+\*\*(.+?)\*\*:`)
)

// applyReasoning records the capability against the models the chat completion
// reference documents a reasoning effort for.
//
// The reference states what each family does with the parameter, down to which
// efforts it accepts and whether reasoning is on when nothing is passed. A
// family named there is a family Fireworks says reasons; a family absent from
// it is not, which is why nothing is inferred for the models it leaves out.
func (b *builder) applyReasoning(doc catalog.Document) {
	families := reasoningFamilies(string(doc.Body))
	for _, id := range b.order {
		m := b.models[id]
		if m.Kind != KindChat {
			continue
		}
		for _, family := range families {
			if !namesModel(family, m.Name) {
				continue
			}
			m.AddList(ListFeatures, FeatureReasoning)
			m.AddSource(doc.URL)
			break
		}
	}
}

// reasoningFamilies returns the names the reference's model-specific
// paragraphs are written against. A paragraph heads several models where one
// behaviour covers them all, either as a comma separated list or as the models
// a chat template's name is followed by in brackets.
func reasoningFamilies(body string) []string {
	section := reasoningSectionRe.FindStringSubmatch(body)
	if section == nil {
		return nil
	}
	var out []string
	for _, match := range reasoningFamilyRe.FindAllStringSubmatch(
		section[1],
		-1,
	) {
		named := match[1]
		if _, inside, ok := strings.Cut(named, "("); ok {
			named = strings.TrimSuffix(inside, ")")
		}
		for _, name := range strings.Split(named, ",") {
			if name = strings.TrimSpace(name); name != "" {
				out = append(out, name)
			}
		}
	}
	return out
}

// namesModel reports whether family names the model: every word of the family
// inside the model's name, in order and adjacent. Adjacency is what keeps a
// paragraph about MiniMax M2 off MiniMax M2.7, which is a different model and
// documented, when it is documented at all, in a paragraph of its own.
func namesModel(family, model string) bool {
	want, have := nameWords(family), nameWords(model)
	if len(want) == 0 || len(want) > len(have) {
		return false
	}
	for at := 0; at+len(want) <= len(have); at++ {
		if equalWords(have[at:at+len(want)], want) {
			return true
		}
	}
	return false
}

// equalWords reports whether two runs of words are the same.
func equalWords(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// The rate limit page states its ceilings per tier, and puts a model in a
// tier by naming it or, failing that, by its total parameter count. These read
// a ceiling, the rule for a model whose count is unknown, and the column
// headers the ceilings are written under.
var (
	ceilingRe     = regexp.MustCompile(`^([\d.]+)\s*([MmKk])$`)
	unknownTierRe = regexp.MustCompile(
		`(?i)without a known parameter count use \*\*(\w+)\*\*`,
	)
	ceilingColumns = map[string]string{
		"uncached":  LimitUncachedTPM,
		"total":     LimitInputTPM,
		"generated": LimitOutputTokenTPM,
	}
)

// rateTier is one row of the rate limit page's ceilings: the tier's name, the
// parameter counts it covers, at least Min and under Max where Max is set, and
// the ceilings it starts a caller at.
type rateTier struct {
	Name     string
	Min, Max float64
	Limits   map[string]int64
}

// covers reports whether a model of this many parameters falls in the tier.
func (t rateTier) covers(params float64) bool {
	return params >= t.Min && (t.Max == 0 || params < t.Max)
}

// applyRateLimits records the default ceilings against every model served on
// the shared fleet.
//
// They apply to nothing else: a deployment of the caller's own is bounded by
// the GPUs under it rather than by a token rate, which the page says outright.
// A model takes the tier the page names it under; a model it does not name
// takes the tier its parameter count falls in, and a model whose count is not
// known takes the tier the page says such models use.
func (b *builder) applyRateLimits(doc catalog.Document) {
	body := string(doc.Body)
	tiers := rateTiers(body)
	if len(tiers) == 0 {
		return
	}
	named := tierMembers(body, doc.URL)
	var unknown string
	if match := unknownTierRe.FindStringSubmatch(body); match != nil {
		unknown = match[1]
	}
	for _, id := range b.order {
		m := b.models[id]
		if !servesServerless(m) {
			continue
		}
		tier, ok := tierFor(tiers, named, unknown, m)
		if !ok {
			continue
		}
		for key, value := range tier.Limits {
			m.SetLimit(key, value)
		}
		m.AddSource(doc.URL)
	}
}

// rateTiers reads the table of ceilings, which is the one whose columns count
// generated tokens.
func rateTiers(body string) []rateTier {
	var out []rateTier
	for _, t := range scanHTMLTables(body) {
		if len(t.Headers) < 3 ||
			!strings.Contains(strings.ToLower(strings.Join(t.Headers, " ")),
				"generated") {
			continue
		}
		for _, row := range t.Rows {
			if len(row) != len(t.Headers) {
				continue
			}
			tier := rateTier{
				Name:   clean(row[0]),
				Limits: map[string]int64{},
			}
			tier.Min, tier.Max = tierBounds(row[1])
			for at, header := range t.Headers[2:] {
				key := ceilingKey(header)
				match := ceilingRe.FindStringSubmatch(clean(row[2+at]))
				if key == "" || match == nil {
					continue
				}
				tier.Limits[key] = tokenCount(match[1], match[2])
			}
			out = append(out, tier)
		}
	}
	return out
}

// ceilingKey says which limit a column of the ceilings table states.
func ceilingKey(header string) string {
	lower := strings.ToLower(header)
	for _, word := range []string{"uncached", "total", "generated"} {
		if strings.Contains(lower, word) {
			return ceilingColumns[word]
		}
	}
	return ""
}

// tierBounds reads the parameter counts a tier covers, which the page writes
// as "< 800B", "800B – < 1.6T" or "≥ 1.6T": a count before a "<" is the
// least the tier takes and a count after it the most it stops short of.
func tierBounds(cell string) (lower, upper float64) {
	counts := scaleRe.FindAllStringSubmatch(cell, -1)
	if len(counts) == 0 {
		return 0, 0
	}
	bounded := strings.Contains(cell, "<")
	if bounded {
		upper = countOf(counts[len(counts)-1])
	}
	if !bounded || len(counts) > 1 {
		lower = countOf(counts[0])
	}
	return lower, upper
}

// tierMembers reads the table naming the models of each tier, returning the
// tier of every model it names, keyed by the words of the name.
func tierMembers(body, source string) map[string]string {
	out := map[string]string{}
	for _, t := range scanTables(body, source) {
		if len(t.Headers) < 2 ||
			!strings.EqualFold(clean(t.Headers[1]), "models") {
			continue
		}
		for _, row := range t.Rows {
			for _, name := range strings.Split(cellAt(row, 1), ",") {
				if words := nameWords(name); len(words) > 0 {
					out[strings.Join(words, " ")] = clean(cellAt(row, 0))
				}
			}
		}
	}
	return out
}

// tierFor returns the tier a model is in: the one the page names it under, or
// the one its parameter count falls in, or for a model whose count is unknown
// the one named unknown.
func tierFor(
	tiers []rateTier,
	named map[string]string,
	unknown string,
	m *catalog.Model,
) (rateTier, bool) {
	name, listed := named[strings.Join(nameWords(m.Name), " ")]
	params := parameterCount(m.Attrs[AttrParameterCount])
	for _, tier := range tiers {
		switch {
		case listed:
			if strings.EqualFold(tier.Name, name) {
				return tier, true
			}
		case params > 0:
			if tier.covers(params) {
				return tier, true
			}
		case strings.EqualFold(tier.Name, unknown):
			return tier, true
		}
	}
	return rateTier{}, false
}

// cachingRe matches the sentence saying which models have a prompt cache in
// front of them without being asked for one.
var cachingRe = regexp.MustCompile(
	`(?i)prompt caching is on by default for every serverless model`,
)

// applyServerlessOverview records the capability the overview states for the
// whole of the shared fleet.
func (b *builder) applyServerlessOverview(doc catalog.Document) {
	if !cachingRe.Match(doc.Body) {
		return
	}
	for _, id := range b.order {
		m := b.models[id]
		if !servesServerless(m) {
			continue
		}
		m.AddList(ListFeatures, FeaturePromptCaching)
		m.AddSource(doc.URL)
	}
}

// routerRe matches the identifier a table gives for a serving variant, which
// is a router in front of the model rather than a model of its own.
var routerRe = regexp.MustCompile(
	`accounts/[a-z0-9-]+/routers/[A-Za-z0-9._-]+`,
)

// applyRouters records the identifiers a caller sends to reach a model's
// faster or US-only serving path.
//
// Both tables name the model in the first column and give the identifier in
// the second, and the name carries the variant as a suffix, so "GLM 5.2 Fast"
// is the model GLM 5.2 reached another way. The identifier is recorded as an
// alias of that model, because it is not a model in its own right: the same
// weights answer, at a different rate.
func (b *builder) applyRouters(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		for _, row := range t.Rows {
			router := routerRe.FindString(strings.Join(row, " "))
			if router == "" {
				continue
			}
			name := clean(cellAt(row, 0))
			for _, suffix := range servingSuffixes {
				if base, ok := strings.CutSuffix(
					strings.ToLower(name),
					" "+suffix,
				); ok {
					name = strings.TrimSpace(name[:len(base)])
					break
				}
			}
			b.addAlias(name, router, doc.URL)
		}
	}
}

// addAlias records a router against the one model of that name, leaving it
// unrecorded where the name reaches none or more than one.
func (b *builder) addAlias(name, router, source string) {
	var found []string
	for _, id := range b.order {
		if equalWords(nameWords(name), nameWords(b.models[id].Name)) {
			found = append(found, id)
		}
	}
	if len(found) != 1 {
		return
	}
	b.models[found[0]].AddList(ListAliases, router)
	b.models[found[0]].AddSource(source)
}

// The shared trainer's rate card is a table the training guide builds from a
// list it carries in the page, one entry per model, keyed by the identifier
// the model is served under.
var trainerEntryRe = regexp.MustCompile(
	`(?s)slug:\s*"([A-Za-z0-9._-]+)".*?ctxTokens:\s*(\d+),\s*` +
		`prefill:\s*"\$([\d.]+)",\s*cachedPrefill:\s*"\$([\d.]+)",\s*` +
		`sample:\s*"\$([\d.]+)",\s*train:\s*"\$([\d.]+)"`,
)

// trainerRowRe matches one row of the shared trainer's rate card where the
// guide writes it as a table keyed by the model's display name: the name, the
// context the trainer takes, and the four rates in the order trainerMetrics
// lists them.
var trainerRowRe = regexp.MustCompile(
	`(?m)^\|\s*([^|]+?)\s*\|\s*(\d+[KkMm])\s*\|` +
		`\s*\\?\$([\d.]+)\s*\|\s*\\?\$([\d.]+)\s*\|` +
		`\s*\\?\$([\d.]+)\s*\|\s*\\?\$([\d.]+)\s*\|`,
)

// trainerMetrics are the four things the shared trainer bills, in the order
// its rate card writes them.
var trainerMetrics = []catalog.Metric{
	MetricTrainingPrefillTokens,
	MetricTrainingCachedPrefillTokens,
	MetricTrainingSampleTokens,
	MetricTrainingTokens,
}

// applyServerlessTraining records what the shared trainer charges for the few
// models it is open on.
//
// The rate card quotes a context window per model as well as four rates, and
// the window is part of the offer rather than the model's own: it is the
// longest sequence that trainer will take. So it is recorded as a dimension of
// the rate and not as the model's context window.
func (b *builder) applyServerlessTraining(doc catalog.Document) {
	body := string(doc.Body)
	for _, entry := range trainerEntryRe.FindAllStringSubmatch(body, -1) {
		if m, ok := b.models[fireworksAccount+"/"+entry[1]]; ok {
			priceTrainer(m, entry[2], entry[3:], doc.URL)
		}
	}
	for _, row := range trainerRowRe.FindAllStringSubmatch(body, -1) {
		if id, ok := b.modelNamed(row[1]); ok {
			priceTrainer(
				b.models[id],
				strings.ToUpper(row[2]),
				row[3:],
				doc.URL,
			)
		}
	}
}

// priceTrainer records the shared trainer's four rates for one model, under
// the context window the trainer takes for it as written by the rate card.
func priceTrainer(
	m *catalog.Model,
	window string,
	rates []string,
	source string,
) {
	dims := catalog.Dims{
		DimSurface:       SurfaceServerlessTrainer,
		DimContextWindow: window,
	}
	for at, metric := range trainerMetrics {
		if at >= len(rates) {
			break
		}
		amount, err := strconv.ParseFloat(rates[at], 64)
		if err != nil {
			continue
		}
		m.AddPrice(catalog.Price{
			Metric:   metric,
			Unit:     UnitPer1MTokens,
			Amount:   amount,
			Currency: currency,
			Dims:     dims,
		})
	}
	m.AddSource(source)
}

// modelNamed returns the one model whose name is the given one word for word,
// reporting false where the name reaches none or more than one.
func (b *builder) modelNamed(name string) (string, bool) {
	var found []string
	for _, id := range b.order {
		if equalWords(nameWords(name), nameWords(b.models[id].Name)) {
			found = append(found, id)
		}
	}
	if len(found) != 1 {
		return "", false
	}
	return found[0], true
}

// applyTrainingPricing reads the managed training rate card, which prices a
// training token by the size of the model being trained and by the method.
//
// The card is on the same page as the hourly rates for GPUs, which price
// nothing per model, so the training card is picked out by its columns: it is
// the one whose columns are training methods.
func (b *builder) applyTrainingPricing(doc catalog.Document) {
	for _, t := range scanHTMLTables(string(doc.Body)) {
		if !trainingCard(t.Headers) {
			continue
		}
		var bands []band
		for _, row := range t.Rows {
			amounts := amountsOf(row[1:])
			if len(amounts) != len(t.Headers)-1 {
				continue
			}
			if parsed, ok := parseBand(row[0], amounts); ok {
				bands = append(bands, parsed)
			}
		}
		b.priceTraining(t.Headers, bands, doc.URL)
	}
}

// trainingCard reports whether a rate card's columns are training methods.
func trainingCard(headers []string) bool {
	if len(headers) < 2 {
		return false
	}
	for _, h := range headers {
		if strings.Contains(strings.ToLower(h), "lora") {
			return true
		}
	}
	return false
}

// priceTraining puts a rate card's bands onto the models the library says can
// be trained. A model that cannot be is left alone: the card prices the job,
// and there is no job to price.
func (b *builder) priceTraining(headers []string, bands []band, source string) {
	for _, id := range b.order {
		m := b.models[id]
		if !trainable(m) {
			continue
		}
		match, ok := bandFor(
			bands,
			parameterCount(m.Attrs[AttrParameterCount]),
			m.Attrs[AttrMixtureOfExperts] == "true",
		)
		if !ok {
			continue
		}
		for at, amount := range match.Amounts {
			m.AddPrice(catalog.Price{
				Metric:   MetricTrainingTokens,
				Unit:     UnitPer1MTokens,
				Amount:   amount,
				Currency: currency,
				Dims: catalog.Dims{
					DimSurface:  SurfaceManaged,
					DimMethod:   methodKey(headers[at+1]),
					DimSizeBand: match.Label,
				},
			})
		}
		m.AddSource(source)
	}
}

// trainable reports whether the library said the model can be fine-tuned.
func trainable(m *catalog.Model) bool {
	for _, v := range m.Lists[ListDeployment] {
		if v == DeploymentFineTuning {
			return true
		}
	}
	return false
}

// methodKey turns a rate card column into the value the method dimension takes.
func methodKey(header string) string {
	key := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		}
		return '_'
	}, header)
	for strings.Contains(key, "__") {
		key = strings.ReplaceAll(key, "__", "_")
	}
	return strings.Trim(key, "_")
}

// amountsOf reads one amount per cell, in column order.
func amountsOf(cells []string) []float64 {
	out := make([]float64, 0, len(cells))
	for _, cell := range cells {
		amount, ok := parseAmount(cell)
		if !ok {
			return nil
		}
		out = append(out, amount)
	}
	return out
}
