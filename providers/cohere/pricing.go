package cohere

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Patterns over the pricing page. Cohere states its rates two ways: current
// products as data behind rate cards, and withdrawn models as sentences in the
// page's questions and answers.
var (
	// cardMarker opens one rate card in the payload. The cards are separated
	// rather than matched whole, because a card runs to several thousand
	// characters of prose and a bounded repeat cannot reach across it.
	cardMarker = `"_type":"model","highlightModel":`
	// cardNameRe matches the product a card is headed by and the denominator
	// its amounts are quoted against.
	cardNameRe = regexp.MustCompile(`"modelName":"([^"]+)","per":"([^"]*)"`)
	// cardRatesRe matches the amounts a card states.
	cardRatesRe = regexp.MustCompile(`"pricings":(\[[^\]]*\])`)
	// legacyRe matches a sentence stating the rate of a withdrawn model.
	legacyRe = regexp.MustCompile(
		`([A-Za-z][A-Za-z0-9+\- ]*?) pricing is \$([\d.]+)/1M tokens for ` +
			`input and \$([\d.]+)/1M tokens for output`,
	)
	// ayaRe matches the sentence stating one rate for two research models,
	// which is the only rate the page states for more than one model at once.
	ayaRe = regexp.MustCompile(
		`(Aya Expanse) models \([^)]*\) on the API are charged at ` +
			`\$([\d.]+)/1M tokens for input and \$([\d.]+)/1M tokens for output`,
	)
	// instanceRe matches the rate a card quotes for a dedicated instance,
	// which Cohere writes as a sentence inside the card rather than as one of
	// the card's amounts.
	instanceRe = regexp.MustCompile(
		`\$+([\d,]*\.?\d+)\s*/\s*(hour|month)\s*/\s*instance`,
	)
	pushRe = regexp.MustCompile(
		`(?s)self\.__next_f\.push\(\[1,("(?:[^"\\]|\\.)*")\]\)`,
	)
	// textRe matches the text of one span, which is the leaf everything the
	// page renders as prose is written in.
	textRe = regexp.MustCompile(`"text":"((?:[^"\\]|\\.)*)"`)
)

// cardRate is one amount on a rate card, with the wording Cohere labels it by.
// A card that quotes one of its two amounts against a different denominator
// overrides it in place.
// A side of a card that is labelled but carries no amount states no rate, so
// the amounts are pointers: a card quoting one figure against a thousand
// searches leaves the other side empty, which is not a rate of zero.
type cardRate struct {
	InputLabel  string   `json:"inputLabel"`
	InputPrice  *float64 `json:"inputPrice"`
	OutputLabel string   `json:"outputLabel"`
	OutputPrice *float64 `json:"outputPrice"`
	OverridePer string   `json:"overridePer"`
}

// applyPricing reads the pricing page.
//
// A rate is recorded only against a model the overview already established,
// because the page names products and platforms as well as models and only the
// overview says which of those names the API answers to.
func (b *builder) applyPricing(doc catalog.Document) {
	body := flight(doc.Body)
	for card := range strings.SplitSeq(body, cardMarker) {
		name := cardNameRe.FindStringSubmatch(card)
		if name == nil {
			continue
		}
		b.nameFromCard(name[1])
		b.addInstanceRate(doc, name[1], card)
		rates := cardRatesRe.FindStringSubmatch(card)
		if rates == nil {
			continue
		}
		var parsed []cardRate
		if err := json.Unmarshal([]byte(rates[1]), &parsed); err != nil {
			continue
		}
		for _, r := range parsed {
			b.addCard(doc, name[1], name[2], r)
		}
	}
	b.addVault(doc, body)
	for _, match := range legacyRe.FindAllStringSubmatch(body, -1) {
		b.addTokenRates(doc, match[1], match[2], match[3])
	}
	for _, match := range ayaRe.FindAllStringSubmatch(body, -1) {
		b.addTokenRates(doc, match[1], match[2], match[3])
	}
}

// nameFromCard takes the product a rate card is headed by as a display name.
//
// The overview's tables state no name, only the identifier, and its prose names
// the Command family and nothing else. The cards name what Cohere sells: the
// model the API calls embed-v4.0 is "Embed 4" there and nowhere else it
// publishes, and the same holds for both fourth generation rerankers.
//
// A card naming more than one model names neither of them. "Aya Expanse" heads
// the rate of two models of different sizes, which is a family and not a display
// name for either, so the lookup must resolve to exactly one model.
func (b *builder) nameFromCard(product string) {
	ids := b.identify(product)
	if len(ids) != 1 {
		return
	}
	if m := b.models[ids[0]]; m != nil && m.Name == "" {
		m.Name = strings.TrimSpace(product)
	}
}

