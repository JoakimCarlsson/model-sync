package bedrock

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Metrics Bedrock bills on.
const (
	MetricInputTokens       catalog.Metric = "input_tokens"
	MetricOutputTokens      catalog.Metric = "output_tokens"
	MetricCachedInputTokens catalog.Metric = "cached_input_tokens"
	MetricCacheWriteTokens  catalog.Metric = "cache_write_tokens"
	MetricImageInput        catalog.Metric = "image_input"
	MetricVideoInput        catalog.Metric = "video_input"
	MetricAudioInput        catalog.Metric = "audio_input"
	MetricImageOutput       catalog.Metric = "image_output"
	MetricUsage             catalog.Metric = "usage"
)

// Units Bedrock quotes amounts against.
const (
	UnitPer1KTokens catalog.Unit = "per_1k_tokens"
	UnitPer1MTokens catalog.Unit = "per_1m_tokens"
	UnitPerImage    catalog.Unit = "per_image"
	UnitPerHour     catalog.Unit = "per_hour"
	UnitPerMinute   catalog.Unit = "per_minute"
	UnitPerRequest  catalog.Unit = "per_request"
	UnitPerMonth    catalog.Unit = "per_month"
	UnitPerTextUnit catalog.Unit = "per_text_unit"
)

// Kinds Bedrock serves. Most are chat, but the same price list carries
// embeddings, image and video generation and speech models, all of them
// billed by the token, so the rate cannot say which and the name has to.
const (
	KindChat          catalog.Kind = "chat"
	KindImage         catalog.Kind = "image"
	KindVideo         catalog.Kind = "video"
	KindEmbedding     catalog.Kind = "embedding"
	KindTranscription catalog.Kind = "transcription"
	KindRerank        catalog.Kind = "rerank"
)

// nameKinds map a fragment of the model name AWS gives onto what it does.
var nameKinds = []struct {
	fragment string
	kind     catalog.Kind
}{
	{"embed", KindEmbedding},
	{"rerank", KindRerank},
	{"voxtral", KindTranscription},
	{"transcribe", KindTranscription},
	{"canvas", KindImage},
	{"image", KindImage},
	{"diffusion", KindImage},
	{"reel", KindVideo},
	{"video", KindVideo},
}

// Dimension keys Bedrock's prices vary along.
const (
	DimRegion   = "region"
	DimTier     = "tier"
	DimContext  = "context"
	DimCacheTTL = "cache_ttl"
)

// ContextLong is the band of a rate AWS meters apart for a long prompt.
const ContextLong = "long"

// Serving paths Bedrock prices separately. The first three are named inside
// the metric field; the rest come from the feature the product belongs to.
const (
	TierOnDemand    = "on-demand"
	TierPriority    = "priority"
	TierFlex        = "flex"
	TierBatch       = "batch"
	TierProvisioned = "provisioned"
	TierCustom      = "custom"
)

// Scalar keys the price list populates.
const (
	AttrAuthor = "author"
)

// tierWords are the serving paths AWS names inside the metric field.
var tierWords = map[string]string{
	"priority": TierPriority,
	"flex":     TierFlex,
	"batch":    TierBatch,
}

// featureTiers maps the product feature onto a serving path, for the rates
// whose metric field names none.
var featureTiers = []struct {
	fragment string
	tier     string
}{
	{"batch", TierBatch},
	{"provisioned", TierProvisioned},
	{"customization", TierCustom},
	{"custom model", TierCustom},
	{"on-demand", TierOnDemand},
}

// serviceTiers maps the serving path the newest meters state in a field of
// their own. They are the only rates naming neither an inference type nor a
// feature, so this is read last and only for them: a meter of the older shape
// carries a service tier as well as an inference type naming the same path,
// and reading it here too would price one path twice.
var serviceTiers = map[string]string{
	"standard": TierOnDemand,
	"priority": TierPriority,
	"flex":     TierFlex,
	"batch":    TierBatch,
}

// metricWords maps a fragment of AWS's metric field onto what is counted. The
// order matters: a video token count is an input, so the more specific
// fragments are checked first.
var metricWords = []struct {
	fragment string
	metric   catalog.Metric
}{
	{"cache read", MetricCachedInputTokens},
	{"cache write", MetricCacheWriteTokens},
	{"input image", MetricImageInput},
	{"input video", MetricVideoInput},
	{"input audio", MetricAudioInput},
	{"image output", MetricImageOutput},
	{"output token", MetricOutputTokens},
	{"input token", MetricInputTokens},
	{"text output", MetricOutputTokens},
	{"text input", MetricInputTokens},
}

