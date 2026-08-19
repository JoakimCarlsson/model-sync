package openai

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Enumeration keys the model pages populate.
const (
	ListFeatures         = catalog.ListFeatures
	ListTools            = "tools"
	ListEndpoints        = "endpoints"
	ListInputModalities  = "input_modalities"
	ListOutputModalities = "output_modalities"
	ListSnapshots        = "snapshots"
	ListReasoningEfforts = "reasoning_efforts"
)

// Sections a model page divides itself into. The first five are headings of
// their own; the rate limits and pricing sections each carry a further
// subheading naming the scope of the table under it.
const (
	sectionModelDetails = "model details"
	sectionEndpoints    = "endpoints"
	sectionFeatures     = "supported features"
	sectionTools        = "supported tools"
	sectionSnapshots    = "snapshots"
	sectionRateLimits   = "rate limits"
	sectionPricing      = "pricing"
)

// Scopes a rate limit table is published under. The first three all state the
// limits that apply to an ordinary request; only a long context table states a
// second, narrower allowance beside them.
const (
	scopeDefault     = "default"
	scopeStandard    = "standard"
	scopeStandardRPM = "standard rpm"
)

// tierFree is the row OpenAI heads the allowance it grants before any payment,
// which the speech models publish alongside the numbered tiers.
const tierFree = "free"

// Scalar keys the model pages populate.
const (
	AttrSummary         = "summary"
	AttrKnowledgeCutoff = "knowledge_cutoff"
	AttrDefaultSnapshot = "default_snapshot"
)

// notePricedAs opens the note recording that a rate was stated for the model
// an alias points at rather than for the alias itself.
const notePricedAs = "priced as "

// noteNoRate records that the rate tables state no amount for a model OpenAI
// documents and serves. It is the counterpart of notePricedAs: both exist so
// that a model the tables leave out does not read as a free one.
const noteNoRate = "no rate stated on the pricing page"

// Numeric keys the model pages populate. Rate limits are suffixed with the
// usage tier they apply to, as in rpm_tier_1.
const (
	LimitContextWindow   = "context_window"
	LimitMaxInputTokens  = "max_input_tokens"
	LimitMaxOutputTokens = "max_output_tokens"
)

var (
	modelIDRe   = regexp.MustCompile("(?m)^Model ID:\\s*`([^`]+)`")
	reasoningRe = regexp.MustCompile(
		`(?i)^Reasoning\.effort supports:\s*(.+)$`,
	)
	numberLeadRe = regexp.MustCompile(`^([\d][\d,]*)\s+(.+)$`)
	maxInputRe   = regexp.MustCompile(
		`(?i)maximum number of input tokens is ([\d,]+)`,
	)
)

