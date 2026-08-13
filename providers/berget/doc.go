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
package berget
