package anthropic

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// ToolReferenceURL is the directory of the tools Anthropic provides. The
// pricing page names a server tool only where it charges for one and says
// nothing else about it; this is the page stating which versions of a tool are
// current, whether Anthropic or the caller executes it, and whether it is
// generally available.
const ToolReferenceURL = baseURL +
	"/agents-and-tools/tool-use/tool-reference.md"

// Keys the tool directory populates.
const (
	AttrExecution     = "execution"
	AttrReleaseStatus = "release_status"
)

var (
	// toolTypeRe matches one of the type identifiers a directory row lists.
	toolTypeRe = regexp.MustCompile("`([a-z0-9_]+)`")
	// toolDateRe matches the release date appended to a tool type, which tells
	// two releases of one tool apart rather than two tools.
	toolDateRe = regexp.MustCompile(`_\d{8}$`)
)

// applyToolReference records what the directory states about the tools the
// pricing page established.
//
// The directory names a tool by its page title, "Web search tool", which is
// neither the identifier its rate is filed under nor anything that slugs onto
// one. Its type column is, once the release date is dropped: web_search_20260318
// becomes web-search. A tool the directory lists and no rate names is not
// created here, because most of the directory is tools that carry no charge of
// their own and so have nothing to bill; the page states their versions, not
// that they are priced.
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
	m, ok := b.models[toolID(versions[0][1])]
	if !ok {
		return
	}
	m.AddSource(t.Source)
	m.SetAttr(
		AttrExecution,
		strings.ToLower(clean(cellAt(row, execution))),
	)
	m.SetAttr(AttrReleaseStatus, clean(cellAt(row, status)))
	for _, version := range versions {
		m.AddList(ListVersions, version[1])
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
