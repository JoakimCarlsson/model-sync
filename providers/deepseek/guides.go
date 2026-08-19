package deepseek

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// contentParts map a content part onto the modality it carries and the
// enumeration that modality belongs in.
var contentParts = map[string]struct {
	list     string
	modality string
}{
	"input_text":  {ListInputModalities, ModalityText},
	"input_image": {ListInputModalities, ModalityImage},
	"output_text": {ListOutputModalities, ModalityText},
}

// The row labels and the column heading the Responses API guide states its
// compatibility with.
const (
	messageRow     = "message"
	webSearchRow   = "web_search"
	parameterHead  = "parameter"
	supportStatus  = "support status"
	unsupported    = "not supported"
	supportedWord  = "supported"
	parameterSplit = " / "
)

// applyResponsesGuide reads what the models take, what they return, and which
// request parameters DeepSeek honours.
//
// DeepSeek states no modality against a model, because both models answer the
// one API. What it does state is the content parts that API carries, in a
// table of input items whose message row names input_text and output_text and
// then says, in the sentence after, that image and file inputs are not
// supported. The row is therefore read a sentence at a time and a sentence
// denying support is skipped, so that the input_image named inside that one is
// not read as something the models accept.
//
// The guide's other tables state a support status per row as well. The one
// headed "Parameter" enumerates request parameters and is the only place
// DeepSeek lists parameters with a status, so it fills the parameter list; the
// two headed "Type" enumerate input items and tools, and are read for the
// rows that name a capability rather than a shape.
func (b *builder) applyResponsesGuide(doc catalog.Document) {
	parameters := false
	for _, match := range rowRe.FindAllStringSubmatch(string(doc.Body), -1) {
		cells := rowCells(match[1])
		if len(cells) < 2 {
			continue
		}
		if strings.EqualFold(cells[1], supportStatus) {
			parameters = strings.EqualFold(cells[0], parameterHead)
			continue
		}
		b.applyCompatibility(doc.URL, cells[0], cells[1], parameters)
	}
}

// applyCompatibility records one row of a compatibility table.
func (b *builder) applyCompatibility(source, label, status string, param bool) {
	switch {
	case strings.EqualFold(label, messageRow):
		for _, sentence := range strings.Split(status, ". ") {
			if strings.Contains(strings.ToLower(sentence), unsupported) {
				continue
			}
			b.applyContentParts(source, sentence)
		}
	case !isSupported(status):
	case param:
		b.applyAll(source, func(m *catalog.Model) {
			m.AddList(ListParameters, strings.Split(label, parameterSplit)...)
		})
	case strings.HasPrefix(label, webSearchRow):
		b.applyAll(source, func(m *catalog.Model) {
			m.AddList(ListFeatures, FeatureWebSearch)
		})
	}
}

// isSupported reports whether a support status grants the row rather than
// denying it.
//
// DeepSeek writes four kinds of status. "Supported" and "Partially supported"
// both grant, and the second cannot be read as denying because what it
// qualifies is stated in the same cell. "Ignored" says nothing of support and
// denies by silence. "Not supported" denies while still containing the word,
// which is why the denial is tested before the grant.
func isSupported(status string) bool {
	lowered := strings.ToLower(strings.TrimSpace(status))
	if strings.HasPrefix(lowered, unsupported) {
		return false
	}
	return strings.Contains(lowered, supportedWord)
}

// applyContentParts records the modality of every content part one sentence
// names, against every model, since the sentence describes the API rather than
// a model.
func (b *builder) applyContentParts(source, sentence string) {
	for name, part := range contentParts {
		if !strings.Contains(sentence, name) {
			continue
		}
		b.applyAll(source, func(m *catalog.Model) {
			m.AddList(part.list, part.modality)
		})
	}
}

// effortHead is the heading of the column listing the effort levels the
// thinking mode accepts.
const effortHead = "requested effort"

var (
	// thinkingDefaultRe matches the footnote stating what the thinking mode
	// does when the caller says nothing.
	thinkingDefaultRe = regexp.MustCompile(
		`(?i)Thinking mode is (\w+) by default, ` +
			`with the default effort being (\w+)`,
	)
	// thinkingIgnoresRe matches the sentence naming the sampling parameters
	// the thinking mode accepts and then disregards.
	thinkingIgnoresRe = regexp.MustCompile(
		`(?i)Thinking mode does not support the (.+?) parameters`,
	)
)

// applyThinkingGuide reads the effort levels and the defaults.
//
// The pricing table says only that both models support a thinking mode and
// default to it. This guide says how far that mode can be pushed, in a table
// mapping the effort a caller asks for onto the effort the model applies, and
// in a footnote naming the default of each. Both models are named in the
// footnote as having the same mapping, so every row applies to both.
func (b *builder) applyThinkingGuide(doc catalog.Document) {
	body := string(doc.Body)
	efforts := false
	for _, match := range rowRe.FindAllStringSubmatch(body, -1) {
		cells := rowCells(match[1])
		if len(cells) < 2 {
			continue
		}
		if strings.EqualFold(cells[0], effortHead) {
			efforts = true
			continue
		}
		if !efforts {
			continue
		}
		b.applyAll(doc.URL, func(m *catalog.Model) {
			m.AddList(ListReasoningEfforts, strings.ToLower(cells[0]))
		})
	}
	prose := text(body)
	if match := thinkingDefaultRe.FindStringSubmatch(prose); match != nil {
		b.applyAll(doc.URL, func(m *catalog.Model) {
			m.SetAttr(AttrThinkingModeDefault, strings.ToLower(match[1]))
			m.SetAttr(AttrDefaultReasoningEffort, strings.ToLower(match[2]))
		})
	}
	if match := thinkingIgnoresRe.FindStringSubmatch(prose); match != nil {
		names := parameterNames(match[1])
		b.applyAll(doc.URL, func(m *catalog.Model) {
			m.AddList(ListThinkingUnsupported, names...)
		})
	}
}

