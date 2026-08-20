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
// so the catalog carries no rate for it: the pricing page goes on carrying a
// row for a model that no longer answers, and such a row is a leftover rather
// than an offer. The model itself is carried, marked with the state, because a
// consumer with a request naming it needs to be told it was withdrawn rather
// than told nothing. Deprecated is deliberately not here, because a deprecated
// model still serves and Google still prices it.
var withdrawnStates = []string{"shutdown", "retired"}

// indexEntry is what the model index states about one model: the name Google
// lists it under and, where it has withdrawn the model, the state it is in.
type indexEntry struct {
	name  string
	state string
}

// cardRe matches one card of the index's grid, which pairs the page a model
// is documented on, and so the endpoint it answers to, with the availability
// Google marks it as. It is the only document stating that a model is in
// preview rather than generally available for a model the deprecation
// schedule has not reached yet.
var cardRe = regexp.MustCompile(
	`(?is)<a href="/gemini-api/docs/models/([a-z0-9._-]+)"[^>]*` +
		`class="gemini-card-centered">(.*?)</a\s*>`,
)

// cardStateRe matches the availability a card states, which sits under the
// description and may be preceded by a badge saying the model is new.
var cardStateRe = regexp.MustCompile(
	`(?is)<p class="status-subtext">(.*?)</p>`,
)

// cardStates map what a card says onto the catalog's vocabulary.
var cardStates = map[string]string{
	"stable":  StateActive,
	"preview": StatePreview,
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
	b.applyCards(doc)
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
		b.indexed = append(b.indexed, id)
		b.setIndex(id, entry)
		b.setIndex(indexKey(name), entry)
		if _, ok := b.byName[indexKey(name)]; !ok {
			b.byName[indexKey(name)] = id
		}
	}
}

// applyIndexModels carries the endpoints the index lists that the pricing page
// does not price.
//
// The pricing page names every model that costs something, and the index names
// several it does not: the two Deep Research agents and the Antigravity agent,
// whose inference Google bills at the underlying model's rates rather than at
// one of their own, and the four models it has shut down, whose rows on the
// pricing page are leftovers. Both kinds answered to a request until recently
// or answer to one now, and a consumer holding an endpoint name has to be able
// to look it up.
//
// It runs after the pricing page rather than before, so that a model both
// documents name keeps the name it is sold under: the index lists Gemini 3 Pro
// Image as "Nano Banana Pro", which is what Google calls it and not how it is
// billed.
func (b *builder) applyIndexModels(src string) {
	for _, id := range b.indexed {
		entry := b.index[id]
		m := b.model(id, nameKind(id))
		m.AddSource(src)
		if m.Name == "" {
			m.Name = entry.name
		}
		m.SetAttr(AttrState, b.stateOf(id, entry))
	}
}

// applyCards reads the availability the index's grid marks each model with.
// The grid names the page rather than the model, which is what makes it
// unambiguous: the page is addressed by the endpoint.
func (b *builder) applyCards(doc catalog.Document) {
	for _, card := range cardRe.FindAllStringSubmatch(string(doc.Body), -1) {
		state := cardStates[lastWord(first(cardStateRe, card[2]))]
		if state == "" {
			continue
		}
		if _, ok := b.cardState[card[1]]; !ok {
			b.cardState[card[1]] = state
		}
	}
}

// lastWord returns the final word of a card's availability, the badge Google
// puts in front of it saying the model is new being a fact about its age
// rather than about its availability.
func lastWord(value string) string {
	fields := strings.Fields(strings.ToLower(text(value)))
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
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
