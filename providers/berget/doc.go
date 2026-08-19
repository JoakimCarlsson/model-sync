// Package berget parses Berget's model API into the catalog model.
//
// Berget is the first source here that does not price in dollars. It quotes
// euros, which is why a price carries its currency rather than assuming one,
// and a reader comparing it against a dollar provider must convert rather than
// compare the numbers directly. The rate recorded is the one Berget publishes:
// a list price in euros excluding VAT, which its pricing page states outright
// at https://berget.ai/pricing, and which the endpoint's own specification
// pins by declaring the currency field "always EUR". Nothing is converted here
// and no tax is added, so a catalog quoting Berget in dollars carries a
// different number by whatever rate it chose on whatever day; Berget's own
// integration guide tells a reader to do that conversion themselves, which is
// the sense in which the euro figure is the published one.
//
// It serves an open JSON endpoint listing every model with its rate, licence,
// quantization and capabilities, and it covers rerankers and speech alongside
// chat and embeddings. The listing is the only document that names a model, so
// a model Berget adds arrives here on the next sync without anything in this
// package being edited. Three of its fields are deliberately not recorded: the
// endpoint reports live health and latency, which change between any two
// fetches and would rewrite every file on each sync, and a creation timestamp
// that on every model resolves to the day the release date already states.
//
// The endpoint states no context window. The documentation site does, on a
// card per model keyed by the same identifier, so that page is read too and
// the two need no reconciling. It is read through the site's own markdown
// mirror at https://docs.berget.ai/llms-full.txt, which carries every page the
// site serves in one document; the rendered HTML says the same thing behind
// more markup. A model added to the endpoint before the site catches up
// carries no context window until it does. The specification promises
// otherwise, describing the per-model endpoint as answering with "pricing,
// context window, and capabilities", but the schema it answers with declares
// no such field and the served response carries none, so the cards remain the
// only statement of a bound.
//
// A card writes that bound in shorthand, "256k tokens", and Berget states it
// no other way, so the suffix is read as a thousand and such a card is
// recorded as 256000. The figure is Berget's own and is what it serves the
// model at, which is not always what the model's author advertises: Z.ai
// publishes GLM-5.2 with a million-token window and Berget cards it at 256k.
// A reseller serving a model shorter than its weights allow is a fact about
// the deployment, and the deployment is what this provider records.
//
// The same card carries two more things. Its link is the model's weights on
// Hugging Face, recorded as both the model card URL and the repository
// identifier, and its heading is a display name Berget writes itself. The
// heading is kept because it is the one place a lifecycle appears that the
// endpoint's own field does not: the endpoint reports GLM-5.2 as stable and
// the card above it reads "GLM 5.2 (maintenance)". The endpoint's field is
// what the state attribute holds, and the heading is kept beside it so the
// disagreement survives rather than being resolved here.
//
// The API's OpenAPI specification is read for a third thing neither states:
// what a request may carry. It names a parameter once per endpoint rather than
// once per model, and a model of a given kind is served by exactly one
// endpoint, so its kind is what carries the parameters to it, along with the
// path itself, the response shapes the endpoint will return and, for
// embeddings, how many inputs one request may batch. They are recorded as
// parameters and not as capabilities, because naming a knob the API accepts is
// not the same as stating what the model does with it. Three are the
// exception: word-level alignment, diarization and hotword biasing name what
// the model does with the audio rather than a knob on the request, and each is
// something another provider here states as a capability outright, so a speech
// model records all three as capabilities as well. The transcription
// endpoint's prose states the container formats it accepts, which no field
// does, and those are recorded as well.
//
// Which models reason is stated only in prose, never in a field, and in three
// sentences across two documents. The specification's reasoning_effort
// parameter is described as vendor neutral except that thinking cannot be
// disabled on models that always think, naming Kimi K3; its thinking parameter
// is described as controlling reasoning on Moonshot's Kimi K2 and as
// unsupported by Kimi K3; and the guide at
// https://docs.berget.ai/models/choosing-a-model sends analytical reasoning to
// GLM 4.7 "with reasoning mode for lower cost". Berget serves exactly one Kimi
// K2 and one Kimi K3, so all three sentences resolve to a model on the
// listing, and those three are the only models recorded as reasoning. The two
// Kimi sentences also say whether the reasoning can be switched off, which is
// what the reasoning-mandatory attribute records. Nothing here is inferred
// from a model family: a reasoning-capable open model Berget says nothing
// about carries no reasoning capability.
//
// Nor does any of the three documents state a modality. What a model takes and
// returns is read from its type, a speech-to-text model hears and writes, and
// from the vision capability, which is the one capability naming a modality
// rather than an API feature and is recorded as both.
//
// An embedding or reranking model works in text on both sides, and that is what
// both modality lists record. It describes the medium and not the return value,
// which is a vector for one and a relevance score for the other; naming those
// would invent a word no other provider here uses. Recording the input alone
// would leave a consumer unable to tell an unstated output from a model that
// returns nothing, so the two sides are always set together.
//
// Berget's reason for existing is where its data goes, and it states that of
// the platform rather than of any one model: all inference, storage and
// processing happens on European infrastructure, data never crosses borders,
// and it is never used to train models. Every model records those two
// guarantees, and that Berget serves open models, because the statement covers
// the catalogue whole and excludes no model in it.
//
// What Berget does not publish:
//
//   - A bound on output length, for any model. The chat endpoint accepts
//     max_tokens and max_completion_tokens and bounds neither, and the nearest
//     thing to a ceiling anywhere is a column of the capability matrix at
//     https://docs.berget.ai/models/capabilities, where "long context + JSON"
//     is defined as JSON output on long-context requests of "~8k output
//     tokens". That describes what the column was tested at, approximately,
//     and not a ceiling the API enforces, so no model carries it as one.
//   - A capability in that matrix the listing does not already carry. It
//     covers five models, one of which the listing no longer serves, and for
//     the four it shares every column but one restates a flag the endpoint
//     sets. Its one added distinction, between JSON mode and JSON schema, is
//     drawn in favour of both on all four, which is the unnarrowed
//     structured-output capability the endpoint already states. The column
//     that does not restate a flag is the multimodal one, and it disagrees:
//     it marks Mistral Small 3.2 and GPT-OSS 120B as taking images where the
//     listing reports the vision flag false on both. The modality lists follow
//     the listing, since asserting image input for a model the API says has
//     none is the worse of the two errors, and the disagreement is recorded on
//     those two models as a note.
//   - A context window for a model the overview has not caught up with. The
//     endpoint carries no bound at all, and the overview's cards are the only
//     place one is stated, so a model the endpoint serves and the overview does
//     not card carries none. Kimi-K3 and Qwen3.8 are in that position, listed
//     by the endpoint in an eval lifecycle state with no card written for them
//     yet, and so are the three speech models, whose cards state a rate and no
//     length.
//   - The width of the vector an embedding model returns. Its endpoint carries
//     the rate, licence, quantization, capabilities and parameter count of both
//     embedding models and no width, the documentation card for each states the
//     identifier, a token limit and the rates and stops there, and the
//     specification's embeddings request takes a dimensions parameter bounded
//     only by one, which asks the caller for a width instead of stating one.
//   - A rate limit as a number. The pricing page sells four plans and
//     describes their limits as basic, increased, higher and maximum, adding
//     that they are shared per account. There is no request or token figure
//     behind those words anywhere, so no model carries one.
//   - A region or a deployment location per model. The residency guarantee is
//     made of the platform, and no document narrows it to a country or a data
//     centre for any model, so none is recorded as being served from one.
//   - Which languages a model handles, for all but one. The overview says its
//     three speech models cover Swedish and Norwegian alongside a multilingual
//     option without saying which model is which. Only KB-Whisper is named
//     outright, on
//     https://docs.berget.ai/models/model-selection-philosophy, as a Swedish
//     model from the National Library of Sweden, and it is the only model that
//     records a language.
package berget
