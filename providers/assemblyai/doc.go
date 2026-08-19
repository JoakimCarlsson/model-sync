// Package assemblyai parses AssemblyAI's documentation into the catalog model.
//
// AssemblyAI sells four things, and only two of them are models in the sense
// this catalog means:
//
//   - Speech-to-text models it trains and serves itself, priced by the hour.
//     Its two hourly rates meter different things: a pre-recorded model is
//     charged by the hour of audio submitted, and a streaming one by the hour
//     the connection stays open whether or not audio is flowing. They are
//     recorded as different metrics because they are not comparable, and a
//     reader that treated both as "per hour" would understate a voice agent
//     that holds a socket open between utterances.
//
//   - Models from Anthropic, Google, OpenAI and Alibaba, resold through the
//     LLM Gateway and priced per million tokens. AssemblyAI publishes a
//     roster for these with an identifier, a context window and a rate per
//     model, which is more than it publishes about the models it trains
//     itself, so they are entries here. What it does not publish about them
//     is what any reseller leaves out: no release date, no output ceiling, no
//     statement of which of them reason.
//
//   - Things done to a transcript once one exists, sold by the hour and split
//     into Speech Understanding, which extracts something, and Guardrails,
//     which removes something. AssemblyAI heads these on its pricing page
//     with the word "Models" and its documentation calls them models too, and
//     each has a page of its own stating which speech models it may be turned
//     on for, which languages it covers and which regions serve it. They are
//     entries. What they are not is a model that can be asked for on its own:
//     each runs on a transcript some speech model produced, and that
//     statement is recorded twice, once as the feature's list of the models
//     it runs on and once as a capability on each of those models, because a
//     consumer asking what a model can do and a consumer asking where a
//     feature can be used are asking about the same sentence.
//
//   - Ways of reaching a model it already sells: the Sync API, which is
//     Universal-3.5 Pro behind a single call, and the Voice Agent API, which
//     is a streaming model, a gateway model and a voice bundled at one rate.
//     Neither is an entry, because neither is a model, and entering them
//     would double every rate already recorded under the model they run.
//
// The transcription models are not in a table. They are MDX cards carrying a
// title and a list of capabilities, and only the rate tables are markdown, so
// the two are read separately and joined on the display name. The gateway
// models are the opposite: two tables keyed by the identifier, one stating
// what each model is and one what it costs, which join without any name
// matching at all.
//
// # The documents
//
// The models page, at getting-started/models.md, for the transcription models
// themselves, their rates, the bullets of their cards and the section each has
// to itself further down, where the languages a card only counts are named one
// code at a time.
//
// The two model-selection pages, for the identifier a request names a model
// by, which the models page never states: it calls a model "Universal-3.5 Pro"
// throughout and only these pages say that an API takes "universal-3-5-pro".
// The streaming one also carries a capability matrix, a row per capability and
// a column per model, which is the only place several capabilities are
// answered per model rather than described in a sentence. Its cells answer in
// a word rather than in a yes, and a model whose multilingual cell says "per
// turn" rather than "native code switching" is saying it does not follow a
// speaker who switches mid-utterance, so the word is what is matched.
//
// The supported-languages page, for the dialects a language code covers.
// Universal-3.5 Pro's own section names eighteen codes; this page names
// twenty-one, because it is where en_au and en_uk are written down.
//
// The LLM Gateway models page, for the resold models. It is the only document
// pairing a display name with an identifier: the pricing page sells "Sonnet 5"
// and "GPT-5.6 Terra" with no identifier anywhere near them.
//
// The chat completions specification, for the two things the gateway roster
// leaves unsaid: that a message carries text and comes back as text, which is
// read from the sentence bounding a content part rather than assumed, and the
// endpoint the models are reached at.
//
// The three rate limit pages, one per product. AssemblyAI bounds an account
// rather than a model, and each product is bounded in a different quantity:
// how many files may transcribe at once, how fast streaming connections may be
// opened, how many requests a gateway model takes a minute. Every model of a
// product carries its product's figure, since an account transcribing with
// Universal-2 and one transcribing with Universal-3.5 Pro are bounded
// identically and a consumer asking what a model may be driven at would
// otherwise find nothing. Where a tier is stated as "200+" the figure is
// recorded under a key saying it is a floor, because "200+" is not the number
// two hundred.
//
// The two FAQ answers that bound and describe a transcript: the one stating
// what may be submitted, and the one stating that a completed transcript times
// every word rather than every segment and how far a time may be out. Both are
// stated once for the endpoint rather than once per model, and both are
// recorded against every model that endpoint serves.
//
// The fourteen feature pages, one per thing done to a transcript. Each answers
// the same three questions in the same shape, so one parser reads them all.
//
// The pricing page, last, because a feature page is what says a feature exists
// and the pricing page only says what it costs. Its tables come in two shapes
// and nothing inside a table says which product it belongs to, so each is
// attributed to the heading above it. An add-on table is priced per model in a
// column per model, and a column carrying a rate says two things: the add-on
// can be had with that model, which is a capability that model records, and
// what it costs there. A cell reading "Not supported" says the opposite and
// records nothing. A product table is one row per thing sold, read for the
// sentence describing each model under every heading, and for the rate as well
// under the two feature headings.
//
// # Where documents disagree
//
// The pricing page names the streaming models differently from the
// documentation, selling as "Universal-3.5 Pro Realtime" and
// "Universal-Streaming" what the documentation lists as "Universal-3.5 Pro
// Streaming" and "Universal-Streaming English". Neither name is a variant of
// the other, so the two vocabularies are mapped to each other by name rather
// than matched loosely. Without that the medical add-on would carry only its
// pre-recorded rate and read as unavailable to a voice agent, which is not
// what either page says. The same mapping carries PII redaction, which the
// documentation writes one page about and the pricing page sells as two rows,
// one for the transcript and one for the audio: one entry, two rates, told
// apart by a dimension.
//
// The pricing page and the gateway roster disagree about the gateway. The
// roster prices qwen3.5-4b-32k-fast at nothing and the pricing page at ten
// cents a million; the pricing page sells a Gemini 3.7 Flash the roster does
// not list. The roster wins and the pricing page's gateway tables are not read
// for rates, because a rate without the identifier it belongs to cannot be
// attached to anything, and a model sold under a display name the roster never
// mentions cannot be entered without inventing the identifier a caller would
// have to send. The disagreement is why the roster is preferred rather than
// merged.
//
// Not read at all: the two reference documents AssemblyAI serves at
// /llms/models.md and /llms/pricing.md for the benefit of language models.
// They are dated in their own front matter, months behind, and they contradict
// the documentation on the identifiers a request must carry, calling the
// streaming model "u3-rt-pro" where every current page calls it
// "universal-3-5-pro". A document that is wrong about the one string a caller
// cannot get wrong is not a source, whatever else it happens to know.
//
// Also not read: the changelog. It is a dated record of every model launch,
// and it announces a model the way a headline does rather than by identifier:
// "Universal-3.5 Pro Async" and "Universal-3.5 Pro Realtime" are two entries
// three weeks apart naming two models whose display names contain each other,
// and a gateway launch usually names the model in prose without its
// identifier. Dating a model from it would mean guessing which of two models
// an announcement is about, and a guessed release date is worse than none.
//
// # How the bounds are recorded
//
// No context window and no output ceiling for a transcription model. Those two
// bound a prompt and a completion, and a transcription model is given neither:
// a request carries audio and gets back the words that were in it. Neither
// field is missing there; neither has anything to hold. What bounds a
// pre-recorded model is the audio, five gigabytes and ten hours per file, and
// what bounds a streaming model is the connection, which AssemblyAI closes
// after three hours and bills in full. A gateway model has a context window
// and nothing else: the roster states one per model, and no page anywhere
// states how much any of them may write back.
//
// The other bounds the cards state are counts: how many terms may be supplied
// to bias a transcription, and how many languages a model covers. Where a card
// counts the languages and the model's own section names them, both are kept,
// since AssemblyAI's count and its list do not always agree: Universal-2's card
// claims 99 languages where its table enumerates a hundred and some, several of
// them dialects of English.
//
// # What AssemblyAI does not publish
//
// A capability list, as a list. A card's bullets are sentences with the size of
// the thing inside them, as "Keyterms prompting up to 200 words" and "6
// languages: en, es, pt, de, fr, it", so each is split where the sentence
// divides: the capability it names, the ceiling it states and the languages it
// lists all go where a consumer can use them, and the sentence itself is not
// kept. A capability list holding sentences is one nothing can be keyed on.
//
// Most bullets survive none of that. A card is sales copy with specification
// mixed into it, and "Good balance of speed and cost-effectiveness" states
// nothing to record, so it is dropped. What the bullets leave short is filled
// from the capability matrix, the add-on tables and the feature pages, which
// answer per model.
//
// That a streaming model transcribes a live connection, other than by listing
// it under a heading that says so. The heading is what that capability is read
// from, since a card is free to spend all five bullets on something else.
//
// A capability list for a gateway model either. What the roster states per
// model is which request parameters that model accepts, so the capability is
// read from the parameter that carries it: a model taking tools calls them, a
// model taking a response format is constrained by one. Nothing says which of
// them reason, so none of them records reasoning.
//
// A lifecycle for anything. The gateway roster has a retirement date column,
// filled in for one model of twenty-nine and empty for the rest, and that is
// the whole of what AssemblyAI states about a model's life anywhere. No model
// is marked deprecated on a page this reads, so none records a state: the
// models AssemblyAI has retired are simply gone from its tables, which is a
// withdrawal this catalog already represents by deleting the entry.
//
// What a Speech Understanding or Guardrails model takes and returns. The
// pricing page bills them by the hour of audio, the documentation says they
// analyze a transcript, and neither is a statement about modality: a
// transcript is text that arrived as audio, and PII redaction returns audio in
// one of the two forms it is sold in and text in the other. Rather than pick,
// none of them records a modality.
package assemblyai
