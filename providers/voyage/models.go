package voyage

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// listingSuffixes are the words an AWS Marketplace title adds to a model's
// identifier to say what the model does.
var listingSuffixes = []string{" embedding model", " reranker"}

var (
	// deprecatedRe matches the word Voyage prefixes a description with when a
	// model is deprecated, which it writes in brackets and italics.
	deprecatedRe = regexp.MustCompile(`(?i)\\?\[\*?deprecated\*?\]\s*`)
	// replacementRe matches the model a description asks the reader to move
	// to.
	replacementRe = regexp.MustCompile(
		"(?i)transition to `(voyage-[^`]+)`",
	)
	// weightsRepoRe matches the repository an open-weight model is published
	// in.
	weightsRepoRe = regexp.MustCompile(
		`huggingface\.co/([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)`,
	)
	// tokenizerRe matches the sentence naming the models that share Voyage's
	// first tokenizer, and the tokenizer they share.
	tokenizerRe = regexp.MustCompile(
		`(?is)earlier models, including (.*?), use the same tokenizer as ` +
			`\[([^\]]+)\]`,
	)
	// listingNameRe matches the title an AWS Marketplace listing states for
	// itself.
	listingNameRe = regexp.MustCompile(`"listingName":"([^"]+)"`)
	// publishedRe matches the day an announcement post went up. The post
	// states it twice, once as an instant in UTC and once as the day the post
	// carries, and the day is the one recorded: the two disagree by a date
	// whenever a post went up in the Pacific evening, and it is the day that
	// Voyage prints on the post and puts in its address.
	publishedRe = regexp.MustCompile(
		`<time datetime="(\d{4}-\d{2}-\d{2})`,
	)
)

// sectionOpenModels is the heading under which Voyage lists the models whose
// weights it publishes.
const sectionOpenModels = "open models"

// applyModelPage reads a capability page: the embeddings, multimodal,
// contextualized-chunk or reranker page, or MongoDB's overview, which states
// the same tables in the same shape under its own column headings.
//
// A model found here but not in any rate table is still recorded. Voyage's
// open-weight model is documented and usable without appearing in the pricing
// tables at all, because running it yourself costs Voyage nothing to state.
func (b *builder) applyModelPage(doc catalog.Document) {
	b.applyModelTables(doc)
	b.applyQuantization(doc)
	b.applyVideoInputs(doc)
	b.applyInstructions(doc)
	b.applyGuideBudget(doc)
}

// guideBudgetRe matches the sentence a guide page states the token budget for
// one request in, which it writes in prose rather than in its table. The
// reference reads the same bound from the endpoint definition, and the guide
// is read as well because it names two models the definition has stopped
// naming while continuing to serve them.
var guideBudgetRe = regexp.MustCompile(
	`(?i)total number of tokens in the list is at most ([^
]+)`,
)

// applyGuideBudget records the per-model token budget a guide page states.
func (b *builder) applyGuideBudget(doc catalog.Document) {
	match := guideBudgetRe.FindStringSubmatch(string(doc.Body))
	if match == nil {
		return
	}
	b.applyPerModel(match[1], LimitTokensPerReq)
}

// endpointSections maps a heading of MongoDB's overview to the guide page
// whose endpoint serves the models under it. The overview gathers every family
// onto one page, and its headings are the only thing there saying which
// endpoint a table belongs to.
var endpointSections = []struct {
	keyword string
	page    string
}{
	{"rerank", baseURL + "/reranker.md"},
	{"multimodal", baseURL + "/multimodal-embeddings.md"},
	{"contextualized", baseURL + "/contextualized-chunk-embeddings.md"},
	{"text embedding", baseURL + "/embeddings.md"},
}

// servingPage returns the guide page whose endpoint serves the models of one
// table, which for Voyage's own pages is the page itself and for the overview
// is whatever the headings above the table name. A chain naming no family
// yields nothing, and the models under it keep whatever Voyage's own pages
// already said.
func servingPage(url, trail string) string {
	if url != overviewURL {
		return url
	}
	for _, s := range endpointSections {
		if strings.Contains(trail, s.keyword) {
			return s.page
		}
	}
	return ""
}

