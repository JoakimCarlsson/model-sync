// Package cohere parses Cohere's model overview, pricing page and capability
// guides into the catalog model.
//
// The documentation publishes no rate at all: its pricing page explains how a
// bill is counted and then points at the marketing site for the amounts. What
// rates Cohere states are stated there four ways: as cards for the products it
// sells today, as sentences in the page's questions and answers for the models
// it has withdrawn, as a table of hourly and monthly rates for an instance of a
// model held on its dedicated deployment platform, and as one sentence inside a
// card giving what such an instance starts at. All four are read.
//
// A dedicated instance is billed for the time it is held rather than for
// anything a request carries, so those rates are recorded as hosting and are
// marked with the platform they belong to; without that they would read as a
// second, contradictory rate for the same call. The one quoted as a floor
// carries a note saying so.
//
// The two are not equally easy to attach to a model. A sentence names a model
// precisely — "Command R+ 08-2024" is command-r-plus-08-2024 — and reduces to
// an identifier on its own. A card names a product, and a product outlives the
// model behind it: the card headed "Command R" states the rate of whichever
// model serves under that name today, which is not the alias command-r that
// points at the version the same page prices separately as legacy. Cards are
// therefore looked up in a table of product names, and only names the overview
// already established reach a model, so a card headed for a platform or a
// deployment plan reaches nothing.
//
// The overview is the authority on everything else. It is five families with a
// different table shape each: chat models carry a context length and an output
// ceiling, embedding models carry the vector width and the similarity metric
// they are trained for, rerankers carry a context length, and audio models
// carry a maximum file size. A further table per family lists the identifier
// the same model answers to on Bedrock, SageMaker, Azure and Oracle.
//
// Its tables state no display name, only the identifier, and a name is
// therefore assembled from the three places Cohere writes one. The summary
// above the tables names each model in prose and links it, and the link's
// address is the identifier without its release date; that covers the Command
// family. The description column of a table names the model it describes
// wherever the description opens by naming it, which covers the whole Aya
// family: "Tiny Aya Global is a 3.35B instruction-tuned multilingual model".
// The pricing page's cards and its table of dedicated instances name what
// Cohere sells, which is where the fourth generation embedder and the
// rerankers get theirs.
//
// Models none of the three names keep no display name rather than one derived
// from their identifier. The identifiers are slugs, and as a display name a
// slug is worse than an empty field, because empty is honest and is also the
// only signal saying this vendor still has names to find.
//
// Two capabilities are stated, each in the guide to the capability rather than
// against the model. The structured outputs guide opens with the list of models
// it works with, which names products the same way the rate cards do and
// resolves through the same table. The tool use guide names a family instead of
// listing its members, saying that tool use connects the Command family to
// external tools through the Chat endpoint, so it reaches the Command models
// the overview gives a Chat endpoint and stops there. Both are matched in the
// document rather than assumed, so a guide rewritten to say something narrower
// stops yielding the capability instead of going on claiming it.
//
// What Cohere does not publish:
//
//   - A rate for the Command A family, for Aya Vision, for Tiny Aya, for the
//     nightly builds, for the third generation embedding and rerank models or
//     for anything older. Those models are served and documented, and no price
//     is stated for any of them on the pricing page, on the model's own page or
//     anywhere else Cohere publishes. The card headed Command A+ quotes nothing
//     but zero for an API key and a model download, which is the open weight
//     licence rather than a rate, and is not read as one.
//   - Anything at all about a nightly build beyond its existence. The two
//     nightly Command models appear only in the table of platform identifiers,
//     which has no column for a context length, an output ceiling or a
//     modality, so they carry none of the three. Cohere documents them nowhere
//     else.
//   - A capability list against a model. Neither the overview nor the pricing
//     page has a capability column, and a model's own page describes what it
//     can do in paragraphs rather than enumerating it. The two guides above are
//     the whole of what is enumerable, so a model outside the Command family
//     carries no capability at all.
//   - Which models reason. Cohere has a reasoning guide and a model named for
//     the capability, and neither states a list: the guide explains the feature
//     and calls one model in its examples, which is a worked example and not an
//     enumeration. Reading a capability out of an example would claim it for
//     whichever model the example happened to use, so none is recorded.
//   - A display name for the older embedders and rerankers or for the nightly
//     builds. Their descriptions describe rather than name, "A model that
//     allows for text to be classified or turned into embeddings", the summary
//     does not link them, and no card sells them.
//   - An output modality. The modality column states what a model accepts and
//     says nothing about what it returns. Text is recorded for every model that
//     has an input, because the medium each family works in is text on both
//     sides: Command generates text following an instruction, and the embedding
//     and rerank families vectorize and score it. That is not a claim about the
//     return value, which is a vector for one and a set of scores for the other,
//     and the catalog has a word for neither. The two sides are set together, so
//     the nightly builds, which appear only in the table of platform
//     identifiers and have no modality column, carry neither.
package cohere
