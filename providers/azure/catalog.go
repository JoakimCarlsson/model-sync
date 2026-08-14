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
	Source     string
	Name       string
	Context    int64
	MaxOut     int64
	Training   string
	Features   []string
	Endpoint   []string
	InputMod   []string
	OutMod     []string
	Languages  []string
	Dimensions []string
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
		body := string(page.Body)
		mergeDocumented(docs, readDocumentation(body), page.URL)
		mergeDocumented(docs, readCollections(body), page.URL)
		mergeDocumented(docs, readAspects(body), page.URL)
	}
	names := slices.Sorted(maps.Keys(docs))
	for _, id := range b.order {
		b.applySKUWindow(b.models[id])
		name := longestPrefix(id, names)
		if name == "" {
			continue
		}
		b.models[id].AddSource(docs[name].Source)
		apply(b.models[id], docs[name])
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
		d, held := into[id]
		if !held {
			found.Source = source
			into[id] = found
			continue
		}
		if d.Name == "" {
			d.Name = found.Name
		}
		if d.Context == 0 {
			d.Context = found.Context
		}
		if d.MaxOut == 0 {
			d.MaxOut = found.MaxOut
		}
		if d.Training == "" {
			d.Training = found.Training
		}
		d.Features = appendNew(d.Features, found.Features...)
		d.Endpoint = appendNew(d.Endpoint, found.Endpoint...)
		d.InputMod = appendNew(d.InputMod, found.InputMod...)
		d.OutMod = appendNew(d.OutMod, found.OutMod...)
		d.Languages = appendNew(d.Languages, found.Languages...)
		d.Dimensions = appendNew(d.Dimensions, found.Dimensions...)
		into[id] = d
	}
}

// applySKUWindow reads a context window out of a meter's own name.
//
// Azure's documentation covers the models it sells today. The price list also
// carries meters for models it has stopped documenting, and those name their
// window in the meter: gpt-4-32k is the 32,000 token deployment. That is Azure
// stating the window, in the only place it still states it.
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

// apply records one documented model onto a catalog entry.
func apply(m *catalog.Model, d documented) {
	if d.Name != "" && d.Name != nameAmbiguous {
		m.Name = d.Name
	}
	m.SetLimit(LimitContextWindow, d.Context)
	m.SetLimit(LimitMaxOutputTokens, d.MaxOut)
	m.SetAttr(AttrTrainingCutoff, d.Training)
	m.AddList(ListFeatures, d.Features...)
	m.AddList(ListEndpoints, d.Endpoint...)
	m.AddList(ListInputModalities, d.InputMod...)
	m.AddList(ListOutputModalities, d.OutMod...)
	m.AddList(ListLanguages, d.Languages...)
	m.AddList(ListDimensions, d.Dimensions...)
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

// longestPrefix returns the longest name that id equals or extends, so that a
// meter reaches the most specific model documented rather than the first. A
// meter named in the alias table reaches what that names instead, and is
// matched the same way, since the alias names a family whose meters carry a
// version after it.
func longestPrefix(id string, names []string) string {
	lower := strings.ToLower(id)
	if alias := longestOf(lower, aliasNames); alias != "" {
		return meterAliases[alias]
	}
	return longestOf(lower, names)
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
