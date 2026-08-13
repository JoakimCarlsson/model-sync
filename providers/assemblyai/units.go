package assemblyai

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Metrics AssemblyAI bills on. Both are quoted per hour but they count
// different things, so they are not one metric with two rates.
const (
	// MetricAudio counts hours of audio submitted.
	MetricAudio catalog.Metric = "audio"
	// MetricSession counts hours a streaming connection stays open,
	// regardless of how much audio crosses it.
	MetricSession catalog.Metric = "session"
)

// UnitPerHour is the only denominator AssemblyAI quotes.
const UnitPerHour catalog.Unit = "per_hour"

// KindTranscription is the only kind AssemblyAI publishes.
const KindTranscription catalog.Kind = "transcription"

// Modes AssemblyAI separates its models into.
const (
	ModePrerecorded = "pre-recorded"
	ModeStreaming   = "streaming"
	ModeAddOn       = "add-on"
)

// DimMode records which of those a rate belongs to.
const DimMode = "mode"

// Scalar keys the models page populates.
const (
	AttrMode             = "mode"
	AttrVolumeDiscounts  = "volume_discounts"
	AttrDocumentationURL = "documentation_url"
)

// ListCapabilities holds the bullet points a model card lists.
const ListCapabilities = "capabilities"

// Enumeration keys holding what a model takes and returns.
const (
	ListInputModalities  = "input_modalities"
	ListOutputModalities = "output_modalities"
)

// Modalities every AssemblyAI model handles. It sells transcription only, so
// each one hears audio and writes text, whether it is given a recording or a
// connection.
const (
	ModalityText  = "text"
	ModalityAudio = "audio"
)

var (
	cardRe = regexp.MustCompile(
		`(?is)<Card\s+title="([^"]*)"[^>]*?(?:href="([^"]*)")?[^>]*>(.*?)</Card>`,
	)
	listItemRe = regexp.MustCompile(`(?is)<li[^>]*>(.*?)</li>`)
	tagRe      = regexp.MustCompile(`(?s)<[^>]*>`)
	linkRe     = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	amountRe   = regexp.MustCompile(`\$\s*([\d,]*\.?\d+)`)
)

// clean strips markdown and MDX decoration from a value.
func clean(text string) string {
	s := linkRe.ReplaceAllString(text, "$1")
	s = tagRe.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, `\$`, "$")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "`", "")
	return strings.Join(strings.Fields(s), " ")
}

// parseAmount reads a rate cell such as "\$0.21/hr".
func parseAmount(cell string) (float64, bool) {
	match := amountRe.FindStringSubmatch(clean(cell))
	if match == nil {
		return 0, false
	}
	value, err := strconv.ParseFloat(strings.ReplaceAll(match[1], ",", ""), 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

// slugID turns a display name such as "Universal-3.5 Pro" into an identifier.
// AssemblyAI names models one way in its cards and rate tables and does not
// publish an API identifier for them, so the display name is what there is.
func slugID(name string) string {
	s := strings.ToLower(clean(name))
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' {
			return r
		}
		return '-'
	}, s)
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}
