// Package assemblyai parses AssemblyAI's models page into the catalog model.
//
// AssemblyAI sells transcription only, so nothing it publishes is priced by
// token. Its two rates are both per hour, but they meter different things: a
// pre-recorded model is charged by the hour of audio submitted, and a
// streaming one by the hour the connection stays open whether or not audio is
// flowing. They are recorded as different metrics because they are not
// comparable, and a reader that treated both as "per hour" would understate a
// voice agent that holds a socket open between utterances.
//
// The models themselves are not in a table. They are MDX cards carrying a
// title and a list of capabilities, and only the rate tables are markdown, so
// the two are read separately and joined on the display name.
//
// Everything it sells hears audio and writes text, so that is what every model
// records as its modalities.
//
// One model is priced elsewhere. The documentation says of the medical add-on
// only that it is billed separately and points at the pricing page, which is
// HTML and quotes an add-on once per model it runs with, in a column each. The
// column heads name models by the same display name, so the mode each of those
// models was recorded under becomes the rate's dimension — an hour of audio for
// a pre-recorded model, an hour of connection for a streaming one.
//
// What this does not read from that page: the add-on columns headed by a model
// the documentation does not list under that name, since there is nothing to
// tie the rate to; and the LLM gateway tables, where AssemblyAI resells GPT,
// Claude, Gemini and Qwen models at per-token rates under display names with no
// identifier beside them.
//
// What AssemblyAI does not publish:
//
//   - A capability list. A card's bullets are sentences with the size of the
//     thing inside them, as "Keyterms prompting up to 200 words" and "Support
//     across 99 languages", so they are kept verbatim under capabilities rather
//     than reduced to feature names, which would invent a vocabulary AssemblyAI
//     never used and state less than the sentence did.
//   - Any bound at all: no context window, no token limit and no ceiling on the
//     length of an audio file. Nothing it sells is billed or bounded by tokens,
//     so the fields that hold those bounds stay empty for every model.
package assemblyai
