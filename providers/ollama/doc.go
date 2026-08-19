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
// pages of the cloud models are fetched on top of the rest, and only those: a
// model Ollama does not run has no page that could quote a rate.
//
// A model shipped at several sizes states neither on its page, because the
// level belongs to the build and its builds differ: its tag listing marks one
// build low usage and another medium. That is not recorded, since the model
// here is the family and a level of the family is not a thing Ollama states.
// The same page states the size of the build Ollama runs, which a model shipped
// at several sizes likewise cannot, and that is recorded where it is stated.
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
// for those alone, that layer's page is fetched. The key is prefixed with the
// model's own architecture, "bert.embedding_length" on one model and
// "qwen3.embedding_length" on the next, so it is found by its ending. The
// tokenizer is on the same page under a key of its own and is taken with it.
//
// That page also states the exact context length, 2048 where the listing says
// 2K, and it is not recorded. The listing is the one document that states a
// window for every model, and taking the exact figure for the twelve models
// whose layer is read would make the field mean the rounded figure on some
// models and the exact one on others. Nor is it read for anything else: it
// carries a row per tensor of the weights, which on a large model runs to
// thousands, and the build's page states in four words what matters of it.
//
// # What the weights are
//
// A build is the model at one size and one quantization, and its page is the
// one document describing the weights themselves. It names the architecture
// they were converted from, how many parameters they hold, the quantization
// they are stored at and the licence they are published under. All four are
// properties of the build rather than of the family, so the build read is the
// one the tag listing marks as the default, the same build whose context
// window is recorded, and the page of that build is fetched for every model.
//
// The licence is published as the licence text rather than as a name, so the
// name is its title: the first of the opening lines that names a licence.
// Three of the common ones spread their title over two lines and are matched
// whole, since Apache writes the version below the name and the Creative
// Commons deed wraps mid-title. A file that opens with its version, with the
// copyright year or with a bare "LICENSE TEXT" heading names nothing there,
// which is why the following lines are read rather than only the first, and a
// licence layer holding a path to a licence elsewhere names nothing at all and
// yields nothing. A good part of the library ships no licence layer, and for
// those Ollama publishes no licence.
//
// That the page lists a weights layer at all is the fact that says whether the
// weights are published, and it is recorded as such. A model Ollama only runs
// in its cloud has a page with no layers on it, because there is nothing to
// download, and those models state their parameter count on the card their own
// page heads with instead.
//
// # When a model changed
//
// Every model's tag listing states when Ollama last changed it, twice: as an
// age in words, "10 months ago", and as the instant behind that age, which it
// writes as the tooltip of the words. The instant is the only one of the two
// that is a date, so it is what is recorded and the age is not: an age is a
// date only in combination with the day it was read, and a document that has
// to be dated by the clock of the reader is not a document stating a date.
//
// When a model first appeared is not published anywhere. Each build of a model
// carries an age of its own in the listing, in the same words and with no
// instant behind it, so the oldest build is dated only relatively; and the
// registry serves the manifest of a build and the configuration blob it names,
// neither of which carries a timestamp.
//
// What Ollama does not publish, on any page:
//
//   - A display name distinct from the identifier. The library entry, the
//     search listing, the model page's title, its og:title and the tooltip of
//     its heading all carry the identifier and nothing else, no page carries
//     an h1 at all, and the layer metadata, which is where a GGUF would keep
//     general.name, states only the architecture and the file type.
//   - The upstream repository a model was converted from. A readme sometimes
//     links one and as often links a dataset or a paper instead, and nothing
//     marks which link is the model, so there is no field to read.
//   - Who published a model. The library is Ollama's own and states no author
//     for any entry, and the only name the weights carry is the architecture.
//   - A date a model was released.
//   - Any bound on output length. Nothing bounds it: generation runs until the
//     model stops or the caller's own num_predict does, and the reader sets
//     that.
//   - Anything an embedding model can do beyond being one. The only capability
//     tag it carries is the one that says it is an embedding model, and that
//     is recorded as its kind rather than repeated as a feature.
package ollama
