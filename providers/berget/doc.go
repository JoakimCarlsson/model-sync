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
// What Berget does not publish: a bound on output length, for any model.
package berget
