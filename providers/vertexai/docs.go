package vertexai

import (
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

const docsBase = "https://cloud.google.com"

// Documents Google publishes about the models Vertex serves, as distinct from
// the billing catalog, which describes none of them.
const (
	// ModelsURL indexes the page each model has of its own.
	ModelsURL = docsBase + "/vertex-ai/generative-ai/docs/models"
	// modelPagePre prefixes those pages. Google files them under the platform
	// the models are sold through rather than under the documentation set the
	// index belongs to.
	modelPagePre = docsBase + "/gemini-enterprise-agent-platform/models/"
	// gemmaSizesURL is the page tabulating the Gemma variants. Gemma is sold
	// for self deployment rather than as a service, so most of its variants
	// have no page of their own, and this table is where Google states what
	// each of them takes and returns.
	gemmaSizesURL = modelPagePre + "open-models/use-gemma"
	// versionsURL is the page stating when each model retires, and which ones
	// already have. It is the only document that says a model Vertex still
	// bills for is no longer served.
	versionsURL = modelPagePre + "model-versions"
	// deprecationsURL is the same table for the models Vertex serves for other
	// labs as a managed API, which the versions page does not cover. It is the
	// only document stating when notice of a withdrawal was given, as against
	// when the withdrawal takes effect.
	deprecationsURL = modelPagePre + "deprecations/open-models"
	// openModelsURL and partnerModelsURL index those models, stating for each
	// the lab that made it, what it is for and the identifier Model Garden
	// serves it under. No model page states any of the three.
	openModelsURL    = modelPagePre + "maas/use-open-models"
	partnerModelsURL = modelPagePre + "partner-models/use-partner-models"
)

// capabilityPages are the pages that enumerate, one capability at a time, the
// models supporting it.
//
// A model page states most of what its model can do and is read first, but it
// never mentions function calling, which Google documents only here. Reading
// these is what tells a Gemini model that can call a tool from one that
// cannot, and it is why a page listing a model is recorded as a source for it.
var capabilityPages = []struct {
	URL     string
	Feature string
}{
	{
		modelPagePre + "capabilities/control-generated-output",
		catalog.CapabilityStructuredOutputs,
	},
	{modelPagePre + "thinking", catalog.CapabilityReasoning},
	{
		modelPagePre + "tools/function-calling",
		catalog.CapabilityFunctionCalling,
	},
}

// sidePages are the documents to read besides the one each model has. They
// are addressed outright because the index links a model's page and not the
// pages that describe a capability or a lifecycle.
func sidePages() []string {
	urls := []string{
		versionsURL,
		deprecationsURL,
		openModelsURL,
		partnerModelsURL,
	}
	for _, page := range capabilityPages {
		urls = append(urls, page.URL)
	}
	return urls
}

// featureOfPage reports which capability a page enumerates the models for.
func featureOfPage(url string) (string, bool) {
	for _, page := range capabilityPages {
		if page.URL == url {
			return page.Feature, true
		}
	}
	return "", false
}

// modelSections are the parts of the documentation that describe a model
// rather than a task. Everything else under the same path is a guide.
var modelSections = []string{
	"gemini/",
	"open-models/",
	"maas/",
	"partner-models/",
}

// Numeric keys the model pages populate.
const (
	LimitContextWindow   = "context_window"
	LimitMaxOutputTokens = "max_output_tokens"
)

// Scalar keys the lifecycle page populates.
const (
	// AttrRetirementDate is when Google stops serving a model. It states one
	// for every model it documents, not only for the ones already gone, so a
	// model still served carries the date it is due to go.
	AttrRetirementDate = "retirement_date"
	// StateRetired is what the lifecycle page's last section says of the models
	// it lists. Nothing is recorded under it, because a model in that state is
	// dropped rather than held; it is the value the drop is decided on.
	StateRetired = "retired"
)

// Enumeration keys the model pages populate.
const (
	ListFeatures         = catalog.ListFeatures
	ListInputModalities  = "input_modalities"
	ListOutputModalities = "output_modalities"
	// ListDimensions holds the widths an embedding model can return, which is
	// the key every other provider here states a vector width under.
	ListDimensions = "embedding_dimensions"
)

// featureNames map a capability a page lists onto the catalog's vocabulary.
// Only the names that differ are listed; the rest keep Google's own words with
// their spacing reduced to an identifier.
var featureNames = map[string]string{
	"thinking":          catalog.CapabilityReasoning,
	"structured output": catalog.CapabilityStructuredOutputs,
	"context caching":   "prompt_caching",
	"grounding":         "web_search",
	"tools":             catalog.CapabilityFunctionCalling,
	"function calling":  catalog.CapabilityFunctionCalling,
}

// modalityNames map a modality a page names onto the catalog's vocabulary.
//
// An embedding page marks "Embeddings" as an output, which is the model's
// return value rather than a modality. The catalog has no word for a vector, so
// it is read as the text the model works in; skipping it left those models
// stating what they take and nothing about what they give back, which a
// consumer cannot tell from a model that returns nothing.
//
// The models Vertex serves for other labs name theirs in a list rather than
// against a grid, and name more of them: code is text, and a page saying it
// reads a PDF or a document says the model takes a file.
var modalityNames = map[string]string{
	"text":       "text",
	"image":      "image",
	"images":     "image",
	"audio":      "audio",
	"video":      "video",
	"code":       "text",
	"pdf":        "file",
	"documents":  "file",
	"embeddings": "text",
}

var (
	// specTableRe matches the specification table, which carries a class of
	// its own and is the only table stating what a model is.
	specTableRe = regexp.MustCompile(
		`(?is)<table class="geap-model-table">(.*?)</table>`,
	)
	specIDRe = regexp.MustCompile(
		`(?is)id="model-id".*?<code[^>]*>(.*?)</code>`,
	)
	// specModalityRe matches one modality and the direction it flows in.
	specModalityRe = regexp.MustCompile(
		`(?is)class="geap-modality-label">(.*?)</span>.*?` +
			`class="geap-supported-modality">(.*?)</span>`,
	)
	// specListedIORe matches the modalities a page lists instead of laying
	// them against a grid, "Inputs: Text, Code, Images", and the side of the
	// request they fall on. The models Vertex serves for other labs state
	// theirs this way.
	specListedIORe = regexp.MustCompile(
		`(?is)<li>\s*(Inputs|Outputs):(.*?)</li>`,
	)
	// specListedModalityRe matches one modality of such a list.
	specListedModalityRe = regexp.MustCompile(`(?is)<span>(.*?)</span>`)
	// specFeatureRe matches one capability and whether the model has it. The
	// page states the answer in a class rather than only in the text.
	specFeatureRe = regexp.MustCompile(
		`(?is)<li class="geap-feature">(.*?)</li>`,
	)
	// specFeatureNameRe matches the capability's own name, which the page
	// links and follows with a break and the variants of it the model offers.
	// Those variants are the capability rather than more of them, and the
	// marker the page hangs off a capability still in preview is not part of
	// its name: reading it left Gemini's computer use recorded as a
	// computer_use_preview_preview_feature.
	specFeatureNameRe = regexp.MustCompile(`(?is)^(.*?)(?:<br|<span|$)`)
	specSupportedRe   = regexp.MustCompile(`(?is)class="geap-supported"`)
	// specSupportedSectionRe matches the capabilities a page groups under one
	// heading rather than marking each with its own. The models Vertex serves
	// for other labs list what they can do and then what they cannot, so the
	// first group is the whole answer.
	specSupportedSectionRe = regexp.MustCompile(
		`(?is)<section class="geap-capabilities-supported">(.*?)</section>`,
	)
	// specSectionFeatureRe matches one capability of such a group. The heading
	// that opens it carries a class and so is not one.
	specSectionFeatureRe = regexp.MustCompile(
		`(?is)<li(?: class="geap-capabilities-not-ga")?>(.*?)</li>`,
	)
	// specHeadRe matches a labelled row of the table. The heading may not run
	// across markup: a row stating a bound carries two headings, the group's
	// and the bound's, and it is the second that names the value beside it.
	specHeadRe = regexp.MustCompile(
		`(?is)<th[^>]*>([^<]*)</th>\s*<td[^>]*>(.*?)</td>`,
	)
	specTagRe = regexp.MustCompile(`(?s)<[^>]*>`)
	// specCountRe matches a quantity and whatever word follows it, since a
	// page may write the quantity out in full or abbreviate it: the Live API
	// page states a context window of "128K" where every other page states
	// 1,048,576. Any other word is a unit and leaves the quantity alone.
	specCountRe = regexp.MustCompile(`([\d][\d,]*)\s*([A-Za-z]*)`)
	// specQuotaRe matches the bounds a page states as prose rather than as
	// rows. The models Vertex serves for other labs state theirs among the
	// per-region quotas, and in either order: the models sold as a service
	// write "65,536 maximum output, 163,840 context length" and the ones sold
	// through a partner write "Max output: 8,192" and "Context length:
	// 524,288". Reading only the first form took the figure beside the wrong
	// label, so a Llama page's output ceiling was recorded as its context.
	//
	// The form that puts the figure first is only read where no colon follows
	// the label, because a quota block states the two forms one after the
	// other: "Output TPM: 80,000 Context length: 200,000" read the other way
	// makes the tokens a minute the context window.
	specQuotaRe = regexp.MustCompile(
		`(?i)([\d][\d,]*)\s*(maximum output|context length)(?:[^:]|$)|` +
			`(max(?:imum)? output|context length)\s*:\s*([\d][\d,]*)`,
	)
	// specSupportedModelsRe matches the models a capability page lists as
	// supporting it, which run from its expandable heading to the next
	// section. Reading the whole page instead would take the navigation, which
	// links every model Google documents, as the list of models supporting it.
	specSupportedModelsRe = regexp.MustCompile(
		`(?is)id="click-to-expand-supported-models"(.*?)(?:<h2|\z)`,
	)
	// gemmaTableRe matches a table on the Gemma page, of which the one read is
	// the one heading a column with a model name.
	gemmaTableRe = regexp.MustCompile(`(?is)<table>(.*?)</table>`)
	gemmaRowRe   = regexp.MustCompile(`(?is)<tr[^>]*>(.*?)</tr>`)
	gemmaCellRe  = regexp.MustCompile(`(?is)<td[^>]*>(.*?)</td>`)
	// versionsRowRe matches a row of the lifecycle tables, which state a model
	// identifier, when it was released, when it retires and what replaces it.
	versionsRowRe = regexp.MustCompile(
		`(?is)<tr[^>]*>\s*<td[^>]*>\s*(?:<a[^>]*></a>\s*)?` +
			`<code[^>]*>(.*?)</code>.*?` +
			`<td[^>]*>(.*?)</td>\s*<td[^>]*>(.*?)</td>` +
			`\s*(?:<td[^>]*>(.*?)</td>)?`,
	)
	// versionsRetiredRe matches the part of the lifecycle page listing the
	// models already withdrawn, which is the last section of it.
	versionsRetiredRe = regexp.MustCompile(`(?is)id="retired-models".*`)
	// modelHrefRe matches a link from the index to a page under the model
	// documentation.
	modelHrefRe = regexp.MustCompile(
		`href="/gemini-enterprise-agent-platform/models/([a-z0-9./-]+)"`,
	)
)

// Rows of the specification table this parser reads, named as Google heads
// them.
const (
	rowContext = "context window"
	rowMaxOut  = "maximum output tokens"
	// rowSequence is what an embedding model's page calls the same bound the
	// generative pages call a context window: the longest input it accepts.
	rowSequence = "maximum sequence length"
	// rowMaxInput is a third heading for that bound, which the pages of the
	// models that answer in an image use in place of a context window.
	rowMaxInput = "maximum input tokens"
	// rowDimensions is the width of the vector an embedding model returns,
	// which its page states as a ceiling, "Up to 1,024".
	rowDimensions = "output dimensions"
	// rowLaunchStage is the stage Google serves a model at. Only the models it
	// serves for other labs head a row with it; a Gemini page states the same
	// thing against the release its version block describes.
	rowLaunchStage = "launch stage"
	// rowKnowledgeCutoff is how recent the data a model was trained on is.
	rowKnowledgeCutoff = "knowledge cutoff date"
	// rowSessions is how many streams a model holds at once.
	rowSessions = "maximum concurrent sessions"
	// rowInputSize is how large a request may be, stated in bytes.
	rowInputSize = "input size limit"
	// rowLayers is the depth of an embedding model.
	rowLayers = "number of layers"
)

// Columns of the Gemma table, which names a variant and then says how large it
// is, what it takes, what it returns, which tunings it comes in and what it
// runs on. Only the three read here say anything the catalog holds.
const (
	gemmaNameCell   = 0
	gemmaInputCell  = 2
	gemmaOutputCell = 3
)

// gemmaModalities are the words that table states a variant's input and output
// in. A row naming none of them describes something the catalog has no word
// for, such as the embedding a Gemma vision encoder returns.
var gemmaModalities = []string{"text", "image", "audio", "video"}

// modelPageURLs derives the model pages the index links to, keeping the
// sections that describe models and dropping the guides filed beside them.
func modelPageURLs(index catalog.Document) []string {
	var urls []string
	for _, match := range modelHrefRe.FindAllStringSubmatch(
		string(index.Body),
		-1,
	) {
		path := match[1]
		if !inModelSection(path) {
			continue
		}
		url := modelPagePre + path
		if !slices.Contains(urls, url) {
			urls = append(urls, url)
		}
	}
	slices.Sort(urls)
	return urls
}

// inModelSection reports whether a path describes a model. A page filed under
// a section's own capabilities is a guide to a feature, not a model.
func inModelSection(path string) bool {
	if strings.Contains(path, "capabilities/") {
		return false
	}
	for _, section := range modelSections {
		if strings.HasPrefix(path, section) {
			return true
		}
	}
	return false
}

// applyModelPages reads every model page onto the models the billing catalog
// established.
//
// A page states the identifier the API answers to, which is the identifier a
// SKU is read as, so the two need no reconciling for the models Vertex sells
// plainly. A SKU can be finer than a model, though: Vertex prices Gemini 2.5
// Flash differently with thinking on and again above a length, and names each
// rate for the model plus the condition. Those reach the model's page through
// the same longest-prefix rule the plain ones match exactly by.
//
// A model the lifecycle page lists as retired is dropped here rather than where
// its rates are read, because the billing catalog is read first and goes on
// metering a model Vertex has stopped serving, so only the two documents
// together settle it. Dropping the model is what removes its file, since the
// store deletes what the parser stops emitting. A model still served keeps the
// date it is due to retire on, that being a fact about something still sold.
func (b *builder) applyModelPages(
	docs []catalog.Document,
	pages map[string]*documented,
) {
	lives := readLifecycles(docs)
	names := slices.Sorted(maps.Keys(pages))
	used := map[string]bool{}
	kept := make([]string, 0, len(b.order))
	for _, id := range b.order {
		life, listed := lives[strings.ToLower(id)]
		if listed && life.State == StateRetired {
			delete(b.models, id)
			continue
		}
		kept = append(kept, id)
		m := b.models[id]
		name := matchPage(id, names, pages)
		if name != "" {
			used[name] = true
			applyDocumented(m, pages[name])
		}
		if listed {
			applyLifecycle(m, life)
		}
	}
	b.order = kept
	b.addDocumented(names, pages, used, lives)
}

// addDocumented holds the models Google documents that the billing catalog
// does not meter.
//
// Vertex bills for the models it serves for other labs through those labs'
// own services rather than through its own, so the Claude, Grok and Mistral
// releases it serves carry no SKU under the Vertex service and were absent
// from the catalog altogether, as were the Gemini releases whose meter names
// two models at once. A page of a model's own is what admits it: the page
// states the identifier the API answers to, what the model takes and returns,
// what it can do, where it answers and when it was released, which is a model
// described rather than a name. An index row without such a page states the
// lab and a sentence and nothing else, so it enriches a model and never
// stands for one.
func (b *builder) addDocumented(
	names []string,
	pages map[string]*documented,
	used map[string]bool,
	lives map[string]lifecycle,
) {
	for _, name := range names {
		page := pages[name]
		if used[name] || !page.Named {
			continue
		}
		life, listed := lives[name]
		if listed && life.State == StateRetired {
			continue
		}
		m := b.model(name, kindFor(name))
		m.Name = name
		applyDocumented(m, page)
		if listed {
			applyLifecycle(m, life)
		}
	}
}

// applyDocumented records everything the documentation states about a model.
func applyDocumented(m *catalog.Model, page *documented) {
	for _, source := range page.Sources {
		m.AddSource(source)
	}
	if page.Title != "" {
		m.Name = page.Title
	}
	m.SetLimit(LimitContextWindow, page.Context)
	m.SetLimit(LimitMaxOutputTokens, page.MaxOut)
	m.SetLimit(LimitMaxInputTokens, page.MaxInput)
	m.SetLimit(LimitConcurrentSessions, page.Sessions)
	for key, value := range page.Quotas {
		m.SetLimit(key, value)
	}
	m.SetAttr(AttrSummary, page.Summary)
	m.SetAttr(AttrAuthor, page.Author)
	m.SetAttr(AttrState, page.State)
	m.SetAttr(AttrReleaseDate, page.ReleaseDate)
	m.SetAttr(AttrKnowledgeCutoff, page.Cutoff)
	m.SetAttr(AttrRetirementDate, page.Retirement)
	m.SetAttr(AttrRetirementQualifier, page.RetireQualifier)
	m.SetAttr(AttrInputSizeLimit, page.InputSize)
	if page.Layers > 0 {
		m.SetAttr(AttrLayers, strconv.FormatInt(page.Layers, 10))
	}
	m.AddList(ListFeatures, page.Features...)
	m.AddList(ListInputModalities, page.InputMod...)
	m.AddList(ListOutputModalities, page.OutMod...)
	m.AddList(ListRegions, page.Regions...)
	m.AddList(ListLanguages, page.Languages...)
	if width := page.Dimensions; width > 0 {
		m.AddList(ListDimensions, strconv.FormatInt(width, 10))
	}
}

// applyLifecycle records what a lifecycle table states, which fills in for a
// model page rather than overriding it: the page describes the release and the
// table describes the schedule, and where both state a date they state the
// same one.
func applyLifecycle(m *catalog.Model, life lifecycle) {
	m.AddSource(life.Source)
	if m.Attrs[AttrRetirementDate] == "" {
		m.SetAttr(AttrRetirementDate, life.Retires)
		m.SetAttr(AttrRetirementQualifier, life.Qualifier)
	}
	if m.Attrs[AttrReleaseDate] == "" {
		m.SetAttr(AttrReleaseDate, life.Released)
	}
	m.SetAttr(AttrDeprecatedOn, life.Deprecated)
	m.SetAttr(AttrReplacement, life.Replacement)
	m.SetAttr(AttrSelfDeploy, life.SelfDeploy)
}

// readDocumented indexes everything the documentation states about a model, by
// the name the billing catalog would call it. A model page is read first,
// because it is the only document naming one model exactly, and the pages that
// describe a family or a capability are folded onto what it established.
func readDocumented(docs []catalog.Document) map[string]*documented {
	pages := map[string]*documented{}
	byURL := map[string]*documented{}
	for _, doc := range docs {
		id, page, ok := readModelPage(string(doc.Body))
		if !ok {
			continue
		}
		page.Named = true
		page.Served = true
		page.ID = id
		entry := documentedFor(pages, servedName(id))
		entry.merge(page, doc.URL)
		byURL[doc.URL] = entry
	}
	for _, doc := range docs {
		readCapabilityPage(byURL, doc)
		readGemmaSizes(pages, doc)
		readModelCards(byURL, doc)
		readModelTable(pages, doc)
	}
	return pages
}

// documentedFor returns the entry for a name, creating it if absent.
func documentedFor(
	pages map[string]*documented,
	name string,
) *documented {
	page, ok := pages[name]
	if !ok {
		page = &documented{}
		pages[name] = page
	}
	return page
}

// documented is what the documentation states about one model.
type documented struct {
	Context int64
	MaxOut  int64
	// MaxInput is the ceiling a page states in place of a context window,
	// which the models Vertex serves for other labs state and Google's own do
	// not.
	MaxInput int64
	// Dimensions is the width of the vector an embedding model returns, and is
	// zero for every model that returns something else.
	Dimensions int64
	// Sessions is how many streams a model holds at once, which only the Live
	// API page states.
	Sessions int64
	// State, ReleaseDate, Retirement and RetireQualifier are what the version
	// block says of the release the page describes.
	State           string
	ReleaseDate     string
	Retirement      string
	RetireQualifier string
	// Cutoff is how recent the data the model was trained on is.
	Cutoff string
	// InputSize is how large a request may be, stated in bytes.
	InputSize string
	// Layers is the depth of an embedding model.
	Layers int64
	// Quotas are the rate limits the page states for every endpoint alike.
	Quotas   map[string]int64
	Regions  []string
	Features []string
	InputMod []string
	OutMod   []string
	// Sources are the URLs of the documents that stated all this.
	Sources []string
	// Languages are the languages the index states the family answers in.
	Languages []string
	// ID is the identifier Model Garden serves the model under, which only the
	// index tables state for a model with no page of its own.
	ID string
	// Served reports that some document named this as a model Vertex serves,
	// as against a row of a table describing a family. Only such an entry may
	// stand for a model on its own.
	Served bool
	// Summary, Author and Title are what the index tables state, which no
	// model page does: what the model is for, which lab made it and the name
	// Google writes for it in prose.
	Summary string
	Author  string
	Title   string
	// Named reports that a page of this model's own stated it, rather than a
	// table naming a family. Only such a page names one model exactly, which
	// is what lets a coarser SKU reach it.
	Named bool
}

// merge folds a second statement of the same model onto the first, which wins
// wherever they both answer. Two pages can name one model, as the page for a
// preview does the page that replaces it.
func (d *documented) merge(other documented, source string) {
	if d.Context == 0 {
		d.Context = other.Context
	}
	if d.MaxOut == 0 {
		d.MaxOut = other.MaxOut
	}
	if d.Dimensions == 0 {
		d.Dimensions = other.Dimensions
	}
	if d.MaxInput == 0 {
		d.MaxInput = other.MaxInput
	}
	if d.Sessions == 0 {
		d.Sessions = other.Sessions
	}
	if d.Layers == 0 {
		d.Layers = other.Layers
	}
	mergeText(&d.State, other.State)
	mergeText(&d.ReleaseDate, other.ReleaseDate)
	mergeText(&d.Retirement, other.Retirement)
	mergeText(&d.RetireQualifier, other.RetireQualifier)
	mergeText(&d.Cutoff, other.Cutoff)
	mergeText(&d.InputSize, other.InputSize)
	mergeText(&d.Summary, other.Summary)
	mergeText(&d.Author, other.Author)
	mergeText(&d.Title, other.Title)
	for key, value := range other.Quotas {
		if d.Quotas == nil {
			d.Quotas = map[string]int64{}
		}
		if _, ok := d.Quotas[key]; !ok {
			d.Quotas[key] = value
		}
	}
	for _, value := range other.Regions {
		d.Regions = appendNew(d.Regions, value)
	}
	for _, value := range other.Languages {
		d.Languages = appendNew(d.Languages, value)
	}
	mergeText(&d.ID, other.ID)
	d.Served = d.Served || other.Served
	d.Named = d.Named || other.Named
	for _, value := range other.Features {
		d.Features = appendNew(d.Features, value)
	}
	for _, value := range other.InputMod {
		d.InputMod = appendNew(d.InputMod, value)
	}
	for _, value := range other.OutMod {
		d.OutMod = appendNew(d.OutMod, value)
	}
	d.Sources = appendNew(d.Sources, source)
}

// mergeText fills a value in from a second statement of the same model, the
// first statement winning wherever both answer.
func mergeText(into *string, value string) {
	if *into == "" {
		*into = value
	}
}

// readCapabilityPage records the capability a page enumerates the models for,
// against every model page it links to.
func readCapabilityPage(byURL map[string]*documented, doc catalog.Document) {
	feature, ok := featureOfPage(doc.URL)
	if !ok {
		return
	}
	for _, block := range specSupportedModelsRe.FindAllStringSubmatch(
		string(doc.Body),
		-1,
	) {
		for _, match := range modelHrefRe.FindAllStringSubmatch(block[1], -1) {
			page, ok := byURL[modelPagePre+match[1]]
			if !ok {
				continue
			}
			page.Features = appendNew(page.Features, feature)
			page.Sources = appendNew(page.Sources, doc.URL)
		}
	}
}

// readGemmaSizes reads the table of Gemma variants, which states what each of
// them takes and returns. Gemma is sold for self deployment, so all but the
// one variant offered as a service lack a page of their own, and this table is
// the only document naming their modalities.
func readGemmaSizes(pages map[string]*documented, doc catalog.Document) {
	if doc.URL != gemmaSizesURL {
		return
	}
	for _, row := range gemmaRowRe.FindAllStringSubmatch(
		gemmaTable(string(doc.Body)),
		-1,
	) {
		cells := gemmaCellRe.FindAllStringSubmatch(row[1], -1)
		if len(cells) <= gemmaOutputCell {
			continue
		}
		name := slugID(specText(cells[gemmaNameCell][1]))
		in := gemmaModalitiesOf(specText(cells[gemmaInputCell][1]))
		out := gemmaModalitiesOf(specText(cells[gemmaOutputCell][1]))
		if name == "" || len(in) == 0 || len(out) == 0 {
			continue
		}
		documentedFor(pages, name).merge(
			documented{InputMod: in, OutMod: out},
			doc.URL,
		)
	}
}

// gemmaTable returns the table of variants. The page carries a second table of
// the same models headed by what they are good for, so the one read is told by
// its heading a column for what a variant takes and one for what it returns.
func gemmaTable(body string) string {
	for _, table := range gemmaTableRe.FindAllStringSubmatch(body, -1) {
		head := specText(gemmaRowRe.FindString(table[1]))
		if strings.Contains(head, "Input") && strings.Contains(head, "Output") {
			return table[1]
		}
	}
	return ""
}

// gemmaModalitiesOf reads a cell naming what a variant works in, "Text and
// image" or "Text, image and audio".
func gemmaModalitiesOf(cell string) []string {
	var out []string
	lower := strings.ToLower(cell)
	for _, name := range gemmaModalities {
		if strings.Contains(lower, name) {
			out = appendNew(out, name)
		}
	}
	return out
}

// firstField returns the first word of a value, dropping the marker the page
// hangs off an identifier to footnote it.
func firstField(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// readModelPage reads one page's specification table, reporting whether the
// page carries one at all. Many pages under the same path are guides.
func readModelPage(body string) (string, documented, bool) {
	table := specTableRe.FindStringSubmatch(body)
	if table == nil {
		return "", documented{}, false
	}
	id := specText(firstOf(specIDRe, table[1]))
	if id == "" {
		return "", documented{}, false
	}
	var page documented
	for _, row := range specHeadRe.FindAllStringSubmatch(table[1], -1) {
		switch strings.ToLower(specText(row[1])) {
		case rowContext:
			page.Context = parseCount(specText(row[2]))
		case rowMaxOut:
			page.MaxOut = parseCount(specText(row[2]))
		case rowSequence, rowMaxInput:
			page.Context = parseCount(specText(row[2]))
		case rowDimensions:
			page.Dimensions = parseCount(specText(row[2]))
		case rowLaunchStage:
			page.State = stateOf(specText(row[2]))
		case rowKnowledgeCutoff:
			page.Cutoff = isoDate(specText(row[2]))
		case rowSessions:
			page.Sessions = parseCount(specText(row[2]))
		case rowInputSize:
			page.InputSize = specText(row[2])
		case rowLayers:
			page.Layers = parseCount(specText(row[2]))
		}
	}
	readQuotas(&page, specText(table[1]))
	readListedLimits(&page, table[1])
	readQuotaLimits(&page, table[1])
	readVersionBlock(&page, table[1], id)
	readRegions(&page, table[1])
	readModalities(&page, table[1])
	readListedModalities(&page, table[1])
	readFeatures(&page, table[1])
	return id, page, true
}

// readQuotas reads the bounds a page states as prose. The models Vertex serves
// for other labs carry no token-limit rows and state the same two figures
// among their per-region quotas instead, once per region and alike in each, so
// the first statement is taken and the rest agree with it.
func readQuotas(page *documented, text string) {
	for _, match := range specQuotaRe.FindAllStringSubmatch(text, -1) {
		label, count := match[2], match[1]
		if label == "" {
			label, count = match[3], match[4]
		}
		n := parseCount(count)
		if strings.EqualFold(label, "context length") {
			if page.Context == 0 {
				page.Context = n
			}
			continue
		}
		if page.MaxOut == 0 {
			page.MaxOut = n
		}
	}
}

// readModalities records what a model takes and returns. A page names every
// modality it knows of and says of each whether it flows in, out, or both.
func readModalities(page *documented, table string) {
	for _, match := range specModalityRe.FindAllStringSubmatch(table, -1) {
		name, ok := modalityNames[strings.ToLower(specText(match[1]))]
		if !ok {
			continue
		}
		direction := strings.ToLower(specText(match[2]))
		if strings.Contains(direction, "input") {
			page.InputMod = appendNew(page.InputMod, name)
		}
		if strings.Contains(direction, "output") {
			page.OutMod = appendNew(page.OutMod, name)
		}
	}
}

// readListedModalities records what a model takes and returns where its page
// lists the two rather than laying them against a grid. Such a page names a
// modality only where the model has it, so there is nothing to leave out.
func readListedModalities(page *documented, table string) {
	for _, match := range specListedIORe.FindAllStringSubmatch(table, -1) {
		for _, listed := range specListedModalityRe.FindAllStringSubmatch(
			match[2],
			-1,
		) {
			name, ok := modalityNames[strings.ToLower(specText(listed[1]))]
			if !ok {
				continue
			}
			if strings.EqualFold(match[1], "inputs") {
				page.InputMod = appendNew(page.InputMod, name)
				continue
			}
			page.OutMod = appendNew(page.OutMod, name)
		}
	}
}

// readFeatures records the capabilities a model has.
//
// A page lists the ones it lacks as plainly as the ones it has, marking each
// in a class of its own, so only the supported ones are kept. Only the
// capabilities row is read: the table carries a second list in the same shape
// below it, of the ways the model may be bought, and provisioned throughput is
// a billing arrangement rather than something the model can do.
func readFeatures(page *documented, table string) {
	readFeatureRow(page, rowBlock(table, blockCapability))
	readFeatureRow(page, rowBlock(table, blockTools))
}

// readFeatureRow records the capabilities one such row states.
//
// Google splits them across two rows, what the model can do and which tools it
// can be given, and reading only the first left every Gemini model without the
// grounding, the code execution and the computer use its page states.
func readFeatureRow(page *documented, row string) {
	for _, match := range specFeatureRe.FindAllStringSubmatch(row, -1) {
		if !specSupportedRe.MatchString(match[1]) {
			continue
		}
		name := specText(firstOf(specFeatureNameRe, match[1]))
		page.Features = appendNew(page.Features, featureName(name))
	}
	for _, match := range specSectionFeatureRe.FindAllStringSubmatch(
		firstOf(specSupportedSectionRe, row),
		-1,
	) {
		name := specText(firstOf(specFeatureNameRe, match[1]))
		page.Features = appendNew(page.Features, featureName(name))
	}
}

// featureWordRe matches whatever in a capability's name is not part of an
// identifier, including the brackets Google writes an abbreviation in.
var featureWordRe = regexp.MustCompile(`[^a-z0-9]+`)

// featureName rewrites a capability into the catalog's vocabulary.
func featureName(name string) string {
	head := strings.ToLower(strings.TrimSpace(name))
	if mapped, ok := featureNames[head]; ok {
		return mapped
	}
	return strings.Trim(featureWordRe.ReplaceAllString(head, "_"), "_")
}

// servingSuffixes are what the documentation appends to a model's name that
// the billing catalog leaves off. They say how a model is served or which
// tuning of it is offered, not which model it is: the catalog's deepseek-v3.2
// is the page's deepseek-v3.2-maas, and its llama-3.3-70b the page's
// llama-3.3-70b-instruct-maas.
var servingSuffixes = []string{"-maas", "-preview", "-instruct", "-it"}

// servedName strips those suffixes, so that the two documents name a model the
// same way. Stripping is repeated because a page can carry two of them.
func servedName(id string) string {
	name := strings.ToLower(strings.TrimSpace(id))
	for changed := true; changed; {
		changed = false
		for _, suffix := range servingSuffixes {
			if trimmed, ok := strings.CutSuffix(name, suffix); ok {
				name, changed = trimmed, true
			}
		}
	}
	return name
}

// publisherPrefixes are the labs the billing catalog names in front of a model
// where its page does not: the catalog's "OpenAI gpt-oss-120b" is the page's
// gpt-oss-120b, and the same model is metered under both spellings.
var publisherPrefixes = []string{"openai-", "meta-", "google-"}

// matchPage returns the name of what the documentation states about the model
// a SKU is read as, or the empty string where it states nothing.
func matchPage(
	id string,
	names []string,
	pages map[string]*documented,
) string {
	if name := longestPrefix(id, names); name != "" {
		return name
	}
	for _, prefix := range publisherPrefixes {
		trimmed, ok := strings.CutPrefix(strings.ToLower(id), prefix)
		if !ok {
			continue
		}
		if name := longestPrefix(trimmed, names); name != "" {
			return name
		}
	}
	return onlyExtension(id, names, pages)
}

// onlyExtension returns the one model page whose identifier extends id, and
// nothing where more than one does. The billing catalog names a model more
// briefly than its page does, "Llama 4 Maverick" against
// llama-4-maverick-17b-128e-instruct-maas, so a name that only begins a page's
// still reaches it. A name beginning two reaches neither, since nothing in
// either document says which was meant, and only a page of a model's own is
// eligible: a table naming a family names every member of it, so an
// identifier standing for the family would match whichever member sorts first.
func onlyExtension(
	id string,
	names []string,
	pages map[string]*documented,
) string {
	lower, found := strings.ToLower(id), ""
	for _, name := range names {
		if !strings.HasPrefix(name, lower+"-") || !pages[name].Named {
			continue
		}
		if found != "" {
			return ""
		}
		found = name
	}
	return found
}

// longestPrefix returns the longest name that id equals or extends, so a SKU
// naming a condition as well as a model still reaches the model.
func longestPrefix(id string, names []string) string {
	lower, best := strings.ToLower(id), ""
	for _, name := range names {
		if lower != name && !strings.HasPrefix(lower, name+"-") {
			continue
		}
		if len(name) > len(best) {
			best = name
		}
	}
	return best
}

// appendNew adds a value not already present.
func appendNew(items []string, value string) []string {
	if value == "" || slices.Contains(items, value) {
		return items
	}
	return append(items, value)
}

// parseCount reads a grouped quantity such as "1,048,576", or an abbreviated
// one such as "128K".
func parseCount(value string) int64 {
	match := specCountRe.FindStringSubmatch(value)
	if match == nil {
		return 0
	}
	n, err := strconv.ParseInt(strings.ReplaceAll(match[1], ",", ""), 10, 64)
	if err != nil {
		return 0
	}
	switch strings.ToLower(match[2]) {
	case "k":
		return n * 1024
	case "m":
		return n * 1024 * 1024
	}
	return n
}

// firstOf returns the first capture of re, or the empty string.
func firstOf(re *regexp.Regexp, body string) string {
	if match := re.FindStringSubmatch(body); match != nil {
		return match[1]
	}
	return ""
}

// specText strips markup and collapses whitespace.
func specText(html string) string {
	return strings.Join(
		strings.Fields(specTagRe.ReplaceAllString(html, " ")),
		" ",
	)
}
