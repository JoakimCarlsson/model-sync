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
// Four documents are read, because what AssemblyAI states about a model is
// spread across four:
//
//   - The models page, for the models themselves, their rates, the bullets of
//     their cards and the section each has to itself further down, where the
//     languages a card only counts are named one code at a time.
//
//   - The two model-selection pages, for the identifier a request names a
//     model by, which the models page never states: it calls a model
//     "Universal-3.5 Pro" throughout and only these pages say that an API
//     takes "universal-3-5-pro". The streaming one also carries a capability
//     matrix, a row per capability and a column per model, which is the only
//     place several capabilities are answered per model rather than described
//     in a sentence. Its cells answer in a word rather than in a yes, and a
//     model whose multilingual cell says "per turn" rather than "native code
//     switching" is saying it does not follow a speaker who switches
//     mid-utterance, so the word is what is matched.
//
//   - The pricing page, for the add-on rates the documentation only points at.
//     An add-on is priced per model rather than once, in a column per model,
//     and a column carrying a rate says two things: the add-on can be had with
//     that model, which is a capability that model records, and what it costs
//     there. A cell reading "Not supported" says the opposite and records
//     nothing.
//
//   - The FAQ answer bounding what may be submitted, which is the only bound
//     AssemblyAI states at all.
//
// The pricing page names the streaming models differently from the
// documentation, selling as "Universal-3.5 Pro Realtime" and
// "Universal-Streaming" what the documentation lists as "Universal-3.5 Pro
// Streaming" and "Universal-Streaming English". Neither name is a variant of
// the other, so the two vocabularies are mapped to each other by name rather
// than matched loosely. Without that the medical add-on would carry only its
// pre-recorded rate and read as unavailable to a voice agent, which is not
// what either page says.
//
// What this does not read from the pricing page: the products sold there that
// are not models, being the synchronous endpoint and the voice agent API,
// which are ways of reaching a model this catalog already holds rather than
// models of their own; the speech understanding and safety features, each of
// which is a rate for something done to a transcript rather than for a model;
// and the LLM gateway tables, where AssemblyAI resells GPT, Claude, Gemini and
// Qwen models at per-token rates under display names with no identifier beside
// them.
//
// How the bounds are recorded:
//
//   - No context window and no output ceiling, for any model. Those two bound
//     a prompt and a completion, and nothing here is given a prompt or
//     produces a completion: a request carries audio and gets back the words
//     that were in it. Neither field is missing here; neither has anything to
//     hold. The one bound AssemblyAI does state is on the audio itself, and it
//     states it once for the endpoint rather than once per model: five
//     gigabytes and ten hours per file, at
//     https://www.assemblyai.com/docs/faq/are-there-any-limits-on-file-size-or-file-duration-for-files-submitted-to-the-api,
//     which every pre-recorded model records. A streaming model is given a
//     connection rather than a file and no page bounds one.
//
//   - The other bounds its cards state are counts: how many terms may be
//     supplied to bias a transcription, and how many languages a model covers.
//     Where a card counts the languages and the model's own section names
//     them, both are kept, since AssemblyAI's count and its list do not always
//     agree: Universal-2's card claims 99 languages where its table enumerates
//     a hundred and some, several of them dialects of English.
//
// What AssemblyAI does not publish:
//
//   - A capability list, as a list. A card's bullets are sentences with the
//     size of the thing inside them, as "Keyterms prompting up to 200 words"
//     and "6 languages: en, es, pt, de, fr, it", so each is split where the
//     sentence divides: the capability it names, the ceiling it states and the
//     languages it lists all go where a consumer can use them, and the
//     sentence itself is not kept. A capability list holding sentences is one
//     nothing can be keyed on.
//
//     Most bullets survive none of that. A card is sales copy with
//     specification mixed into it, and "Good balance of speed and
//     cost-effectiveness" states nothing to record, so it is dropped. What the
//     bullets leave short is filled from the capability matrix and the add-on
//     tables, which answer per model.
//
//   - That a streaming model transcribes a live connection, other than by
//     listing it under a heading that says so. The heading is what that
//     capability is read from, since a card is free to spend all five bullets
//     on something else.
package assemblyai