// noteNoRate records that Cohere states no rate for a model it serves. Nothing
// reading the aggregate can see a package comment, so without this a served
// model with no amount is indistinguishable from a free one.
const noteNoRate = "no rate stated on the pricing page"

// noteUnpriced marks the served models the pricing page states no amount for.
//
// This is most of what Cohere serves: the whole Command A family, Aya Vision,
// the four Tiny Aya models, the third generation embedding and rerank models and
// the nightly builds. The page prices the products it sells today as cards and
// the models it has withdrawn as sentences, and none of those models appears in
// either. The card headed Command A+ quotes nothing but zero for an API key and
// a model download, which is the open weight licence rather than a rate and is
// not read as one, so that model is marked here too.
//
// A withdrawn model is skipped: its missing rate is correct, and the page's
// questions and answers outlive the models they answer for.
func (b *builder) noteUnpriced() {
	for _, id := range b.order {
		m := b.models[id]
		if len(m.Prices) > 0 || withdrawn(m) {
			continue
		}
		m.AddNote(noteNoRate)
	}
}

// withdrawn reports whether Cohere has taken a model out of service, which its
// status column states as deprecated or retired.
func withdrawn(m *catalog.Model) bool {
	switch m.Attrs[AttrState] {
	case StateDeprecated, StateRetired:
		return true
	}
	return false
}

// addCard records both amounts of one rate card.
func (b *builder) addCard(
	doc catalog.Document,
	product, per string,
	r cardRate,
) {
	for _, side := range []struct {
		label  string
		amount *float64
	}{{r.InputLabel, r.InputPrice}, {r.OutputLabel, r.OutputPrice}} {
		metric, ok := cardLabels[strings.ToLower(strings.TrimSpace(side.label))]
		if !ok || side.amount == nil {
			continue
		}
		denominator := per
		if r.OverridePer != "" {
			denominator = r.OverridePer
		}
		quoted, ok := cardUnits[strings.ToLower(strings.TrimSpace(denominator))]
		if !ok {
			continue
		}
		if quoted.Metric != "" {
			metric = quoted.Metric
		}
		for _, id := range b.identify(product) {
			b.price(doc, id, catalog.Price{
				Metric: metric,
				Unit:   quoted.Unit,
				Amount: *side.amount,
			})
		}
	}
}

// addTokenRates records the pair of per-token amounts a sentence states.
func (b *builder) addTokenRates(doc catalog.Document, product, in, out string) {
	for _, id := range b.identify(product) {
		b.price(doc, id, catalog.Price{
			Metric: MetricInputTokens,
			Unit:   UnitPer1MTokens,
			Amount: amount(in),
		})
		b.price(doc, id, catalog.Price{
			Metric: MetricOutputTokens,
			Unit:   UnitPer1MTokens,
			Amount: amount(out),
		})
	}
}

// addInstanceRate records the rate a card quotes for running the model on a
// dedicated instance, which Cohere states only as a sentence and only as a
// floor: the card for its transcription model says what an instance starts at
// and the vault table, which states the rest, does not list the model.
func (b *builder) addInstanceRate(
	doc catalog.Document,
	product, card string,
) {
	match := instanceRe.FindStringSubmatch(card)
	if match == nil {
		return
	}
	unit, ok := instanceUnits[match[2]]
	if !ok {
		return
	}
	for _, id := range b.identify(product) {
		b.price(doc, id, catalog.Price{
			Metric: MetricHosting,
			Unit:   unit,
			Amount: amount(strings.ReplaceAll(match[1], ",", "")),
			Dims:   catalog.Dims{DimDeployment: DeploymentVault},
			Note:   noteStartingRate,
		})
	}
}

// addVault reads the table of dedicated deployment rates, which quotes an
// hourly and a monthly amount per instance for each performance tier a model is
// offered in.
func (b *builder) addVault(doc catalog.Document, body string) {
	for _, row := range scanVault(body) {
		b.nameFromCard(row.Model)
		for _, quoted := range []struct {
			unit  catalog.Unit
			value string
		}{
			{UnitPerHour, row.Hourly},
			{UnitPerMonth, row.Monthly},
		} {
			value := amount(strings.ReplaceAll(quoted.value, ",", ""))
			if value == 0 {
				continue
			}
			for _, id := range b.identify(row.Model) {
				b.price(doc, id, catalog.Price{
					Metric: MetricHosting,
					Unit:   quoted.unit,
					Amount: value,
					Dims: catalog.Dims{
						DimDeployment: DeploymentVault,
					}.With(DimTier, strings.ToLower(row.Tier)),
				})
			}
		}
	}
}