// parameterNames splits a sentence's list of parameter names into the names.
func parameterNames(list string) []string {
	out := []string{}
	for _, part := range strings.Split(strings.ReplaceAll(list, " or ", ","), ",") {
		if name := strings.TrimSpace(part); name != "" {
			out = append(out, name)
		}
	}
	return out
}

var (
	// fimMaxRe matches the ceiling the beta FIM endpoint generates under.
	fimMaxRe = regexp.MustCompile(
		`(?i)max tokens of FIM completion is ([\d,]*\.?\d+\s*[km]?)`,
	)
	// betaBaseRe matches the base URL the beta endpoints answer on.
	betaBaseRe = regexp.MustCompile(`base_url\s*=\s*"?(https://\S+?/beta)"?`)
)

// applyFIMGuide reads the beta endpoint's own ceiling and its own base URL.
//
// The pricing table marks FIM completion supported and says which mode it runs
// in, and stops there. It is a different endpoint on a different base URL with
// an output ceiling two orders below the chat endpoints', and none of that is
// derivable from the pricing table, so it is read here.
func (b *builder) applyFIMGuide(doc catalog.Document) {
	prose := text(string(doc.Body))
	if match := fimMaxRe.FindStringSubmatch(prose); match != nil {
		limit := parseCount(match[1])
		b.applyAll(doc.URL, func(m *catalog.Model) {
			m.SetLimit(LimitFIMMaxOutputTokens, limit)
		})
	}
	if match := betaBaseRe.FindStringSubmatch(prose); match != nil {
		b.applyAll(doc.URL, func(m *catalog.Model) {
			m.SetAttr(AttrBetaBaseURL, match[1])
		})
	}
}

var (
	// cacheDefaultRe matches the sentence saying the cache needs no opting
	// into.
	cacheDefaultRe = regexp.MustCompile(
		`(?i)Context Caching on Disk Technology is enabled by default`,
	)
	// cacheLifetimeRe matches the sentence saying how long a cached prefix
	// outlives its last use.
	cacheLifetimeRe = regexp.MustCompile(
		`(?i)automatically cleared, usually within ([^.]+)\.`,
	)
)

// applyCacheGuide reads the context cache.
//
// The two input rates on the pricing table are a cache hit and a cache miss,
// and the page charging them says nothing about what makes one. This guide
// does: the cache is on by default for every account, it is matched on whole
// persisted prefix units rather than on any prefix, and a unit survives its
// last use for a stated while. The capability is recorded because a caller
// choosing a model on cache pricing needs to know the cache is not something
// to opt into.
func (b *builder) applyCacheGuide(doc catalog.Document) {
	prose := text(string(doc.Body))
	if cacheDefaultRe.MatchString(prose) {
		b.applyAll(doc.URL, func(m *catalog.Model) {
			m.AddList(ListFeatures, FeatureContextCaching)
		})
	}
	if match := cacheLifetimeRe.FindStringSubmatch(prose); match != nil {
		b.applyAll(doc.URL, func(m *catalog.Model) {
			m.SetAttr(AttrContextCacheLifetime, strings.TrimSpace(match[1]))
		})
	}
}

var (
	// perUserRe matches the sentence introducing the per-user_id ceilings,
	// and perUserLimitRe matches one model's ceiling inside it.
	perUserRe = regexp.MustCompile(
		`(?i)For each user_id\s*,\s*the concurrency limit[^.]*\.`,
	)
	perUserLimitRe = regexp.MustCompile(`for ([a-z0-9.\-]+) is ([\d,]+)`)
	// scopeRe matches what the account-level ceiling is counted against.
	scopeRe = regexp.MustCompile(
		`(?i)Concurrency limits are calculated at the (\w+) level`,
	)
	// queueRe matches how long an unstarted request is held open.
	queueRe = regexp.MustCompile(
		`(?i)has not started inference after (\d+) minutes?, ` +
			`the server will close the connection`,
	)
)

// secondsPerMinute converts the one duration DeepSeek states in minutes.
const secondsPerMinute = 60

// applyRateLimitPage reads the second concurrency ceiling and the queue.
//
// DeepSeek limits concurrency and nothing else: this page states no requests
// per minute and no tokens per minute, and the pricing table's concurrency row
// points here for the detail. What it adds is that the ceiling is counted per
// account rather than per key, that an expanded account is additionally
// ceilinged per user_id it passes, and that a request which has not started
// inference is dropped after a stated wait.
func (b *builder) applyRateLimitPage(doc catalog.Document) {
	prose := text(string(doc.Body))
	if sentence := perUserRe.FindString(prose); sentence != "" {
		for _, match := range perUserLimitRe.FindAllStringSubmatch(sentence, -1) {
			m, ok := b.models[match[1]]
			if !ok {
				continue
			}
			m.SetLimit(LimitConcurrencyPerUserID, parseCount(match[2]))
			m.AddSource(doc.URL)
		}
	}
	if match := scopeRe.FindStringSubmatch(prose); match != nil {
		b.applyAll(doc.URL, func(m *catalog.Model) {
			m.SetAttr(AttrConcurrencyScope, strings.ToLower(match[1]))
		})
	}
	if match := queueRe.FindStringSubmatch(prose); match != nil {
		seconds := parseCount(match[1]) * secondsPerMinute
		b.applyAll(doc.URL, func(m *catalog.Model) {
			m.SetLimit(LimitInferenceStartTimeout, seconds)
		})
	}
}