// applyModelTables reads the model tables of a capability page.
func (b *builder) applyModelTables(doc catalog.Document) {
	for _, t := range scanTables(string(doc.Body), doc.URL) {
		idCol := columnOf(t.Headers, "model")
		contextCol := columnOf(
			t.Headers,
			"context length (tokens)",
			"context length",
		)
		if idCol < 0 || contextCol < 0 {
			continue
		}
		dimCol := columnOf(t.Headers, "embedding dimension", "dimensions")
		descCol := columnOf(t.Headers, "description")
		chunkCol := columnOf(t.Headers, "per chunk context window")
		for _, row := range t.Rows {
			description := cellAt(row, descCol)
			for _, id := range splitModels(cellAt(row, idCol)) {
				m := b.model(id, kindFor(id))
				m.AddSource(t.Source)
				if page := servingPage(doc.URL, t.Trail); page != "" {
					b.pageModels[page] = append(b.pageModels[page], id)
				}
				addModalities(m, modalitiesFor(doc.URL, t.Section))
				m.SetLimit(
					LimitContextWindow,
					parseCount(cellAt(row, contextCol)),
				)
				m.SetLimit(
					LimitChunkContext,
					parseCount(cellAt(row, chunkCol)),
				)
				m.SetAttr(AttrSummary, summaryOf(description))
				applyDimensions(m, cellAt(row, dimCol))
				applyDescription(m, description)
				if t.Section == sectionOpenModels {
					m.SetAttr(AttrOpenWeights, "true")
					m.AddNote(noteOpenWeights)
					applyWeightsRepo(m, description)
				}
			}
		}
	}
}

// applyDimensions records the embedding widths a model offers, the one it uses
// when none is asked for, and the capability that having more than one is.
//
// The first document to state the widths wins, as it does for every scalar
// here. Voyage's own pages and MongoDB's overview disagree about voyage-4-nano,
// and merging two sets of widths would report a set that neither of them
// states.
func applyDimensions(m *catalog.Model, cell string) {
	if len(m.Lists[ListDimensions]) > 0 {
		return
	}
	dimensions, defaultDim := parseDimensions(cell)
	m.AddList(ListDimensions, dimensions...)
	m.SetAttr(AttrDefaultDimension, defaultDim)
	if len(dimensions) > 1 {
		m.AddList(ListFeatures, FeatureReducibleDims)
	}
}

// applyDescription reads the parts of a description cell that are statements
// about the model rather than prose about it: the post Voyage announced it in,
// the model it asks the reader to move to and the word deprecated where it
// prefixes the description.
//
// The word is recorded beside the model's state rather than as it. Voyage
// deprecates a model without withdrawing it: the five it marks are still
// served, still priced and still in its rate tables, and the catalog's
// deprecated state means a model that is gone.
func applyDescription(m *catalog.Model, cell string) {
	if url := blogLinkRe.FindString(cell); url != "" {
		m.SetAttr(AttrAnnouncementURL, url)
	}
	if match := replacementRe.FindStringSubmatch(cell); match != nil {
		m.SetAttr(AttrReplacement, match[1])
	}
	if deprecatedRe.MatchString(cell) {
		m.SetAttr(AttrDeprecated, "true")
	}
}

// applyWeightsRepo records where an open-weight model's weights are published,
// which its description links. It is read only under the open models heading,
// every other description linking a repository that holds something other than
// the model, such as the leaderboard one of them cites.
func applyWeightsRepo(m *catalog.Model, cell string) {
	if match := weightsRepoRe.FindStringSubmatch(cell); match != nil {
		m.SetAttr(AttrHuggingFaceID, match[1])
	}
}

