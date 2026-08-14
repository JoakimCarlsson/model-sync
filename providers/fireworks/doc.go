// Package fireworks parses Fireworks AI's serverless pricing into the catalog
// model.
//
// Fireworks packs three rates into one cell. A cell reads
// "$3.00 / $0.30 / $15.00", which the page explains as input, cached input and
// output per million tokens, and there is one such cell per serving path:
// Standard and Priority. The same model also appears more than once under
// names that differ only by a serving suffix, as "Kimi K3", "Kimi K3 Fast" and
// "Kimi K3 US", all linking to the same model. Those are one model served
// three ways rather than three models, so the suffix becomes a dimension and
// the identifier comes from the link they share.
//
// The pricing page states nothing else about a model, but every row links to
// the model's page in the console, and that page carries a record of it as
// embedded JSON: the context window, the display name, the description, where
// its weights are published, and flags for image input and tool use. Those
// pages are fetched too. The link is the join, so nothing has to be matched on
// names, and the three rows of a model served three ways all reach the same
// page.
//
// The record's quotes arrive escaped, and how many times depends on how deeply
// the page nested it, so the escaping is matched rather than undone.
//
// The pricing page prices one model it does not link: its embedding model,
// which it prices as "Qwen3 8B". The guide to the embeddings API writes that
// name out as "Qwen3 Embedding 8B", links the model and states the identifier
// the API takes, so the guide is fetched first and the model is keyed by what
// the guide found. Its tables list models that run only on a deployment of the
// caller's own, and rerankers under names the embedding model's name is a
// subset of, so a row is read only when it links a model, says serverless, and
// is not under the reranking heading.
//
// Two capabilities are stated somewhere other than against a model:
//
//   - Constrained output. Fireworks works an example of grammar-constrained
//     output through one model by name and then says the feature is not
//     particular to it, all its models support it. That sentence is matched
//     and the capability recorded for every model that generates a response,
//     which is why it needs no flag of its own. It stops there: the sentence
//     says all of them and means all the models the guide is about, and an
//     embedding model returns a vector, which no schema describes.
//   - Reasoning. The reference for the chat completion request documents its
//     reasoning_effort parameter model family by model family, down to which
//     efforts each family accepts and whether reasoning is on when nothing is
//     passed. A family documented there is one Fireworks says reasons, so the
//     capability is read off that list and matched to models by name, word for
//     word and adjacent: a paragraph about MiniMax M2 is not about MiniMax
//     M2.7, which the reference does not document.
//
// Two of the pricing page's tables are not model tables at all: they price by
// parameter count band, with rows like "4B - 16B parameters". Those bands are
// what the rest of the model library costs, and Fireworks publishes no list of
// which models fall in which band, so there is nothing to key them to and they
// are not read. The catalog therefore holds the models Fireworks prices one by
// one, which is every model on the pricing page and no more.
//
// A last document is fetched for the few models the console record left
// something open on: the model library's page for the same model, at
// fireworks.ai rather than app.fireworks.ai. It renders rather than embeds
// what it states, and it rounds the context window for display, so it is read
// only where the record has nothing: the context window of a model whose
// record puts it at zero, and the width of the vector an embedding model
// returns, which the record has no field for. Fireworks publishes no library
// page for every model, and one it omits is a document less to read rather
// than a failed refresh.
//
// What Fireworks does not publish:
//
//   - A bound on output length, for any model. Neither record nor page carries
//     one, and the API reference says why: max_tokens is bounded by the
//     context window alone, and a request asking for more than fits is lowered
//     to fit rather than refused.
//   - Which of the remaining models reason. The reference documents a
//     reasoning effort for the DeepSeek, GLM and Harmony families and says
//     nothing about the Kimi, MiniMax M2.7, Muse or Nemotron models it serves.
//     The guide to reasoning is no help either: every example calls a model
//     named "<reasoning-model>", and the one document that touches it
//     otherwise says the "Kimi K2 family" produces long reasoning traces
//     without naming a single model in it.
//   - Any capability of its embedding model. It answers one endpoint with a
//     vector: the record flags neither tools nor image input, its library page
//     states outright that streaming and function calling are not supported,
//     and constrained output is not something a vector can be asked for.
//   - The number of widths its embedding model can return. The width is
//     recorded as the one it returns unasked, because the vector can be cut to
//     any length between 32 and that, which is a range rather than a set.
package fireworks
