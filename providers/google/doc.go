// Package google parses Google's Gemini pricing and model documentation into
// the catalog model.
//
// Two documents are needed because neither is enough. The pricing page is the
// only one naming every model that costs anything, and states no context
// window, no capability and no modality. Each model's own page states all
// three and no rate at all.
//
// What Google does not publish: a page for every model it prices. Seven models
// appear in the pricing tables and have no page of their own — the Gemma
// release, the two robotics previews, the flash-lite preview, a native-audio
// live model and the two older Veo versions — so they carry rates and nothing
// else. That is the whole of the catalog's remaining gap for this provider, and
// it is Google's silence rather than a document this parser fails to read.
//
// A page that does exist states its bounds in a row headed "Token limits" or,
// on the video and omni pages, "Limits", and names the input bound three ways
// inside it: "Input token limit" almost everywhere, "Context window" on the omni
// model and "Text input" on the video models, where it bounds the prompt. All
// are read, being one fact under several of Google's own names. Reading only the
// first heading skipped those rows whole.
//
// A model page writes its modalities as prose rather than as a list, and the
// wordings matter. "Video with audio" names two modalities and not a quality of
// one, so the video models return both. "Text embeddings" is the return value
// of an embedding model, and since the catalog has no word for a vector it is
// read as the text those models work in; dropping it left them stating what
// they take and nothing about what they give back, which a consumer cannot tell
// from a model that returns nothing.
//
// Google states rates twice over: a model has one table per serving tier, and
// each table has one column per plan. A single rate therefore needs both a tier
// and a plan to identify it, and the free plan's "Free of charge" is a real
// rate of zero rather than an absence. Only one column of a table states the
// denominator its rates are quoted against — "Paid Tier, per 1M tokens in USD"
// beside a bare "Free Tier" — and since both price the same rows, the one
// denominator stated is the denominator of all of them. Without that the free
// plan's rates have no unit and are dropped, which is what left the open Gemma
// models, whose only rate is a free one, reading as unpriced.
//
// Neither the model nor the tier appears
// inside its table — both are headings above it — so the pricing page is read
// as a running state of which model and which tier are in force rather than as
// a list of self-describing tables.
//
// A model page is the opposite: one table of properties, each row a labelled
// fact, with the capabilities enumerated and marked supported or not. Only the
// supported ones are recorded, and one marked supported in preview counts as
// supported. An embedding model's page labels the same field in the singular,
// "Input" where a chat model's says "Inputs", and adds the width of the vector
// it returns: a range followed by the widths Google recommends. Only the named
// widths are recorded, since a range is not a list of values to pick from.
//
// Joining the two takes care. The pricing page heads a model with the name it
// sells under and the model page addresses it by the identifier the API
// answers to, and the two drift: what pricing heads "Gemini 3.1 Flash Image
// (Nano Banana 2)" the API calls gemini-3.1-flash-image. One name is a prefix
// of the other about as often as they are equal, in either direction, and
// prefix matching alone would let one model's page attach to another, since
// gemini-2.5-flash prefixes several models that are not it. The pairing is
// therefore made one to one, exact matches first, and a page that matches
// nothing left is dropped rather than guessed at.
//
// What Google does not publish: a page for every model it prices. Gemma, the
// second-generation robotics models and the preview build of Gemini 2.5
// Flash-Lite are priced with no page of their own, so they carry rates and no
// context window. Their absence is Google's, not this parser's.
package google
