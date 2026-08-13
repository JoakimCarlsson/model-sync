package berget

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Metrics Berget bills on.
const (
	MetricInputTokens  catalog.Metric = "input_tokens"
	MetricOutputTokens catalog.Metric = "output_tokens"
	MetricAudio        catalog.Metric = "audio"
)

// Units Berget quotes amounts against.
const (
	UnitPer1MTokens catalog.Unit = "per_1m_tokens"
	UnitPerSecond   catalog.Unit = "per_second"
)

// Kinds of model Berget serves.
const (
	KindChat          catalog.Kind = "chat"
	KindEmbedding     catalog.Kind = "embedding"
	KindRerank        catalog.Kind = "rerank"
	KindTranscription catalog.Kind = "transcription"
)

// Scalar keys the endpoint populates.
const (
	AttrAuthor       = "author"
	AttrLicense      = "license"
	AttrQuantization = "quantization"
	AttrState        = "lifecycle_state"
	AttrModelPath    = "model_path"
	AttrReleased     = "release_date"
	AttrModelSize    = "model_size_billions"
)

// Enumeration keys the endpoint populates.
const (
	ListFeatures = catalog.ListFeatures
	ListAliases  = "aliases"
)

// capabilityFeatures map a capability flag the endpoint sets onto the
// catalog's vocabulary. A flag not listed keeps Berget's own word.
//
// Berget raises formatted_output and json_mode together on every model that
// raises either, so the two are one capability under two names and both become
// the canonical one. Neither says which of the two strengths is meant, so
// neither yields the narrowing marker.
var capabilityFeatures = map[string]string{
	"formatted_output": catalog.CapabilityStructuredOutputs,
	"json_mode":        catalog.CapabilityStructuredOutputs,
}

// typeKinds maps Berget's model type onto what the model does.
var typeKinds = map[string]catalog.Kind{
	"text":           KindChat,
	"embedding":      KindEmbedding,
	"rerank":         KindRerank,
	"speech-to-text": KindTranscription,
}

// unitMetrics maps the denominator Berget states onto the unit it means and
// what a rate against it counts.
var unitMetrics = map[string]struct {
	unit   catalog.Unit
	metric catalog.Metric
}{
	"€ / m token": {UnitPer1MTokens, ""},
	"€ / second":  {UnitPerSecond, MetricAudio},
}

// listing is the shape of the models endpoint.
type listing struct {
	Data []entry `json:"data"`
}

// entry is one model as Berget publishes it.
type entry struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	OwnedBy        string          `json:"owned_by"`
	Pricing        pricing         `json:"pricing"`
	Capabilities   map[string]bool `json:"capabilities"`
	Quantization   string          `json:"quantization"`
	Aliases        []string        `json:"aliases"`
	License        string          `json:"license"`
	LifecycleState string          `json:"lifecycle_state"`
	ModelPath      string          `json:"model_path"`
	ModelSize      int64           `json:"model_size"`
	ModelType      string          `json:"model_type"`
	ReleaseDate    string          `json:"release_date"`
}

// pricing is one model's rate, which states the currency it is quoted in.
type pricing struct {
	Currency string  `json:"currency"`
	Input    float64 `json:"input"`
	Output   float64 `json:"output"`
	Unit     string  `json:"unit"`
}

// applyListing reads the endpoint's response.
func (b *builder) applyListing(doc catalog.Document) error {
	var list listing
	if err := json.Unmarshal(doc.Body, &list); err != nil {
		return fmt.Errorf("decode %s: %w", doc.URL, err)
	}
	for _, e := range list.Data {
		b.applyEntry(e, doc.URL)
	}
	return nil
}

// applyEntry records one model.
func (b *builder) applyEntry(e entry, source string) {
	if e.ID == "" {
		return
	}
	m := b.model(e.ID, typeKinds[e.ModelType])
	m.Name = e.Name
	m.AddSource(source)
	m.SetAttr(AttrAuthor, e.OwnedBy)
	m.SetAttr(AttrLicense, e.License)
	m.SetAttr(AttrQuantization, e.Quantization)
	m.SetAttr(AttrState, e.LifecycleState)
	m.SetAttr(AttrModelPath, e.ModelPath)
	m.SetAttr(AttrReleased, isoDate(e.ReleaseDate))
	m.SetLimit(AttrModelSize, e.ModelSize)
	m.AddList(ListAliases, e.Aliases...)
	for capability, present := range e.Capabilities {
		if !present || capability == FeatureVision {
			continue
		}
		if name, ok := capabilityFeatures[capability]; ok {
			capability = name
		}
		m.AddList(ListFeatures, capability)
	}
	applyModalities(m, e)
	b.applyPricing(m, e.Pricing)
}

// applyPricing records a model's input and output rates.
//
// A rate quoted per second is charged for audio rather than for tokens, so the
// denominator decides the metric as well as the unit.
func (b *builder) applyPricing(m *catalog.Model, p pricing) {
	billing, ok := unitMetrics[strings.ToLower(strings.TrimSpace(p.Unit))]
	if !ok {
		if p.Unit != "" {
			m.AddNote("unmapped pricing unit " + p.Unit)
		}
		return
	}
	currency := p.Currency
	if currency == "" {
		currency = defaultCurrency
	}
	for _, side := range []struct {
		metric catalog.Metric
		amount float64
	}{
		{MetricInputTokens, p.Input},
		{MetricOutputTokens, p.Output},
	} {
		if side.amount == 0 {
			continue
		}
		metric, dims := side.metric, catalog.Dims(nil)
		if billing.metric != "" {
			metric = billing.metric
			dims = catalog.Dims{DimDirection: directionOf(side.metric)}
		}
		m.AddPrice(catalog.Price{
			Metric:   metric,
			Unit:     billing.unit,
			Amount:   side.amount,
			Currency: currency,
			Dims:     dims,
		})
	}
}

// DimDirection separates the two halves of a rate that would otherwise share
// one metric. It is set only for audio, where both directions are charged per
// second and collapse onto the same metric; a token rate needs no such
// dimension because its metric already names the direction.
const DimDirection = "direction"

// directionOf reports which half of a rate a metric is, so that a per-second
// audio rate charged both ways records as two prices rather than one.
func directionOf(metric catalog.Metric) string {
	if metric == MetricOutputTokens {
		return "output"
	}
	return "input"
}

// isoDate normalizes a release date.
func isoDate(value string) string {
	text := strings.TrimSpace(value)
	if t, err := time.Parse("2006-01-02", text); err == nil {
		return t.Format("2006-01-02")
	}
	return text
}
