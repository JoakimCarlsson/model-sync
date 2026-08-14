// Package deepgram parses Deepgram's pricing page and feature overviews into
// the catalog model.
//
// Deepgram splits what it publishes in two. The rates live only in the
// rendered pricing page, which names the models, so this package reads HTML
// for them. Everything a model can do lives in the documentation, which is
// published as markdown at the same path with .md appended, and which names no
// rate. Neither document alone describes a model.
//
// Three things about the pricing page need care. Its rate cells often hold two
// amounts, the second struck through, and only the first is what Deepgram
// charges now; reading both would record a withdrawn price as if it were
// current. Its rates are quoted against four different denominators depending
// on the product: per minute of audio for transcription, per thousand
// characters for speech, per thousand tokens for the agent's language model,
// and per hour for some plans. And a rate cell that reaches down several rows
// is how it prices four audio intelligence models at once, so a row holding a
// name and nothing else is a model priced above it rather than a model with no
// rate. Reading the row alone lost three of the four.
//
// Rates also vary by plan rather than by anything about the model, so the plan
// is a dimension.
//
// An introductory rate is written into the same cell as the rate that replaces
// it, as "$0.110/min through 9/12" and "Then $0.146/min", or as "Free until
// 9/12" where the offer is that there is nothing to pay. The cell divides at
// the word introducing the successor: what comes before it is dimensioned as
// promotional and noted with the date it lapses, what comes after is the
// standard rate, and an offer stating no amount is recorded as zero against
// the unit its successor is quoted in.
//
// Two cells hold a word where the others hold an amount. "Included" is a rate
// of nothing, charged on top of the transcription the add-on runs on, and is
// recorded as zero with a note rather than left looking unpriced. "Contact
// Sales" is Deepgram declining to publish one, and is recorded as an attribute
// against a model that then correctly carries no price.
//
// What a model takes and returns is said by the product heading its table sits
// under: speech to text hears and writes, text to speech reads and speaks, an
// agent does both at once, and an add-on or an intelligence feature runs on the
// audio and answers in text.
//
// Capabilities are stated once per product rather than once per model, in a
// feature overview per product, so the page a model's product is documented on
// is what says what it supports. Flux has an overview of its own because it
// supports a different set from the models sold beside it: it times words and
// takes keyterms but does not diarize or smart-format, and two of its rows
// apply to the multilingual model alone, as does the codeswitching row on the
// speech-to-text pages. A row restricted that way is recorded only against the
// multilingual model of its pair.
//
// The add-ons and the audio intelligence features are those same capabilities
// sold in their own right: the pricing page lists Speaker Diarization as a
// thing with a rate and the documentation lists it as a thing a transcription
// can do, so the two are joined on the name, which is all either states. That
// also settles whether one of them runs on a live connection or only on a
// recording, since Deepgram documents the two separately: diarization is on
// both pages and summarization only on the pre-recorded one.
//
// Not every row of an overview is a capability. Encodings, sample rates,
// containers, callbacks and interim results are how a request is shaped rather
// than what a model can do, and they are dropped rather than recorded as
// features. They are not recorded as parameters either: the tables label a
// feature by its display name and never by the query parameter that switches
// it on, so nothing keyed on a parameter name could be got from them.
//
// What this does not read: the language tables in the models and languages
// overview. They are keyed on the model option a request names, "nova-3" with
// a language of "multi", while the pricing page sells that same option twice
// under two marketing names, so joining the two would rest on a mapping
// neither document states.
//
// What Deepgram does not publish:
//
//   - A rate for the custom models it trains on a customer's own data, writing
//     "contact sales" where an amount goes on the only page that quotes
//     amounts. That is kept as a note so the model does not read as a free one.
//
//   - A growth-plan rate for speaker diarization as its own cell. The cell
//     beside it reaches down a row to cover it, which is where that rate comes
//     from.
//
//   - A context window or an output token ceiling for anything it sells, and
//     neither would mean anything if it did. Transcription is billed and
//     bounded by minutes of audio, speech by characters of text, and the agent
//     by minutes of connection; the only model with a token rate at all is
//     audio intelligence, which summarizes a transcript Deepgram has just
//     produced rather than a prompt a caller sends. The two bounds it does
//     state are the ceiling on the text one speech request may carry, 2000
//     characters for either Aura generation, and the two hours after which the
//     Voice Agent API closes a session.
//
//   - An embedding width, a reasoning setting or a structured output mode,
//     none of which apply to anything it sells. The one capability of that
//     family it does have is function calling, which the Voice Agent API
//     documents as a pair of messages rather than as a feature.
package deepgram
