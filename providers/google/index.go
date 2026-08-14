package google

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// AttrState is the lifecycle the model index states for a model it has
// withdrawn.
const AttrState = "state"

// States the index writes. Google marks a withdrawal as an aside on the
// model's name rather than as a field of its own, and writes it in its own
// words; the values on the right are the ones a consumer counts.
var indexStates = map[string]string{
	"shut down":  "shutdown",
	"deprecated": "deprecated",
	"retired":    "retired",
}

// withdrawnStates are the states that mean Google has stopped serving a model,
// so the catalog does not carry it: the pricing page goes on carrying a row for
// a model that no longer answers, and such a row is a leftover rather than an
// offer. Deprecated is deliberately not here, because a deprecated model still
// serves and Google still prices it.
var withdrawnStates = []string{"shutdown", "retired"}

// indexEntry is what the model index states about one model: the name Google
// lists it under and, where it has withdrawn the model, the state it is in.
type indexEntry struct {
	name  string
	state string
}

var (
	// endpointRe matches the identifier an index row ends with, which is what
	// tells a row of the model tables from a row of any other table on the
	// page.
	endpointRe = regexp.MustCompile(`^[a-z][a-z0-9.]*(?:-[a-z0-9.]+)+$`)
	// nbspReplacer restores the spaces the index tables write as an entity,
	// which text would otherwise leave inside a model's name.
	nbspReplacer = strings.NewReplacer("&nbsp;", " ", "&#160;", " ")
)

// applyIndex reads the model index. It is the only document pairing the name
// Google lists a model under with the endpoint the API answers to, and the
// only one saying which models Google has withdrawn.
func (b *builder) applyIndex(doc catalog.Document) {
	for _, row := range pageRowRe.FindAllStringSubmatch(string(doc.Body), -1) {
		cells := pageCellRe.FindAllStringSubmatch(row[1], -1)
		if len(cells) < 2 {
			continue
		}
		id := text(cells[len(cells)-1][1])
		if !endpointRe.MatchString(id) {
			continue
		}
		name, state := nameState(cells[0][1])
		if name == "" {
			continue
		}
		entry := indexEntry{name: name, state: state}
		b.setIndex(id, entry)
		b.setIndex(indexKey(name), entry)
	}
}

// setIndex records one entry under one key, keeping the first, since a model
// listed in two of the index's tables is listed the same way in both.
func (b *builder) setIndex(key string, entry indexEntry) {
	if key == "" {
		return
	}
	if _, ok := b.index[key]; !ok {
		b.index[key] = entry
	}
}

// nameState splits an index name from the aside saying Google has withdrawn
// the model, which is the only place the index says so.
func nameState(cell string) (string, string) {
	name := text(nbspReplacer.Replace(cell))
	state := ""
	for _, aside := range parenRe.FindAllString(name, -1) {
		word := strings.ToLower(strings.Trim(aside, "()"))
		if mapped, ok := indexStates[word]; ok {
			state = mapped
		}
	}
	return strings.TrimSpace(parenRe.ReplaceAllString(name, "")), state
}

// indexKey reduces a name to what it is looked up by. Google hangs an aside
// off one in both documents, writing "Imagen 4 (Deprecated)" on the index and
// "Gemini 3.1 Flash Image (Nano Banana 2)" on the pricing page, and the
// families the two pages agree on are the ones with the aside taken off.
func indexKey(name string) string {
	return slugID(parenRe.ReplaceAllString(name, ""))
}
