// Package ollama parses Ollama's model library into the catalog model.
//
// Ollama runs models on your own machine, so nothing it publishes has a price
// and none is recorded. What it publishes instead is which models exist, what
// they can do, and at which parameter sizes each is available, which is the
// part a reader comparing it against a hosted provider actually needs.
//
// Its library marks both capabilities and sizes as tags in the same list, so
// the two are told apart by shape: a tag that reads as a parameter count is a
// size, and the rest are capabilities. A mixture of experts writes its count
// as a product, "8x7b", and a model shipped at an effective size prefixes it,
// "e4b"; both are sizes and neither is anything a model can do.
//
// Ollama's words for a capability are shared with no other provider, so they
// are translated: tools is function calling and thinking is reasoning. Two of
// them name a modality rather than a feature — a model tagged for vision takes
// an image — and are recorded as modalities instead. For the same reason a
// model that reads images is a chat model here rather than a kind of its own.
//
// The library states no bound on any model. Each model's tag listing does,
// once per build, so the listings are read too. The builds differ, since a
// quantization at one size may hold a shorter context than another, and the
// one recorded is the build Ollama serves by default, because that is what
// running the model plainly gives.
//
// What Ollama does not publish: a display name distinct from the identifier,
// and any bound on output length.
package ollama
