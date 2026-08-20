package together

import (
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// DeprecationsURL is the page stating Together's model lifecycle policy, the
// redirects in force, and the date every model it has withdrawn was removed
// on.
const DeprecationsURL = "https://docs.together.ai/docs/deprecations.md"

// Headings of the tables that page holds. The removal history is split in two,
// one table for serverless inference and one for the fine-tuning service, and
// only the first says anything about a model the catalog carries: a model
// withdrawn as a fine-tuning base is still sold per token.
const (
	sectionInference = "inference"
	sectionRedirects = "active model redirects"
)

// Columns of those tables.
const (
	colRemoval     = "removal date"
	colModel       = "model"
	colDedicated   = "supported by on-demand dedicated endpoints"
	colOriginal    = "original model"
	colRedirectsTo = "redirects to"
)

// AttrDedicatedAfter records what the removal table's last column says: that
// the model can still be deployed on an on-demand dedicated endpoint after it
// stops being sold per token. It is the migration path Together names, and it
// is not the same fact as the model appearing in the dedicated catalog today.
const AttrDedicatedAfter = "dedicated_after_retirement"

// noteStillListed says that two documents disagree about a model, which is
// left standing rather than resolved: the deprecation history says it was
// removed from serverless inference, and the catalog page still lists it with
// a price.
const noteStillListed = "the deprecation history gives this model a removal " +
	"date from serverless inference, and the catalog page still lists it " +
	"with a per-token rate"

// applyDeprecations reads the lifecycle page.
//
// Two of its tables name models. The removal history states, for every model
// Together has ever withdrawn from serverless inference, the day it went and
// whether it can still be deployed on demand; the redirect table states which
// identifiers are answered by a different model than the one they name.
//
// Almost every row of the removal history names a model the catalog page no
// longer lists, and each is carried as a model of its own marked retired, with
// the day it went. A consumer holding code that names one needs to be able to
// tell a model Together withdrew from a model this catalog never read: the
// first is answered by the retirement date and the replacement, the second
// only by silence, and both looked the same while these rows created nothing.
// They carry no rate, no window and no capabilities, because the only document
// naming them states none: what it states is that they are gone.
//
// A row naming a model the catalog page does still list is the interesting
// case, and it is recorded with a note, because the two documents are then
// saying different things and neither is a misreading of the other.
//
// The fine-tuning removals are still not read. A model withdrawn as a tuning
// base is not withdrawn from inference, and the table says nothing about
// whether it is sold.
func (b *builder) applyDeprecations(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		switch t.Section {
		case sectionInference:
			b.applyRemovals(t)
		case sectionRedirects:
			b.applyRedirects(t)
		}
	}
}

// applyRemovals reads the table of models withdrawn from serverless inference.
func (b *builder) applyRemovals(t table) {
	dateCol := columnOf(t.Headers, colRemoval)
	idCol := columnOf(t.Headers, colModel)
	if dateCol < 0 || idCol < 0 {
		return
	}
	dedicatedCol := columnOf(t.Headers, colDedicated)
	for _, row := range t.Rows {
		id := clean(cellAt(row, idCol))
		if id == "" {
			continue
		}
		m := b.model(id, "")
		m.SetAttr(AttrState, StateRetired)
		m.SetAttr(AttrRetirementDate, clean(cellAt(row, dateCol)))
		m.SetAttr(
			AttrDedicatedAfter,
			strings.ToLower(clean(cellAt(row, dedicatedCol))),
		)
		if len(m.Prices) > 0 {
			m.AddNote(noteStillListed)
		}
		m.AddSource(t.Source)
	}
}

// applyRedirects reads the table of identifiers answered by another model.
func (b *builder) applyRedirects(t table) {
	fromCol := columnOf(t.Headers, colOriginal)
	toCol := columnOf(t.Headers, colRedirectsTo)
	if fromCol < 0 || toCol < 0 {
		return
	}
	for _, row := range t.Rows {
		m, ok := b.models[clean(cellAt(row, fromCol))]
		if !ok {
			continue
		}
		target := clean(cellAt(row, toCol))
		m.SetAttr(AttrRedirectsTo, target)
		m.SetAttr(AttrReplacement, target)
		m.AddSource(t.Source)
	}
}
