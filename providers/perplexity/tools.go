package perplexity

import (
	"regexp"
	"slices"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// AgentToolsURL indexes the tools an Agent API run can call. The pricing page
// creates the tool products and gives each a rate; this page is what says what
// each is called in a request and where its own reference lives.
const AgentToolsURL = baseURL + "/docs/agent-api/tools/overview.md"

// toolHrefRe matches the link from the tool index to one tool's reference.
var toolHrefRe = regexp.MustCompile(`/docs/agent-api/tools/([a-z0-9-]+)`)

// toolPagePre prefixes the page each built-in tool has of its own.
const toolPagePre = baseURL + "/docs/agent-api/tools/"

// applyToolIndex reads the index of built-in tools.
//
// The pricing page names a tool by the string a request enables it with, so
// the identifier is already the type; what the index adds is the type stated
// as such, and the address of the reference page that then states the tool's
// parameters. A row naming a tool the pricing page did not price is skipped,
// because the two documents disagree about nothing else and a tool with no
// rate is one this catalog has no entry for.
func (b *builder) applyToolIndex(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		at := columnOf(t.Headers, "type")
		if at < 0 {
			continue
		}
		for _, row := range t.Rows {
			kind := clean(cellAt(row, at))
			m, ok := b.models[slugID(kind)]
			if !ok {
				continue
			}
			m.AddSource(doc.URL)
			m.SetAttr(AttrAPIType, kind)
			b.linkToolPage(m.ID, cellAt(row, 0))
		}
	}
}

// linkToolPage records which tool a reference page is about, which is the only
// thing tying the two together: a tool's page is addressed by its title rather
// than by the type a request enables it with.
func (b *builder) linkToolPage(id, cell string) {
	match := toolHrefRe.FindStringSubmatch(cell)
	if match == nil {
		return
	}
	if b.toolPages == nil {
		b.toolPages = map[string]string{}
	}
	b.toolPages[toolPagePre+match[1]+".md"] = id
}

// toolPageURLs returns the tool reference pages an index links to.
func toolPageURLs(index catalog.Document) []string {
	var urls []string
	for _, match := range toolHrefRe.FindAllStringSubmatch(
		string(index.Body),
		-1,
	) {
		url := toolPagePre + match[1] + ".md"
		if !slices.Contains(urls, url) {
			urls = append(urls, url)
		}
	}
	return urls
}

// applyToolPage reads one tool's reference, which states the settings a
// request may give the tool as a table of parameters.
func (b *builder) applyToolPage(doc catalog.Document) {
	id, ok := b.toolPages[doc.URL]
	if !ok {
		return
	}
	m, ok := b.models[id]
	if !ok {
		return
	}
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		if columnOf(t.Headers, "parameter") != 0 {
			continue
		}
		m.AddSource(doc.URL)
		for _, row := range t.Rows {
			name := clean(cellAt(row, 0))
			if name == "" || strings.Contains(name, " ") {
				continue
			}
			m.AddList(ListParameters, name)
		}
	}
}
