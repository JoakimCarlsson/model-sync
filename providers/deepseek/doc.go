// Package deepseek parses DeepSeek's pricing page, change log, API guides,
// rate limit page and Hugging Face model cards into the catalog model.
//
// DeepSeek publishes two models and describes them in a transposed table:
// models are columns and each row is one fact about both. The table also uses
// spanning cells, so a row can carry one or two section labels ahead of its own
// label, and a fact shared by both models appears once rather than twice. Rows
// are therefore read from the right: the last cells are the per-model values,
// what precedes them names the row, and what precedes that names the section.
// A row with fewer values than there are models states one value that applies
// to all of them.
//
// Every URL on api-docs.deepseek.com needs its trailing slash. Without it the
// site answers with its home page at a 200 and with no redirect, so the fetch
// succeeds and yields a document holding one table of base URLs and no model
// at all.
//
// # Pricing
//
// DeepSeek prices nothing unconditionally. Every rate is stated twice, once
// for the peak period and once for the off-peak period, so the period is a
// dimension on the price rather than a metric of its own, and both amounts are
// recorded. The three sections of the table each span the two rows, which is
// why the section label is carried on the builder: the row beneath it states
// only a period and its amounts. The footnote naming the peak window itself is
// attached to every rate, because a period on a price says nothing without the
// hours the period covers, and DeepSeek states those hours once.
//
// It separates a cache hit from a cache miss rather than charging for cache
// writes, so its cheapest input rate is thirty times below its standard one.
// There is no rate for storing a cached prefix, and none is recorded: DeepSeek
// states no such charge, which is not the same as stating it is free.
//
// # Rows that are not ticks
//
// Two rows are prose where the rest are ticks. The thinking mode row says the
// models support thinking and non-thinking modes and which they default to,
// which is more than a tick can carry, so the sentence is kept whole and the
// capability in it recorded as well. The FIM completion row says the feature
// runs in one mode only, so the capability is recorded from the row's label
// and the restriction from its value.
//
// The JSON output row is recorded as two capabilities, not one. DeepSeek's
// JSON output constrains the answer to being JSON and not to a caller-supplied
// schema, which is the narrower of the two strengths the catalog names, and
// the catalog's convention is that the narrower value is carried alongside the
// broader one and never instead of it.
//
// # The change log
//
// The change log heads the entry for a release with the model's name as
// DeepSeek writes it, "DeepSeek-V4-Pro" against the identifier deepseek-v4-pro,
// and it is read for that. A heading is taken only where it is the identifier
// of a model the pricing page stated, which is what keeps "DeepSeek-V4", the
// heading of the release that announced both models, and the headings of every
// model withdrawn before them, from naming anything. The model version on the
// pricing table is not that name: it is the dated build the identifier
// currently resolves to, which is also what the catalog calls the default
// snapshot.
//
// The dated heading above the entry gives the release date and the paragraph
// below it gives the summary, so all three are matched by one expression and
// read in document order. Only the leading sentence of that paragraph is kept,
// because what follows it is about how to call the model rather than about the
// model.
//
// That sentence is also the only place DeepSeek says how available a model is,
// and it says it in its own words: the V4-Pro entry announces a GA release and
// the V4-Flash entry says the release is in public beta. Those two phrases are
// translated to the catalog's states and nothing else in the entry is read for
// a state, since the rest of the entry describes the release rather than the
// model. DeepSeek publishes no deprecation date, no retirement date and no
// replacement for either model; it published all three for the models these
// two replaced, in an entry whose heading names no current model.
//
// The change log carries exactly one dated entry per current model, so the
// release date and the last update would be the same day, and only the release
// date is recorded.
//
// # The guides
//
// The Responses API guide states the modalities, and states them of the API
// rather than of a model, which is right here because both models answer the
// one API. Its table of input items says a message carries input_text and
// output_text content parts and then, in the sentence after, that image and
// file inputs are not supported. So the row is read a sentence at a time and a
// sentence denying support is skipped, since the one denying images names
// input_image inside itself. The Anthropic API guide says the same in its own
// table, marking type="text" supported and type="image" not supported, and is
// not read because it would add nothing.
//
// The same guide states a support status against every top-level request
// parameter, which is the only place DeepSeek enumerates parameters at all,
// and against every tool type, which is where the server-side web search is
// stated. Its tables are told apart by their heading row rather than by their
// position, because two of the three head their first column the same way.
//
// The thinking mode guide states the effort levels the mode accepts and, in a
// footnote, that the mode is on by default at a stated effort. It states the
// mapping as identical for both models, so every level is recorded against
// both. It also names the sampling parameters the mode accepts and disregards,
// which is recorded because accepting a parameter without acting on it is not
// something a caller can discover from a response.
//
// The FIM guide is read for the two things the pricing table's FIM row cannot
// carry: the beta base URL the endpoint answers on, and an output ceiling far
// below the chat endpoints'. The FIM API reference names only deepseek-v4-pro
// in its model enumeration where the pricing table marks the feature supported
// on both models. The pricing table is preferred, because it is the page
// DeepSeek maintains as the statement of what each model supports, and the
// reference is not read.
//
// The context caching guide is read because the pricing page charges a cache
// hit and a cache miss without saying what makes one. The cache is on for
// every account without opting in, which is recorded as a capability, and a
// persisted prefix outlives its last use for a stated while, which is recorded
// as an attribute. DeepSeek no longer publishes a minimum cacheable prefix
// length: the guide describes prefixes persisted at request boundaries, at
// detected common prefixes and at fixed token intervals, and states no
// threshold in tokens, so none is recorded.
//
// The rate limit page is the only page stating a limit on how hard the API may
// be called, and the only limit it states is concurrency. There is no requests
// per minute and no tokens per minute anywhere in DeepSeek's documentation,
// which is why neither appears here. What the page adds to the pricing table's
// concurrency row is that the ceiling is counted per account rather than per
// key, that an account with an expanded quota is additionally ceilinged per
// user_id it passes, and that a request which has not begun inference is
// dropped after a stated wait.
//
// # The model cards
//
// The API documentation never says that a model is open, under what licence,
// or where its weights are. The Hugging Face card does. It is one card written
// for the series and served from each repository in it, so it names every
// model of the series in a download table with the parameter counts, the
// context length, the precision the weights are released in and the link to
// each repository.
//
// The licence is the one thing the card states about its own repository rather
// than about the series, since it is declared in the front matter, so it is
// recorded only against the model that repository holds. That is why both
// cards are fetched even though their bodies are the same document.
//
// # What DeepSeek does not publish
//
// Anything beyond text. Neither API guide offers a modality its models accept
// other than text, and the chat completion reference types every message's
// content as text.
//
// A knowledge cutoff. The cards state how many tokens the models were
// pre-trained on and say nothing about when that corpus ends.
//
// A default for max_tokens. The pricing table states the maximum and the chat
// completions reference, asked for the value range and the default, points
// back at the pricing table. Only the maximum is recorded.
//
// A tokenizer name. DeepSeek publishes a tokenizer as a downloadable archive
// and does not name the algorithm behind it.
package deepseek
