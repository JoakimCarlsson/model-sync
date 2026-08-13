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
package deepgram
