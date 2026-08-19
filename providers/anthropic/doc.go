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
// Sixteen documents are read. The first four are ordered, because each depends
// on identifiers the ones before it establish, and the rest are read after
// them in any order, because each attaches something to models and tools those
// four have already created and none of them establishes one of its own:
//
//   - the deprecations page, whose status table is the only place every
//     model's real API identifier appears, along with whether it is retired
//     and when. Without it a retired model is filed under a guess and looks
//     current, and a current model whose display name is all the pricing page
//     writes resolves onto nothing.
//   - the model overview, whose transposed table is authoritative for current
//     models and carries context windows, cutoffs and capabilities.
//   - the tool reference, the directory of the tools Anthropic provides. It is
//     read before the pricing page rather than after it because it is the
//     register of which tools exist; the pricing page is the register of which
//     ones are billed for, which is a smaller set.
//   - the pricing page, whose standard, batch and fast-mode tables carry the
//     rates, whose prose carries the server-side tool prices, and whose tool
//     use table carries the size of the system prompt any tool at all adds. It
//     names models only by display name, which is why it is read after every
//     page that states identifiers.
//   - the release notes, the only document that dates a model. The comparison
//     table states what a model knows and the deprecations page states when it
//     goes away; neither states when it arrived, and the changelog does, to
//     the day, for every model the catalog carries and for most of the tools.
//   - the rate limits page, the only document bounding how much of a model an
//     organization may use, tabulated in full for three of the four usage
//     tiers.
//   - the thinking troubleshooting page, whose first section is a per-model
//     table of thinking configurations. The comparison table answers Yes or No
//     to two thinking rows; this states which modes a model accepts and which
//     one a request that asks for nothing gets, which the comparison table
//     cannot express and no other page states.
//   - the context window guide, read for the one bound only it states: how
//     many images or PDF pages a single request may carry.
//   - the effort guide, which states both the models the effort parameter
//     works on and, level by level, which of the five levels each model
//     accepts.
//   - the fast mode page, which names the two models the pricing page's fast
//     tier exists for. The rate is on the pricing page and the list of models
//     it applies to is here.
//   - the service tiers page, which states Priority Tier's scope by
//     subtraction: every available model less four it names.
//   - the structured outputs, computer use, compaction and task budgets
//     guides, each of which opens with a compatibility block listing the API
//     identifiers it is available on and the beta header it needs.
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
// and pricing". That is Anthropic stating its context window and output
// ceiling in the only place it states them, so both are taken from the model
// named. Nothing else is. Its rates are not, because the pricing page lists the
// model itself; its modalities are not, because the sentence covering every
// current model already reaches it; and its capabilities are not, because every
// guide that lists identifiers lists this one by name, and one of them lists it
// on the other side. Priority Tier names Claude Mythos 5 among the models it is
// not available on while Claude Fable 5 carries it, so copying across would
// contradict a document rather than stand in for a missing one.
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
// That page states one bound the comparison table does not, and states it
// against the window rather than against a model: a single request may carry up
// to 600 images or PDF pages, "100 for models with a 200k-token context
// window". Every model's window is already known, so the sentence resolves to a
// number for each of them without anything being assumed about any of them.
//
// Rate limits are a large table Anthropic publishes and the catalog carried
// none of. They are per model, per usage tier, and three numbers rather than
// one: requests, input tokens and output tokens, each per minute. The three are
// kept apart because Anthropic keeps them apart, and because its input limit
// counts only uncached tokens, so a tokens-per-minute figure adding input to
// output would describe no limit the API enforces. The tiers are named rather
// than numbered, so the key is rpm_tier_start rather than rpm_tier_1, and the
// fourth tier, Custom, is arranged with an account team and publishes nothing,
// so it yields no key at all. Two rows of every table name a generation rather
// than a model, because Anthropic meters one bucket across four Opus versions
// and two Sonnet versions; the footnote each row's asterisks point at names
// them, so the row is expanded onto each model named and the sharing is kept as
// a note. A row whose footnote goes missing names nothing that exists and is
// dropped, which is the right outcome: a family is not a model.
//
// Release dates come from the changelog and only from there. It is written
// newest first and an entry names models other than the one it announces, so
// neither position nor mere presence identifies the subject: the entry
// launching Claude Opus 5 also names Claude Opus 4.8, and reading every name
// would date the older model to the newer one's launch. Only a name reached
// through one of the two phrases that announce a release is read, "We've
// launched" and, for a model shipped the same day as another, "alongside". The
// same entries carry the only one-line description Anthropic publishes for a
// model that is no longer current: the comparison table has a Description row
// and the legacy table below it does not, so for six of the eleven models the
// appositive after the name in the changelog is the description, and it is read
// up to the comma or full stop that closes it.
//
// There is no last_updated to record, and its absence is a fact rather than a
// gap. The overview says that "every Claude model ID is a pinned snapshot", and
// that the dateless identifiers introduced with the 4.6 generation are pinned
// too rather than evergreen pointers. A pinned snapshot does not change after
// it ships, so the day it shipped is the only day it has, and a last_updated
// copied from release_date would assert a second fact where Anthropic states
// one.
//
// Thinking and effort are the two controls Anthropic documents per model, and
// each is stated in a register the comparison table cannot hold. The table's
// two thinking rows answer Yes or No, which cannot say that a model has
// thinking permanently on, or that it accepts the older mode but rejects the
// newer one. The troubleshooting table says that, per model, in three
// columns of which two are read: the modes and the default. The third states
// which values return a 400, the same fact from the other side, and carrying
// both
// would mean keeping two spellings of one thing in agreement. Effort is a
// parameter rather than a capability, so it is recorded as one, and its levels
// are recorded per model because Anthropic states them per model: two of the
// five rows of the level table narrow their own availability to a named list,
// and the other three state no restriction, so their scope is the parameter's,
// which its compatibility block lists outright.
//
// A beta header is not a capability and is not recorded as one. It is the
// condition on a capability, so it is kept as a note against the feature it
// gates rather than as a second entry beside it, and the beta headers page
// itself is not read: it explains the mechanism and lists nothing per model.
//
// Every tool in the directory is carried, whether or not a rate names it. This
// is a change from filing only the priced ones, and the reason is that a rate
// is a fact about a tool rather than the thing that makes it one. The directory
// states a type to send, which side executes it, whether it is generally
// available or behind a header, and which versions are still accepted, which is
// four published facts about something a caller can use today. Web fetch is the
// case that settles it: it has its own page, its own versions, and a sentence
// on the pricing page saying it is "available on the Claude API at no
// additional cost", which is a published rate of zero and not a missing one. It
// is recorded as such, so that a reader can tell a free tool from a tool whose
// price nobody has read. The other six unpriced tools carry no rate at all, and
// a consumer counting that as a gap is counting the absence of something
// Anthropic never charged for.
//
// The directory names a tool by its page title, "Web search tool", which is
// neither the identifier its rate is filed under nor anything that slugs onto
// one. Its type column is, once the release date is dropped:
// web_search_20260318 becomes web-search. So the title becomes the name and the
// type becomes the identifier, and the pricing page's shorter wording no longer
// overwrites the directory's.
//
// What the catalog carries is what Anthropic still serves. The status table
// defines four states, and they do not all mean the same thing: active is fully
// supported, legacy stops receiving updates, deprecated is still functional but
// no longer recommended and has a retirement date, and retired means requests
// fail. Only the last of those is a model nobody can call, so only the last is
// dropped, along with a shutdown endpoint, which is the same fact said of the
// endpoint rather than of the model. A legacy or deprecated model is kept:
// Anthropic still sells it and still prints its rate, and a catalog that hid it
// would be missing a model a reader can use today. A tool's Status column says
// the same thing in its own words, GA or Beta, and is recorded in both: the
// column's own word, and the lifecycle it means, so that a consumer filtering
// models and tools together does not have to know two vocabularies.
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
//   - A tool's bounds. A tool is not a model and has no context window, no
//     output ceiling, no modalities and no model capabilities. The directory
//     states its versions, its execution side and its status, the pricing page
//     states a rate where there is one, and the individual tool pages state
//     request parameters and error codes. Nothing there fills a model's fields,
//     and nothing should.
//   - Claude Mythos Preview. The overview names the identifier in prose and
//     five capability guides list it, but no document states a rate, a bound, a
//     cutoff or a comparison column for it, since access is invitation-only. It
//     is therefore not created, the same way the capability guides create
//     nothing: a page saying a model supports something is not a page saying
//     the model exists on terms anyone can read.
//   - A per-model thinking budget. The extended thinking guide states a
//     minimum of 1,024 tokens and states it of the API, with no model named and
//     no model excepted. It is one number about a parameter, not a column, and
//     recording it against eleven models would turn one published fact into
//     eleven.
//   - A Priority Tier rate. The tier is recorded as a capability because the
//     service tiers page names the models it reaches, but the pricing page has
//     no Priority Tier table, and the service tiers page says capacity
//     commitments "are no longer available for purchase". A tier nobody can buy
//     has no list price to read.
//   - A named tokenizer. The pricing page says that Claude 4.7 and later models
//     use "a newer tokenizer" producing about 30% more tokens for the same
//     text, and that earlier models use "the previous" one. That is a
//     comparison between models rather than a name for either tokenizer, and
//     there is nothing to put in a tokenizer field but Anthropic's adjectives.
//   - A separate rate for a long prompt, a US-only request or a regional
//     endpoint. Long context bills "at standard pricing". US-only inference and
//     the partner clouds' regional endpoints are multipliers on every rate at
//     once, 1.1x and 1.1x, rather than rates of their own, and a price is a
//     number here rather than a formula.
package anthropic
