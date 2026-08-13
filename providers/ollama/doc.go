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
// One capability has no tag, and would be the same tag on every model if it
// had one. Ollama constrains the decoding itself, so a JSON schema holds for
// whatever model is loaded, and it documents that once rather than per model.
// The page saying so is read, and the capability recorded for every model
// Ollama generates with. An embedding model returns a vector, which no schema
// describes, and is left alone.
//
// The library states no bound on any model. Each model's tag listing does,
// once per build, so the listings are read too. The builds differ, since a
// quantization at one size may hold a shorter context than another, and the
// one recorded is the build Ollama serves by default, because that is what
// running the model plainly gives.
//
// A context window is recorded as the listing writes it. A tag reading "2K
// context window" becomes 2000 and not 2048: the rounded figure is what Ollama
// states, and the exact one is in the model's own metadata, which its pages do
// not carry.
//
// What Ollama does not publish, on any page:
//
//   - A display name distinct from the identifier.
//   - Any bound on output length.
//   - The width of the vector an embedding model returns. Twelve embedding
//     models are in the library and none of its pages states one. The figure
//     does exist, as the embedding_length of the GGUF metadata behind the
//     registry, but nothing in the library or on a model's page links to it:
//     reaching it means resolving a manifest against the registry service and
//     then a blob, per model, rather than reading a document Ollama publishes.
//     That is why the width is absent here rather than being derived.
package ollama
