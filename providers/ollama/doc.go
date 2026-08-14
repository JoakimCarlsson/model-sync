// Package ollama parses Ollama's model library into the catalog model.
//
// Ollama distributes open-weight models to run on your own machine and also
// runs some of them on its own hardware, so which of the two a model is
// offered under decides whether it has a price at all. The library says which
// models exist, what they can do, at which parameter sizes each is available
// and which of them Ollama runs, which is the part a reader comparing it
// against a hosted provider actually needs.
//
// Its library marks capabilities, sizes and the cloud in the same list of
// tags, so they are told apart by shape and by name: a tag that reads as a
// parameter count is a size, the cloud tag says where the model runs rather
// than what it can do and is recorded as an attribute, and the rest are
// capabilities. A mixture of experts writes its count as a product, "8x7b",
// and a model shipped at an effective size prefixes it, "e4b"; both are sizes
// and neither is anything a model can do.
//
// Ollama's words for a capability are shared with no other provider, so they
// are translated: tools is function calling and thinking is reasoning. Two of
// them name a modality rather than a feature, a model tagged for vision takes
// an image, and are recorded as modalities instead. For the same reason a
// model that reads images is a chat model here rather than a kind of its own.
//
// One capability has no tag, and would be the same tag on every model if it
// had one. Ollama constrains the decoding itself, so a JSON schema holds for
// whatever model is loaded, and it documents that once rather than per model.
// The page saying so is read, and the capability recorded for every model
// Ollama generates with. An embedding model returns a vector, which no schema
// describes, and is left alone.
//
// # What a model costs
//
// Running a model on your own hardware is free, which is why most of the
// library carries no rate and its absence is the fact rather than a gap. What
// Ollama sells is the cloud: https://ollama.com/pricing prices that as a
// subscription, twenty dollars a month for Pro and a hundred for Max, with an
// allowance rather than a per-token rate, and a plan is not a model's price so
// none of it is recorded against one.
//
// Underneath the plan each cloud model does have a cost, and its own page is
// where Ollama states it. Most state only how heavily the model draws on an
// allowance, as a word from "low" to "extra high" over four bars, and that is
// recorded as an attribute since it is a rank and not an amount. A few quote
// the thing itself, three rates per million tokens for input, cached input and
// output, and those are recorded as prices. They carry the deployment they
// belong to, because the same model run locally costs nothing and a rate left
// undimensioned would read as the price of running it at all. So the model
// pages of the cloud models are fetched, and only those: a model Ollama does
// not run has no page that could quote a rate.
//
// A model shipped at several sizes states neither on its page, because the
// level belongs to the build and its builds differ: its tag listing marks one
// build low usage and another medium. That is not recorded, since the model
// here is the family and a level of the family is not a thing Ollama states.
//
// # Bounds and widths
//
// The library states no bound on any model. Each model's tag listing does,
// once per build, so the listings are read too. The builds differ, since a
// quantization at one size may hold a shorter context than another, and the
// one recorded is the build Ollama serves by default, because that is what
// running the model plainly gives.
//
// A context window is recorded as the listing writes it. A tag reading "2K
// context window" becomes 2000 and not 2048: the rounded figure is what Ollama
// states, and a window under a thousand is written plainly, "512 context
// window", which is how every small embedding model states its own.
//
// The width of the vector an embedding model returns is not in the library,
// but Ollama does publish it. A build's page lists the layers it is made of,
// and the model layer has a page of its own carrying the metadata the weights
// were built with, embedding_length among it. So for the embedding models, and
// for those alone, the build's page and then the layer's are fetched. The key
// is prefixed with the model's own architecture, "bert.embedding_length" on
// one model and "qwen3.embedding_length" on the next, so it is found by its
// ending.
//
// That page also states the exact context length, 2048 where the listing says
// 2K, and it is not recorded. The listing is the one document that states a
// window for every model, and taking the exact figure for the twelve models
// whose layer is read would make the field mean the rounded figure on some
// models and the exact one on others.
//
// What Ollama does not publish, on any page:
//
//   - A display name distinct from the identifier. The library entry, the
//     model page's title and its og:title all carry the identifier, and the
//     layer metadata, which is where a GGUF would keep general.name, states
//     only the architecture and the file type.
//   - Any bound on output length. Nothing bounds it: generation runs until the
//     model stops or the caller's own num_predict does, and the reader sets
//     that.
//   - Anything an embedding model can do beyond being one. The only capability
//     tag it carries is the one that says it is an embedding model, and that
//     is recorded as its kind rather than repeated as a feature.
package ollama
