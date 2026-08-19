package deepseek

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

var (
	// frontMatterRe matches the metadata block a Hugging Face card opens with.
	frontMatterRe = regexp.MustCompile(`(?s)\A---\r?\n(.*?)\r?\n---`)
	// licenseRe matches the licence declared in that block.
	licenseRe = regexp.MustCompile(`(?mi)^license:\s*(\S+)`)
	// reportRe matches the link to the technical report.
	reportRe = regexp.MustCompile(
		`(?is)<a href="([^"]+)"[^>]*>\s*<b>\s*Technical Report`,
	)
	// downloadRe matches the Hugging Face repository in a download cell.
	downloadRe = regexp.MustCompile(
		`\[HuggingFace\]\(https://huggingface\.co/([^)\s]+)\)`,
	)
)

// The columns of the card's download table, in the order it writes them.
const (
	cardName       = 1
	cardTotal      = 2
	cardActivated  = 3
	cardPrecision  = 5
	cardDownload   = 6
	cardCellCount  = 7
	huggingFaceURL = "https://huggingface.co/"
)

// escapedPipe is how the card writes a pipe inside a cell, and pipeHolder
// stands in for it while the row is split on the pipes that are not escaped.
const (
	escapedPipe = `\|`
	pipeHolder  = "\x00"
)

// applyModelCard reads the weights DeepSeek publishes for a model.
//
// The API documentation never says that a model is open, under what licence,
// or where the weights are. The card does, and it is one card written for the
// series and served from each repository in it, so it names every model of the
// series in a download table: a row per repository with the parameter count,
// the context length, the precision the weights are released in and the link
// to the repository itself.
//
// The licence is the one thing the card states about its own repository rather
// than about the series, since it is declared in the front matter and the
// front matter belongs to the repository the card was fetched from. It is
// therefore recorded only against the model that repository holds, which is
// why both cards are fetched even though their bodies are the same document.
func (b *builder) applyModelCard(doc catalog.Document, id string) {
	body := string(doc.Body)
	if m, ok := b.models[id]; ok {
		if match := frontMatterRe.FindStringSubmatch(body); match != nil {
			if license := licenseRe.FindStringSubmatch(
				match[1],
			); license != nil {
				m.SetAttr(AttrLicense, license[1])
				m.AddSource(doc.URL)
			}
		}
	}
	report := ""
	if match := reportRe.FindStringSubmatch(body); match != nil {
		report = match[1]
	}
	for _, line := range strings.Split(body, "\n") {
		b.applyCardRow(doc.URL, line, report)
	}
}

// applyCardRow records one row of the card's download table.
//
// A row naming something other than a model DeepSeek serves is skipped, which
// is what keeps the base checkpoints, whose names carry a suffix no served
// model has, and the header and rule rows out of the catalog.
func (b *builder) applyCardRow(source, line, report string) {
	cells := cardCells(line)
	if len(cells) < cardCellCount {
		return
	}
	m, ok := b.models[strings.ToLower(cells[cardName])]
	if !ok {
		return
	}
	repo := downloadRe.FindStringSubmatch(cells[cardDownload])
	if repo == nil {
		return
	}
	m.SetAttr(AttrOpenWeights, "true")
	m.SetAttr(AttrHuggingFaceID, repo[1])
	m.SetAttr(AttrModelCardURL, huggingFaceURL+repo[1])
	m.SetAttr(AttrTotalParameters, cells[cardTotal])
	m.SetAttr(AttrActivatedParameters, cells[cardActivated])
	m.SetAttr(AttrQuantization, strings.TrimSuffix(cells[cardPrecision], "*"))
	m.SetAttr(AttrTechnicalReportURL, report)
	m.AddSource(source)
}

// cardCells splits one markdown table row into its cells, keeping the pipes
// the card escapes inside a cell out of the split.
func cardCells(line string) []string {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") {
		return nil
	}
	held := strings.ReplaceAll(trimmed, escapedPipe, pipeHolder)
	cells := strings.Split(held, "|")
	for i, cell := range cells {
		cells[i] = strings.TrimSpace(
			strings.ReplaceAll(cell, pipeHolder, "|"),
		)
	}
	return cells
}
