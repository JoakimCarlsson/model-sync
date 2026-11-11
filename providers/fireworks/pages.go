package fireworks

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// A model's page in the library states everything Fireworks records about it
// in two labelled blocks, Metadata and Specification, and a third listing what
// the model can be asked to do. Every row of all three is a label and a value
// in the same markup, so one expression reads them all.
var pageFieldRe = regexp.MustCompile(
	`text-copy">([^<]+)</span></div><div[^>]*><(?:span|a)[^>]*>([^<]*)<`,
)

// The rest of what a page states outside those blocks: the title, the path the
// model is served under, the paragraph describing it, and the rate a
// serverless model is billed at. The paragraph is looked for only above the
// Metadata block, since the answers of the FAQ further down are set in the
// same markup.
var (
	pageTitleRe = regexp.MustCompile(`<h1[^>]*>([^<]*)</h1>`)
	pagePathRe  = regexp.MustCompile(
		`accounts/([a-z0-9-]+)/models/([A-Za-z0-9._-]+)`,
	)
	pageSummaryRe = regexp.MustCompile(
		`(?s)</h1>.*?<p class="font-sans text-jumper-rich-body[^"]*">(.*?)</p>`,
	)
	pageMetadataRe = regexp.MustCompile(`>Metadata</h2>`)
	pagePriceRe    = regexp.MustCompile(
		`(?s)Available Serverless</h2><p[^>]*>([^<]*)</p>.*?` +
			`>Per ([^<]*)</p>`,
	)
)

// pageValues returns the labelled rows of a model's page.
func pageValues(body string) map[string]string {
	out := map[string]string{}
	for _, m := range pageFieldRe.FindAllStringSubmatch(body, -1) {
		label := strings.TrimSpace(m[1])
		if _, seen := out[label]; !seen {
			out[label] = strings.TrimSpace(text(m[2]))
		}
	}
	return out
}

// Labels the blocks on a model's page state their values under.
const (
	fieldState      = "State"
	fieldCreated    = "Created on"
	fieldKind       = "Kind"
	fieldProvider   = "Provider"
	fieldHugging    = "Huggingface"
	fieldCalibrated = "Calibrated"
	fieldMoE        = "Mixture-of-Experts"
	fieldParameters = "Parameters"
	fieldFineTuning = "Fine-tuning"
	fieldServerless = "Serverless"
	fieldContext    = "Context Length"
	fieldTools      = "Function Calling"
	fieldEmbeddings = "Embeddings"
	fieldRerankers  = "Rerankers"
	fieldImageInput = "Supporting image input"
)

// supported is the word a page uses for a capability a model has. Its opposite
// is written out in full, so the presence of the word is the whole test.
const supported = "Supported"

// applyLibraryPage reads one model's page onto the catalog.
//
// The page is where a model gets its identifier. Fireworks addresses the page
// under the name of whoever published the model and serves the model under an
// account of its own, and it is the served path that a request carries, so
// that is what the model is keyed by. The page's address is kept alongside it.
func (b *builder) applyLibraryPage(doc catalog.Document) {
	body := string(doc.Body)
	path := pagePathRe.FindStringSubmatch(body)
	if path == nil {
		return
	}
	fields := pageValues(body)
	id, kind := path[1]+"/"+path[2], pageKind(fields)
	m := b.model(id, kind)
	m.Kind = kind
	b.byPage[doc.URL] = id
	m.AddSource(doc.URL)
	m.SetAttr(AttrModelURL, doc.URL)
	m.SetAttr(AttrModelPath, path[0])
	m.SetAttr(catalog.APIID, path[0])
	b.applyPageIdentity(m, doc, body, fields)
	b.applyPageSpecification(m, fields)
	b.applyPageCapabilities(m, fields)
	b.applyPagePrice(m, body)
	b.applyPageFAQ(m, body)
}

// applyPageIdentity records what the page says the model is: what it is
// called, who published it, where its weights are, and when Fireworks took it
// on.
//
// The name is the index's where the index has one, because the index titles
// every model and a page does not always repeat the title.
func (b *builder) applyPageIdentity(
	m *catalog.Model,
	doc catalog.Document,
	body string,
	fields map[string]string,
) {
	if c, ok := b.cards[doc.URL]; ok {
		if m.Name == "" {
			m.Name = c.Name
		}
		m.SetLimit(LimitContextWindow, c.Context)
	}
	if m.Name == "" {
		if title := pageTitleRe.FindStringSubmatch(body); title != nil {
			m.Name = strings.TrimSpace(text(title[1]))
		}
	}
	above := body
	if at := pageMetadataRe.FindStringIndex(body); at != nil {
		above = body[:at[0]]
	}
	if summary := pageSummaryRe.FindStringSubmatch(above); summary != nil {
		m.SetAttr(AttrSummary, text(summary[1]))
	}
	m.SetAttr(AttrAuthor, fields[fieldProvider])
	m.SetAttr(AttrState, lifecycle(fields[fieldState]))
	m.SetAttr(AttrReleaseDate, isoDate(fields[fieldCreated]))
	m.SetAttr(AttrModelKind, fields[fieldKind])
	if repo := huggingFaceID(fields[fieldHugging]); repo != "" {
		m.SetAttr(AttrHuggingFaceID, repo)
		m.SetAttr(AttrOpenWeights, "true")
	}
}

