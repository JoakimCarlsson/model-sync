package deepgram

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// The services the concurrency reference divides itself into. Deepgram states
// a different ceiling for each, and for two of them a different ceiling again
// depending on how the audio arrives.
const (
	serviceAgent        = "voice agent"
	serviceSTT          = "speech to text"
	serviceSpeechREST   = "text to speech rest"
	serviceSpeechStream = "text to speech streaming"
	serviceIntelligence = "audio intelligence"
)

// concurrencyKeys say what a service counts, since Deepgram calls the same
// ceiling requests for most of them and connections for the agent.
var concurrencyKeys = map[string]string{
	serviceAgent:        "max_concurrent_connections",
	serviceSTT:          "max_concurrent_requests",
	serviceSpeechREST:   "max_concurrent_rest_requests",
	serviceSpeechStream: "max_concurrent_streaming_requests",
	serviceIntelligence: "max_concurrent_requests",
}

// ttsModels are the models the concurrency reference names in its two text to
// speech tables. It calls the first generation Aura, which is the name the
// voice catalog and the pricing page write as Aura-1.
var ttsModels = map[string]string{
	"aura":     "aura-1",
	"aura-2":   "aura-2",
	"flux-tts": "flux-tts",
}

var (
	// concurrencyRe matches one ceiling as the reference writes it, either as
	// a cap on a self-serve plan or as the allocation an enterprise contract
	// starts from.
	concurrencyRe = regexp.MustCompile(
		`(?i)(?:up to|starting at)\s+(\d+)\s+concurrent`,
	)
	// deliveryLabelRe matches the label a cell puts in front of a ceiling
	// where the same model has one ceiling for a live connection and another
	// for a recording.
	deliveryLabelRe = regexp.MustCompile(
		"(?i)`(Streaming|Pre-Recorded)`",
	)
	// hostRe matches the endpoint a column heads, which is how the reference
	// names the region a ceiling applies in.
	hostRe = regexp.MustCompile(`\s*\(`)
)

// applyRateLimits reads the concurrency reference. It is the only document
// stating how much of Deepgram a plan may use at once, and the only one
// saying which models answer on a live connection and which on a recording,
// which is what decides which feature overview describes a model.
func (b *builder) applyRateLimits(doc catalog.Document) {
	plan, service := "", ""
	for _, s := range mdSections(string(doc.Body)) {
		if s.level <= 2 {
			plan = limitSlug(text(s.heading))
			continue
		}
		service = strings.ToLower(text(s.heading))
		if _, ok := concurrencyKeys[service]; !ok {
			continue
		}
		b.applyLimitTables(s.body, plan, service, doc.URL)
	}
}

// applyLimitTables reads every table under one service heading.
func (b *builder) applyLimitTables(
	body, plan, service, source string,
) {
	var regions []string
	for _, match := range tableRowRe.FindAllStringSubmatch(body, -1) {
		cells := strings.Split(match[1], "|")
		if len(cells) < 2 || separator(cells[0]) {
			continue
		}
		if header, ok := regionHeader(cells); ok {
			regions = header
			continue
		}
		b.applyLimitRow(cells, regions, plan, service, source)
	}
}

// regionHeader reports whether a row names the regions its columns limit, and
// what they are.
func regionHeader(cells []string) ([]string, bool) {
	first := strings.ToLower(plain(cells[0]))
	if first != "model" && first != "api" {
		return nil, false
	}
	out := make([]string, 0, len(cells)-1)
	for _, c := range cells[1:] {
		out = append(out, limitSlug(hostRe.Split(plain(c), 2)[0]))
	}
	return out, true
}

// applyLimitRow records one model's ceiling in every region.
func (b *builder) applyLimitRow(
	cells, regions []string,
	plan, service, source string,
) {
	targets := b.limitTargets(cells[0], service)
	if len(targets) == 0 {
		return
	}
	for i, c := range cells[1:] {
		if i >= len(regions) {
			break
		}
		for _, l := range concurrencies(c, service) {
			for _, m := range targets {
				m.SetLimit(
					l.key+"_"+plan+"_"+regions[i],
					l.value,
				)
				m.AddList(catalog.ListFeatures, l.feature)
				m.AddSource(source)
			}
		}
	}
}

// limitTargets returns the models a row of the reference limits. Its speech to
// text rows link to the section of the overview describing a family, which is
// what says that the ceiling covers every model option listed there; its other
// rows name a model the pricing page sells.
func (b *builder) limitTargets(cell, service string) []*catalog.Model {
	var out []*catalog.Model
	if service == serviceAgent {
		b.each(func(m *catalog.Model) {
			if m.Kind == KindAgent {
				out = append(out, m)
			}
		})
		return out
	}
	if family := linkAnchor(cell); family != "" {
		b.each(func(m *catalog.Model) {
			if m.Attrs[AttrFamily] == family {
				out = append(out, m)
			}
		})
		return out
	}
	id := slugID(label(cell))
	if tts, ok := ttsModels[id]; ok {
		id = tts
	}
	if m, ok := b.models[id]; ok {
		out = append(out, m)
	}
	return out
}

// concurrency is one ceiling read from a cell, with the key it belongs under
// and what its being stated says the model can do.
type concurrency struct {
	key     string
	value   int64
	feature string
}

// concurrencies reads what one cell allows. A speech to text cell holds one
// ceiling for a live connection and another for a recording, each behind the
// label naming it, and a model Deepgram serves only one way is given only the
// ceiling it serves.
func concurrencies(cell, service string) []concurrency {
	base := concurrencyKeys[service]
	labels := deliveryLabelRe.FindAllStringSubmatchIndex(cell, -1)
	amounts := concurrencyRe.FindAllStringSubmatchIndex(cell, -1)
	out := make([]concurrency, 0, len(amounts))
	for _, at := range amounts {
		value, err := strconv.ParseInt(cell[at[2]:at[3]], 10, 64)
		if err != nil {
			continue
		}
		mode := labelBefore(cell, labels, at[0])
		out = append(out, concurrency{
			key:     concurrencyKey(base, mode),
			value:   value,
			feature: deliveryFeatures[mode],
		})
	}
	return out
}

// labelBefore returns the delivery label a ceiling sits behind, which is the
// last one written before it.
func labelBefore(cell string, labels [][]int, at int) string {
	out := ""
	for _, l := range labels {
		if l[0] >= at {
			break
		}
		out = strings.ToLower(cell[l[2]:l[3]])
	}
	return out
}

// concurrencyKey qualifies a ceiling by the delivery it applies to, where the
// reference states one per delivery.
func concurrencyKey(base, delivery string) string {
	switch delivery {
	case DeliveryStreaming:
		return "max_concurrent_streaming_requests"
	case DeliveryBatch:
		return "max_concurrent_prerecorded_requests"
	}
	return base
}

// limitSlug turns a heading into the part of a limit key it contributes.
func limitSlug(s string) string {
	return strings.ReplaceAll(slugID(s), "-", "_")
}
