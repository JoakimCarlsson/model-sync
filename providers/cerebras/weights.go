package cerebras

import (
	"encoding/json"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// weightsBase is where the repository Cerebras names for a model states what
// that repository is licensed under.
//
// Cerebras publishes the repository and not the licence. It says which weights
// it serves, by naming a public repository per model in its own model list,
// and the licence of those weights is stated by that repository. Reading it is
// the only way to record under what terms a model may be used, and nothing is
// taken from it that the repository does not state for itself.
const weightsBase = "https://huggingface.co/api/models/"

// AttrLicense is the licence the repository of the weights is published under.
const AttrLicense = "license"

// weightsCard is the part of a repository's answer that states its licence.
type weightsCard struct {
	ID       string `json:"id"`
	CardData struct {
		License string `json:"license"`
	} `json:"cardData"`
}

// weightsURL returns where a repository's metadata is answered from.
func weightsURL(repo string) string {
	return weightsBase + repo
}

// weightsRepos returns the repositories the public model list names, in the
// order it names them, so a run fetches the same documents in the same order.
func weightsRepos(doc catalog.Document) []string {
	var list publicListing
	if err := json.Unmarshal(doc.Body, &list); err != nil {
		return nil
	}
	var repos []string
	for _, e := range list.Data {
		if e.HuggingFaceID != "" {
			repos = append(repos, e.HuggingFaceID)
		}
	}
	return repos
}

// applyWeights reads one repository's metadata onto every model Cerebras
// serves those weights as.
func (b *builder) applyWeights(doc catalog.Document) {
	var card weightsCard
	if err := json.Unmarshal(doc.Body, &card); err != nil {
		return
	}
	if card.CardData.License == "" {
		return
	}
	for _, m := range b.models {
		if !strings.EqualFold(m.Attrs[AttrHuggingFaceID], card.ID) {
			continue
		}
		m.SetAttr(AttrLicense, card.CardData.License)
		m.AddSource(doc.URL)
	}
}