// price records one rate against a model the overview established. A retired
// model is left unpriced whatever the page still says about it: the page's
// questions and answers outlive the models they answer for.
func (b *builder) price(doc catalog.Document, id string, p catalog.Price) {
	m, ok := b.models[id]
	if !ok || strings.HasPrefix(m.Attrs[AttrState], "retired") {
		return
	}
	p.Currency = currency
	m.AddSource(doc.URL)
	m.AddPrice(p)
}

// Markers of the dedicated deployment table in the page's payload. It is the
// only table the pricing page carries, and it is written as a header of cells
// followed by rows of cells, each cell a block of spans.
const (
	headerCellMarker = `"_type":"headerTableCell"`
	rowMarker        = `"_type":"row"`
	cellMarker       = `"_type":"tableCell"`
	rowsMarker       = `"rows":[`
)

// Headings the dedicated deployment table is written under.
const (
	vaultModelColumn   = "model"
	vaultTierColumn    = "performance tier"
	vaultHourlyColumn  = "hourly rate per instance"
	vaultMonthlyColumn = "monthly rate per instance"
)

// vaultRow is one line of the dedicated deployment table.
type vaultRow struct {
	Model   string
	Tier    string
	Hourly  string
	Monthly string
}

// scanVault reads the dedicated deployment table. Its columns are matched by
// heading rather than by position, because the amounts are the two rightmost
// cells and nothing else in a row says which denominator they are quoted
// against.
func scanVault(body string) []vaultRow {
	start := strings.Index(body, headerCellMarker)
	if start < 0 {
		return nil
	}
	header, rest, ok := strings.Cut(body[start:], rowsMarker)
	if !ok {
		return nil
	}
	columns := map[string]int{}
	for i, heading := range cellTexts(header, headerCellMarker) {
		columns[strings.ToLower(heading)] = i
	}
	var out []vaultRow
	for segment := range strings.SplitSeq(rest, rowMarker) {
		cells := cellTexts(segment, cellMarker)
		row := vaultRow{
			Model:   column(cells, columns, vaultModelColumn),
			Tier:    column(cells, columns, vaultTierColumn),
			Hourly:  column(cells, columns, vaultHourlyColumn),
			Monthly: column(cells, columns, vaultMonthlyColumn),
		}
		if row.Model == "" {
			continue
		}
		out = append(out, row)
	}
	return out
}

// cellTexts returns the first span of each cell of one row, which is the cell's
// value. A cell holds one block of one span wherever the table states a name,
// a tier or an amount.
func cellTexts(segment, marker string) []string {
	var out []string
	for i, cell := range strings.Split(segment, marker) {
		if i == 0 {
			continue
		}
		match := textRe.FindStringSubmatch(cell)
		if match == nil {
			out = append(out, "")
			continue
		}
		out = append(out, unquote(match[1]))
	}
	return out
}

// column returns the cell under a heading.
func column(cells []string, columns map[string]int, heading string) string {
	i, ok := columns[heading]
	if !ok || i >= len(cells) {
		return ""
	}
	return strings.TrimSpace(strings.TrimLeft(cells[i], "$"))
}

// unquote resolves the escapes a JSON string carries.
func unquote(value string) string {
	var out string
	if err := json.Unmarshal([]byte(`"`+value+`"`), &out); err != nil {
		return value
	}
	return out
}

// identify reports which models a name on the pricing page refers to.
//
// A product name is looked up first, because a product and a withdrawn model
// can share a name: the card headed "Command R" states the rate of the model
// serving under that name today, which is command-r-08-2024, not the alias
// command-r that points at the 2024 version the page prices separately.
//
// A name the page states precisely reduces to an identifier instead, so that
// "Command R+ 08-2024" reaches command-r-plus-08-2024 without a table.
func (b *builder) identify(name string) []string {
	if ids, ok := productModels[strings.ToLower(strings.TrimSpace(name))]; ok {
		var out []string
		for _, id := range ids {
			if _, ok := b.models[id]; ok {
				out = append(out, id)
			}
		}
		return out
	}
	if _, ok := b.models[slugID(name)]; ok {
		return []string{slugID(name)}
	}
	return nil
}

// slugID reduces a model named in prose to the identifier it is called by.
func slugID(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, "+", "-plus")
	s = strings.Join(strings.Fields(s), "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return s
}

// amount reads a decimal rate.
func amount(text string) float64 {
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0
	}
	return value
}

// flight returns the payload a rendered page carries, which the page embeds a
// piece at a time, each piece a JSON string.
func flight(body []byte) string {
	text := string(body)
	if !strings.Contains(text, "self.__next_f.push") {
		return text
	}
	var out strings.Builder
	for _, match := range pushRe.FindAllStringSubmatch(text, -1) {
		var piece string
		if err := json.Unmarshal([]byte(match[1]), &piece); err != nil {
			continue
		}
		out.WriteString(piece)
	}
	return out.String()
}
