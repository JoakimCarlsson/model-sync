package cerebras

import (
	"encoding/json"
	"fmt"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// PublicModelsURL answers, without a key, with the models Cerebras serves on
// its public endpoints. It is the one place the two bounds the documentation
// rounds are stated exactly.
const PublicModelsURL = "https://api.cerebras.ai/public/v1/models"

// Scalar keys the public model list populates.
const (
	AttrSummary      = "summary"
	AttrAuthor       = "author"
	AttrQuantization = "quantization"
)

// publicListing is the shape of the public model list.
type publicListing struct {
	Data []publicModel `json:"data"`
}

// publicModel is one model as the endpoint states it.
type publicModel struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	OwnedBy      string       `json:"owned_by"`
	Quantization string       `json:"quantization"`
	Limits       publicLimits `json:"limits"`
}

// publicLimits are the ceilings a paid caller is held to.
type publicLimits struct {
	MaxContextLength    int64 `json:"max_context_length"`
	MaxCompletionTokens int64 `json:"max_completion_tokens"`
}

// applyPublicModels reads the public model list.
//
// It is read before either document, because it states the two ceilings to the
// token where the catalog and the model pages round them to "131k" and "40k",
// and because it is Cerebras answering for itself which models it currently
// sells rather than a page describing them.
func (b *builder) applyPublicModels(doc catalog.Document) error {
	var list publicListing
	if err := json.Unmarshal(doc.Body, &list); err != nil {
		return fmt.Errorf("decode %s: %w", doc.URL, err)
	}
	for _, e := range list.Data {
		if e.ID == "" {
			continue
		}
		m := b.model(e.ID, KindChat)
		m.AddSource(doc.URL)
		if m.Name == "" {
			m.Name = e.Name
		}
		m.SetAttr(AttrSummary, e.Description)
		m.SetAttr(AttrAuthor, e.OwnedBy)
		m.SetAttr(AttrQuantization, e.Quantization)
		m.SetLimit(LimitContextWindow, e.Limits.MaxContextLength)
		m.SetLimit(LimitMaxOutputTokens, e.Limits.MaxCompletionTokens)
	}
	return nil
}
