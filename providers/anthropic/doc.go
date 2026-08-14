// Package anthropic parses Anthropic's published documentation into the
// catalog model.
//
// Nothing here is shared with any other provider, and the differences from
// OpenAI are the reason. Anthropic quotes rates as "$10 / MTok" with the
// denominator inside the cell, treats cache lifetime as a pricing dimension
// (5m writes cost more than 1h reads), names models by display name and states
// the API identifier somewhere else entirely, writes MDX rather than plain
// markdown, glues footnote digits onto values ("May 20262"), and lists two
// models in one cell when they share a rate. Its capability table is also
// transposed: models are columns and features are rows.
//
// Six documents are read, in this order, because the later ones depend on
// identifiers the earlier ones establish:
//
//   - the deprecations page, whose status table is the only place every
//     model's real API identifier appears, along with whether it is retired
//     and when. Without it a retired model is filed under a guess and looks
//     current, and a current model whose display name is all the pricing page
//     writes resolves onto nothing.
//   - the model overview, whose transposed table is authoritative for current
//     models and carries context windows, cutoffs and capabilities.
//   - the pricing page, whose standard, batch and fast-mode tables carry the
//     rates, and whose prose carries the server-side tool prices. It names
//     models only by display name, which is why it is read after both pages
//     that state identifiers.
//   - the tool reference, a directory of the tools Anthropic provides. The
//     pricing page names a server tool only where it charges for one and says
//     nothing else about it; this page states each tool's type identifiers,
//     whether Anthropic or the caller executes it, and whether it is generally
//     available. It names tools by page title, which slugs onto nothing, so a
//     row is filed under its type with the release date dropped: web_search
//     rather than "Web search tool".
//   - the structured outputs guide, whose compatibility block lists the API
//     identifiers the capability is available on. The comparison table has no
//     row for it, and this list is the only place Anthropic states it per
//     model.
//   - the tool use guide, which states tool calling of Claude rather than of
//     any list of models: tool use lets Claude call the functions a caller
//     defines, with no exception named for any model. Its scope is therefore
//     the same as the modality sentence's, every chat model still served.
//
// What a model takes and returns is not in the comparison table. The overview
// states it once, in a sentence above the table saying that every current model
// takes text and images and returns text, and that sentence is the only place
// Anthropic states it outside the Models API, which needs a key. It is read and
// then applied after every document, with its own scope: every chat model still
// served, including the one the overview describes in prose and only the
// pricing page names. Its mention of vision is the image modality under another
// word and is not recorded twice.
//
// One model's bounds are stated the same way. Claude Mythos 5 is offered to
// approved customers rather than generally, so it has no column in the
// comparison table and one sentence instead: it "shares Claude Fable 5's specs
// and pricing". That is Anthropic stating its context window and output ceiling
// in the only place it states them, so both are taken from the model named,
// along with its capabilities. Its rates are not, because the pricing page
// lists the model itself. Like the modality sentence, this is applied after
// every document, since the model reaches the catalog only when that page names
// it.
//
// Two more bounds are stated in prose under the comparison table rather than in
// it. A note names the models that produce up to 300k output tokens on the
// Batches API behind a beta header, which is a second ceiling rather than a
// correction: the table's Max output row remains the synchronous Messages API's
// and both are kept. The note shortens every name after the first, writing
// "Claude Opus 5, Opus 4.8", which resolves anyway because the index those are
// looked up in drops the vendor prefix. On the pricing page, the code execution
// rate applies only past a monthly free allowance, which is pricing the amount
// alone does not state and is kept as the price's note.
//
// A context window, unlike that output ceiling, has one value and needs no
// second key. Anthropic once gated a 1M window behind a beta header and no
// longer does: the context windows page says that Opus 5, Opus 4.8, Opus 4.7,
// Opus 4.6, Sonnet 5 and Sonnet 4.6 have "a 1M-token context window on the
// Claude API, Amazon Bedrock, Google Cloud, and Microsoft Foundry", that for
// every one of them "1M is the default: you don't need a beta header", and that
// "Other Claude models, including Claude Sonnet 4.5, have a 200k-token context
// window". The comparison table states the same numbers, so it stays the only
// source read for them, and the 200k it gives Sonnet 4.5 is the whole window
// rather than the standard half of one. Nothing above 200k prompt tokens costs
// more either: the pricing page's long context section says the full window
// bills "at standard pricing", which is why no rate here carries a band. A
// catalog reporting 1M for Sonnet 4.5 is reporting the withdrawn beta.
//
// What the catalog carries is what Anthropic still serves. The status table
// defines four states, and they do not all mean the same thing: active is fully
// supported, legacy stops receiving updates, deprecated is still functional but
// no longer recommended and has a retirement date, and retired means requests
// fail. Only the last of those is a model nobody can call, so only the last is
// dropped, along with a shutdown endpoint, which is the same fact said of the
// endpoint rather than of the model. A legacy or deprecated model is kept:
// Anthropic still sells it and still prints its rate, and a catalog that hid it
// would be missing a model a reader can use today.
//
// Dropping happens at the end rather than at the row. The deprecations page has
// to be read in full whatever the outcome, because it is the only page writing
// an API identifier for a model the overview has stopped listing, and those
// identifiers are the index a pricing row's display name resolves against. So a
// retired row is parsed, indexed, and then left out of the result, and the store
// deletes the file of anything the parser stops emitting.
//
// What Anthropic does not publish:
//
//   - Anything about a server-side tool beyond its rate, its versions, where it
//     executes and whether it is GA. A tool is not a model and has no context
//     window, no output ceiling, no modalities and no model capabilities: the
//     pricing page states a rate, the tool reference states the rest, and the
//     individual tool pages state request parameters and error codes. So the
//     three tool entries carry a price, a name and the directory's facts, and
//     nothing that would fill a model's fields.
//   - Claude Mythos Preview. The overview names the identifier in prose and the
//     structured outputs guide lists it, but no document states a rate, a
//     bound, a cutoff or a comparison column for it, since access is
//     invitation-only. It is therefore not created, the same way the capability
//     guides create nothing: a page saying a model supports something is not a
//     page saying the model exists on terms anyone can read.
package anthropic
