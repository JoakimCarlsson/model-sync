// Package cohere parses Cohere's model overview into the catalog model.
//
// Cohere states rates only on its marketing site, in a layout with no reliable
// tie between an amount and a model, so no prices are recorded. Guessing which
// number belongs to which model would be worse than recording none.
//
// What it publishes properly is a catalog of five families with a different
// table shape each: chat models carry a context length and an output ceiling,
// embedding models carry the vector width and the similarity metric they are
// trained for, rerankers carry a context length, and audio models carry a
// maximum file size. A further table per family lists the identifier the same
// model answers to on Bedrock, SageMaker, Azure and Oracle, which is recorded
// against the model so a reader on one of those platforms can find it.
package cohere
