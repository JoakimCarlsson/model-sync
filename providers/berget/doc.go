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
// chat and embeddings. Two of its fields are deliberately not recorded: the
// endpoint reports live health and latency, which change between any two
// fetches and would rewrite every file on each sync.
//
// The endpoint states no context window. The documentation site does, on a
// card per model keyed by the same identifier, so that page is read too and
// the two need no reconciling. A model added to the endpoint before the site
// catches up carries no context window until it does. The specification
// promises otherwise, describing the per-model endpoint as answering with
// "pricing, context window, and capabilities", but the schema it answers with
// declares no such field and the served response carries none, so the cards
// remain the only statement of a bound.
//
// A card writes that bound in shorthand, "256k tokens", and Berget states it
// no other way, so the suffix is read as a thousand and such a card is
// recorded as 256000. The figure is Berget's own and is what it serves the
// model at, which is not always what the model's author advertises: Z.ai
// publishes GLM-5.2 with a million-token window and Berget cards it at 256k.
// A reseller serving a model shorter than its weights allow is a fact about
// the deployment, and the deployment is what this provider records.
//
// The API's OpenAPI specification is read for a third thing neither states:
// what a request may carry. It names a parameter once per endpoint rather than
// once per model, and a model of a given kind is served by exactly one
// endpoint, so its kind is what carries the parameters to it. They are
// recorded as parameters and not as capabilities, because naming a knob the
// API accepts is not the same as stating what the model does with it. Three
// are the exception: word-level alignment, diarization and hotword biasing
// name what the model does with the audio rather than a knob on the request,
// and each is something another provider here states as a capability outright,
// so a speech model records all three as capabilities as well.
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
// What Berget does not publish:
//
//   - A bound on output length, for any model. The nearest thing to one is a
//     column of the capability matrix at
//     https://docs.berget.ai/models/capabilities, where "long context + JSON"
//     is defined as JSON output on long-context requests of "~8k output
//     tokens". That describes what the column was tested at, approximately,
//     and not a ceiling the API enforces, so no model carries it as one.
//   - A context window for a model the overview has not caught up with. The
//     endpoint carries no bound at all, and the overview's cards are the only
//     place one is stated, so a model the endpoint serves and the overview does
//     not card carries none. Kimi-K3 is in that position, listed by the
//     endpoint in an eval lifecycle state with no card written for it yet, and
//     so are the three speech models, whose cards state a rate and no length.
//   - The width of the vector an embedding model returns. Its endpoint carries
//     the rate, licence, quantization, capabilities and parameter count of both
//     embedding models and no width, the documentation card for each states the
//     identifier, a token limit and the rates and stops there, and the
//     specification's embeddings request takes a dimensions parameter bounded
//     only by one, which asks the caller for a width instead of stating one.
//   - Which models reason. The specification takes reasoning_effort and a
//     thinking parameter on the chat endpoint, describing both as vendor
//     neutral controls converted to whatever the backend wants, and neither the
//     endpoint's capability flags nor the capability matrix says which models
//     honour them. Both are recorded as the parameters they are.
package berget
