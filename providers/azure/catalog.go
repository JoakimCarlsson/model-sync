package azure

import (
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

// Numeric keys the documentation populates.
const (
	LimitContextWindow   = "context_window"
	LimitMaxOutputTokens = "max_output_tokens"
)

// Enumeration keys the documentation populates.
const (
	ListFeatures         = "features"
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
	// stating both in one cell as a labelled pair.
	colRequest = "max request"
)

// capabilityFeatures map one of the documentation's capability bullets onto
// the features it states. Azure writes several capabilities into one bullet,
// so a bullet yields a list rather than a single name.
var capabilityFeatures = map[string][]string{
	"reasoning":                 {"reasoning"},
	"structured outputs":        {"structured_outputs"},
	"json mode":                 {"json_mode"},
	"streaming":                 {"streaming"},
	"computer use":              {"computer_use"},
	"function calling":          {"function_calling"},
	"functions and tools":       {"function_calling"},
	"parallel function calling": {"function_calling", "parallel_tool_calls"},
	"functions, tools, and parallel tool calling": {
		"function_calling",
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
	"audio model for real-time audio processing": {
		[]string{"audio", "text"},
		[]string{"audio", "text"},
	},
	"audio model for audio and text generation": {
		[]string{"audio", "text"},
		[]string{"audio", "text"},
	},
}

// endpointBullets are the bullets naming an API a model answers on rather than
// something it can do.
var endpointBullets = map[string]string{
	"chat completions api": "Chat Completions",
	"responses api":        "Responses",
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
	Context  int64
	MaxOut   int64
	Training string
	Features []string
	Endpoint []string
	InputMod []string
	OutMod   []string
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
func (b *builder) applyCatalog(doc catalog.Document) {
	docs := readDocumentation(string(doc.Body))
	names := slices.Sorted(maps.Keys(docs))
	for _, id := range b.order {
		name := longestPrefix(id, names)
		if name == "" {
			b.applySKUWindow(b.models[id])
			continue
		}
		b.models[id].AddSource(doc.URL)
		apply(b.models[id], docs[name])
		b.applySKUWindow(b.models[id])
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
	m.SetLimit(LimitContextWindow, d.Context)
	m.SetLimit(LimitMaxOutputTokens, d.MaxOut)
	m.SetAttr(AttrTrainingCutoff, d.Training)
	m.AddList(ListFeatures, d.Features...)
	m.AddList(ListEndpoints, d.Endpoint...)
	m.AddList(ListInputModalities, d.InputMod...)
	m.AddList(ListOutputModalities, d.OutMod...)
}

// longestPrefix returns the longest name that id equals or extends, so that a
// meter reaches the most specific model documented rather than the first.
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
		colModelID:  -1,
		colDescribe: -1,
		colContext:  -1,
		colMaxOut:   -1,
		colTraining: -1,
		colRequest:  -1,
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
func readRow(out map[string]documented, at map[string]int, cells [][]string) {
	id := strings.ToLower(cellText(cells, at[colModelID]))
	if before, _, ok := strings.Cut(id, "("); ok {
		id = strings.TrimSpace(before)
	}
	if id == "" {
		return
	}
	d := out[id]
	context, maxOut := parseSides(cellText(cells, at[colRequest]))
	if d.Context == 0 {
		d.Context = firstOf(
			parseCount(cellText(cells, at[colContext])),
			context,
		)
	}
	if d.MaxOut == 0 {
		d.MaxOut = firstOf(parseCount(cellText(cells, at[colMaxOut])), maxOut)
	}
	if d.Training == "" {
		d.Training = cellText(cells, at[colTraining])
	}
	readBullets(&d, cellAt(cells, at[colDescribe]))
	out[id] = d
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
		}
	}
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

// docText strips markup and collapses whitespace.
func docText(html string) string {
	return strings.Join(
		strings.Fields(docTagRe.ReplaceAllString(html, " ")),
		" ",
	)
}
