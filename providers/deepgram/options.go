package deepgram

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Keys the models and languages overview populates.
const (
	// ListLanguages holds the language codes a model option accepts, as the
	// overview writes them: BCP-47 tags, plus the value "multi" where the
	// option follows a speaker who changes language.
	ListLanguages = "languages"
	// ListVariants holds the other model options Deepgram lists under the
	// same heading, which are the same generation of model tuned for another
	// domain.
	ListVariants = "variants"
	// ListAliases holds the other strings a request may name a model option
	// by, which the overview writes as "X or Y" in one row.
	ListAliases = "aliases"
	// AttrFamily is the heading a model option is listed under, which is what
	// the concurrency reference and the feature overviews key on: neither
	// states a limit or a capability per option.
	AttrFamily = "family"
	// AttrState is where a model stands, which the overview says by listing
	// three families as general models and the rest under a legacy heading.
	AttrState = "state"
	// AttrCustomOption is the model string an enterprise customer names its
	// own trained model with. The overview writes it as a placeholder rather
	// than as a model, so it is kept as a fact about the family instead of
	// becoming a model of its own.
	AttrCustomOption = "custom_model_option"
)

// States the overview distinguishes.
const (
	StateActive = "active"
	StateLegacy = "legacy"
)

var (
	// mdHeadingRe matches a markdown heading and its level.
	mdHeadingRe = regexp.MustCompile(`(?m)^(#{2,4})\s+(.+?)\s*$`)
	// codeRe matches a backticked string, which is how every Deepgram
	// document writes a model option and a language code.
	codeRe = regexp.MustCompile("`([^`]+)`")
	// optionSplitRe matches how the overview writes two names for one option,
	// "`nova-3` or `nova-3-general`", in either case it uses.
	optionSplitRe = regexp.MustCompile(`(?i)\s+or\s+`)
	// sentenceRe matches the first sentence of a paragraph, which is what a
	// family section opens with where no table describes it.
	sentenceRe = regexp.MustCompile(`^(.+?\.)(?:\s|$)`)
)

// legacyHeading is the heading the overview groups its older models under,
// which is the only place it says that they are older.
const legacyHeading = "legacy models"

// generalHeading is the first column of the table listing the models Deepgram
// recommends, which is how it separates those from the legacy ones.
const generalHeading = "general models"

// applyOptions reads the models and languages overview, which is the only
// document naming the model options a request may ask for and the only one
// stating which languages each of them accepts.
func (b *builder) applyOptions(doc catalog.Document) {
	body := string(doc.Body)
	summaries := familySummaries(body)
	for _, s := range mdSections(body) {
		if s.legacy && s.level < 3 {
			continue
		}
		b.applyFamily(s, summaries, doc.URL)
	}
}

// applyFamily records every model option one family section lists.
func (b *builder) applyFamily(
	s mdSection,
	summaries map[string]string,
	source string,
) {
	rows := optionRows(s.body)
	if len(rows) == 0 {
		return
	}
	family := slugID(s.heading)
	options := make([]string, 0, len(rows))
	for _, r := range rows {
		options = append(options, r.id)
	}
	for _, r := range rows {
		m := b.model(r.id, KindTranscription)
		m.AddSource(source)
		if m.Name == "" {
			m.Name = r.id
		}
		m.SetAttr(AttrFamily, family)
		m.SetAttr(AttrSummary, summaries[family])
		m.SetAttr(AttrState, state(s))
		m.SetAttr(AttrCustomOption, customOption(s.body))
		m.AddList(ListInputModalities, ModalityAudio)
		m.AddList(ListOutputModalities, ModalityText)
		m.AddList(ListLanguages, r.languages...)
		m.AddList(ListAliases, r.aliases...)
		for _, other := range options {
			if other != r.id {
				m.AddList(ListVariants, other)
			}
		}
	}
}

// state reports where a section's models stand. The overview lists three
// families as general models and puts the rest under a legacy heading; a
// family under neither, which is Whisper Cloud, is left unsaid rather than
// assumed to be either.
func state(s mdSection) string {
	switch {
	case s.legacy:
		return StateLegacy
	case s.general:
		return StateActive
	}
	return ""
}

// optionRow is one row of a family's table: a model option, the other names it
// answers to, and the languages it accepts.
type optionRow struct {
	id        string
	aliases   []string
	languages []string
}

// optionRows reads the model options a family section lists. A row whose name
// is a placeholder for a customer's own model is not an option anybody can
// request and is left out.
func optionRows(body string) []optionRow {
	var out []optionRow
	for _, match := range tableRowRe.FindAllStringSubmatch(body, -1) {
		cells := strings.Split(match[1], "|")
		if len(cells) < 2 {
			continue
		}
		names := optionNames(cells[0])
		if len(names) == 0 || placeholder(cells[0]) {
			continue
		}
		out = append(out, optionRow{
			id:        names[0],
			aliases:   names[1:],
			languages: codes(cells[1]),
		})
	}
	return out
}