// applyPageSpecification records what the Specification block states, which is
// the shape of the model rather than what it can be asked to do.
func (b *builder) applyPageSpecification(
	m *catalog.Model,
	fields map[string]string,
) {
	m.SetAttr(AttrParameterCount, fields[fieldParameters])
	m.SetAttr(AttrMixtureOfExperts, boolean(fields[fieldMoE]))
	m.SetAttr(AttrCalibrated, boolean(fields[fieldCalibrated]))
	if window := contextTokens(fields[fieldContext]); window > 0 {
		m.SetLimit(LimitContextWindow, window)
	}
}

// applyPageCapabilities records what the model can be asked to do and how it
// can be run.
//
// Every model takes text and answers with text, apart from the two the library
// files under Flumina, which is where it keeps the models that draw. Image
// input is a flag of its own and is the only other input a page states.
func (b *builder) applyPageCapabilities(
	m *catalog.Model,
	fields map[string]string,
) {
	m.AddList(ListInputModalities, "text")
	if m.Kind == KindImage {
		m.AddList(ListOutputModalities, "image")
	} else {
		m.AddList(ListOutputModalities, "text")
	}
	if fields[fieldImageInput] == supported {
		m.AddList(ListInputModalities, "image")
	}
	if fields[fieldTools] == supported {
		m.AddList(ListFeatures, FeatureFunctionCalling)
	}
	if fields[fieldServerless] == supported {
		m.AddList(ListDeployment, DeploymentServerless)
	}
	if fields[fieldFineTuning] == supported {
		m.AddList(ListDeployment, DeploymentFineTuning)
	}
	if strings.Contains(fields[fieldKind], "addon") {
		return
	}
	m.AddList(ListDeployment, DeploymentOnDemand)
}

// pagePrice is the rate one model's page quoted.
type pagePrice struct {
	ID      string
	Amounts []float64
}

// applyPagePrice takes down the rate the page quotes for a serverless model.
//
// A page quotes one rate, the standard serving path's, and writes it as the
// three amounts a text model is billed on or as the single amount an embedding
// model is. It is held rather than recorded, because the serverless pricing
// page is the document Fireworks calls the source of truth for rates: where
// that page prices the same model, it prices it for every serving path and to
// a precision this one rounds away.
func (b *builder) applyPagePrice(m *catalog.Model, body string) {
	match := pagePriceRe.FindStringSubmatch(body)
	if match == nil || !strings.Contains(text(match[2]), "1M Tokens") {
		return
	}
	amounts := parseTriple(text(match[1]))
	if len(amounts) == 0 {
		return
	}
	b.pagePrices = append(b.pagePrices, pagePrice{ID: m.ID, Amounts: amounts})
}

// applyPagePrices records the rates the library pages quoted, for the models
// no rate card priced.
func (b *builder) applyPagePrices() {
	for _, quoted := range b.pagePrices {
		if b.priced[quoted.ID] {
			continue
		}
		m := b.models[quoted.ID]
		b.priced[quoted.ID] = true
		dims := catalog.Dims{DimTier: TierStandard}
		if len(quoted.Amounts) == 1 {
			m.AddPrice(catalog.Price{
				Metric:   MetricInputTokens,
				Unit:     UnitPer1MTokens,
				Amount:   quoted.Amounts[0],
				Currency: currency,
				Dims:     dims,
			})
			continue
		}
		for at, amount := range quoted.Amounts {
			if at >= len(tripleOrder) {
				break
			}
			m.AddPrice(catalog.Price{
				Metric:   tripleOrder[at],
				Unit:     UnitPer1MTokens,
				Amount:   amount,
				Currency: currency,
				Dims:     dims,
			})
		}
	}
}

// The three answers of a model's own FAQ that state a fact no block on the
// page has a row for.
var (
	faqLicenseRe = regexp.MustCompile(
		`(?:under|by|governed by) (?:the |a |an )?` +
			`([A-Z][A-Za-z0-9.& -]{1,40}?) [Ll]icen[cs]e`,
	)
	faqDimensionRe = regexp.MustCompile(
		`(?i)embedding dimensions from [\d,]+ to ([\d,]+)`,
	)
	faqMaxOutputRe = regexp.MustCompile(
		`(?i)maximum (?:generation length|output)(?: length)? ` +
			`(?:of|is) ([\d,]+) tokens`,
	)
)

// faqEntry is one question of a model's FAQ and the answer under it.
type faqEntry struct {
	Question string
	Answer   string
}

