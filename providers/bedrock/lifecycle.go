package bedrock

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Scalar and enumeration keys the guide's per-model pages populate.
const (
	AttrLegacyDate     = "legacy_date"
	AttrExtendedAccess = "public_extended_access_date"
	ListBatchRegions   = "batch_regions"
	ListBatchProfiles  = "batch_inference_profile_regions"
	ListLatencyRegions = "latency_optimized_regions"
)

// byIdentifier indexes the models by every identifier they answer to.
//
// It is what joins the guide's per-model pages to the models, and it is the
// only join those pages allow: they key on the model ID, which a card states
// and the price list does not, so a model the list meters and the guide does
// not card is reachable from none of them.
func (b *builder) byIdentifier() map[string][]*catalog.Model {
	index := map[string][]*catalog.Model{}
	for _, id := range b.order {
		m := b.models[id]
		for _, alias := range m.Lists[ListAliases] {
			index[alias] = append(index[alias], m)
		}
	}
	return index
}

// lifecycleFieldRe matches one labelled fact of the lifecycle page, which
// states a model as a nested list rather than as a row.
var lifecycleFieldRe = regexp.MustCompile(
	`(?m)^\s*-\s+\*\*([^*]+):\*\*\s*(.*?)\s*$`,
)

// lifecycleEntryRe matches where one model's entry begins.
var lifecycleEntryRe = regexp.MustCompile(`(?m)^-\s+\*\*`)

// Fields of a lifecycle entry, named as AWS labels them.
const (
	fieldLifecycleID = "model id"
	fieldLegacy      = "legacy date"
	fieldEOLDate     = "eol date"
	fieldExtended    = "public extended access start date"
)

// applyLifecycle records the dates the model lifecycle page sets for the
// models it is retiring.
//
// The page is the only document dating the end of a model's life exactly. A
// card states an end of life AWS will not come before, and states none at all
// for the four models in five that are not being retired; this page names the
// day a model stops being offered to new callers, the day its price may rise,
// and the day it stops answering.
//
// Its entries are read defensively. One of them has its labels and its values
// out of step, giving the Command R model an identifier of us-east-1 and a
// Region of a date, so a value that is not shaped like the thing its label
// names is dropped rather than recorded.
func (b *builder) applyLifecycle(doc catalog.Document) {
	index := b.byIdentifier()
	body := string(doc.Body)
	for _, entry := range splitEntries(body) {
		fields := map[string]string{}
		for _, field := range lifecycleFieldRe.FindAllStringSubmatch(
			entry,
			-1,
		) {
			label := strings.ToLower(strings.TrimSpace(field[1]))
			fields[label] = linkText(field[2])
		}
		id := fields[fieldLifecycleID]
		if !identifierRe.MatchString(id) {
			continue
		}
		for _, m := range index[id] {
			m.AddSource(doc.URL)
			m.SetAttr(AttrLegacyDate, isoDate(fields[fieldLegacy]))
			m.SetAttr(AttrRetirementDate, isoDate(fields[fieldEOLDate]))
			m.SetAttr(AttrExtendedAccess, isoDate(fields[fieldExtended]))
		}
	}
}

// splitEntries divides the lifecycle page into one span per model.
func splitEntries(body string) []string {
	starts := lifecycleEntryRe.FindAllStringIndex(body, -1)
	entries := make([]string, 0, len(starts))
	for i, start := range starts {
		end := len(body)
		if i+1 < len(starts) {
			end = starts[i+1][0]
		}
		entries = append(entries, body[start[0]:end])
	}
	return entries
}

// Headings of the guide's tables of what a model may be invoked through.
const (
	headingSupportModelID = "model id"
	headingSingleRegion   = "single-region model support"
	headingProfileRegions = "cross-region inference profile support"
)

// applySupport records a serving path the guide states per model rather than
// per card: which Regions a model may be batched in, and which it may be
// invoked in on the latency-optimized path.
//
// Both pages are one table keyed on the model ID, and both list the Regions
// rather than answering yes: a model named in either table with no Region
// against it is one the page mentions and does not offer the path in.
func (b *builder) applySupport(
	doc catalog.Document,
	feature, regions, profiles string,
) {
	index := b.byIdentifier()
	for _, t := range parseTables(string(doc.Body)) {
		if !t.hasHeading(headingSupportModelID) {
			continue
		}
		for _, row := range t.rows {
			applySupportRow(index, t, row, doc.URL, support{
				feature:  feature,
				regions:  regions,
				profiles: profiles,
			})
		}
	}
}

// support names the keys one of the guide's support tables populates.
type support struct {
	feature  string
	regions  string
	profiles string
}

// applySupportRow records one model's row of a support table.
func applySupportRow(
	index map[string][]*catalog.Model,
	t table,
	row []string,
	source string,
	keys support,
) {
	id := ""
	for i := range row {
		if t.heading(i) == headingSupportModelID {
			id = strings.TrimSpace(cell(row, i))
		}
	}
	if !identifierRe.MatchString(id) {
		return
	}
	for _, m := range index[id] {
		found := false
		for i := range row {
			key := ""
			switch t.heading(i) {
			case headingSingleRegion:
				key = keys.regions
			case headingProfileRegions:
				key = keys.profiles
			}
			if key == "" {
				continue
			}
			for _, region := range regionCodes(cell(row, i)) {
				m.AddList(key, region)
				found = true
			}
		}
		if !found {
			continue
		}
		m.AddSource(source)
		m.AddList(ListFeatures, keys.feature)
	}
}

// regionCodes reads the Regions out of a cell, which states none as "N/A".
func regionCodes(value string) []string {
	var out []string
	for _, entry := range entries(value) {
		if regionCodeRe.MatchString(entry) {
			out = append(out, entry)
		}
	}
	return out
}
