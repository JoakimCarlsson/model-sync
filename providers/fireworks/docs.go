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

// The ceilings the rate limit page states, which it states once for the three
// things it counts. They are the starting point of a limit that then grows and
// shrinks with use, and the page says they are scoped per account and per
// model, which is why they are recorded against a model at all.
var (
	inputTPMRe    = regexp.MustCompile(`([\d.]+)([MmKk]) Total Prompt TPM`)
	uncachedTPMRe = regexp.MustCompile(`([\d.]+)([MmKk]) Uncached Prompt TPM`)
	outputTPMRe   = regexp.MustCompile(`([\d.]+)([MmKk]) Generated TPM`)
)

// applyRateLimits records the default ceilings against every model served on
// the shared fleet.
//
// They apply to nothing else: a deployment of the caller's own is bounded by
// the GPUs under it rather than by a token rate, which the page says outright.
func (b *builder) applyRateLimits(doc catalog.Document) {
	body := string(doc.Body)
	limits := map[string]int64{
		LimitInputTPM:       tpm(inputTPMRe, body),
		LimitUncachedTPM:    tpm(uncachedTPMRe, body),
		LimitOutputTokenTPM: tpm(outputTPMRe, body),
	}
	for _, id := range b.order {
		m := b.models[id]
		if !servesServerless(m) {
			continue
		}
		for key, value := range limits {
			m.SetLimit(key, value)
		}
		if limits[LimitInputTPM] > 0 {
			m.AddSource(doc.URL)
		}
	}
}

// tpm reads one ceiling, which the page abbreviates.
func tpm(re *regexp.Regexp, body string) int64 {
	match := re.FindStringSubmatch(body)
	if match == nil {
		return 0
	}
	return tokenCount(match[1], match[2])
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
	for _, entry := range trainerEntryRe.FindAllStringSubmatch(
		string(doc.Body),
		-1,
	) {
		m, ok := b.models[fireworksAccount+"/"+entry[1]]
		if !ok {
			continue
		}
		dims := catalog.Dims{
			DimSurface:       SurfaceServerlessTrainer,
			DimContextWindow: entry[2],
		}
		for at, metric := range trainerMetrics {
			amount, err := strconv.ParseFloat(entry[3+at], 64)
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
		m.AddSource(doc.URL)
	}
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