// applyModelPage reads one /api/docs/models/<id>.md page.
//
// Nearly everything a page states is written as a bullet or a table row, and
// the exceptions are read where they are: the reasoning efforts as a sentence
// of their own, and the longest input a speech model takes as a clause of the
// paragraph introducing it, which is the only place that bound appears.
//
// The pricing tables on these pages are deliberately not read as prices when
// the pricing page already stated a rate for the same metric: the pricing page
// is richer, breaking every rate out by tier and context band, while a model
// page states only its standard rate. Models absent from the pricing page do
// take their rates from here, so nothing is lost. The prose bullets under the
// pricing heading are always kept as notes, because that is where multipliers
// such as the long-context uplift are written.
func (b *builder) applyModelPage(doc catalog.Document) {
	body := string(doc.Body)
	id := pageID(doc.URL, body)
	if id == "" {
		return
	}
	m := b.model(id, "")
	m.AddSource(doc.URL)
	m.SetAttr(AttrState, StateActive)

	var section, scope, priceHeader string
	var rateHeaders []string
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "# "):
			if m.Name == "" {
				m.Name = strings.TrimSpace(line[2:])
			}
			continue
		case strings.HasPrefix(line, "## "):
			section, scope = heading(line), ""
			priceHeader, rateHeaders = "", nil
			continue
		case strings.HasPrefix(line, "#"):
			scope = heading(line)
			priceHeader, rateHeaders = "", nil
			continue
		case line == "":
			continue
		}
		if strings.HasPrefix(line, "> ") {
			if text := strings.TrimSpace(
				line[2:],
			); !strings.Contains(
				text,
				"llms.txt",
			) {
				m.SetAttr(AttrSummary, text)
			}
			continue
		}
		if match := reasoningRe.FindStringSubmatch(line); match != nil {
			m.AddList(ListReasoningEfforts, splitEfforts(match[1])...)
			continue
		}
		if match := maxInputRe.FindStringSubmatch(line); match != nil {
			m.SetLimit(LimitMaxInputTokens, parseCount(match[1]))
			continue
		}
		if strings.HasPrefix(line, "|") {
			cells := splitRow(line)
			if isSeparator(cells) {
				continue
			}
			switch section {
			case sectionEndpoints:
				applyEndpointRow(m, cells)
			case sectionRateLimits:
				rateHeaders = applyRateRow(m, cells, rateHeaders, scope)
			case sectionPricing:
				priceHeader = applyModelPriceRow(m, cells, priceHeader)
			}
			continue
		}
		if bullet, ok := strings.CutPrefix(line, "- "); ok {
			switch section {
			case sectionModelDetails:
				applyDetail(m, bullet)
			case sectionFeatures:
				applyFeature(m, cleanToken(bullet))
			case sectionTools:
				m.AddList(ListTools, cleanToken(bullet))
			case sectionSnapshots:
				m.AddList(ListSnapshots, cleanToken(bullet))
			case sectionPricing:
				m.AddNote(bullet)
			}
		}
	}
	applyWeights(m, body)
	m.Kind = kindFor(m.ID, m.Lists[ListEndpoints])
}

// heading lowercases a markdown heading and strips its hashes, which is how
// every section of a model page is recognized.
func heading(line string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimLeft(line, "# ")))
}

// pageID prefers the identifier the page states over the one in its URL.
func pageID(url, body string) string {
	if match := modelIDRe.FindStringSubmatch(body); match != nil {
		return strings.TrimSpace(match[1])
	}
	base := url[strings.LastIndex(url, "/")+1:]
	return strings.TrimSuffix(base, ".md")
}

// featureModalities map a listed feature onto the modality it really names.
// OpenAI lists image input among a model's features and again among its input
// modalities, and only the second is the form the rest of the catalog states
// it in, so the feature is recorded there instead of twice.
var featureModalities = map[string]string{
	"image_input": "image",
}

// applyFeature records one bullet of the supported features list.
func applyFeature(m *catalog.Model, feature string) {
	if modality, ok := featureModalities[feature]; ok {
		m.AddList(ListInputModalities, modality)
		return
	}
	m.AddList(ListFeatures, feature)
}

// applyDetail reads one bullet from the Model details list. OpenAI writes
// these two ways: as "Key: value" and as "<number> <name>".
func applyDetail(m *catalog.Model, bullet string) {
	if key, value, ok := strings.Cut(bullet, ": "); ok {
		value = strings.TrimSpace(value)
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "default snapshot":
			m.SetAttr(AttrDefaultSnapshot, cleanToken(value))
			m.AddList(ListSnapshots, cleanToken(value))
		case "input modalities":
			m.AddList(ListInputModalities, splitList(value)...)
		case "output modalities":
			m.AddList(ListOutputModalities, splitList(value)...)
		case "maximum input tokens":
			m.SetLimit(LimitMaxInputTokens, parseCount(value))
		case "maximum output tokens":
			m.SetLimit(LimitMaxOutputTokens, parseCount(value))
		case "context window":
			m.SetLimit(LimitContextWindow, parseCount(value))
		case "knowledge cutoff":
			m.SetAttr(AttrKnowledgeCutoff, isoDate(value))
		default:
			m.AddNote(bullet)
		}
		return
	}
	if match := numberLeadRe.FindStringSubmatch(bullet); match != nil {
		count := parseCount(match[1])
		switch strings.ToLower(strings.TrimSpace(match[2])) {
		case "context window":
			m.SetLimit(LimitContextWindow, count)
		case "max output tokens":
			m.SetLimit(LimitMaxOutputTokens, count)
		case "max input tokens":
			m.SetLimit(LimitMaxInputTokens, count)
		default:
			m.AddNote(bullet)
		}
		return
	}
	if suffix, ok := strings.CutSuffix(bullet, " knowledge cutoff"); ok {
		m.SetAttr(AttrKnowledgeCutoff, isoDate(suffix))
		return
	}
	if strings.EqualFold(bullet, "reasoning token support") {
		m.AddList(ListFeatures, "reasoning")
		return
	}
	m.AddNote(bullet)
}

