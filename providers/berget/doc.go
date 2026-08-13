// Package berget parses Berget's model API into the catalog model.
//
// Berget is the first source here that does not price in dollars. It quotes
// euros, which is why a price carries its currency rather than assuming one,
// and a reader comparing it against a dollar provider must convert rather than
// compare the numbers directly.
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
// catches up carries no context window until it does.
//
// Nor does either document state a modality. What a model takes and returns is
// read from its type — a speech-to-text model hears and writes — and from the
// vision capability, which is the one capability naming a modality rather than
// an API feature and is recorded as both.
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
//   - A bound on output length, for any model.
//   - The width of the vector an embedding model returns. Its endpoint carries
//     the rate, licence, quantization, capabilities and parameter count of both
//     embedding models and no width, and the documentation card for each states
//     the identifier, a token limit and the rates and stops there.
package berget
