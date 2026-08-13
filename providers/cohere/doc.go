// Package cohere parses Cohere's model overview and pricing page into the
// catalog model.
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
// Its tables state no display name, only the identifier, but the summary above
// them names each model in prose and links it, and the link's address is the
// identifier without its release date. That is the tie between the two, and it
// covers the Command family. Models the summary does not name keep no display
// name rather than one derived from their identifier.
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
//   - A capability list. What a model can do is described in paragraphs on its
//     own page, not enumerated, so no features are recorded. The endpoints
//     column is the nearest thing the overview states and is kept as one.
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
