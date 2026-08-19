package assemblyai

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// LimitsURL is the answer bounding what a request may carry. No card and no
// rate table states a bound, and this is the one AssemblyAI does state: not
// per model but per endpoint, for the endpoint every pre-recorded model is
// served by.
const LimitsURL = "https://www.assemblyai.com/docs/faq/" +
	"are-there-any-limits-on-file-size-or-file-duration-for-files-" +
	"submitted-to-the-api.md"

// The endpoints that answer names, one taking the file to transcribe and one
// taking the upload of a local file first.
const (
	transcriptEndpoint = "/v2/transcript"
	uploadEndpoint     = "/v2/upload"
)

// LimitMaxAudioSeconds is the longest recording a request may carry. It is
// stated in hours and kept in seconds, since a bound quoted in the unit a
// sentence happened to use is one a consumer has to convert before comparing.
const LimitMaxAudioSeconds = "max_audio_duration_seconds"

// Scalar keys holding the two sizes. They are attributes rather than limits
// because AssemblyAI writes them as "5GB" and "2.2GB", and reading either as a
// number of bytes would mean choosing between a gigabyte of 10^9 and one of
// 2^30 where the sentence chooses neither.
const (
	AttrMaxFileSize   = "max_file_size"
	AttrMaxUploadSize = "max_upload_file_size"
)

var (
	// sizeRe matches a size as the answer writes it, digits and a unit.
	sizeRe = regexp.MustCompile(`(?i)\b([\d.]+\s*[KMGT]B)\b`)
	// durationRe matches the ceiling on the length of a recording.
	durationRe = regexp.MustCompile(`(?i)maximum duration is\s*([\d.]+)\s*hour`)
)

// applyLimits records the bound on what may be submitted against every model
// the bounded endpoint serves. It is the nearest thing AssemblyAI publishes to
// a context window: what a transcription request carries is audio, so what
// bounds it is a length of audio and a size of file.
func (b *builder) applyLimits(doc catalog.Document) {
	for _, line := range strings.Split(string(doc.Body), "\n") {
		text := clean(line)
		switch {
		case strings.Contains(line, transcriptEndpoint):
			b.eachPrerecorded(doc.URL, func(m *catalog.Model) {
				if match := sizeRe.FindStringSubmatch(text); match != nil {
					m.SetAttr(AttrMaxFileSize, compact(match[1]))
				}
				m.SetLimit(LimitMaxAudioSeconds, seconds(text))
			})
		case strings.Contains(line, uploadEndpoint):
			b.eachPrerecorded(doc.URL, func(m *catalog.Model) {
				if match := sizeRe.FindStringSubmatch(text); match != nil {
					m.SetAttr(AttrMaxUploadSize, compact(match[1]))
				}
			})
		}
	}
}

// eachPrerecorded runs a function over every model served by the endpoint
// these bounds are stated for, which is the pre-recorded ones: a streaming
// model is given a connection rather than a file, and the add-on is given
// neither since it runs on whichever model is transcribing.
func (b *builder) eachPrerecorded(source string, apply func(*catalog.Model)) {
	for _, id := range b.order {
		m := b.models[id]
		if m.Attrs[AttrMode] != ModePrerecorded {
			continue
		}
		m.AddSource(source)
		apply(m)
	}
}

// seconds reads the ceiling on a recording's length, which the answer states
// in hours.
func seconds(text string) int64 {
	match := durationRe.FindStringSubmatch(text)
	if match == nil {
		return 0
	}
	return hoursToSeconds(match[1])
}

// compact removes the space a size is sometimes written with.
func compact(size string) string {
	return strings.ReplaceAll(size, " ", "")
}

// TimestampsURL is the answer stating that a completed transcript times every
// word rather than every segment. No model page says it and no card claims it,
// and it is stated once for the endpoint every pre-recorded model is served
// by, which is how the file bounds are stated too.
const TimestampsURL = "https://www.assemblyai.com/docs/faq/" +
	"does-your-api-return-timestamps-for-individual-words.md"

// LimitTimestampAccuracyMs is how far a word's timestamp may be out, which the
// same answer bounds and nothing else in the documentation does.
const LimitTimestampAccuracyMs = "word_timestamp_accuracy_ms"

var (
	// timestampsRe matches the sentence answering that words carry times.
	timestampsRe = regexp.MustCompile(
		`(?i)timestamp values for when a given word`,
	)
	// accuracyRe matches how far one of those may be out.
	accuracyRe = regexp.MustCompile(
		`(?i)accurate to within about ([\d,]+) milliseconds`,
	)
)

// applyTimestamps records that a pre-recorded transcript times every word.
// Nothing is recorded when the sentence stating it is gone, so a rewritten
// answer withdraws the claim rather than leaving it standing.
func (b *builder) applyTimestamps(doc catalog.Document) {
	body := string(doc.Body)
	if !timestampsRe.MatchString(body) {
		return
	}
	b.eachMode(ModePrerecorded, doc.URL, func(m *catalog.Model) {
		m.AddList(ListFeatures, FeatureWordTimestamps)
		if match := accuracyRe.FindStringSubmatch(body); match != nil {
			m.SetLimit(LimitTimestampAccuracyMs, count(match[1]))
		}
	})
}
