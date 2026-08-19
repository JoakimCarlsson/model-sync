package anthropic

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// ToolReferenceURL is the directory of the tools Anthropic provides. The
// pricing page names a server tool only where it charges for one and says
// nothing else about it; this is the page stating which tools exist, which
// versions of each are current, whether Anthropic or the caller executes it,
// and whether it is generally available.
const ToolReferenceURL = baseURL +
	"/agents-and-tools/tool-use/tool-reference.md"

// Keys the tool directory populates.
const (
	AttrExecution     = "execution"
	AttrReleaseStatus = "release_status"
)

// statusBeta is what the directory's Status column says of a tool that needs a
// header, written with the headers after it.
const statusBeta = "beta"

// stateBeta and stateActive are the lifecycle the directory's Status column
// states, in the vocabulary the model status table already uses. GA and Active
// are the same fact said of a tool and of a model, and a consumer filtering on
// one should not have to know both words.
const (
	stateBeta   = "beta"
	stateActive = "active"
)

var (
	// toolTypeRe matches one of the type identifiers a directory row lists.
	toolTypeRe = regexp.MustCompile("`([a-z0-9_]+)`")
	// toolDateRe matches the release date appended to a tool type, which tells
	// two releases of one tool apart rather than two tools.
	toolDateRe = regexp.MustCompile(`_\d{8}$`)
	// betaNameRe matches one beta header named in the Status column, which is
	// how a tool that is not generally available states its condition.
	betaNameRe = regexp.MustCompile(
		"`([a-z0-9-]+-[0-9]{4}-[0-9]{2}-[0-9]{2})`",
	)
)

// applyToolReference records the tools Anthropic provides.
//
// Every row is a tool, whether or not a rate names it. A row states a type to
// send, a side that executes it, a lifecycle, and the versions still accepted,
// which is four published facts about a thing a caller can use; that most of
// these tools carry no charge of their own says something about their pricing
// and nothing about whether they exist. Creating only the priced ones made the
// pricing page the register of what Anthropic offers, which it is not: it is
// the register of what Anthropic bills for, and web fetch is in the directory,
// is documented at length, and is explicitly free.
//
// The directory names a tool by its page title, "Web search tool", which is
// neither the identifier its rate is filed under nor anything that slugs onto
// one. Its type column is, once the release date is dropped:
// web_search_20260318 becomes web-search. So the title becomes the name and the
// type becomes the identifier.
func (b *builder) applyToolReference(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		types, execution, status :=
			columnOf(t, "type"),
			columnOf(t, "execution"),
			columnOf(t, "status")
		if types < 0 || execution < 0 || status < 0 {
			continue
		}
		for _, row := range t.Rows {
			b.applyToolRow(t, row, types, execution, status)
		}
	}
}

// applyToolRow records one directory row against the tool it names.
func (b *builder) applyToolRow(
	t mdTable,
	row []string,
	types, execution, status int,
) {
	versions := toolTypeRe.FindAllStringSubmatch(cellAt(row, types), -1)
	if len(versions) == 0 {
		return
	}
	m := b.model(toolID(versions[0][1]), KindTool)
	m.AddSource(t.Source)
	if m.Name == "" {
		m.Name = clean(cellAt(row, 0))
	}
	m.SetAttr(
		AttrExecution,
		strings.ToLower(clean(cellAt(row, execution))),
	)
	b.applyToolStatus(m, cellAt(row, status))
	for _, version := range versions {
		m.AddList(ListVersions, version[1])
	}
}

// applyToolStatus records what the Status column says, which is either that
// the tool is generally available or that it is in beta behind one or more
// headers. The column's own word is kept alongside the lifecycle because it
// says which release channel a tool is on, and the headers are kept as a list
// because Anthropic keeps an older one working next to a new one.
func (b *builder) applyToolStatus(m *catalog.Model, cell string) {
	text := clean(cell)
	label, _, _ := strings.Cut(text, ":")
	m.SetAttr(AttrReleaseStatus, strings.TrimSpace(label))
	if !strings.EqualFold(strings.TrimSpace(label), statusBeta) {
		m.SetAttr(AttrState, stateActive)
		return
	}
	m.SetAttr(AttrState, stateBeta)
	for _, match := range betaNameRe.FindAllStringSubmatch(cell, -1) {
		m.AddList(ListBetaHeaders, match[1])
	}
}

// toolID reduces a type identifier to the tool it names, dropping the release
// date only some of them carry.
func toolID(toolType string) string {
	return strings.ReplaceAll(
		toolDateRe.ReplaceAllString(toolType, ""),
		"_",
		"-",
	)
}