// applyEndpointRow records a route the model supports.
func applyEndpointRow(m *catalog.Model, cells []string) {
	if len(cells) < 3 || strings.EqualFold(cells[0], "endpoint") {
		return
	}
	if strings.EqualFold(strings.TrimSpace(cells[2]), "supported") {
		m.AddList(ListEndpoints, cleanToken(cells[1]))
	}
}

// applyRateRow records one usage tier's rate limits, returning the header row
// to use for the rows that follow.
//
// Every rate table sits under a subheading naming the scope its numbers apply
// to. Most pages write "default" and mean the model's only limits; the eleven
// models sold with a separate allowance for long prompts write "Standard" and
// "Long Context" instead, and the scope of the second is carried into the key
// so the two sets do not collide. The free tier OpenAI grants the speech
// models is a row like any other, headed "free" rather than "Tier 1", and is
// recorded under that name.
func applyRateRow(
	m *catalog.Model,
	cells, headers []string,
	scope string,
) []string {
	if strings.EqualFold(cells[0], "tier") {
		return cells
	}
	tier, ok := rateTier(cells[0])
	if headers == nil || !ok {
		return headers
	}
	for i := 1; i < len(cells) && i < len(headers); i++ {
		m.SetLimit(
			rateMetric(headers[i])+rateScope(scope)+"_"+tier,
			parseCount(cells[i]),
		)
	}
	return headers
}

// rateTier reads the label of a rate table's first column.
func rateTier(cell string) (string, bool) {
	label := strings.ToLower(strings.TrimSpace(cell))
	if label == tierFree {
		return tierFree, true
	}
	if !strings.HasPrefix(label, "tier") {
		return "", false
	}
	return strings.ReplaceAll(label, " ", "_"), true
}

// rateScope turns a rate table's subheading into the fragment that qualifies
// the keys it produces. The scopes meaning "the ordinary limit" qualify
// nothing.
func rateScope(scope string) string {
	switch scope {
	case "", scopeDefault, scopeStandard, scopeStandardRPM:
		return ""
	}
	return "_" + strings.ReplaceAll(scope, " ", "_")
}

// rateMetrics map the abbreviations OpenAI heads a rate column with onto the
// names the catalog states a rate limit under.
var rateMetrics = map[string]string{
	"rpm":                         "requests_per_minute",
	"rpd":                         "requests_per_day",
	"tpm":                         "tokens_per_minute",
	"tpd":                         "tokens_per_day",
	"ipm":                         "images_per_minute",
	"batch queue limit":           "batch_queue_limit",
	"minutes-of-audio per minute": "audio_minutes_per_minute",
}

// rateMetric names what one rate column counts. A heading OpenAI has not used
// before is slugged rather than dropped, so a new column arrives as an odd key
// instead of as silence.
func rateMetric(header string) string {
	h := strings.ToLower(strings.Join(strings.Fields(header), " "))
	if name, ok := rateMetrics[h]; ok {
		return name
	}
	return strings.ReplaceAll(h, " ", "_")
}