// optionNames reads the model strings a row names, the first of which is the
// option and the rest of which are other spellings of it.
func optionNames(cell string) []string {
	var out []string
	for _, part := range optionSplitRe.Split(cell, -1) {
		for _, match := range codeRe.FindAllStringSubmatch(part, -1) {
			out = append(out, strings.TrimSpace(match[1]))
			break
		}
	}
	return out
}

// placeholder reports whether a row stands for a customer's own trained model
// rather than for a model Deepgram serves, which it writes in angle brackets.
func placeholder(cell string) bool {
	return strings.Contains(cell, "<CUSTOM>")
}

// customOption returns the placeholder a family writes for a customer's own
// model, so that the fact survives without becoming a model.
func customOption(body string) string {
	for _, match := range tableRowRe.FindAllStringSubmatch(body, -1) {
		cells := strings.Split(match[1], "|")
		if !placeholder(cells[0]) {
			continue
		}
		if match := codeRe.FindStringSubmatch(cells[0]); match != nil {
			return match[1]
		}
	}
	return ""
}

// codes reads every backticked value in a cell, which is how the overview
// writes the languages an option accepts.
func codes(cell string) []string {
	var out []string
	for _, match := range codeRe.FindAllStringSubmatch(cell, -1) {
		out = append(out, strings.TrimSpace(match[1]))
	}
	return out
}

// familySummaries reads the table of general models, which is where the
// overview describes what each family is for. A family with no row there is
// described by the sentence its own section opens with.
func familySummaries(body string) map[string]string {
	out := map[string]string{}
	for _, s := range mdSections(body) {
		if summary := opening(s.body); summary != "" {
			out[slugID(s.heading)] = summary
		}
	}
	for _, match := range tableRowRe.FindAllStringSubmatch(body, -1) {
		cells := strings.Split(match[1], "|")
		if len(cells) < 2 {
			continue
		}
		anchor := linkAnchor(cells[0])
		if anchor == "" {
			continue
		}
		out[anchor] = plain(cells[1])
	}
	return out
}

// opening returns the first sentence of the prose a section starts with.
func opening(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "|") ||
			strings.HasPrefix(line, "*") || strings.HasPrefix(line, "#") {
			continue
		}
		if match := sentenceRe.FindStringSubmatch(plain(line)); match != nil {
			return match[1]
		}
		return plain(line)
	}
	return ""
}

// linkAnchorRe matches the fragment a link points at, which is how a table
// says which section of the same page its row is about.
var linkAnchorRe = regexp.MustCompile(`\]\([^)]*#([a-z0-9-]+)\)`)

// linkAnchor returns the section a cell's link points at.
func linkAnchor(cell string) string {
	match := linkAnchorRe.FindStringSubmatch(cell)
	if match == nil {
		return ""
	}
	return match[1]
}

// separatorRe matches the rule a markdown table draws under its header, which
// is a row like any other to a reader that goes by pipes.
var separatorRe = regexp.MustCompile(`^[\s:-]+$`)

// separator reports whether a cell is part of that rule.
func separator(cell string) bool {
	return separatorRe.MatchString(cell)
}

// mdSection is one heading of a markdown document and the body under it.
type mdSection struct {
	heading string
	level   int
	body    string
	// legacy records that the section sits under the heading the overview
	// groups its older models below.
	legacy bool
	// general records that the family is named in the table of models
	// Deepgram recommends.
	general bool
}

// mdSections divides a markdown document by heading, remembering for each
// section whether it is nested under the legacy heading.
func mdSections(body string) []mdSection {
	locations := mdHeadingRe.FindAllStringSubmatchIndex(body, -1)
	out := make([]mdSection, 0, len(locations))
	general := generalFamilies(body)
	legacy := false
	for i, at := range locations {
		end := len(body)
		if i+1 < len(locations) {
			end = locations[i+1][0]
		}
		heading := body[at[4]:at[5]]
		level := at[3] - at[2]
		if level <= 2 {
			legacy = strings.EqualFold(text(heading), legacyHeading)
		}
		out = append(out, mdSection{
			heading: heading,
			level:   level,
			body:    body[at[1]:end],
			legacy:  legacy,
			general: general[slugID(heading)],
		})
	}
	return out
}

// generalFamilies returns the families the overview lists as the models it
// recommends, which is the table whose first heading names them so.
func generalFamilies(body string) map[string]bool {
	out := map[string]bool{}
	seen := false
	for _, match := range tableRowRe.FindAllStringSubmatch(body, -1) {
		cells := strings.Split(match[1], "|")
		if strings.EqualFold(plain(cells[0]), generalHeading) {
			seen = true
			continue
		}
		if !seen || separator(cells[0]) {
			continue
		}
		anchor := linkAnchor(cells[0])
		if anchor == "" {
			return out
		}
		out[anchor] = true
	}
	return out
}
