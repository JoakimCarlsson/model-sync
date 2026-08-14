// Package together parses Together AI's model catalog into the catalog model.
//
// Together is a host rather than a lab: it serves other people's models, so
// every entry carries the organization that made it and an API string that is
// a path rather than a name. It publishes one page holding eight tables of
// eight different shapes, one per modality, and the shape is the only thing
// saying what a rate means. An image model is priced per megapixel with a
// default step count, a video model per video at a fixed resolution and
// duration, an audio model per million characters or per minute of audio
// depending on which direction it runs, and an embedding model per million
// tokens beside the width of the vector it returns.
//
// That page is the only one naming what Together sells per token, and it is
// read first for that reason. Three further sets of documents answer about the
// models it established, and none of them creates one.
//
// The chat table reports two capabilities and the quantization a model is
// served at. The capabilities are a yes-or-no column each, and a good third of
// its rows answer neither: Together writes a dash there for a model it has not
// filled the column in for, not for one that cannot do it.
//
// The third capability has no column. Together states which models reason on a
// page of its own, as a table of the models that do, so that page is read too
// and its rows matched to the catalog's by the identifier both state. That
// table says more than the fact: its type column separates a model that always
// reasons from one that can be switched and one that takes an effort setting,
// which is recorded alongside. A row naming a model the catalog does not carry
// is skipped rather than creating one, because the page closes by naming
// several more that run only on dedicated inference.
//
// The dashes and the rest of the reasoning models are answered by the model
// library on Together's own site, which carries a page per model stating a
// specification list: the identifier the API answers to, a context length, the
// tags saying what the model is, the capabilities it has, and the media it
// takes and returns. The list is what makes the library usable here, since it
// states the API string; the library's index pages name a model without one,
// so a card on them cannot be tied to a catalog row. The pages are found
// through the site's sitemap, there being no other listing of them, and the
// library covers every model Together has ever carried, so the pages naming
// one the catalog did not establish are the majority and are skipped.
//
// Two things the library states are deliberately not taken over the catalog
// page. Its context length is rounded to a power-of-two label, 256K where the
// table says 262144, so it is recorded only where the table has none, which is
// what fills in the two chat models whose context cell is a dash. Its
// structured-output tag says JSON mode, the weaker of the two strengths the
// catalog distinguishes, so a model carrying it records that and the general
// value both.
//
// What a model takes and returns is stated twice, and the two are merged. The
// catalog page states it by which of its eight tables a model is listed in: a
// model in the vision table is a chat model that also takes images, and every
// one of them is in the chat table too. The library states it a model at a
// time, which is finer, and is how a chat model that also takes video is
// distinguishable from one that does not. The audio table is the only one
// whose models run in two directions, and it has a column saying which. An
// embedding model and a reranker are recorded as working in text on both
// sides, which names the medium and not the return value: one answers with a
// vector and the other with a set of scores, and the catalog has a word for
// neither. Both sides are always set together, since a consumer reading one
// alone cannot tell an unstated output from a model that returns nothing.
//
// A bound on output length is stated for one model and one model only. No
// table has a column for it, and the reference says only that the input plus
// max_tokens must fit the context window, which is a consequence of the
// context length rather than a bound of its own. Where a model may generate
// less than its window holds, Together says so in the prose of the guide it
// writes for that model, and it says nothing where the two are the same: the
// Kimi K3 guide spells that out, and the GLM-5.2 guide is the one that states
// a ceiling. So the guides are read, found through the documentation index,
// and each is matched to its model by the API string it opens by stating. Its
// listing API answers with more per model, but it needs an account.
package together