// applyTokenizer records which tokenizer a model uses, which Voyage names for
// its earlier models only. The newer ones each have their own, published as a
// repository rather than as a name, and no page names them.
func (b *builder) applyTokenizer(doc catalog.Document) {
	match := tokenizerRe.FindStringSubmatch(string(doc.Body))
	if match == nil {
		return
	}
	for _, id := range modelIDs(match[1]) {
		m := b.model(id, kindFor(id))
		m.AddSource(doc.URL)
		m.SetAttr(AttrTokenizer, clean(match[2]))
	}
}

// applyListing records the display name an AWS Marketplace listing gives a
// model.
//
// The listing is not told which model it is for. Its title is reduced to the
// identifier it spells, and recorded only where that identifier is a model
// already known from Voyage's own pages, so a listing that is renamed or
// retired drops out instead of naming the wrong thing.
func (b *builder) applyListing(doc catalog.Document) {
	match := listingNameRe.FindStringSubmatch(string(doc.Body))
	if match == nil {
		return
	}
	title := match[1]
	for id, m := range b.models {
		if !titleNames(title, id) || m.Name != "" {
			continue
		}
		m.Name = title
		m.AddSource(doc.URL)
	}
}

// titleNames reports whether a listing title is the display name of one model,
// which it is when stripping the words for what the model does leaves the
// model's identifier and nothing else.
func titleNames(title, id string) bool {
	stripped := strings.ToLower(strings.TrimSpace(title))
	for _, suffix := range listingSuffixes {
		if after, ok := strings.CutSuffix(stripped, suffix); ok {
			return squash(after) == squash(id)
		}
	}
	return false
}

// squash reduces a name to its letters and digits, so that a title writing a
// model's identifier with spaces and capitals still matches it.
func squash(s string) string {
	var out strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			out.WriteRune(r)
		}
	}
	return out.String()
}

// applyAnnouncement dates a model from the post its table links to.
//
// Voyage publishes no changelog and no release date. What it publishes is a
// blog post per model, linked from the model's own row, and that post states
// the day it went up. That day is what is recorded, and it is the day Voyage
// announced the model rather than a separate date it might have started
// serving it, which no document states.
func (b *builder) applyAnnouncement(doc catalog.Document) {
	match := publishedRe.FindStringSubmatch(string(doc.Body))
	if match == nil {
		return
	}
	for _, m := range b.models {
		if m.Attrs[AttrAnnouncementURL] != doc.URL {
			continue
		}
		m.SetAttr(AttrReleaseDate, match[1])
		m.AddSource(doc.URL)
	}
}

// applyQuantization records the models that can return a vector narrower than a
// 32-bit float, which Voyage states in one sentence naming them rather than in
// the model table.
func (b *builder) applyQuantization(doc catalog.Document) {
	for _, match := range quantizedRe.FindAllStringSubmatch(
		string(doc.Body),
		-1,
	) {
		for _, id := range modelIDs(match[1]) {
			m := b.model(id, kindFor(id))
			m.AddSource(doc.URL)
			m.AddList(ListFeatures, FeatureQuantizedOutput)
		}
	}
}

// applyVideoInputs records the models that take video, which the multimodal
// page states in a sentence withholding it from the rest of its own table.
func (b *builder) applyVideoInputs(doc catalog.Document) {
	for _, match := range videoInputRe.FindAllStringSubmatch(
		string(doc.Body),
		-1,
	) {
		for _, id := range modelIDs(match[1]) {
			m := b.model(id, kindFor(id))
			m.AddSource(doc.URL)
			m.AddList(ListInputModalities, ModalityVideo)
		}
	}
}

// summaryOf reduces a description cell to its first sentence. Voyage's
// descriptions run to several sentences of links and compatibility notes, and
// only the first says what the model is. The bracketed word marking a model
// deprecated is dropped from it, being recorded as the model's state instead.
func summaryOf(cell string) string {
	text := deprecatedRe.ReplaceAllString(clean(cell), "")
	if sentence, _, ok := strings.Cut(text, ". "); ok {
		return sentence + "."
	}
	return text
}
