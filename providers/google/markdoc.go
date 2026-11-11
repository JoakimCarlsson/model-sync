package google

import (
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Markdoc expressions recognize the structural tags and Markdown emitted by AI Studio.
var (
	markTableRe       = regexp.MustCompile(`(?s)\{% table\b([^%]*)%\}(.*?)\{% /table %\}`)
	markTagRe         = regexp.MustCompile(`\{%\s*(/)?([\w-]+)\b(.*?)%\}`)
	markAttrRe        = regexp.MustCompile(`([\w-]+)="([^"]*)"`)
	markHeadingRe     = regexp.MustCompile(`(?m)^\s*(#{1,6})\s+(.+)$`)
	markLinkRe        = regexp.MustCompile(`\[([^\]\n]*)\]\(([^\s)]+)\)`)
	markCodeRe        = regexp.MustCompile("`([^`\n]+)`")
	markBoldRe        = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	markItemRe        = regexp.MustCompile(`^(\s*)\*\s?(.*)$`)
	markFieldRe       = regexp.MustCompile(`(?s)<b>(.*?)</b>`)
	markCardRe        = regexp.MustCompile(`(?s)\{% card\b([^%]*)%\}`)
	markPipeDividerRe = regexp.MustCompile(`^\s*\|?\s*:?-+:?\s*(?:\|\s*:?-+:?\s*)+\|?\s*$`)
	markFenceRe       = regexp.MustCompile("(?ms)^\\s*```[^\\n]*\\n.*?^\\s*```[^\\n]*$")
	markBadgeLabelRe  = regexp.MustCompile(`\blabel=(.*?)(?:\s+[\w-]+=|\s*/?$)`)
)

// canonicalURL maps old documentation addresses to AI Studio while retaining external sources.
func canonicalURL(value string) string {
	value = strings.Replace(value, "https://ai.google.dev/gemini-api/docs", baseURL+"/docs", 1)
	return strings.TrimRight(value, "/")
}

// normalizeDocument adapts Markdoc to the semantic HTML consumed by the existing parsers.
// It leaves legacy HTML intact and retains the original document outside this parsing copy.
func normalizeDocument(doc catalog.Document) catalog.Document {
	doc.URL = canonicalURL(doc.URL)
	body := string(doc.Body)
	if strings.Contains(body, "{%") || strings.HasPrefix(strings.TrimSpace(body), "#") || strings.HasPrefix(strings.TrimSpace(body), "> ## AI Agent Context") {
		doc.Body = []byte(markdocHTML(body, doc.URL))
	}
	return doc
}

// markAttrs returns the quoted attributes of a Markdoc tag.
func markAttrs(value string) map[string]string {
	attrs := map[string]string{}
	for _, match := range markAttrRe.FindAllStringSubmatch(value, -1) {
		attrs[match[1]] = html.UnescapeString(match[2])
	}
	return attrs
}

// markInline renders inline Markdown without discarding links, identifiers or pricing qualifiers.
func markInline(value string) string {
	value = strings.ReplaceAll(value, "<=", "&lt;=")
	value = markCodeRe.ReplaceAllString(value, `<code>$1</code>`)
	value = markLinkRe.ReplaceAllString(value, `<a href="$2">$1</a>`)
	value = markBoldRe.ReplaceAllString(value, `<b>$1</b>`)
	return value
}

// markTags translates semantic Markdoc tags and drops layout-only wrappers.
func markTags(value string) string {
	return markTagRe.ReplaceAllStringFunc(value, func(tag string) string {
		parts := markTagRe.FindStringSubmatch(tag)
		attrs := markAttrs(parts[3])
		closing := parts[1] != ""
		switch parts[2] {
		case "sup":
			if closing {
				return "</sup>"
			}
			return "<sup>"
		case "badge":
			label := attrs["label"]
			if label == "" {
				if match := markBadgeLabelRe.FindStringSubmatch(strings.TrimSpace(parts[3])); match != nil {
					label = strings.Trim(match[1], `"`)
				}
			}
			if label == "" {
				label = strings.ReplaceAll(attrs["type"], "-", " ")
			}
			return label
		case "tab":
			if !closing {
				return "<h3>" + html.EscapeString(attrs["title"]) + "</h3>"
			}
		case "button":
			if closing {
				return "</a>"
			}
			if strings.HasSuffix(strings.TrimSpace(parts[3]), "/") {
				return `<a href="` + html.EscapeString(attrs["href"]) + `"></a>`
			}
			return `<a href="` + html.EscapeString(attrs["href"]) + `">`
		case "section":
			return "\n\n"
		case "hr":
			return "<hr>"
		case "br":
			return "<br>"
		}
		return ""
	})
}

// markdocHTML renders the documented table, heading, card and field structures into parser input.
func markdocHTML(body, url string) string {
	summary := markSummary(body)
	body = markFenceRe.ReplaceAllString(body, "")
	body = markTableRe.ReplaceAllStringFunc(body, func(block string) string {
		parts := markTableRe.FindStringSubmatch(block)
		class := "pricing-table"
		if strings.HasPrefix(url, modelPagePre) || url != ModelsURL && markAttrs(parts[1])["variant"] == "model" {
			class = "gemini-api-model-table"
		}
		return markTable(parts[2], class)
	})
	body = markPipeTables(body)
	cards := ""
	if url == ModelsURL {
		for _, card := range markCardRe.FindAllStringSubmatch(body, -1) {
			attrs := markAttrs(card[1])
			if attrs["href"] == "" {
				continue
			}
			cards += `<a href="` + html.EscapeString(attrs["href"]) + `" class="gemini-card-centered"><p class="status-subtext">` + html.EscapeString(attrs["badge"]) + `</p></a>`
		}
	}
	body = markTags(body)
	body = markInline(body)
	body = markHeadingRe.ReplaceAllStringFunc(body, func(line string) string {
		parts := markHeadingRe.FindStringSubmatch(line)
		level := strconv.Itoa(len(parts[1]))
		class := ""
		if level == "1" {
			class = ` class="devsite-page-title"`
		}
		return "<h" + level + class + ">" + strings.TrimSpace(parts[2]) + "</h" + level + ">"
	})
	if url == PricingURL {
		body = markPricingGroups(body)
	}
	return cards + "<p>" + html.EscapeString(summary) + "</p>" + body
}

// markSummary returns the introductory paragraph following a Markdown page title.
func markSummary(body string) string {
	inTitle := false
	var lines []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			inTitle = true
			continue
		}
		if !inTitle {
			continue
		}
		if line == "" {
			if len(lines) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "{%") {
			break
		}
		lines = append(lines, line)
	}
	return text(markInline(strings.Join(lines, " ")))
}

