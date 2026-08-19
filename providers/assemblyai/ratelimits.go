package assemblyai

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// The three rate limit pages. AssemblyAI bounds a product rather than a model,
// and each of its three products is bounded in a different quantity: how many
// files may transcribe at once, how many connections may be opened a minute,
// how many requests a model may take a minute.
const (
	PrerecordedLimitsURL = "https://www.assemblyai.com/docs/" +
		"pre-recorded-audio/rate-limits.md"
	StreamingLimitsURL = "https://www.assemblyai.com/docs/streaming/" +
		"rate-limits.md"
	GatewayLimitsURL = "https://www.assemblyai.com/docs/llm-gateway/" +
		"rate-limits.md"
)

// The quantity each page bounds, which is not the same quantity and so is not
// the same key. A pre-recorded account is bounded on how many files are in
// flight, a streaming one on how fast connections may be opened, and a gateway
// one on requests to one model.
const (
	LimitConcurrentTranscriptions = "concurrent_transcriptions"
	LimitNewSessionsPerMinute     = "new_sessions_per_minute"
	LimitRequestsPerMinute        = "requests_per_minute"
)

// LimitRequestsPer5Min is the ceiling on API calls of every kind, which the
// pre-recorded page states beside the parallel-job limit and separately from
// it: one bounds work in flight, the other bounds calls made.
const LimitRequestsPer5Min = "requests_per_five_minutes"

// LimitMaxSessionSeconds is how long a streaming connection may stay open
// before AssemblyAI closes it. It bounds a session the way the file-size
// answer bounds a file, and it is the bound a streaming model has instead.
const LimitMaxSessionSeconds = "max_session_duration_seconds"

// limitSuffixMinimum marks a bound stated as a floor. AssemblyAI writes the
// paid tier of two of these tables as "200+", which is not the number 200: it
// is the smallest number the account starts at. Recording it under the plain
// key would state a ceiling the page does not.
const limitSuffixMinimum = "_minimum"

// headAccountType is what the rate limit tables head their first column with.
// The streaming page carries a second table with the same shape illustrating
// how the limit grows, which this is what tells apart.
const headAccountType = "account type"

var (
	// tierValueRe matches a rate limit cell, which is a count and, where the
	// account grows past it, a plus sign.
	tierValueRe = regexp.MustCompile(`^([\d,]+)(\+?)$`)
	// httpLimitRe matches the ceiling on calls of every kind.
	httpLimitRe = regexp.MustCompile(
		`(?i)maximum of ([\d,]+) requests per five minutes`,
	)
	// sessionCloseRe matches how long an abandoned connection is held open.
	sessionCloseRe = regexp.MustCompile(
		`(?i)auto-?close after ([\d.]+) hours`,
	)
)

// rateLimitKeys name what each page bounds, keyed by the page it is stated on.
var rateLimitKeys = map[string]string{
	PrerecordedLimitsURL: LimitConcurrentTranscriptions,
	StreamingLimitsURL:   LimitNewSessionsPerMinute,
	GatewayLimitsURL:     LimitRequestsPerMinute,
}

// applyRateLimits records the bound one page states against every model the
// product it bounds serves. The bound is per account rather than per model, so
// every model of that product carries the same figure: an account transcribing
// with Universal-2 and an account transcribing with Universal-3.5 Pro are
// bounded identically, and a consumer asking what a model may be driven at
// would otherwise find nothing.
func (b *builder) applyRateLimits(doc catalog.Document) {
	key, ok := rateLimitKeys[doc.URL]
	if !ok {
		return
	}
	body := string(doc.Body)
	apply := func(m *catalog.Model) {
		applyTiers(m, body, key)
		if match := httpLimitRe.FindStringSubmatch(body); match != nil {
			m.SetLimit(LimitRequestsPer5Min, count(match[1]))
		}
		m.SetLimit(LimitMaxSessionSeconds, sessionSeconds(body))
	}
	switch doc.URL {
	case PrerecordedLimitsURL:
		b.eachMode(ModePrerecorded, doc.URL, apply)
	case StreamingLimitsURL:
		b.eachMode(ModeStreaming, doc.URL, apply)
	case GatewayLimitsURL:
		b.eachKind(KindChat, doc.URL, apply)
	}
}

// applyTiers records the figure each account type is given. A cell stating no
// figure, which is how the gateway page says the free tier cannot call it at
// all, records nothing: no number is the honest reading of "Not available".
func applyTiers(m *catalog.Model, body, key string) {
	for _, table := range pipeTables(body) {
		if len(table) < 2 ||
			!strings.EqualFold(clean(cellAt(table[0], 0)), headAccountType) {
			continue
		}
		for _, row := range table[1:] {
			tier := strings.ToLower(clean(cellAt(row, 0)))
			match := tierValueRe.FindStringSubmatch(clean(cellAt(row, 1)))
			if tier == "" || match == nil {
				continue
			}
			name := key + "_" + tier
			if match[2] != "" {
				name += limitSuffixMinimum
			}
			m.SetLimit(name, count(match[1]))
		}
	}
}

// sessionSeconds reads how long a connection nobody closed is held open, which
// the streaming page states in hours and bills for in full.
func sessionSeconds(body string) int64 {
	match := sessionCloseRe.FindStringSubmatch(body)
	if match == nil {
		return 0
	}
	return hoursToSeconds(match[1])
}

// eachMode runs a function over every model recorded under one of AssemblyAI's
// three lists.
func (b *builder) eachMode(
	mode, source string,
	apply func(*catalog.Model),
) {
	for _, id := range b.order {
		m := b.models[id]
		if m.Attrs[AttrMode] != mode {
			continue
		}
		m.AddSource(source)
		apply(m)
	}
}

// eachKind runs a function over every model of one kind.
func (b *builder) eachKind(
	kind catalog.Kind,
	source string,
	apply func(*catalog.Model),
) {
	for _, id := range b.order {
		m := b.models[id]
		if m.Kind != kind {
			continue
		}
		m.AddSource(source)
		apply(m)
	}
}
