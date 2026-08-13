package elevenlabs

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// fixtures returns the documents to parse, read from disk so the test never
// touches the network.
func fixtures(t *testing.T) []catalog.Document {
	t.Helper()
	docs := []catalog.Document{}
	for _, f := range []struct {
		url  string
		file string
	}{
		{ModelsURL, "models.md"},
		{PricingURL, "pricing_api.html"},
	} {
		body, err := os.ReadFile(filepath.Join("testdata", f.file))
		if err != nil {
			t.Fatal(err)
		}
		docs = append(docs, catalog.Document{URL: f.url, Body: body})
	}
	return docs
}

// parse runs the parser over the fixtures and indexes the result.
func parse(t *testing.T) map[string]catalog.Model {
	t.Helper()
	models, err := New().Parse(fixtures(t))
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]catalog.Model, len(models))
	for _, m := range models {
		byID[m.ID] = m
	}
	return byID
}

func TestParsePrices(t *testing.T) {
	byID := parse(t)
	want := map[string]catalog.Price{
		"eleven_flash_v2_5": {
			Metric: MetricSpeech, Unit: UnitPer1KChars, Amount: 0.05,
		},
		"eleven_turbo_v2": {
			Metric: MetricSpeech, Unit: UnitPer1KChars, Amount: 0.05,
		},
		"eleven_multilingual_v2": {
			Metric: MetricSpeech, Unit: UnitPer1KChars, Amount: 0.10,
		},
		"eleven_v3": {
			Metric: MetricSpeech, Unit: UnitPer1KChars, Amount: 0.10,
		},
		"scribe_v2": {
			Metric: MetricAudio, Unit: UnitPerHour, Amount: 0.22,
		},
		"scribe_v2_realtime": {
			Metric: MetricAudio, Unit: UnitPerHour, Amount: 0.39,
		},
		"music_v2": {
			Metric: MetricAudioOutput, Unit: UnitPerMinute, Amount: 0.15,
		},
		"eleven_multilingual_sts_v2": {
			Metric: MetricAudio, Unit: UnitPerMinute, Amount: 0.12,
		},
		"eleven_text_to_sound_v2": {
			Metric: MetricAudioOutput, Unit: UnitPerMinute, Amount: 0.12,
		},
	}
	for id, price := range want {
		m, ok := byID[id]
		if !ok {
			t.Errorf("%s: not parsed", id)
			continue
		}
		if len(m.Prices) != 1 {
			t.Errorf("%s: got %d prices, want 1", id, len(m.Prices))
			continue
		}
		got := m.Prices[0]
		if got.Metric != price.Metric || got.Unit != price.Unit ||
			got.Amount != price.Amount {
			t.Errorf(
				"%s: got %s %s %v, want %s %s %v",
				id,
				got.Metric,
				got.Unit,
				got.Amount,
				price.Metric,
				price.Unit,
				price.Amount,
			)
		}
		if got.Currency != currency {
			t.Errorf("%s: got currency %q", id, got.Currency)
		}
		if !hasSource(m, PricingURL) {
			t.Errorf("%s: price is not attributed to the pricing page", id)
		}
	}
}

// TestParseUnpriced pins the models ElevenLabs states no dollar rate for, so
// that a rate appearing for them later is noticed rather than assumed.
func TestParseUnpriced(t *testing.T) {
	byID := parse(t)
	for _, id := range []string{
		"eleven_multilingual_v1",
		"eleven_ttv_v3",
		"eleven_multilingual_ttv_v2",
		"scribe_v1",
	} {
		m, ok := byID[id]
		if !ok {
			t.Errorf("%s: not parsed", id)
			continue
		}
		if len(m.Prices) != 0 {
			t.Errorf("%s: got %d prices, want none", id, len(m.Prices))
		}
	}
}

func TestParseModels(t *testing.T) {
	byID := parse(t)
	if len(byID) != 18 {
		t.Errorf("got %d models, want 18", len(byID))
	}
	kinds := map[string]catalog.Kind{
		"eleven_flash_v2_5":          KindSpeech,
		"scribe_v2":                  KindTranscription,
		"eleven_english_sts_v2":      KindVoiceChanger,
		"eleven_ttv_v3":              KindVoiceDesign,
		"music_v1":                   KindMusic,
		"eleven_text_to_sound_v2":    KindSoundEffects,
		"eleven_multilingual_ttv_v2": KindVoiceDesign,
	}
	for id, kind := range kinds {
		if got := byID[id].Kind; got != kind {
			t.Errorf("%s: got kind %q, want %q", id, got, kind)
		}
	}
	if got := byID["eleven_turbo_v2"].Attrs[AttrState]; got != StateDeprecated {
		t.Errorf("eleven_turbo_v2: got state %q", got)
	}
	if got := byID["scribe_v2"].Attrs[AttrState]; got != StateCurrent {
		t.Errorf("scribe_v2: got state %q", got)
	}
	if got := byID["eleven_flash_v2_5"].Limits[LimitCharacterLimit]; got !=
		40000 {
		t.Errorf("eleven_flash_v2_5: got character limit %d", got)
	}
}