// unitNames maps AWS's denominator onto a unit.
var unitNames = map[string]catalog.Unit{
	"1k tokens":                 UnitPer1KTokens,
	"1m tokens":                 UnitPer1MTokens,
	"image":                     UnitPerImage,
	"images processed":          UnitPerImage,
	"hour":                      UnitPerHour,
	"minutes processed":         UnitPerMinute,
	"custom model unit per min": UnitPerMinute,
	"requests":                  UnitPerRequest,
	"model/month":               UnitPerMonth,
	"textunit":                  UnitPerTextUnit,
}

// priceList is the shape of the published price list.
type priceList struct {
	Products map[string]product        `json:"products"`
	Terms    map[string]map[string]any `json:"-"`
	OnDemand map[string]map[string]term
}

// UnmarshalJSON reads the list, lifting the on-demand terms out of the nesting
// AWS wraps every offer type in.
func (p *priceList) UnmarshalJSON(data []byte) error {
	var raw struct {
		Products map[string]product `json:"products"`
		Terms    struct {
			OnDemand map[string]map[string]term `json:"OnDemand"`
		} `json:"terms"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	p.Products = raw.Products
	p.OnDemand = raw.Terms.OnDemand
	return nil
}

// product is one billable combination of model, region and serving path.
type product struct {
	SKU        string     `json:"sku"`
	Attributes attributes `json:"attributes"`
}

type attributes struct {
	Model         string `json:"model"`
	Provider      string `json:"provider"`
	InferenceType string `json:"inferenceType"`
	RegionCode    string `json:"regionCode"`
	Feature       string `json:"feature"`
	UsageType     string `json:"usagetype"`
	// TokenType and ServiceTier are what the meters reaching the model through
	// the bedrock-mantle endpoint state instead of an inference type and a
	// feature. They carry the same two facts under different names, and a
	// product stating them states neither of the others.
	TokenType   string `json:"tokenType"`
	ServiceTier string `json:"service_tier"`
}

// term is one offer against a product.
type term struct {
	PriceDimensions map[string]priceDimension `json:"priceDimensions"`
}

// priceDimension is one rate within an offer.
type priceDimension struct {
	Unit         string            `json:"unit"`
	PricePerUnit map[string]string `json:"pricePerUnit"`
	BeginRange   string            `json:"beginRange"`
	Description  string            `json:"description"`
}

// applyPriceList reads the published price list.
func (b *builder) applyPriceList(doc catalog.Document) error {
	var list priceList
	if err := json.Unmarshal(doc.Body, &list); err != nil {
		return fmt.Errorf("decode %s: %w", doc.URL, err)
	}
	for sku, p := range list.Products {
		for _, offer := range list.OnDemand[sku] {
			for _, dimension := range offer.PriceDimensions {
				b.applyRate(p.Attributes, dimension, doc.URL)
			}
		}
	}
	return nil
}

// applyRate records one rate against the model it belongs to.
func (b *builder) applyRate(
	a attributes,
	d priceDimension,
	source string,
) {
	id := modelID(a)
	if id == "" {
		return
	}
	metric, ok := metricFor(a)
	if !ok {
		return
	}
	unit, ok := unitNames[strings.ToLower(strings.TrimSpace(d.Unit))]
	if !ok {
		return
	}
	amount, ok := amountOf(d)
	if !ok {
		return
	}
	m := b.model(id, kindFor(metric, a.Model))
	m.AddSource(source)
	m.SetAttr(AttrAuthor, a.Provider)
	m.AddList(ListAliases, meterModelID(a.UsageType))
	if preferName(m.Name, a.Model) {
		m.Name = a.Model
	}
	m.AddPrice(catalog.Price{
		Metric:   metric,
		Unit:     unit,
		Amount:   amount,
		Currency: currency,
		Dims: catalog.Dims{}.
			With(DimRegion, a.RegionCode).
			With(DimTier, tierFor(a)).
			With(DimContext, contextBand(a.TokenType)).
			With(DimCacheTTL, cacheTTL(a.TokenType)),
	})
}

// preferName reports whether a candidate should replace the name a model
// already carries.
//
// The price list can name one model two ways, writing the display name against
// some of its meters and the bare identifier against others, and the meters
// arrive in no fixed order. The prose name is the one kept, recognized by its
// having words in it, and two of a kind are settled by order so that a run is
// reproducible.
func preferName(current, candidate string) bool {
	if candidate == "" {
		return false
	}
	if current == "" {
		return true
	}
	if prose(current) != prose(candidate) {
		return prose(candidate)
	}
	return candidate < current
}

// prose reports whether a name reads as one rather than as an identifier.
func prose(name string) bool { return strings.Contains(name, " ") }

// amountOf reads the rate, which AWS states as a decimal string per currency.
// A rate of zero is not recorded: the list carries zero-priced rows for
// combinations that are not charged for rather than free.
func amountOf(d priceDimension) (float64, bool) {
	raw, ok := d.PricePerUnit[currency]
	if !ok {
		return 0, false
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || value == 0 {
		return 0, false
	}
	return value, true
}

// modelID names a model by the lab that made it and the name AWS gives it,
// since the price list states no API identifier.
func modelID(a attributes) string {
	model := slug(a.Model)
	if model == "" {
		return ""
	}
	if author := slug(a.Provider); author != "" {
		return author + "/" + model
	}
	return model
}

// metricFor reads what a rate counts out of AWS's combined metric field, or
// out of the token type where the meter states it there instead.
func metricFor(a attributes) (catalog.Metric, bool) {
	if metric, ok := countedBy(a.InferenceType); ok {
		return metric, true
	}
	if metric, ok := countedBy(a.TokenType); ok {
		return metric, true
	}
	if a.InferenceType == "" {
		return "", false
	}
	return MetricUsage, true
}

// countedBy reads a metric out of one field, which AWS writes as prose in one
// shape of meter and as an identifier in the other.
func countedBy(field string) (catalog.Metric, bool) {
	lower := strings.ToLower(fieldWords(field))
	for _, entry := range metricWords {
		if strings.Contains(lower, entry.fragment) {
			return entry.metric, true
		}
	}
	return "", false
}

// fieldWords separates the words of a field written as an identifier, so that
// input_tokens_mantle reads the same as the "Input tokens" the older meters
// state.
func fieldWords(field string) string {
	return strings.Map(func(r rune) rune {
		if r == '_' || r == '-' {
			return ' '
		}
		return r
	}, field)
}

// tierFor reads the serving path, which AWS names inside the metric field for
// some rates, in the product's feature for others and in a field of its own
// for the newest.
func tierFor(a attributes) string {
	field := strings.ToLower(a.InferenceType)
	for word, tier := range tierWords {
		if strings.Contains(field, word) {
			return tier
		}
	}
	feature := strings.ToLower(a.Feature)
	for _, entry := range featureTiers {
		if strings.Contains(feature, entry.fragment) {
			return entry.tier
		}
	}
	if field == "" && feature == "" {
		return serviceTiers[strings.ToLower(a.ServiceTier)]
	}
	return ""
}

// meterIDRe matches the identifier a usage type names between the region it
// bills in and the endpoint it reaches the model through: the meter
// USE1-nvidia.nemotron-nano-9b-v2-mantle-input-tokens-standard is the only
// place the price list states what a model is called.
var meterIDRe = regexp.MustCompile(
	`^[A-Z0-9]+-((?:[a-z0-9-]+\.)+[a-z0-9.-]+?)-mantle-`,
)

// meterModelID reads the model identifier out of a usage type, which is what
// joins a rate to the card describing what it buys without going through the
// two documents' disagreeing prose names.
func meterModelID(usageType string) string {
	match := meterIDRe.FindStringSubmatch(usageType)
	if match == nil {
		return ""
	}
	return match[1]
}

// tokenCacheTTLRe matches the cache lifetime a token type names.
var tokenCacheTTLRe = regexp.MustCompile(`(?i)\b(\d+[smhd])\b`)

// cacheTTL reads how long a cache write a rate covers is kept for, which the
// token type states and nothing else does.
func cacheTTL(tokenType string) string {
	match := tokenCacheTTLRe.FindStringSubmatch(tokenType)
	if match == nil {
		return ""
	}
	return strings.ToLower(match[1])
}

// contextBand reports whether a rate is the one charged above the prompt
// length AWS meters separately.
func contextBand(tokenType string) string {
	if strings.Contains(strings.ToLower(tokenType), "long-ctx") {
		return ContextLong
	}
	return ""
}

// kindFor reports what a model is, from what it is billed on where that
// settles it and from its name where it does not.
func kindFor(metric catalog.Metric, model string) catalog.Kind {
	if metric == MetricImageOutput {
		return KindImage
	}
	lower := strings.ToLower(model)
	for _, entry := range nameKinds {
		if strings.Contains(lower, entry.fragment) {
			return entry.kind
		}
	}
	return KindChat
}

// slug turns a display name into an identifier.
func slug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
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