// applyModelPriceRow records a rate from a model page, skipping metrics the
// pricing page already covered.
func applyModelPriceRow(
	m *catalog.Model,
	cells []string,
	header string,
) string {
	if len(cells) < 2 {
		return header
	}
	if strings.EqualFold(cells[0], "metric") {
		return "metric"
	}
	if header != "metric" {
		return header
	}
	metric, ok := metricFromName(cells[0])
	if !ok || hasMetric(m, metric) {
		return header
	}
	a := parseAmount(cells[1])
	if !a.Found {
		return header
	}
	unit := a.Unit
	if unit == "" && len(cells) > 2 {
		unit, _ = unitFor(cells[2])
	}
	m.AddPrice(catalog.Price{
		Metric:   metric,
		Unit:     unit,
		Amount:   a.Value,
		Currency: currency,
		Dims:     catalog.Dims{DimTier: TierStandard},
		Note:     a.Note,
	})
	return header
}

// metricFromName maps the Metric column of a model page's pricing table.
func metricFromName(name string) (catalog.Metric, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "input":
		return MetricInputTokens, true
	case "cached input":
		return MetricCachedInputTokens, true
	case "cache writes", "cache write":
		return MetricCacheWriteTokens, true
	case "output":
		return MetricOutputTokens, true
	}
	return "", false
}

// hasMetric reports whether a rate for this metric is already recorded.
func hasMetric(m *catalog.Model, metric catalog.Metric) bool {
	for _, p := range m.Prices {
		if p.Metric == metric {
			return true
		}
	}
	return false
}

// nameKinds map a fragment of an identifier onto what a model does, for the
// models whose routes do not say. An audio model reached over the chat route
// still works on audio.
var nameKinds = []struct {
	fragment string
	kind     catalog.Kind
}{
	{"dall-e", KindImage},
	{"image", KindImage},
	{"sora", KindVideo},
	{"moderation", KindModeration},
	{"embedding", KindEmbedding},
	{"whisper", KindTranscription},
	{"transcribe", KindTranscription},
	{"realtime", KindRealtime},
	{"tts", KindAudio},
	{"audio", KindAudio},
}

// kindFor settles what a model is, preferring the routes it serves and
// falling back to its name.
func kindFor(id string, endpoints []string) catalog.Kind {
	if kind := kindFromEndpoints(endpoints); kind != KindChat {
		return kind
	}
	lower := strings.ToLower(id)
	for _, entry := range nameKinds {
		if strings.Contains(lower, entry.fragment) {
			return entry.kind
		}
	}
	return KindChat
}

// kindFromEndpoints infers what a model is from the routes it serves, for
// models the pricing page never mentioned.
func kindFromEndpoints(endpoints []string) catalog.Kind {
	has := func(route string) bool {
		for _, e := range endpoints {
			if strings.Contains(e, route) {
				return true
			}
		}
		return false
	}
	switch {
	case has("images/"):
		return KindImage
	case has("videos"):
		return KindVideo
	case has("embeddings"):
		return KindEmbedding
	case has("moderations"):
		return KindModeration
	case has("audio/transcriptions"), has("audio/translations"),
		has("realtime/transcription"), has("realtime/translations"):
		return KindTranscription
	case has("audio/speech"):
		return KindAudio
	case has("realtime"):
		return KindRealtime
	}
	return KindChat
}

// splitList splits a comma-separated bullet value.
func splitList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, cleanToken(p))
	}
	return out
}

// splitEfforts reads the reasoning effort levels out of the sentence stating
// them, which lists them comma separated with a trailing "and".
func splitEfforts(value string) []string {
	value = strings.TrimSuffix(strings.TrimSpace(value), ".")
	value = strings.ReplaceAll(value, " and ", ", ")
	out := make([]string, 0, 8)
	for _, p := range strings.Split(value, ",") {
		token := cleanToken(p)
		token = strings.TrimSpace(strings.TrimSuffix(token, "(default)"))
		if token != "" {
			out = append(out, token)
		}
	}
	return out
}

// cleanToken strips the decoration OpenAI puts around identifiers.
func cleanToken(s string) string {
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(s), "`*"))
}

// parseCount reads a grouped decimal such as "1,050,000".
func parseCount(s string) int64 {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
	n, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return 0
	}
	return n
}
