package deepgram

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// whisperFamily is the heading the models and languages overview lists the
// Whisper models under, and so the value their family attribute carries.
const whisperFamily = "deepgram-whisper-cloud"

// AttrParameterCount is how many weights a model has. Deepgram states it for
// the Whisper sizes alone, which are otherwise identical to one another.
const AttrParameterCount = "parameter_count"

// Numeric bounds the pre-recorded guide states.
const (
	// LimitProcessingSeconds is how long Deepgram will spend on one recording
	// before returning a gateway timeout.
	LimitProcessingSeconds = "max_processing_seconds"
)

// AttrFileSize is the largest recording Deepgram accepts, kept as the guide
// writes it because it states a size in gigabytes and not a count of bytes.
const AttrFileSize = "max_file_size"

var (
	// whisperLanguagesRe matches the sentence listing the languages Whisper
	// understands, which the guide writes as backticked codes.
	whisperLanguagesRe = regexp.MustCompile(
		"(?i)Languages supported by whisper include:([^\n]+)",
	)
	// supportedRe matches the mark the Whisper guide uses for a Deepgram
	// feature Whisper answers for.
	supportedRe = regexp.MustCompile(`\x{2705}`)
	// unsupportedRe matches the mark it uses for one Whisper does not.
	unsupportedRe = regexp.MustCompile(`\x{274C}`)
	// sizeRe matches one Whisper size and the weights it carries, which the
	// model reference writes as a bullet per size.
	sizeRe = regexp.MustCompile(
		"(?im)^\\*\\s+`([a-z]+)`:\\s+Contains\\s+([\\d,]+)\\s*M\\s+parameters",
	)
	// processingRe matches the two processing ceilings the pre-recorded guide
	// states, each against the models it applies to.
	processingRe = regexp.MustCompile(
		`(?i)(\d+)\s+minutes?\s+\(([A-Za-z/ ]+)\)`,
	)
	// fileSizeRe matches the largest recording the guide accepts.
	fileSizeRe = regexp.MustCompile(`(?i)Maximum\s+(\d+\s*[KMGT]B)`)
)

// applyWhisper reads the Whisper Cloud guide. The overview names the Whisper
// sizes but points at this page for what they understand, and this page is
// the only one stating which Deepgram features Whisper answers for and which
// it does not, which a feature overview written for the whole product cannot
// say.
func (b *builder) applyWhisper(doc catalog.Document) {
	body := string(doc.Body)
	languages := whisperLanguages(body)
	granted, denied := whisperFeatures(body)
	b.each(func(m *catalog.Model) {
		if m.Attrs[AttrFamily] != whisperFamily {
			return
		}
		m.AddList(ListLanguages, languages...)
		for _, f := range denied {
			b.deny(m.ID, f)
		}
		b.addFeature(m, granted...)
		m.AddSource(doc.URL)
	})
}

// whisperLanguages reads the language codes the guide lists.
func whisperLanguages(body string) []string {
	match := whisperLanguagesRe.FindStringSubmatch(body)
	if match == nil {
		return nil
	}
	return codes(match[1])
}

// whisperFeatures reads the table of Deepgram features against their status,
// returning what Whisper supports and what it does not.
func whisperFeatures(body string) (granted, denied []string) {
	for _, match := range tableRowRe.FindAllStringSubmatch(body, -1) {
		cells := strings.Split(match[1], "|")
		if len(cells) < 2 {
			continue
		}
		feature, ok := docFeatures[slugID(label(cells[0]))]
		if !ok {
			continue
		}
		switch {
		case supportedRe.MatchString(cells[1]):
			granted = append(granted, feature)
		case unsupportedRe.MatchString(cells[1]):
			denied = append(denied, feature)
		}
	}
	return granted, denied
}

// applyOptionDetail reads the model parameter reference, which is where
// Deepgram states how large each Whisper size is and who may ask for a model
// trained on their own data.
func (b *builder) applyOptionDetail(doc catalog.Document) {
	body := string(doc.Body)
	for _, match := range sizeRe.FindAllStringSubmatch(body, -1) {
		m, ok := b.models["whisper-"+match[1]]
		if !ok {
			continue
		}
		m.SetAttr(AttrParameterCount, match[2]+"M")
		m.AddSource(doc.URL)
	}
}

// applyBatchLimits reads the pre-recorded transcription guide, which states
// what one submitted recording may weigh and how long Deepgram will spend on
// it. The ceilings are properties of the request rather than of a plan, which
// is why they are not in the concurrency reference.
func (b *builder) applyBatchLimits(doc catalog.Document) {
	body := string(doc.Body)
	size := fileSizeRe.FindStringSubmatch(body)
	seconds := processingSeconds(body)
	b.each(func(m *catalog.Model) {
		if !batch(m) {
			return
		}
		if size != nil {
			m.SetAttr(AttrFileSize, size[1])
			m.AddSource(doc.URL)
		}
		limit, ok := seconds[m.Attrs[AttrFamily]]
		if !ok {
			return
		}
		m.SetLimit(LimitProcessingSeconds, limit)
		m.AddSource(doc.URL)
	})
}

// processingSeconds reads how long Deepgram will spend on a recording, which
// the guide states as one ceiling for the models it names and a longer one for
// Whisper. A family the sentence does not name carries no ceiling rather than
// the one written beside a family it does name.
func processingSeconds(body string) map[string]int64 {
	out := map[string]int64{}
	for _, match := range processingRe.FindAllStringSubmatch(body, -1) {
		minutes, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		for _, name := range strings.Split(match[2], "/") {
			family := slugID(name)
			if family == "whisper" {
				family = whisperFamily
			}
			out[family] = int64(minutes) * 60
		}
	}
	return out
}

// batch reports whether a model transcribes a finished recording, which is
// what the guide's ceilings apply to. They are ceilings on the audio a
// transcription request carries, so a capability sold to run inside such a
// request does not carry them.
func batch(m *catalog.Model) bool {
	if m.Kind != KindTranscription {
		return false
	}
	for _, f := range m.Lists[catalog.ListFeatures] {
		if f == FeatureBatch {
			return true
		}
	}
	return false
}