// markTable converts list-delimited Markdoc rows, respecting nested lists inside cells.
func markTable(body, class string) string {
	lines := strings.Split(body, "\n")
	indent := -1
	for _, line := range lines {
		if item := markItemRe.FindStringSubmatch(line); item != nil {
			if indent < 0 || len(item[1]) < indent {
				indent = len(item[1])
			}
		}
	}
	var rows [][]string
	var cells []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "---" {
			if len(cells) > 0 {
				rows = append(rows, cells)
				cells = nil
			}
			continue
		}
		if item := markItemRe.FindStringSubmatch(line); item != nil && len(item[1]) == indent {
			cells = append(cells, item[2])
		} else if len(cells) > 0 {
			cells[len(cells)-1] += "\n" + line
		}
	}
	if len(cells) > 0 {
		rows = append(rows, cells)
	}
	var out strings.Builder
	out.WriteString(`<table class="` + class + `">`)
	for i, row := range rows {
		tag := "td"
		if i == 0 {
			tag = "th"
			out.WriteString("<thead>")
		}
		out.WriteString("<tr>")
		for _, cell := range row {
			out.WriteString("<" + tag + ">" + markCell(cell, class) + "</" + tag + ">")
		}
		out.WriteString("</tr>")
		if i == 0 {
			out.WriteString("</thead>")
		}
	}
	out.WriteString("</table>")
	return out.String()
}

// markCell renders a table cell and the model properties expressed as bold field names.
func markCell(cell, class string) string {
	cell = markInline(markTags(cell))
	if class != "gemini-api-model-table" {
		return cell
	}
	fields := markFieldRe.FindAllStringSubmatchIndex(cell, -1)
	if len(fields) > 0 {
		var rendered strings.Builder
		for i, field := range fields {
			end := len(cell)
			if i+1 < len(fields) {
				end = fields[i+1][0]
			}
			rendered.WriteString("<section><p><b>" + cell[field[2]:field[3]] + "</b></p><p>" + strings.TrimSpace(cell[field[1]:end]) + "</p></section>")
		}
		cell = rendered.String()
	}
	var out []string
	for _, line := range strings.Split(cell, "\n") {
		if item := markItemRe.FindStringSubmatch(line); item != nil {
			line = "<li>" + strings.TrimSpace(item[2]) + "</li>"
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// markPipeTables renders ordinary Markdown tables used alongside Markdoc guide tables.
func markPipeTables(body string) string {
	lines := strings.Split(body, "\n")
	var out []string
	for i := 0; i < len(lines); i++ {
		if i+1 >= len(lines) || !strings.Contains(lines[i], "|") || !markPipeDividerRe.MatchString(lines[i+1]) {
			out = append(out, lines[i])
			continue
		}
		var table strings.Builder
		table.WriteString(`<table class="pricing-table">`)
		header := true
		for j := i; j < len(lines) && strings.Contains(lines[j], "|"); j++ {
			i = j
			if markPipeDividerRe.MatchString(lines[j]) {
				continue
			}
			tag := "td"
			if header {
				tag = "th"
				table.WriteString("<thead>")
			}
			table.WriteString("<tr>")
			for _, cell := range strings.Split(strings.Trim(strings.TrimSpace(lines[j]), "|"), "|") {
				table.WriteString("<" + tag + ">" + markInline(markTags(strings.TrimSpace(cell))) + "</" + tag + ">")
			}
			table.WriteString("</tr>")
			if header {
				table.WriteString("</thead>")
				header = false
			}
		}
		table.WriteString("</table>")
		out = append(out, table.String())
	}
	return strings.Join(out, "\n")
}

// markPricingGroups attaches the endpoint introduction to each model heading, excluding later prose.
func markPricingGroups(body string) string {
	headings := headingRe.FindAllStringIndex(body, -1)
	var out strings.Builder
	end := 0
	for i, at := range headings {
		heading := body[at[0]:at[1]]
		if !modelHeadingRe.MatchString(text(first(headingRe, heading))) {
			continue
		}
		stop := len(body)
		if i+1 < len(headings) {
			stop = headings[i+1][0]
		}
		intro := body[at[1]:stop]
		if table := strings.Index(intro, "<table"); table >= 0 {
			intro = intro[:table]
		}
		if boundary := strings.Index(intro, "<hr"); boundary >= 0 {
			intro = intro[:boundary]
		}
		out.WriteString(body[end:at[0]])
		out.WriteString(`<div class="heading-group">` + heading + intro + `</div>`)
		end = at[1] + len(intro)
	}
	out.WriteString(body[end:])
	return out.String()
}
