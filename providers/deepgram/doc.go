// Package deepgram parses Deepgram's pricing page into the catalog model.
//
// Deepgram publishes no markdown rates: its documentation describes models and
// languages while the numbers live only in the rendered pricing page, so this
// package reads HTML.
//
// Two things about that page need care. Its rate cells often hold two amounts,
// the second struck through, and only the first is what Deepgram charges now;
// reading both would record a withdrawn price as if it were current. And its
// rates are quoted against four different denominators depending on the
// product: per minute of audio for transcription, per thousand characters for
// speech, per thousand tokens for the agent's language model, and per hour for
// some plans.
//
// Rates also vary by plan rather than by anything about the model, so the plan
// is a dimension.
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
// What Deepgram does not publish: a capability list or a context window for any
// model. Its pricing page describes a model in one sentence of prose and its
// documentation enumerates nothing per model. Nor does it state a rate for the
// custom models it trains on a customer's own data, writing "contact sales"
// where an amount goes; that is kept as a note so the model does not read as a
// free one.
package deepgram