// pageFAQ returns the questions of a model's FAQ with their answers.
//
// The page renders only the answer that starts open; the others are held in
// the data the page was built from, each question naming the record its
// answer is in. So the FAQ is read from that data, for every answer alike.
func pageFAQ(body string) []faqEntry {
	records := flightRecords(body)
	var out []faqEntry
	for _, item := range flightObjects(body, isFAQItem) {
		question, _ := item["label"].(string)
		ref, _ := item["content"].(string)
		var answer any
		if json.Unmarshal(
			[]byte(records[strings.TrimPrefix(ref, "$L")]),
			&answer,
		) != nil {
			continue
		}
		out = append(out, faqEntry{
			Question: question,
			Answer:   strings.Join(strings.Fields(flightText(answer)), " "),
		})
	}
	return out
}

// isFAQItem reports whether an object of a page's data is a question of the
// FAQ, which names its answer by reference rather than carrying it.
func isFAQItem(obj map[string]any) bool {
	label, _ := obj["label"].(string)
	ref, _ := obj["content"].(string)
	return label != "" && strings.HasPrefix(ref, "$L")
}

// applyPageFAQ reads the model's own FAQ, which answers three things the
// labelled blocks have no row for.
//
// The FAQ is prose and only some models carry one, so each answer is read
// through an expression narrow enough that a sentence phrased another way
// yields nothing rather than something wrong. The bound on output length is
// the strictest of the three: many answers say only that output is limited by
// the context window, which is not a bound of its own, so a figure is taken
// only from the sentence that states one outright.
func (b *builder) applyPageFAQ(m *catalog.Model, body string) {
	for _, qa := range pageFAQ(body) {
		question, answer := qa.Question, qa.Answer
		switch {
		case strings.Contains(question, "license governs"):
			if match := faqLicenseRe.FindStringSubmatch(answer); match != nil {
				m.SetAttr(AttrLicense, strings.TrimSpace(match[1]))
			}
		case strings.Contains(question, "embedding dimensions"):
			if match := faqDimensionRe.FindStringSubmatch(
				answer,
			); match != nil {
				m.SetAttr(
					AttrDefaultDimension,
					strings.ReplaceAll(match[1], ",", ""),
				)
			}
		case strings.Contains(question, "maximum output length"):
			if match := faqMaxOutputRe.FindStringSubmatch(
				answer,
			); match != nil {
				m.SetLimit(LimitMaxOutput, digits(match[1]))
			}
		}
	}
}

// pageKind decides what the model is from the capabilities its page flags. A
// reranker scores pairs and an embedder returns a vector, both of which the
// page flags outright; the models filed under Flumina are the ones that draw;
// everything else answers a completion.
func pageKind(fields map[string]string) catalog.Kind {
	switch {
	case fields[fieldRerankers] == supported:
		return KindRerank
	case fields[fieldEmbeddings] == supported:
		return KindEmbedding
	case strings.Contains(fields[fieldKind], "Flumina"):
		return KindImage
	}
	return KindChat
}

// lifecycle translates the word Fireworks files a model's readiness under.
// Every model it publishes is Ready, and a word it has not used before is
// recorded as it stands rather than forced into a value it may not mean.
func lifecycle(state string) string {
	if strings.EqualFold(state, "Ready") {
		return "active"
	}
	return strings.ToLower(state)
}

// boolean translates the Yes and No a specification row is written with.
func boolean(value string) string {
	switch {
	case strings.EqualFold(value, "yes"):
		return "true"
	case strings.EqualFold(value, "no"):
		return "false"
	}
	return ""
}

// dateRe matches the date a page states a model was taken on, which it writes
// as day, abbreviated month and year, such as "28 AUG, 2026".
var dateRe = regexp.MustCompile(`^(\d{1,2}) ([A-Za-z]{3}),? (\d{4})$`)

// isoDate rewrites that date the way every other provider records one.
func isoDate(value string) string {
	match := dateRe.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil {
		return ""
	}
	day, err := time.Parse(
		"2 Jan 2006",
		match[1]+" "+strings.ToUpper(match[2][:1])+
			strings.ToLower(match[2][1:])+" "+match[3],
	)
	if err != nil {
		return ""
	}
	return day.Format(time.DateOnly)
}

// contextLengthRe matches the window a page states, which it rounds for
// display and writes as "262k tokens".
var contextLengthRe = regexp.MustCompile(`^([\d.]+)\s*([kKmM]) tokens$`)

// contextTokens reads that rounded window. A model whose page states none
// writes "N/A", which yields nothing.
func contextTokens(value string) int64 {
	match := contextLengthRe.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil {
		return 0
	}
	return tokenCount(match[1], match[2])
}

// digits reads a count written with thousands separators.
func digits(value string) int64 {
	n, err := strconv.ParseInt(strings.ReplaceAll(value, ",", ""), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// huggingFaceID reduces what the Hugging Face row states to the identifier the
// weights are published under. The row is a link on the models whose weights
// are public and the owner alone on the models whose are not, and only the
// former names a repository.
func huggingFaceID(value string) string {
	id := strings.TrimSpace(value)
	if _, rest, ok := strings.Cut(id, "huggingface.co/"); ok {
		id = rest
	}
	owner, repo, ok := strings.Cut(strings.Trim(id, "/"), "/")
	if !ok || owner == "" || repo == "" {
		return ""
	}
	return owner + "/" + repo
}
