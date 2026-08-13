package elevenlabs

import (
	"os"
	"path/filepath"
	"slices"
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
		if len(byID[id].Lists[ListCapabilities]) == 0 {
			t.Errorf("%s: no capabilities", id)
		}
	}
	if got := byID["eleven_flash_v2"].Name; got != "" {
		t.Errorf("eleven_flash_v2: got name %q, want none", got)
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
