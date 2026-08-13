package elevenlabs

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Numeric keys a card's bullets state.
const (
	// LimitSpeakers is how many speakers a transcription model tells apart.
	LimitSpeakers = "max_speakers"
	// LimitKeyterms is how many terms may be given to bias a transcription.
	LimitKeyterms = "max_keyterms"
	// LimitEntityTypes is how many kinds of entity a model detects.
	LimitEntityTypes = "entity_types"
	// LimitLanguageCount is how many languages a model covers where the card
	// counts them instead of naming them. Where it names them they go in
	// ListLanguages, which is a list of languages and not a count of them.
	LimitLanguageCount = "language_count"
)

// AttrLatency is the response delay a card quotes, kept as written because
// ElevenLabs writes it as an approximation with a footnote marker on it and
// rounding that away would state a precision it did not.
const AttrLatency = "latency"

// Capabilities a card's bullets name.
const (
	FeatureDiarization   = "speaker_diarization"
	FeatureKeyterms      = "keyterm_prompting"
	FeatureEntities      = "entity_detection"
	FeatureTimestamps    = "word_timestamps"
	FeatureRealtime      = "realtime"
	FeatureLangDetection = "language_detection"
	FeatureCodeSwitching = "code_switching"
	FeatureDialogue      = "multi_speaker_dialogue"
)

// bulletRules read one card bullet.
//
// A bullet is a sentence, not a capability name, and most of them hold a fact
// with a number in it: a ceiling, a count, a delay. Each rule matches the
// sentences that carry one fact, names the capability it implies and says
// where the number belongs. A bullet no rule matches states no fact that the
// catalog has a place for and is dropped.
var bulletRules = []struct {
	re      *regexp.Regexp
	feature string
	limit   string
}{
	{
		regexp.MustCompile(`(?i)([\d,]+)\s*character limit`),
		"",
		LimitCharacterLimit,
	},
	{
		regexp.MustCompile(`(?i)diarization.*?up to ([\d,]+) speakers`),
		FeatureDiarization,
		LimitSpeakers,
	},
	{
		regexp.MustCompile(`(?i)keyterms? prompting,?\s*up to ([\d,]+)`),
		FeatureKeyterms,
		LimitKeyterms,
	},
	{
		regexp.MustCompile(`(?i)entity detection,\s*([\d,]+) entity types`),
		FeatureEntities,
		LimitEntityTypes,
	},
	{
		regexp.MustCompile(`(?i)\b([\d,]+)\+?\s*languages\b`),
		"",
		LimitLanguageCount,
	},
	{
		regexp.MustCompile(`(?i)word-level timestamps`),
		FeatureTimestamps,
		"",
	},
	{
		regexp.MustCompile(`(?i)real-?time transcription`),
		FeatureRealtime,
		"",
	},
	{
		regexp.MustCompile(`(?i)language detection`),
		FeatureLangDetection,
		"",
	},
	{
		regexp.MustCompile(`(?i)code switching`),
		FeatureCodeSwitching,
		"",
	},
	{
		regexp.MustCompile(`(?i)multi-speaker dialogue`),
		FeatureDialogue,
		"",
	},
}

// latencyRe matches the delay a card quotes, which it writes in brackets as an
// approximation and hangs a footnote marker off.
var latencyRe = regexp.MustCompile(`(?i)latency\s*\(([^)]*\d[^)]*)\)`)

// applyBullet records what one card bullet states.
//
// Nothing goes in as written. A bullet is prose and the enumerations it used
// to be added to are not: a consumer reading a capability list should find
// capability names in it, not "40,000 character limit", which is a bound, nor
// "Accurate transcription in 90+ languages", which is a count.
func applyBullet(m *catalog.Model, bullet string) {
	if match := latencyRe.FindStringSubmatch(bullet); match != nil {
		m.SetAttr(AttrLatency, strings.TrimSpace(match[1]))
	}
	for _, rule := range bulletRules {
		match := rule.re.FindStringSubmatch(bullet)
		if match == nil {
			continue
		}
		if rule.feature != "" {
			m.AddList(ListFeatures, rule.feature)
		}
		if rule.limit != "" && len(match) > 1 {
			m.SetLimit(rule.limit, count(match[1]))
		}
	}
}

// count reads a figure a card writes with thousands separators in it.
func count(text string) int64 {
	n, err := strconv.ParseInt(strings.ReplaceAll(text, ",", ""), 10, 64)
	if err != nil {
		return 0
	}
	return n
}
