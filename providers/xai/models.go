package xai

import (
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// modalityArrow separates the input modalities from the output modalities in
// the one line xAI states both on.
const modalityArrow = "→"

// rateLimitKeys maps the rate limit rows of a model page onto numeric keys.
var rateLimitKeys = map[string]string{
	"requests per second":  LimitRequestsPerSec,
	"requests per minute":  LimitRequestsPerMin,
	"requests per hour":    LimitRequestsPerHour,
	"tokens per minute":    LimitTokensPerMinute,
	"images per minute":    LimitImagesPerMinute,
	"videos per minute":    LimitVideosPerMinute,
	"sessions per hour":    LimitSessionsPerHour,
	"minutes per minute":   LimitMinutesPerMinute,
	"concurrent jobs":      LimitConcurrentJobs,
	"max session duration": LimitSessionMinutes,
}

// voiceCapabilities translate the capability bullets of a voice page onto
// feature names. Those pages state a capability as a sentence rather than as a
// labelled yes, so the phrase that opens the bullet is what names it; a bullet
// that names something other than a capability, such as the audio formats a
// model accepts, matches nothing here and is left out rather than turned into
// a feature nobody could ask for.
var voiceCapabilities = []struct {
	phrase  string
	feature string
}{
	{"function calling", catalog.CapabilityFunctionCalling},
	{"keyterm", catalog.CapabilityKeyterms},
	{"real-time interim", catalog.CapabilityRealtime},
	{"streaming", FeatureStreaming},
	{"web search", FeatureWebSearch},
	{"x search", FeatureXSearch},
	{"collections search", FeatureCollectionsSearch},
	{"mcp", FeatureRemoteMCP},
}

// applyModelPage reads one /developers/models/<id> page.
//
// The pricing table on these pages restates the pricing page in a transposed
// form, with the prompt bands as columns, and is read only for models the
// pricing page did not cover.
//
// xAI writes these pages in two shapes. A text or generation model states its
// facts as bullets; a voice model states the same facts as a two-column table
// and its capabilities as prose, which is why the two sections are read by
// their own readers rather than line by line as one.
func (b *builder) applyModelPage(doc catalog.Document) {
	body := string(doc.Body)
	m := b.model(b.pageFor(doc.URL, body), "")
	m.AddSource(doc.URL)

	var section string
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "# "):
			if m.Name == "" {
				m.Name = strings.TrimSpace(line[2:])
			}
			continue
		case strings.HasPrefix(line, "#"):
			section = strings.ToLower(
				strings.TrimSpace(strings.TrimLeft(line, "# ")),
			)
			continue
		case line == "":
			continue
		}
		switch section {
		case "at a glance", "availability":
			applyGlance(m, line)
		case "capabilities":
			applyCapability(m, line)
		case "pricing":
			applyBullet(m, line)
		case "rate limits":
			applyLimitRow(m, line)
		case "regions":
			applyRegions(m, line)
		}
	}
	if m.Kind == "" {
		m.Kind = kindFromModalities(m.Lists[ListOutputModalities])
	}
}

// pageFor resolves the model a page describes, following the link from a page
// published under a mode's name to the model that mode runs on.
func (b *builder) pageFor(url, body string) string {
	id := pageID(url, body)
	if mapped, ok := b.pages[id]; ok {
		return mapped
	}
	return id
}

// pageID prefers the model name the page states over the one in its URL.
func pageID(url, body string) string {
	for _, raw := range strings.Split(body, "\n") {
		if key, value, ok := bulletParts(raw); ok && key == "model name" {
			return value
		}
	}
	base := url[strings.LastIndex(url, "/")+1:]
	return strings.TrimSuffix(base, ".md")
}

// bulletParts splits a "- **Key:** value" bullet.
func bulletParts(line string) (key, value string, ok bool) {
	bullet, ok := strings.CutPrefix(strings.TrimSpace(line), "- ")
	if !ok {
		return "", "", false
	}
	key, value, ok = strings.Cut(bullet, ":")
	if !ok {
		return "", "", false
	}
	return strings.ToLower(clean(key)), clean(value), true
}

// applyGlance records one fact of the summary, which a text or generation page
// states as a bullet and a voice page as a row of a two-column table.
func applyGlance(m *catalog.Model, line string) {
	if !strings.HasPrefix(line, "|") {
		applyBullet(m, line)
		return
	}
	cells := splitRow(line)
	if isSeparator(cells) || len(cells) < 2 {
		return
	}
	applyFact(m, strings.ToLower(clean(cells[0])), clean(cells[1]))
}

