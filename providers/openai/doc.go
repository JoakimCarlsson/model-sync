// Package openai parses OpenAI's published documentation into the catalog
// model.
//
// Every string OpenAI uses lives here: its metric and unit names, its
// dimension keys, its column headers, its section and tier labels, and the
// shapes of its documents. OpenAI publishes markdown by appending .md to a doc
// URL, which makes it cheap to parse; that is a fact about OpenAI and not a
// pattern other providers follow, so the readers in this package are local to
// it and shared with nobody.
//
// Three documents shapes are read, because one provider already publishes in
// more than one:
//
//   - pricing.md, whose rates live in markdown pipe tables qualified by the
//     bare prose lines above them.
//   - the per-model pages under /api/docs/models/, which carry capabilities,
//     modalities, limits, endpoints, snapshots and rate limits.
//   - the guides, which hold in markdown tables and in plain prose several
//     facts no model page states, and whose per-image dollar tables are raw
//     JSX with rowSpan and are invisible to a markdown table reader.
//
// Five guides are read, each for something stated nowhere else: the image
// generation guide for the per-image rates, the embeddings guide for the width
// of a vector, the web search guide for the context window of the search
// models sold without a page, and the transcription and file transcription
// guides for what a listening model can do.
//
// Two models are priced only by reference. Daybreak Blue and Daybreak Red are
// aliases pointing at whichever frontier model the program has reached, and the
// rate table leaves their rows out, saying in a sentence beside it that an
// alias is priced as its target. Their rates are therefore taken from the
// snapshot their page names, with a note recording that they were.
//
// An embedding model records the width of its vector as the default it is and
// not as a list of the widths on offer. OpenAI exposes a dimensions parameter
// that shortens the vector to any smaller length, so there is no set of
// discrete widths to enumerate; a list would invent one. The default and the
// fact that it can be reduced are what it states, and both are recorded. The
// older ada model predates that parameter, and the guide gives its width only
// in passing, comparing a shortened vector against an unshortened one of its;
// that sentence is read too, and it alone carries no shortening.
//
// The longest input those models take is in the same guide, as a Max input
// column reading 8192 for text-embedding-3-small, text-embedding-3-large and
// text-embedding-ada-002 alike. That count is recorded as written. Third party
// catalogs carry 8191 for the same three models, one short of it; OpenAI
// prints 8192 today and prints it nowhere as 8191, so 8192 is what this
// records.
//
// What a transcription model does is not on its page. The pages list at most
// "streaming" under supported features, and diarization, word timestamps,
// translation, language detection and the prompt that biases a transcript
// towards terms the caller supplies are stated in the two transcription
// guides: the first as a table keyed by what the caller wants rather than by
// what a model does, the second as one sentence per group of models. Both are
// read, and the sentence saying which model takes no prompt is read as denying
// support rather than stating it.
//
// A tool is named nowhere but the pricing page, whose tool table writes a
// display name where the model tables write an identifier. That name is kept.
// The rest of what the catalog asks of a model, a context window, an output
// ceiling, modalities and features, is not something a tool has, and the four
// tool rows carry none of it.
//
// What OpenAI does not publish: a rate for gpt-5.4-cyber, whose row in the rate
// table is a line of dashes, or for either open-weight gpt-oss model, which the
// tables leave out entirely. Those three are served and documented and carry no
// price for that reason.
//
// Nor does it publish a page for every model it prices. Six chat models are in
// the rate table and have no page: gpt-5-search-api, the two cyber models and
// three dated snapshots of the older GPT-4 and GPT-3.5 releases. The web search
// guide states the context window of the first of those in its limitations
// table, which is why it has one; for the other five, and for the output
// ceiling of all six, a page is the only place OpenAI states such a bound and
// there is none. The same silence covers the six fine-tuning rows, which are
// dated snapshots of models whose own pages state these figures for the base
// model and never for the snapshot.
//
// Nor does OpenAI state a context window or an output ceiling for every model
// that does have a page. The image model, the moderation model, the three
// speech models and gpt-transcribe, gpt-live-transcribe and whisper-1 all have
// pages whose Model details list modalities and a snapshot and stop there, and
// no guide supplies the numbers either. The one figure any of them gives is the
// longest input gpt-4o-mini-tts takes, written into the paragraph above the
// list, and that is read. An embedding model has no output ceiling at all,
// since what it returns is a vector rather than tokens.
//
// The moderation model is the one model whose feature list is empty by
// construction. The single feature its page lists is image_input, which names a
// modality rather than a capability and is recorded as one, and OpenAI lists
// nothing else it does.
//
// What this package does not emit: a model OpenAI has shut down. The
// deprecations page splits its announcements into those that have taken effect
// and those that have not, and a model in the first group is left out of the
// catalog entirely, including the twenty that still keep a page of their own. A
// model in the second group is kept, because OpenAI still serves it, still
// prices it and states the date it will go, and all three are recorded. The
// page is read for both: it is what marks a deprecated model as deprecated and
// the only place a retirement date is stated, and what it says about a shut
// down model is used only to withhold it. Of the 116 models emitted, 36 are
// deprecated and none are shut down.
//
// A pointer to a shut down model is left as OpenAI wrote it. Six models list a
// default snapshot or a snapshot that is now absent from the catalog, gpt-4
// naming gpt-4-0314 among them, because the page states that snapshot and
// dropping the pointer would report a lineage OpenAI does not describe. The
// same page states a recommended replacement for every model it withdraws, and
// those name current models.
//
// Nor does OpenAI publish a display name for a snapshot or a deprecated
// model. Every model it currently sells is named on a page of its own, and the
// dated snapshots of those models and the ones on their way out appear only in
// the deprecation table, which lists identifiers and dates and no names. That
// is why 24 of the 69 chat models carry none: 18 are deprecated and the other
// six are the rate table rows that have no page.
package openai
