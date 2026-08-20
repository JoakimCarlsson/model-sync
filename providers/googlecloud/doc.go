// Package googlecloud parses the model documentation of Google Cloud's own AI
// services, which today means Speech-to-Text.
//
// It exists because Google sells models under three names and this catalog had
// only two of them. The Gemini API is read by the google package and Vertex AI
// by the vertexai package, and neither carries a transcription model: Gemini
// transcribes as a side effect of being multimodal, and Speech-to-Text is a
// service of its own with its own models, its own identifiers and its own
// price list. A consumer looking for a model that writes speech down found
// nothing under either name.
//
// Two documents are read and they do not agree with each other.
//
// The model page lists what the v2 API accepts today: chirp_3, chirp_2 and
// telephony, each with a sentence saying what it is for. It states no rate.
//
// The pricing page states the rates, and states them per category rather than
// per model: one rate for recognition, one for dynamic batch, two for the v1
// API depending on whether the audio is logged, and two for the medical
// models. Which models a category covers is written in a footnote under the
// table, and those footnotes name the older set — default, command_and_search,
// latest_short, latest_long, phone_call, video and chirp for the standard
// categories, medical_conversation and medical_dictation for the medical ones.
// So the models come from both documents and the rates reach only the models a
// footnote names.
//
// That leaves the three models the model page lists with no rate, which is
// what Google publishes: its pricing page has not been rewritten for them, and
// it names the models each category covers precisely enough that reading the
// standard rate onto chirp_3 would be this parser's claim rather than Google's.
// Each of them carries a note saying so, so the gap reads as a gap in the
// source and not as a model that is free.
//
// A rate carries the SKU it is billed under as a dimension. Google publishes
// one per category, and it is what a bill cites, which makes it the one thing
// that tells two rates of one model apart without ambiguity. The volume band
// rides along the same way: recognition costs less per minute the more minutes
// an account sends in a month, and the band is the condition rather than a
// separate rate.
//
// What Google does not publish here: a context window, a maximum audio length
// or a list of languages per model on either page. The languages are on a
// third page, one enormous table per model family, and it states support for a
// language rather than anything about the model.
package googlecloud
