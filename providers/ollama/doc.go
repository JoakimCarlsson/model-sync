// Package ollama parses Ollama's model library into the catalog model.
//
// Ollama runs models on your own machine, so nothing it publishes has a price
// and none is recorded. What it publishes instead is which models exist, what
// they can do, and at which parameter sizes each is available, which is the
// part a reader comparing it against a hosted provider actually needs.
//
// Its library marks both capabilities and sizes as tags in the same list, so
// the two are told apart by shape: a tag that reads as a parameter count is a
// size, and the rest are capabilities.
package ollama
