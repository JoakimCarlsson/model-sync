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
// Four document shapes are read, because one provider already publishes in
// more than one:
//
//   - pricing.md, whose rates live in markdown pipe tables qualified by the
//     bare prose lines above them.
//   - the per-model pages under /api/docs/models/, which carry capabilities,
//     modalities, limits, endpoints, snapshots and rate limits.
//   - the guides, which hold in markdown tables, in raw HTML tables and in
//     plain prose several facts no model page states, and whose per-image
//     dollar tables are raw JSX with rowSpan and are invisible to a markdown
//     table reader.
//   - changelog.md, which is dated announcements rather than a description of
//     anything, and is the only document saying when a model arrived.
//
// Eight guides are read, each for something stated nowhere else: the image
// generation guide for the per-image rates and for the sizes, qualities,
// formats and pixel bounds of a generated image, the video guide for how long
// a generation may run, the text to speech guide for the voices, formats and
// languages of the speech models, the moderation guide for what the moderation
// model detects, the embeddings guide for the width of a vector, the web
// search guide for the context window of the search models sold without a
// page, and the transcription and file transcription guides for what a
// listening model can do and how large a recording it takes.
//
// Two models are priced only by reference. Daybreak Blue and Daybreak Red are
// aliases pointing at whichever frontier model the program has reached, and the
// rate table leaves their rows out, saying in a sentence beside it that an
// alias is priced as its target. Their rates are therefore taken from the
// snapshot their page names, with a note recording that they were.
//
// A model's kind is settled by the routes its page says it serves, and only by
// its name where there is no page. The pricing page groups the realtime models
// and the speech models under one heading and would otherwise make a
// text-to-speech model a realtime one, and it lists two live transcription
// models under transcription while their pages serve only the realtime
// transcription and translation routes. The route is the sharper statement, so
// the page settles it and the transcription routes are checked before the bare
// realtime one.
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
// A rate limit is published on every model page still in the catalog, as a
// table per usage tier under a subheading naming the scope the numbers hold
// for. Most pages
// write that subheading as "default" and mean the model's only allowance;
// eleven models sold with a separate allowance for long prompts write
// "Standard" and "Long Context" instead, and the second scope is carried into
// the key so the two sets do not overwrite each other. The columns are
// abbreviations, RPM, RPD, TPM, IPM, a batch queue limit and, for the streaming
// audio models, minutes of audio per minute, and each is recorded under the
// name the catalog states a rate limit by. The speech models publish a free
// allowance above the numbered tiers, headed "free" rather than "Tier 1", and
// it is recorded under that name. The two open-weight pages are the exception:
// their tables are published with a zero in every cell, which is not a limit
// to record, and they end up with none.
//
// Dates come from the changelog and the deprecations page and from nowhere
// else. No model page carries one. The deprecations page gives two: the heading
// of each announcement opens with the day the notice was given, which is when
// the models under it became deprecated, and the table beside it gives the day
// each stops being served. The changelog gives the other two. Its entries are
// tagged with the models they concern, so the newest entry tagged with a model
// is when OpenAI last published anything about it; and an entry whose prose
// opens with Released, Launched or Introduced is announcing something, which
// makes the models it names the ones released that day. Two wordings are read,
// because OpenAI uses two: a sentence linking the model pages it announces, and
// a sentence naming a family and linking the guide for it, where the entry's
// own tags are what say which models arrived. A model link following an
// apposition is not read as a release, because "Released o1-pro, a version of
// the o1 reasoning model" announces one model and describes another. The
// changelog creates no models: it names models OpenAI has since removed and
// writes some identifiers as the old platform URLs spelled them, and a document
// read for dates should not be able to add a model on the strength of a slug.
//
// A snapshot suffix is not read as a date. OpenAI names a snapshot
// gpt-5-2025-08-07 and the day in that name is plainly a day, but what it dates
// is that version of the model rather than the day it was published, and OpenAI
// nowhere says the two coincide. The changelog states publication and is read
// for it, and a suffix is left as part of an identifier.
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
// The models that generate rather than write are described in their guides and
// not on their pages. An image model's page lists two modalities, a snapshot
// and a rate limit; the sizes it renders, the qualities it offers, the file
// formats it returns and the bounds on a generated image's pixels are in the
// image generation guide, in a raw HTML table whose rows are a label and a list
// rather than a header and a row. The sizes and bounds there are stated for
// gpt-image-2 by name, in the paragraph saying that model accepts any
// resolution satisfying them, and are recorded against it alone; the formats
// are stated for the endpoint, and the models reaching it are the four the
// guide's limitations paragraph enumerates. A video model's page is the same
// shape, and the two lengths a Sora generation may run are one sentence of the
// video guide. The resolutions and pixel sizes those models render at are the
// dimensions of the video rate table, which prices a second of output by both,
// and are recorded as lists as well, because a resolution on sale is a
// resolution offered.
//
// A speech model's page is barer still, listing text in and audio out and
// stopping. The thirteen voices, the six output formats and the fifty-seven
// languages are in the text to speech guide, which also states that the two
// older models take a smaller set of nine; a model named in that sentence takes
// the smaller set instead of the full one rather than as well as it. The
// longest recording a transcription model accepts and the formats it reads are
// in the speech to text guide, stated for the endpoint rather than for a model,
// and are recorded against the models whose pages list that route.
//
// The moderation model is described the same way. Its page lists image_input
// under supported features, which names a modality and is recorded as one,
// leaving the page saying nothing about what the model does. The moderation
// guide answers that with thirteen categories in a raw HTML table, introduced
// by a sentence naming the model they hold for, which is what they are recorded
// against.
//
// Two models publish their weights, and their pages are the only two carrying a
// download link, a licence and a parameter count. Those are read where they
// are: the HuggingFace URL as the repository and the model card, the licence
// from the key features bullet stating it, and the total and active parameter
// counts from the parenthesis in the opening paragraph. The other ninety-four
// pages state none of the three, and that silence is the fact that those models
// are not open.
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
// Nor does it publish a page for every model it prices. Six models are in the
// rate table and have no page: gpt-5-search-api, the two cyber models and three
// dated snapshots of the older GPT-4 and GPT-3.5 releases. The web search guide
// states the context window of the first of those in its limitations table,
// which is why it has one; for the other five, and for the output ceiling of
// all six, a page is the only place OpenAI states such a bound and there is
// none. The same silence covers the six fine-tuning rows, which are dated
// snapshots of models whose own pages state these figures for the base model
// and never for the snapshot.
//
// Nor does OpenAI state a context window or an output ceiling for every model
// that does have a page. The image models, the moderation model, the speech
// models and gpt-transcribe, gpt-live-transcribe and whisper-1 all have pages
// whose Model details list modalities and a snapshot and stop there, and no
// guide supplies the numbers either. The one figure any of them gives is the
// longest input gpt-4o-mini-tts takes, written into the paragraph above the
// list, and that is read. An embedding model has no output ceiling at all,
// since what it returns is a vector rather than tokens.
//
// Nor is there a per-model rate limit table anywhere off the model pages. The
// rate limits guide explains the tiers, the response headers and the
// qualification thresholds and then sends the reader to the model pages and to
// the console for the numbers themselves, so a model without a page has no
// stated limit and there is no second document to try. The compare page lists
// four models and repeats what their own pages already say.
//
// What this package does not emit: a model OpenAI has shut down. The
// deprecations page splits its announcements into those that have taken effect
// and those that have not, and a model in the first group is left out of the
// catalog entirely, including the twenty that still keep a page of their own. A
// model in the second group is kept, because OpenAI still serves it, still
// prices it and states the dates it was deprecated and will go, and all of
// those are recorded. The page is read for both: it is what marks a deprecated
// model as deprecated and the only place a retirement date is stated, and what
// it says about a shut down model is used only to withhold it. Of the 116
// models emitted, 36 are deprecated and none are shut down.
//
// A withdrawal without a replacement is recorded as one. The Sora rows write a
// rule in the replacement column rather than a model, meaning there is nothing
// to move to, and a cell holding nothing but a rule yields nothing rather than
// a replacement named after the rule.
//
// A pointer to a shut down model is left as OpenAI wrote it. Six models list a
// default snapshot or a snapshot that is now absent from the catalog, gpt-4
// naming gpt-4-0314 among them, because the page states that snapshot and
// dropping the pointer would report a lineage OpenAI does not describe. The
// same page states a recommended replacement for most models it withdraws, and
// those name current models.
//
// Forty of the emitted models carry no capability, modality or bound at all.
// Twenty-four are dated snapshots and family aliases named only in the
// deprecations table, six are rate table rows with no page, four are the tool
// rows and six are the fine-tuning rows, and OpenAI
// describes none of them anywhere but as a row of numbers. Nothing is dropped
// from a page that has one: every capability, modality, bound and rate limit
// the ninety-six pages state is recorded, and the shortfall against the model
// count is exactly the models with no page.
//
// Nor does OpenAI publish a display name for a snapshot or a deprecated
// model. Every model it currently sells is named on a page of its own, and the
// dated snapshots of those models and the ones on their way out appear only in
// the deprecation table, which lists identifiers and dates and no names. That
// is why 22 of the 67 chat models carry none: 16 are deprecated and the other
// six are the rate table rows that have no page.
package openai
