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
	"requests per second": LimitRequestsPerSec,
	"requests per minute": LimitRequestsPerMin,
	"requests per hour":   LimitRequestsPerHour,
	"tokens per minute":   LimitTokensPerMinute,
	"images per minute":   LimitImagesPerMinute,
	"videos per minute":   LimitVideosPerMinute,
	"sessions per hour":   LimitSessionsPerHour,
	"minutes per minute":  LimitMinutesPerMinute,
	"concurrent jobs":     LimitConcurrentJobs,
}

// applyModelPage reads one /developers/models/<id> page.
//
// The pricing table on these pages restates the pricing page in a transposed
// form, with the prompt bands as columns, and is read only for models the
// pricing page did not cover.
func (b *builder) applyModelPage(doc catalog.Document) {
	body := string(doc.Body)
	m := b.model(pageID(doc.URL, body), "")
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
		case "at a glance", "capabilities", "pricing":
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

// applyBullet records one fact stated as a bullet.
func applyBullet(m *catalog.Model, line string) {
	key, value, ok := bulletParts(line)
	if !ok || value == "" {
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

// applyModalities reads the "text, image → text" line.
func applyModalities(m *catalog.Model, value string) {
	in, out, ok := strings.Cut(value, modalityArrow)
	if !ok {
		return
	}
	for _, modality := range strings.Split(in, ",") {
		m.AddList(ListInputModalities, clean(modality))
	}
	for _, modality := range strings.Split(out, ",") {
		m.AddList(ListOutputModalities, clean(modality))
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
