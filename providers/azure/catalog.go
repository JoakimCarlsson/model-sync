package azure

import (
	"html"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// ModelsURL is Azure's model documentation, which is the only document stating
// what a model holds and can do. The price list states neither.
const ModelsURL = "https://learn.microsoft.com/en-us/azure/ai-foundry/" +
	"openai/concepts/models"

// PartnersURL is where Azure documents the models it resells rather than
// sells: Phi, Codestral, Ministral and the rest of the catalog that is not its
// own. Its tables have the same shape as the collections tables on ModelsURL
// and state the same facts, so the same reader takes both.
const PartnersURL = "https://learn.microsoft.com/en-us/azure/ai-foundry/" +
	"foundry-models/concepts/models-from-partners"

// ImagesURL and VideoURL are the image and video generation guides, which are
// where Azure states what those models take and return. The model tables state
// only how long a prompt may be, in characters, and say nothing about
// modality. Both guides compare their models in one table laid out the other
// way round, a column per model and a row per fact.
const (
	ImagesURL = "https://learn.microsoft.com/en-us/azure/ai-foundry/" +
		"openai/how-to/dall-e"
	VideoURL = "https://learn.microsoft.com/en-us/azure/ai-foundry/" +
		"openai/concepts/video-generation"
)

// Numeric keys the documentation populates.
const (
	LimitContextWindow   = "context_window"
	LimitMaxOutputTokens = "max_output_tokens"
)

// Enumeration keys the documentation populates.
const (
	ListFeatures         = catalog.ListFeatures
	ListEndpoints        = "endpoints"
	ListInputModalities  = "input_modalities"
	ListOutputModalities = "output_modalities"
)

// AttrTrainingCutoff is the last date a model's training data runs to.
const AttrTrainingCutoff = "training_data_cutoff"

// Columns of the documentation's model tables, named as Azure heads them.
const (
	colModelID  = "model id"
	colDescribe = "description"
	colContext  = "context window"
	colMaxOut   = "max output"
	colTraining = "training data"
	// colRequest is what the older tables head the same two bounds with,
	// stating both in one cell as a labelled pair, and what the embedding
	// tables head the one bound they have with.
	colRequest = "max request"
	// colDimensions is the width of the vector an embedding model returns.
	colDimensions = "output dimensions"
	// colModality is what the fine tuning table heads its last column with,
	// written as the flow through the model: "Text and vision to text". It is
	// the only place Azure states the modalities of the models it documents
	// nowhere else, Qwen-32B among them.
	colModality = "modality"
)

// capabilityFeatures map one of the documentation's capability bullets onto
// the features it states. Azure writes several capabilities into one bullet,
// so a bullet yields a list rather than a single name.
var capabilityFeatures = map[string][]string{
	"reasoning":                    {catalog.CapabilityReasoning},
	"enhanced reasoning abilities": {catalog.CapabilityReasoning},
	"new reasoning model, offering enhanced reasoning abilities": {
		catalog.CapabilityReasoning,
	},
	"structured outputs": {catalog.CapabilityStructuredOutputs},
	"structured outputs (chat completions)": {
		catalog.CapabilityStructuredOutputs,
	},
	"json mode": {
		catalog.CapabilityStructuredOutputs,
		catalog.CapabilityJSONMode,
	},
	"streaming":           {"streaming"},
	"computer use":        {"computer_use"},
	"function calling":    {catalog.CapabilityFunctionCalling},
	"functions and tools": {catalog.CapabilityFunctionCalling},
	"functions & tools":   {catalog.CapabilityFunctionCalling},
	"tools":               {catalog.CapabilityFunctionCalling},
	"parallel function calling": {
		catalog.CapabilityFunctionCalling,
		"parallel_tool_calls",
	},
	"functions, tools, and parallel tool calling": {
		catalog.CapabilityFunctionCalling,
		"parallel_tool_calls",
	},
}

// capabilityModalities map a bullet describing what a model handles onto the
// modalities it names. Azure writes these as prose rather than as a list, and
// writes the same fact several ways.
var capabilityModalities = map[string]struct{ in, out []string }{
	"text and image processing": {[]string{"text", "image"}, []string{"text"}},
	"text and image input":      {[]string{"text", "image"}, nil},
	"input : text/image":        {[]string{"text", "image"}, nil},
	"input : text":              {[]string{"text"}, nil},
	"text output":               {nil, []string{"text"}},
	"output : text only":        {nil, []string{"text"}},
	"text-only processing":      {[]string{"text"}, []string{"text"}},
	"text in/text out only":     {[]string{"text"}, []string{"text"}},
	"text (input/output)":       {[]string{"text"}, []string{"text"}},
	"image (input)":             {[]string{"image"}, nil},
	"general-purpose speech recognition model": {
		[]string{"audio"},
		[]string{"text"},
	},
	"audio model for audio and text generation": {
		[]string{"audio", "text"},
		[]string{"audio", "text"},
	},
}

// modalityPrefixes read what a model handles out of a description Azure wrote
// as a sentence rather than as a bullet. The audio, transcription and speech
// tables describe a model instead of listing what it takes, and each family
// says it the same way with a different tail: "Audio model for real-time
// low-latency transcription. Current recommended model for realtime
// transcription scenarios" is the first of them with a recommendation added.
var modalityPrefixes = []struct {
	prefix  string
	in, out []string
}{
	{
		"audio model for real-time low-latency transcription",
		[]string{"audio"},
		[]string{"text"},
	},
	{
		"audio model for real-time multilingual translation",
		[]string{"audio", "text"},
		[]string{"audio", "text"},
	},
	{
		"audio model for real-time audio processing",
		[]string{"audio", "text"},
		[]string{"audio", "text"},
	},
	{
		"audio models for real-time audio processing",
		[]string{"audio", "text"},
		[]string{"audio", "text"},
	},
	{"speech-to-text model", []string{"audio"}, []string{"text"}},
	{
		"offline speech-to-text model",
		[]string{"audio"},
		[]string{"text"},
	},
	{"text-to-speech model", []string{"text"}, []string{"audio"}},
}

// endpointBullets are the bullets naming an API a model answers on rather than
// something it can do.
var endpointBullets = map[string]string{
	"chat completions api": "Chat Completions",
	"responses api":        "Responses",
	"responses api only":   "Responses",
}

var (
	docTableRe = regexp.MustCompile(`(?is)<table[^>]*>(.*?)</table>`)
	docRowRe   = regexp.MustCompile(`(?is)<tr[^>]*>(.*?)</tr>`)
	docCellRe  = regexp.MustCompile(`(?is)<t[dh][^>]*>(.*?)</t[dh]\s*>`)
	docTagRe   = regexp.MustCompile(`(?s)<[^>]*>`)
	// docBreakRe matches the separator Azure writes between bullets, which is
	// a line break rather than a list.
	docBreakRe = regexp.MustCompile(`(?i)<br\s*/?>`)
	// docCountRe matches the first grouped number in a cell, which is the
	// model's own bound. Azure follows it with the lower ceilings that
	// particular deployments impose, which are the deployment's and not the
	// model's.
	docCountRe = regexp.MustCompile(`([\d][\d,]{2,})`)
	// docSideRe matches one half of a labelled pair, which is how the older
	// tables state both bounds in a single cell.
	docSideRe = regexp.MustCompile(`(?i)(input|output)\s*:\s*([\d][\d,]{2,})`)
	// skuWindowRe matches a context window Azure states inside a meter's own
	// name, as in "gpt-4-32k" or "gpt-35-turbo16k-0125", for the models its
	// documentation has dropped. The count must end the name or a segment of
	// it, so that a version such as "kimi-k2.5" is not read as one.
	skuWindowRe = regexp.MustCompile(`(?i)(?:^|[^\d])(\d{1,3})k(?:$|-)`)
)

// documented is what the documentation states about one model.
type documented struct {
	Sources      []string
	Name         string
	Summary      string
	State        string
	Publisher    string
	Author       string
	License      string
	Version      string
	Release      string
	Added        string
	Retire       string
	Replacement  string
	TrainRetire  string
	DeployRetire string
	Context      int64
	MaxOut       int64
	// Bounds marks a reading whose token bounds Azure states as fields rather
	// than as prose, which is the gallery listing and nothing else.
	Bounds      bool
	Training    string
	Features    []string
	Endpoint    []string
	InputMod    []string
	OutMod      []string
	Languages   []string
	Dimensions  []string
	Keywords    []string
	Tasks       []string
	Deployments []string
	Regions     []string
}

// applyCatalog reads the documentation onto the models the price list
// established.
//
// The two name models differently. A meter's name is a billing SKU and carries
// what is being charged for as a suffix, so one model has several: gpt-5-mini
// and gpt-5-mini-inpt are the same model billed two ways. The documentation
// names the model alone. A meter is therefore matched to the longest
// documented name it equals or extends, which attaches one document to every
// meter of the model and keeps gpt-5 from claiming gpt-5-mini.
func (b *builder) applyCatalog(pages []catalog.Document) {
	docs := map[string]documented{}
	for _, page := range pages {
		if page.URL == GalleryURL {
			mergeDocumented(docs, readGallery(page.Body), page.URL)
			continue
		}
		body := string(page.Body)
		mergeDocumented(docs, readDocumentation(body), page.URL)
		mergeDocumented(docs, readCollections(body), page.URL)
		mergeDocumented(docs, readAspects(body), page.URL)
		mergeDocumented(docs, readSchedule(body), page.URL)
	}
	names := slices.Sorted(maps.Keys(docs))
	for _, id := range b.order {
		matched := matchingPrefixes(id, names)
		for _, name := range matched {
			for _, source := range docs[name].Sources {
				b.models[id].AddSource(source)
			}
		}
		if len(matched) > 0 {
			apply(b.models[id], docs[matched[0]])
			matched = matched[1:]
		}
		b.applySKUWindow(b.models[id])
		for _, name := range matched {
			applyFamily(b.models[id], docs[name])
		}
	}
}

// mergeDocumented folds one page's readings into the whole, field by field and
// letting whoever stated one first keep it.
//
// The pages overlap rather than partition: Azure's fine tuning table names
// Ministral-3B and states only the modalities it fine tunes in, and the page
// for the models it resells states everything else about the same model. A
// merge that took the first page's reading whole would drop the second.
func mergeDocumented(into, from map[string]documented, source string) {
	for id, found := range from {
		found.Sources = appendNew(found.Sources, source)
		d := into[id]
		fold(&d, found)
		into[id] = d
	}
}

// fold merges one reading into another, field by field and letting whoever
// stated one first keep it.
//
// The token bounds are the exception: a reading that states them as fields
// displaces one that read them out of a table cell, whichever came first. The
// prose is where they go wrong. Azure's models page gives gpt-audio "Input:
// 128,00", a digit short of the 128,000 the same model's catalog entry states,
// and gives embed-v-4-0 the 512 tokens Cohere's third embedding model took
// rather than the 131,072 its fourth does. Both figures are below the output
// ceiling stated beside them, which is a pair no request can satisfy, and both
// are stated correctly in the fields.
func fold(into *documented, found documented) {
	scalars := []struct {
		at    *string
		found string
	}{
		{&into.Name, found.Name},
		{&into.Summary, found.Summary},
		{&into.State, found.State},
		{&into.Publisher, found.Publisher},
		{&into.Author, found.Author},
		{&into.License, found.License},
		{&into.Version, found.Version},
		{&into.Release, found.Release},
		{&into.Added, found.Added},
		{&into.Retire, found.Retire},
		{&into.Replacement, found.Replacement},
		{&into.TrainRetire, found.TrainRetire},
		{&into.DeployRetire, found.DeployRetire},
		{&into.Training, found.Training},
	}
	for _, s := range scalars {
		if *s.at == "" {
			*s.at = s.found
		}
	}
	stated := found.Bounds && !into.Bounds
	if found.Context > 0 && (into.Context == 0 || stated) {
		into.Context = found.Context
	}
	if found.MaxOut > 0 && (into.MaxOut == 0 || stated) {
		into.MaxOut = found.MaxOut
	}
	into.Bounds = into.Bounds || (found.Bounds && found.Context > 0)
	lists := []struct {
		at    *[]string
		found []string
	}{
		{&into.Sources, found.Sources},
		{&into.Features, found.Features},
		{&into.Endpoint, found.Endpoint},
		{&into.InputMod, found.InputMod},
		{&into.OutMod, found.OutMod},
		{&into.Languages, found.Languages},
		{&into.Dimensions, found.Dimensions},
		{&into.Keywords, found.Keywords},
		{&into.Tasks, found.Tasks},
		{&into.Deployments, found.Deployments},
		{&into.Regions, found.Regions},
	}
	for _, l := range lists {
		*l.at = appendNew(*l.at, l.found...)
	}
}

// applySKUWindow reads a context window out of a meter's own name.
//
// Azure's documentation covers the models it sells today. The price list also
// carries meters for models it has stopped documenting, and those name their
// window in the meter: gpt-4-32k is the 32,000 token deployment. That is Azure
// stating the window, in the only place it still states it.
//
// It runs after the document naming this model and before the documents naming
// its family, which is where it belongs in the order of specificity. The name
// states a round number where the document states the window: Azure documents
// gpt-4-turbo-128k as holding 128,000 tokens and Phi-3-mini-4k-instruct as
// holding 4,096, and read first the name gave Phi-3 a 4,000 token window with
// an output ceiling of 4,096 that would not fit in it. Read last it was worse,
// since the family document for gpt-4 gave gpt-4-32k the 128,000 tokens of the
// turbo deployment; a meter naming its own window is more specific than
// anything said about the family, and less specific than anything said about
// the model.
func (b *builder) applySKUWindow(m *catalog.Model) {
	match := skuWindowRe.FindStringSubmatch(m.ID)
	if match == nil {
		return
	}
	n, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil {
		return
	}
	m.SetLimit(LimitContextWindow, n*1_000)
}

// applyFamily records what a document naming a model's family states about it.
//
// Only what a family shares is taken. Azure states a token bound, a training
// cutoff and a set of capabilities once for a family, and gpt-4-32k has none of
// its own; it states a name, a description, a lifecycle and a retirement per
// model, and the family's are not the model's. The schedule marks gpt-4o-mini
// as having no replacement and gpt-4o as replaced by gpt-5.1, and reading the
// family's answer into the gap would contradict the row written about the
// model.
func applyFamily(m *catalog.Model, d documented) {
	m.SetLimit(LimitContextWindow, d.Context)
	m.SetLimit(LimitMaxOutputTokens, d.MaxOut)
	m.SetAttr(AttrTrainingCutoff, d.Training)
	m.SetAttr(AttrKnowledgeCutoff, isoMonth(d.Training))
	m.AddList(ListFeatures, d.Features...)
	m.AddList(ListEndpoints, d.Endpoint...)
	m.AddList(ListInputModalities, d.InputMod...)
	m.AddList(ListOutputModalities, d.OutMod...)
	m.AddList(ListLanguages, d.Languages...)
	m.AddList(ListDimensions, d.Dimensions...)
}

// apply records everything one documented model states onto a catalog entry.
// Every field is first stated wins, and this is called for the most specific
// name a meter reaches before applyFamily is called for the rest.
func apply(m *catalog.Model, d documented) {
	if m.Name == "" && d.Name != "" && d.Name != nameAmbiguous {
		m.Name = d.Name
	}
	applyFamily(m, d)
	m.SetAttr(AttrSummary, d.Summary)
	m.SetAttr(AttrState, d.State)
	m.SetAttr(AttrPublisher, d.Publisher)
	m.SetAttr(AttrAuthor, d.Author)
	m.SetAttr(AttrLicense, d.License)
	m.SetAttr(AttrVersion, d.Version)
	m.SetAttr(AttrReleaseDate, d.Release)
	m.SetAttr(AttrCatalogAdded, d.Added)
	m.SetAttr(AttrRetirementDate, d.Retire)
	m.SetAttr(AttrReplacement, d.Replacement)
	m.SetAttr(AttrTrainingRetirement, d.TrainRetire)
	m.SetAttr(AttrDeploymentRetirement, d.DeployRetire)
	m.AddList(ListKeywords, d.Keywords...)
	m.AddList(ListTasks, d.Tasks...)
	m.AddList(ListDeployments, d.Deployments...)
	m.AddList(ListRegions, d.Regions...)
}

// meterAliases name the documented model behind a SKU that abbreviates it past
// recognition. A meter reaching its documentation by prefix is the rule; these
// are the meters that drop a word the documentation keeps, and there is no
// shape that recovers it.
var meterAliases = map[string]string{
	"embedding-ada":      "text-embedding-ada-002",
	"embeddings-ada":     "text-embedding-ada-002",
	"computer-use":       "computer-use-preview",
	"gpt4-turbo-128k":    "gpt-4",
	"gpt4-turbo-vision":  "gpt-4",
	"gpt-latest":         "gpt-chat-latest",
	"speech-text-to":     "tts",
	"speech-to-text":     "whisper",
	"gpt-4o-tcrb":        "gpt-4o-transcribe",
	"gpt4o-mn-trscb":     "gpt-4o-mini-transcribe",
	"gpt4o-mn-tts":       "gpt-4o-mini-tts",
	"gpt-aud":            "gpt-audio",
	"gpt-aud-mini":       "gpt-audio-mini",
	"gpt-aud-mn":         "gpt-audio-mini",
	"gpt4omini-aud1217":  "gpt-4o-mini-audio-preview",
	"gpt-4o-aud":         "gpt-4o-audio-preview",
	"gpt-4o-rt":          "gpt-4o-realtime-preview",
	"gpt-rt":             "gpt-realtime",
	"gpt-rt-aud":         "gpt-realtime",
	"gpt-rt-txt":         "gpt-realtime",
	"gpt-rt-img":         "gpt-realtime",
	"gpt-rt-aud-mini":    "gpt-realtime-mini",
	"gpt-rt-txt-mini":    "gpt-realtime-mini",
	"gpt-rt-img-mini":    "gpt-realtime-mini",
	"gpt-rt-aud-mn":      "gpt-realtime-mini",
	"gpt-rt-txt-mn":      "gpt-realtime-mini",
	"gpt-rt-img-mn":      "gpt-realtime-mini",
	"gpt4o-realtime":     "gpt-4o-realtime-preview",
	"gpt4o-realtimeprvw": "gpt-4o-realtime-preview",

	"gpt4o-realtimeprvwaudinp":  "gpt-4o-realtime-preview",
	"gpt4o-realtimeprvwaudoutp": "gpt-4o-realtime-preview",
	"gpt4o-realtimeprvwtxtinp":  "gpt-4o-realtime-preview",
	"gpt4o-realtimeprvwtxtoutp": "gpt-4o-realtime-preview",
	"gpt4o-rtime":               "gpt-4o-realtime-preview",
	"gpt4omini-rt-aud1217":      "gpt-4o-mini-realtime-preview",
	"gpt4omini-rt-txt1217":      "gpt-4o-mini-realtime-preview",
	"gpt-img-1-mini":            "gpt-image-1-mini",
	"gpt-img-1.5":               "gpt-image-1.5",
	"qwen3-32b":                 "qwen-32b",
	"mnstrl-3b":                 "ministral-3b",
	"ministral-3bftregnl":       "ministral-3b",
	"fw-deepseek-v3.2":          "v3.2",
	"fw-deepseekv3.2":           "v3.2",
	"fw-deepseek-v4-pro":        "v4-pro",
	"fw-kimi-k2.5":              "kimi-k2.5",
	"fw-kimi-k2.6":              "kimi-k2.6",
	"fw-kimi-k2.7-code":         "kimi-k2.7-code",
	"fw-gpt-oss-120b":           "gpt-oss-120b",
}

// matchingPrefixes returns every name id equals or extends, longest first, so
// that a meter reaches the most specific model documented and then the family
// it belongs to for whatever that model does not state.
//
// Azure documents the same model in more than one place at more than one
// grain: gpt-4-32k is an entry in the catalog listing, which states its
// publisher, its licence and its lifecycle and no token bound, while the
// concept page states the bound under gpt-4 for the whole family. Reading the
// specific entry first and the family after is what gives the model both.
//
// A meter named in the alias table reaches what that names instead, and is
// matched the same way, since the alias names a family whose meters carry a
// version after it.
func matchingPrefixes(id string, names []string) []string {
	lower := strings.ToLower(id)
	if alias := longestOf(lower, aliasNames); alias != "" {
		lower = meterAliases[alias]
	}
	var out []string
	for _, name := range names {
		if lower == name || strings.HasPrefix(lower, name+"-") {
			out = append(out, name)
		}
	}
	slices.SortFunc(out, func(a, b string) int {
		if len(a) != len(b) {
			return len(b) - len(a)
		}
		return strings.Compare(a, b)
	})
	return out
}

// longestOf returns the longest name that id equals or extends.
func longestOf(id string, names []string) string {
	best := ""
	for _, name := range names {
		if id != name && !strings.HasPrefix(id, name+"-") {
			continue
		}
		if len(name) > len(best) {
			best = name
		}
	}
	return best
}

// aliasNames are the alias table's keys, in the order longestOf wants them.
var aliasNames = slices.Sorted(maps.Keys(meterAliases))

// readDocumentation reads every model table on the documentation page, keyed
// by the identifier in lower case, since the page and the meters disagree on
// capitalization.
//
// A model appears in more than one table, once per release it has had, and the
// first is the current one. Later rows therefore only fill what earlier ones
// left empty.
func readDocumentation(body string) map[string]documented {
	out := map[string]documented{}
	for _, table := range docTableRe.FindAllStringSubmatch(body, -1) {
		rows := docRowRe.FindAllStringSubmatch(table[1], -1)
		if len(rows) < 2 {
			continue
		}
		at := headerColumns(rows[0][1])
		if at[colModelID] < 0 {
			continue
		}
		for _, row := range rows[1:] {
			readRow(out, at, docCellRe.FindAllStringSubmatch(row[1], -1))
		}
	}
	return out
}

// headerColumns locates the columns this parser reads, by the wording of the
// heading rather than by position.
func headerColumns(row string) map[string]int {
	at := map[string]int{
		colModelID:    -1,
		colDescribe:   -1,
		colContext:    -1,
		colMaxOut:     -1,
		colTraining:   -1,
		colRequest:    -1,
		colDimensions: -1,
		colModality:   -1,
	}
	for i, cell := range docCellRe.FindAllStringSubmatch(row, -1) {
		header := strings.ToLower(docText(cell[1]))
		for name := range at {
			if at[name] < 0 && strings.Contains(header, name) {
				at[name] = i
			}
		}
	}
	return at
}

// readRow records one documented model.
//
// The bound the older tables state as a labelled pair is the same column an
// embedding table states one bare count under, because a model returning a
// vector has no output tokens to bound. A bare count there is read as the
// window only where the table also states a vector width, since the image
// tables head the same column with a count of characters.
func readRow(out map[string]documented, at map[string]int, cells [][]string) {
	cell := cellAt(cells, at[colModelID])
	ids := rowIdentifiers(cell)
	if len(ids) == 0 {
		return
	}
	name := rowName(cell)
	request := cellText(cells, at[colRequest])
	bare := int64(0)
	if at[colDimensions] >= 0 {
		bare = parseCount(request)
	}
	context, maxOut := parseSides(request)
	for _, id := range ids {
		d := out[id]
		if d.Name == "" {
			d.Name = name
		}
		if d.Context == 0 {
			d.Context = firstOf(
				parseCount(cellText(cells, at[colContext])),
				context,
				bare,
			)
		}
		if len(d.Dimensions) == 0 {
			d.Dimensions = appendNew(
				d.Dimensions,
				splitCounts(cellText(cells, at[colDimensions]))...,
			)
		}
		if d.MaxOut == 0 {
			d.MaxOut = firstOf(
				parseCount(cellText(cells, at[colMaxOut])),
				maxOut,
			)
		}
		if d.Training == "" {
			d.Training = cellText(cells, at[colTraining])
		}
		readBullets(&d, cellAt(cells, at[colDescribe]))
		readFlow(&d, cellText(cells, at[colModality]))
		out[id] = d
	}
}

// rowIdentifiers reads the models one row documents. Azure writes an
// identifier in code style and the release it documents in plain text beside
// it, and puts more than one in a cell where a table covers a model and its
// mini variant in one row, so the code spans are the models and everything
// else in the cell is not.
func rowIdentifiers(cell string) []string {
	var out []string
	for _, match := range codeRe.FindAllStringSubmatch(cell, -1) {
		if id := strings.ToLower(docText(match[1])); id != "" {
			out = appendNew(out, id)
		}
	}
	if len(out) > 0 {
		return out
	}
	id := strings.ToLower(docText(cell))
	if before, _, ok := strings.Cut(id, "("); ok {
		id = strings.TrimSpace(before)
	}
	if id == "" {
		return nil
	}
	return []string{id}
}

// rowName is the display name an OpenAI table writes beside an identifier.
//
// The identifier is written in code style with its release in parentheses
// after it, and a name, where there is one, is written on a line of its own
// under both. A line holding an identifier is therefore never a name, and
// neither is one holding only the release or the preview marker.
func rowName(cell string) string {
	for _, part := range docBreakRe.Split(cell, -1) {
		if codeRe.MatchString(part) {
			continue
		}
		name := strings.TrimSpace(docText(part))
		if name == "" || rowMarkers[strings.ToLower(name)] {
			continue
		}
		if strings.HasPrefix(name, "(") && strings.HasSuffix(name, ")") {
			continue
		}
		return name
	}
	return ""
}

// rowMarkers are what a line beside an identifier says when it marks the row
// rather than naming the model.
var rowMarkers = map[string]bool{
	"preview": true,
	"ga":      true,
	"new":     true,
}

// readBullets reads the description cell, which is where Azure states what a
// model can do, what it takes and which APIs answer for it.
func readBullets(d *documented, cell string) {
	for _, part := range docBreakRe.Split(cell, -1) {
		bullet := strings.ToLower(strings.Trim(docText(part), " -."))
		if bullet == "" {
			continue
		}
		if names, ok := capabilityFeatures[bullet]; ok {
			d.Features = appendNew(d.Features, names...)
		}
		if endpoint, ok := endpointBullets[bullet]; ok {
			d.Endpoint = appendNew(d.Endpoint, endpoint)
		}
		if flow, ok := capabilityModalities[bullet]; ok {
			d.InputMod = appendNew(d.InputMod, flow.in...)
			d.OutMod = appendNew(d.OutMod, flow.out...)
			continue
		}
		for _, flow := range modalityPrefixes {
			if !strings.HasPrefix(bullet, flow.prefix) {
				continue
			}
			d.InputMod = appendNew(d.InputMod, flow.in...)
			d.OutMod = appendNew(d.OutMod, flow.out...)
			break
		}
	}
}

// readFlow reads a modality cell written as the flow through the model, where
// the halves either side of "to" are what it takes and what it returns.
func readFlow(d *documented, cell string) {
	before, after, ok := strings.Cut(strings.ToLower(cell), " to ")
	if !ok {
		return
	}
	d.InputMod = appendNew(d.InputMod, flowModalities(before)...)
	d.OutMod = appendNew(d.OutMod, flowModalities(after)...)
}

// flowModalities reads the modalities named in one half of a flow. Azure calls
// image input vision there, which is the same modality under another word.
func flowModalities(half string) []string {
	var out []string
	for _, word := range wordRe.FindAllString(half, -1) {
		switch word {
		case "text":
			out = appendNew(out, "text")
		case "vision", "image":
			out = appendNew(out, "image")
		case "audio":
			out = appendNew(out, "audio")
		}
	}
	return out
}

// appendNew adds values not already present.
func appendNew(items []string, values ...string) []string {
	for _, value := range values {
		if !slices.Contains(items, value) {
			items = append(items, value)
		}
	}
	return items
}

// parseSides reads the labelled pair the older tables state both bounds as,
// where the input half is the context window and the output half the ceiling
// on what a request may generate.
func parseSides(cell string) (context, maxOut int64) {
	for _, side := range docSideRe.FindAllStringSubmatch(cell, -1) {
		n, err := strconv.ParseInt(
			strings.ReplaceAll(side[2], ",", ""),
			10,
			64,
		)
		if err != nil {
			continue
		}
		if strings.EqualFold(side[1], "input") {
			context = n
			continue
		}
		maxOut = n
	}
	return context, maxOut
}

// firstOf returns the first bound stated, since a table heads the same fact
// two different ways and only one column of the two is present.
func firstOf(values ...int64) int64 {
	for _, v := range values {
		if v != 0 {
			return v
		}
	}
	return 0
}

// parseCount reads the first grouped number in a cell.
func parseCount(cell string) int64 {
	match := docCountRe.FindStringSubmatch(cell)
	if match == nil {
		return 0
	}
	n, err := strconv.ParseInt(strings.ReplaceAll(match[1], ",", ""), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// cellAt returns the raw markup of one cell, tolerating a short row.
func cellAt(cells [][]string, i int) string {
	if i < 0 || i >= len(cells) {
		return ""
	}
	return cells[i][1]
}

// cellText returns the text of one cell, tolerating a short row.
func cellText(cells [][]string, i int) string {
	return docText(cellAt(cells, i))
}

// docText strips markup, resolves entities and collapses whitespace. The
// entities matter because Azure writes an ampersand in "Functions & tools" and
// a bullet is matched against a closed set of wordings.
func docText(markup string) string {
	return strings.Join(
		strings.Fields(
			html.UnescapeString(docTagRe.ReplaceAllString(markup, " ")),
		),
		" ",
	)
}

// cutoffMonths name the months Azure writes a training cutoff with, in both
// the full and the shortened form its tables use.
var cutoffMonths = map[string]string{
	"january": "01", "jan": "01",
	"february": "02", "feb": "02",
	"march": "03", "mar": "03",
	"april": "04", "apr": "04",
	"may":  "05",
	"june": "06", "jun": "06",
	"july": "07", "jul": "07",
	"august": "08", "aug": "08",
	"september": "09", "sep": "09", "sept": "09",
	"october": "10", "oct": "10",
	"november": "11", "nov": "11",
	"december": "12", "dec": "12",
}

// cutoffRe matches a training cutoff as Azure writes it, which is a month and
// a year with the day between them where the table states one: "October 2023",
// "May 31, 2024", "Sep 2021".
var cutoffRe = regexp.MustCompile(
	`(?i)^([a-z]+)\s+(?:(\d{1,2}),\s*)?(\d{4})$`,
)

// isoMonth restates a training cutoff as an ISO date, to the precision Azure
// wrote it at and never further. A cutoff written any other way is not
// restated, since guessing at its precision is what the catalog forbids.
func isoMonth(cutoff string) string {
	match := cutoffRe.FindStringSubmatch(strings.TrimSpace(cutoff))
	if match == nil {
		return ""
	}
	month, ok := cutoffMonths[strings.ToLower(match[1])]
	if !ok {
		return ""
	}
	if match[2] == "" {
		return match[3] + "-" + month
	}
	day := match[2]
	if len(day) == 1 {
		day = "0" + day
	}
	return match[3] + "-" + month + "-" + day
}
