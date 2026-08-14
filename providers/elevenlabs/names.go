package elevenlabs

import (
	"github.com/joakimcarlsson/model-sync/catalog"
)

// NamesURL is the help center page pairing a display name with an identifier.
// The models page names only the flagships it opens with; this is where
// ElevenLabs names the rest of the range it still offers.
const NamesURL = "https://elevenlabs.io/docs/help-center/technical/" +
	"how-do-i-find-the-model-id.md"

// applyNames reads the tables pairing a model name with its identifier.
//
// The page states nothing else a model can be described by, so a row naming an
// identifier the models page never listed is ignored rather than creating a
// model out of a name. A model the cards already named keeps that name, which
// is the fuller of the two: the cards write "Eleven Multilingual v2" where this
// page writes "Multilingual v2".
func (b *builder) applyNames(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		nameCol := columnOf(t.Headers, "model name")
		idCol := columnOf(t.Headers, "model id")
		if nameCol < 0 || idCol < 0 {
			continue
		}
		for _, row := range t.Rows {
			m, ok := b.models[clean(cellAt(row, idCol))]
			if !ok {
				continue
			}
			m.AddSource(doc.URL)
			if m.Name == "" {
				m.Name = clean(cellAt(row, nameCol))
			}
		}
	}
}