// applyCapability records one capability, which a text page states as a
// labelled yes and a voice page as a sentence naming it.
func applyCapability(m *catalog.Model, line string) {
	if _, _, ok := bulletParts(line); ok {
		applyBullet(m, line)
		return
	}
	phrase, ok := capabilityPhrase(line)
	if !ok {
		return
	}
	for _, c := range voiceCapabilities {
		if strings.Contains(phrase, c.phrase) {
			m.AddList(ListFeatures, c.feature)
		}
	}
}

// capabilityPhrase reduces a prose capability bullet to the words that name
// it, which is what precedes the parenthesis or the dash xAI qualifies it
// with. Reading the whole sentence instead would match the qualification: the
// end-of-turn detection of the transcriber is described as being a streaming
// feature, and is not itself streaming.
func capabilityPhrase(line string) (string, bool) {
	text := strings.TrimSpace(line)
	for _, mark := range []string{"* ", "- "} {
		if rest, ok := strings.CutPrefix(text, mark); ok {
			head, _, _ := strings.Cut(rest, "(")
			head, _, _ = strings.Cut(head, voiceStateMark)
			return strings.ToLower(clean(head)), true
		}
	}
	return "", false
}

// applyBullet records one fact stated as a bullet.
func applyBullet(m *catalog.Model, line string) {
	key, value, ok := bulletParts(line)
	if !ok {
		return
	}
	applyFact(m, key, value)
}

// applyFact records one key and value, however the page stated the pair.
func applyFact(m *catalog.Model, key, value string) {
	if key == "" || value == "" {
		return
	}
	switch key {
	case "model name":
	case "modalities":
		applyModalities(m, value)
	case "context window":
		m.SetLimit(LimitContextWindow, parseCount(value))
	case "aliases":
		for _, alias := range strings.Split(value, ",") {
			m.AddList(ListAliases, clean(alias))
		}
	case "knowledge cutoff", "knowledge cut-off":
		m.SetAttr(AttrKnowledgeCutoff, isoDate(value))
	case "region", "cluster":
		for _, region := range strings.Split(value, ",") {
			m.AddList(ListRegions, clean(region))
		}
	case "output", "input", "cached input":
		applyBulletPrice(m, key, value)
	default:
		if strings.EqualFold(value, "yes") {
			m.AddList(ListFeatures, strings.ReplaceAll(slugID(key), "-", "_"))
		}
	}
}

// applyBulletPrice records a rate stated as a bullet rather than in a table,
// which is how the image and video pages give their single rate.
func applyBulletPrice(m *catalog.Model, key, value string) {
	a := parseAmount(value)
	if !a.Found {
		return
	}
	metric := MetricOutputTokens
	switch {
	case key == "input":
		metric = MetricInputTokens
	case key == "cached input":
		metric = MetricCachedInputTokens
	case a.Unit == UnitPerImage:
		metric = MetricImageOutput
	case a.Unit == UnitPerSecond:
		metric = MetricVideoOutput
	}
	m.AddPrice(catalog.Price{
		Metric:   metric,
		Unit:     a.Unit,
		Amount:   a.Value,
		Currency: currency,
		Note:     a.Note,
	})
}

// applyModalities reads the "text, image → text" line. The voice pages write
// the same line capitalised, so the names are lowered rather than passed
// through: a catalog holding both "Audio" and "audio" answers a question about
// modality with half its models.
func applyModalities(m *catalog.Model, value string) {
	in, out, ok := strings.Cut(value, modalityArrow)
	if !ok {
		return
	}
	for _, modality := range strings.Split(in, ",") {
		m.AddList(ListInputModalities, strings.ToLower(clean(modality)))
	}
	for _, modality := range strings.Split(out, ",") {
		m.AddList(ListOutputModalities, strings.ToLower(clean(modality)))
	}
}

// applyLimitRow reads one row of the rate limits table.
func applyLimitRow(m *catalog.Model, line string) {
	if !strings.HasPrefix(line, "|") {
		return
	}
	cells := splitRow(line)
	if isSeparator(cells) || len(cells) < 2 {
		return
	}
	if key, ok := rateLimitKeys[strings.ToLower(clean(cells[0]))]; ok {
		m.SetLimit(key, parseCount(cells[1]))
	}
}

// applyRegions reads the line listing where a model is served.
func applyRegions(m *catalog.Model, line string) {
	rest, ok := strings.CutPrefix(clean(line), "Available in:")
	if !ok {
		return
	}
	for _, region := range strings.Split(rest, ",") {
		m.AddList(ListRegions, clean(region))
	}
}

// kindFromModalities infers what a model is from what it emits, for models the
// pricing tables never named.
func kindFromModalities(outputs []string) catalog.Kind {
	for _, out := range outputs {
		switch strings.ToLower(out) {
		case "image":
			return KindImage
		case "video":
			return KindVideo
		case "audio", "speech":
			return KindVoice
		}
	}
	return KindChat
}