// hasSource reports whether a model is attributed to a URL.
func hasSource(m catalog.Model, url string) bool {
	return slices.Contains(m.Sources, url)
}

// TestParseCards covers the flagship cards, which are the only place
// ElevenLabs states a display name or what a model can do.
func TestParseCards(t *testing.T) {
	byID := parse(t)
	names := map[string]string{
		"eleven_v3":          "Eleven v3",
		"eleven_flash_v2_5":  "Eleven Flash v2.5",
		"scribe_v2_realtime": "Scribe v2 Realtime",
		"music_v2":           "Eleven Music v2",
	}
	for id, name := range names {
		if got := byID[id].Name; got != name {
			t.Errorf("%s: got name %q, want %q", id, got, name)
		}
	}
	if got := byID["eleven_flash_v2"].Name; got != "" {
		t.Errorf("eleven_flash_v2: got name %q, want none", got)
	}
}

// TestParseBullets covers the split every card bullet goes through. A bullet
// is a sentence with a fact in it, and each half of the fact lands where a
// consumer can use it: the bound as a number, the capability as a name, and
// the sentence itself nowhere.
func TestParseBullets(t *testing.T) {
	byID := parse(t)
	for _, c := range []struct {
		id    string
		key   string
		value int64
	}{
		{"eleven_v3", LimitCharacterLimit, 5_000},
		{"eleven_v3", LimitLanguageCount, 70},
		{"eleven_flash_v2_5", LimitCharacterLimit, 40_000},
		{"scribe_v2_realtime", LimitLanguageCount, 90},
		{"scribe_v2_realtime", LimitEntityTypes, 65},
	} {
		if got := byID[c.id].Limits[c.key]; got != c.value {
			t.Errorf("%s: got %s %d, want %d", c.id, c.key, got, c.value)
		}
	}
	features := byID["scribe_v2_realtime"].Lists[ListFeatures]
	for _, want := range []string{
		FeatureRealtime,
		FeatureTimestamps,
		FeatureEntities,
	} {
		if !slices.Contains(features, want) {
			t.Errorf("got features %q, want %q among them", features, want)
		}
	}
	if got := byID["scribe_v2_realtime"].Attrs[AttrLatency]; got == "" {
		t.Error("no latency recorded from the bullet quoting one")
	}
	if v3 := byID["eleven_v3"].Lists[ListFeatures]; !slices.Contains(
		v3,
		FeatureDialogue,
	) {
		t.Errorf("got %q, want the dialogue bullet named", v3)
	}
	for id, m := range byID {
		for _, feature := range m.Lists[ListFeatures] {
			if strings.ContainsAny(feature, " ,(") {
				t.Errorf("%s: prose in the capability list: %q", id, feature)
			}
		}
		if len(m.Lists["capabilities"]) != 0 {
			t.Errorf("%s: bullets still recorded as written", id)
		}
	}
}

// TestParseModalities covers what a model takes and returns, which its
// identifier and the heading above its card say.
func TestParseModalities(t *testing.T) {
	byID := parse(t)
	cases := map[string][2]string{
		"eleven_v3":                  {"text", "audio"},
		"scribe_v2":                  {"audio", "text"},
		"eleven_multilingual_sts_v2": {"audio", "audio"},
		"eleven_text_to_sound_v2":    {"text", "audio"},
	}
	for id, want := range cases {
		m := byID[id]
		if got := m.Lists[ListInputModalities]; !slices.Equal(
			got,
			[]string{want[0]},
		) {
			t.Errorf("%s: got input %v, want %v", id, got, want[0])
		}
		if got := m.Lists[ListOutputModalities]; !slices.Equal(
			got,
			[]string{want[1]},
		) {
			t.Errorf("%s: got output %v, want %v", id, got, want[1])
		}
	}
}

// TestParseLanguageReference covers the cell naming another model instead of a
// language, which is a reference to that model's list rather than a code.
func TestParseLanguageReference(t *testing.T) {
	byID := parse(t)
	flash := byID["eleven_flash_v2_5"].Lists[ListLanguages]
	base := byID["eleven_multilingual_v2"].Lists[ListLanguages]
	if len(base) == 0 {
		t.Fatal("eleven_multilingual_v2: no languages")
	}
	if len(flash) != len(base)+3 {
		t.Errorf("got %d languages, want the %d of its base plus three",
			len(flash), len(base))
	}
	if slices.Contains(flash, "eleven_multilingual_v2") {
		t.Errorf("a model identifier was recorded as a language: %v", flash)
	}
}

// TestParseUnpricedCarryReason covers the served models no card covers. Voice
// design has no card at all, and the card covering the multilingual line is
// headed for its later versions, so the first generation model falls outside it.
// Each carries a note saying so rather than reading as free.
func TestParseUnpricedCarryReason(t *testing.T) {
	byID := parse(t)
	for _, id := range []string{
		"eleven_multilingual_ttv_v2",
		"eleven_ttv_v3",
		"eleven_multilingual_v1",
	} {
		m, ok := byID[id]
		if !ok {
			t.Errorf("%s: not parsed", id)
			continue
		}
		if len(m.Prices) != 0 {
			t.Errorf("%s: got %d prices, want none", id, len(m.Prices))
		}
		if !slices.Contains(m.Notes, noteNoCard) {
			t.Errorf("%s: got notes %v, want the uncovered note", id, m.Notes)
		}
	}
	for id, m := range byID {
		if len(m.Prices) > 0 && slices.Contains(m.Notes, noteNoCard) {
			t.Errorf("%s: priced and marked uncovered", id)
		}
		if m.Attrs[AttrState] == StateDeprecated &&
			slices.Contains(m.Notes, noteNoCard) {
			t.Errorf("%s: withdrawn and marked uncovered", id)
		}
	}
}

// TestParseWithoutPricingPage covers the guard on that note: losing the pricing
// page must not turn every model into one ElevenLabs states no rate for.
func TestParseWithoutPricingPage(t *testing.T) {
	docs := []catalog.Document{}
	for _, doc := range fixtures(t) {
		if doc.URL != PricingURL {
			docs = append(docs, doc)
		}
	}
	models, err := New().Parse(docs)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range models {
		if slices.Contains(m.Notes, noteNoCard) {
			t.Errorf("%s: marked uncovered with no pricing page read", m.ID)
		}
	}
}

// TestApplyBulletSplits covers each shape of bullet ElevenLabs writes, taken
// from the cards as published. The models page carries whichever of them its
// current flagships happen to have, so the rules are exercised here rather
// than left to whatever the fixture caught.
func TestApplyBulletSplits(t *testing.T) {
	cases := []struct {
		bullet  string
		feature string
		key     string
		value   int64
	}{
		{"40,000 character limit", "", LimitCharacterLimit, 40_000},
		{
			"Speaker diarization, up to 32 speakers",
			FeatureDiarization,
			LimitSpeakers,
			32,
		},
		{
			"Keyterm prompting, up to 1000 terms",
			FeatureKeyterms,
			LimitKeyterms,
			1_000,
		},
		{
			"Entity detection, 65 entity types",
			FeatureEntities,
			LimitEntityTypes,
			65,
		},
		{"Accurate transcription in 90+ languages", "", LimitLanguageCount, 90},
		{"32 languages supported", "", LimitLanguageCount, 32},
		{"Smart language detection", FeatureLangDetection, "", 0},
		{"Precise word-level timestamps", FeatureTimestamps, "", 0},
	}
	for _, c := range cases {
		var m catalog.Model
		applyBullet(&m, c.bullet)
		if c.key != "" && m.Limits[c.key] != c.value {
			t.Errorf(
				"%q: got %s %d, want %d",
				c.bullet,
				c.key,
				m.Limits[c.key],
				c.value,
			)
		}
		if c.feature == "" {
			if len(m.Lists[ListFeatures]) != 0 {
				t.Errorf("%q: named %q", c.bullet, m.Lists[ListFeatures])
			}
			continue
		}
		if !slices.Contains(m.Lists[ListFeatures], c.feature) {
			t.Errorf("%q: got %q, want %q", c.bullet, m.Lists[ListFeatures], c.feature)
		}
	}
	var m catalog.Model
	applyBullet(&m, "Most stable on long-form generations")
	if len(m.Lists[ListFeatures]) != 0 || len(m.Limits) != 0 {
		t.Errorf("a bullet stating no fact was recorded: %+v", m)
	}
}
