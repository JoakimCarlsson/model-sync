// Package cohere parses Cohere's model overview, its two pricing pages, its
// deprecation announcements, the pages of its two transcription models and of
// six of its Command models, its capability guides, its rate limit page, the
// references for the four endpoints its models answer on and every one of its
// release notes into the catalog model.
//
// Cohere states its rates in two places and five ways. The marketing site
// carries four of them: cards for the products it sells today, sentences in
// the page's questions and answers for the models it has withdrawn, a table of
// hourly and monthly rates for an instance of a model held on its dedicated
// deployment platform, and one sentence inside a card giving what such an
// instance starts at. The documentation carries the fifth, a page of rates for
// that same platform which is longer than the marketing table in both
// directions: it adds an annual rate to the models sold self-serve, and it is
// the only document anywhere quoting a rate for the Command A family. All five
// are read.
//
// A dedicated instance is billed for the time it is held rather than for
// anything a request carries, so those rates are recorded as hosting and are
// marked with the platform they belong to; without that they would read as a
// second, contradictory rate for the same call. The one quoted as a floor
// carries a note saying so. The two documents overlap on the models sold
// self-serve and agree on them, and an identical rate is recorded once.
//
// The two platform tables state a tier differently. The self-serve one gives a
// model a row per tier and a column per denominator; the generative one gives
// a model one row and names the larger tier in the heading of a second hourly
// column, leaving the cell blank for a model offered in one size only.
//
// Rates are not equally easy to attach to a model. A sentence names a model
// precisely, "Command R+ 08-2024" is command-r-plus-08-2024, and reduces to an
// identifier on its own. A card and a table row name a product, and a product
// outlives the model behind it: the card headed "Command R" states the rate of
// whichever model serves under that name today, which is not the alias
// command-r that points at the version the same page prices separately as
// legacy. Both are therefore looked up in a table of product names, and only
// names the overview already established reach a model, so a row headed for a
// platform bundle or a deployment plan reaches nothing.
//
// The overview is the authority on everything else. It is five families with a
// different table shape each: chat models carry a context length and an output
// ceiling, embedding models carry the vector width and the similarity metric
// they are trained for, rerankers carry a context length, and audio models
// carry a maximum file size. A further table per family lists the identifier
// the same model answers to on Bedrock, SageMaker, Azure and Oracle. One of
// those cells is written in quotes and the rest are not, so the marks belong to
// the cell rather than to the identifier and are stripped.
//
// Two documents say what the overview's tables cannot. The deprecation
// announcements give the standing of a model listed only in a table of
// platform identifiers, which has no status column, and that is the whole of
// what is known about the three second generation embedders: they were retired
// in April 2026, and reading it is the only thing that keeps them out of the
// catalog. Each announcement is headed by the date it takes effect and
// lists what it withdraws under one of two phrases, and both phrases are
// matched rather than assumed, so an announcement worded some third way yields
// nothing instead of a standing read off the wrong list. The two transcription
// models have a page each, which states the model's identifier, what it takes
// and what it returns, and its file ceiling; the Arabic model is documented
// nowhere else and reaches the catalog from there alone.
//
// A model Cohere no longer serves is not published at all. Whichever document
// states the standing, the overview's status column or an announcement, a model
// that is retired or shut down is dropped rather than recorded with a date
// against it: it cannot be called, no rate is quoted for it, and a catalog
// entry for it would offer a reader something to choose that is gone. That is
// five models, the two 8B Aya models and the three second generation embedders.
//
// A deprecated model is kept. Cohere goes on serving one to the customers
// already using it until its retirement date and goes on quoting its rate in
// the pricing page's questions and answers, so it is a model with a date on it
// rather than a model that is gone, and the date is a reason to publish it and
// not a reason to withhold it.
//
// Two of the identifiers the overview lists are not models. Its description
// column says the whole of what it has to say about them in four words, "Alias
// for command-r-03-2024", and that sentence is read: the alias records what it
// stands for, the model records what stands for it, and the alias takes the
// model's rate with a note saying where the amount came from. A call to either
// is the same call and costs the same thing, and without this an identifier the
// API answers to and bills for would read as one Cohere states no rate for.
//
// An alias is also why a bare product name is resolved through a table rather
// than reduced to an identifier. "Command R" and "Command R+" are both live
// identifiers and both aliases for a version deprecated in September 2025, so a
// document listing what Cohere sells or how often it may be called today, which
// is a rate card, the rate limit table or a capability guide, means the version
// serving under the name and not the alias. The consequence is worth stating
// plainly: the structured outputs guide lists six names that are four models,
// because it names Command R and Command R+ once dated and once not.
//
// Its tables state no display name, only the identifier, and a name is
// therefore assembled from the four places Cohere writes one. The summary above
// the tables names each model in prose and links it, and the link's address is
// the identifier without its release date; that covers the Command family. The
// description column of a table names the model it describes wherever the
// description opens by naming it, which covers the whole Aya family: "Tiny Aya
// Global is a 3.35B instruction-tuned multilingual model". The rate cards and
// the two tables of dedicated instances name what Cohere sells, which is where
// the fourth generation embedder and the rerankers get theirs. A transcription
// model's page is headed by its name.
//
// Models none of the four names keep no display name rather than one derived
// from their identifier. The identifiers are slugs, and as a display name a
// slug is worse than an empty field, because empty is honest and is also the
// only signal saying this vendor still has names to find.
//
// Four capabilities are stated, each somewhere other than against the model in
// the overview. The structured outputs guide opens with the list of models it
// works with, which names products the same way the rate cards do and resolves
// through the same table. The tool use guide names a family instead of listing
// its members, saying that tool use connects the Command family to external
// tools through the Chat endpoint, so it reaches the Command models the
// overview gives a Chat endpoint and stops there. The streaming guide names
// only an endpoint, saying the Chat API is capable of streaming events as they
// come, so it reaches every model the overview gives that endpoint, which is
// the Command family and both Aya families. A transcription model's page
// answers a capability as a question against the model, and the Arabic model
// answers yes to following a speaker who changes language mid-utterance. All
// four are matched in the document rather than assumed, so a guide rewritten to
// say something narrower stops yielding the capability instead of going on
// claiming it.
//
// Six of the Command models have a page of their own, and a page is the only
// document that answers a capability against one model. Two of them, Command A+
// and Command A Reasoning, point at the reasoning guide for how to turn the
// model's thinking off, which is a thing only a model that thinks has, and that
// is where reasoning comes from: the guide itself enumerates nothing and calls
// one model in an example, which is a worked example and not a list. One of
// them contradicts the family the tool use guide claims, Command A Vision
// saying
// outright that tool use is not supported with it, and a document naming a
// model
// outranks a document naming the family it is in, so that model is excepted.
// The
// newest page also carries a properties list, which is where the day Command A+
// was released, who published it, how large it is and what licence its weights
// are under all come from.
//
// The four endpoint references say what a call may carry. Every parameter they
// list is recorded against the models the overview answers on that endpoint,
// and the bounds they state are recorded the same way: how many texts and
// images one embedding call takes, in what formats a vector comes back and what
// an input may be declared to be for, how many documents a rerank call should
// carry and how long each may be, and which containers a transcription accepts.
// For the embedding and rerank families this is the whole of what bounds a call
// beyond the context length, because their tables in the overview have no other
// column. The embed reference states its image rules per generation rather than
// per model, one for the third and one for the fourth and anything after it,
// and
// an identifier carries its generation, so each rule reaches the models whose
// version it names.
//
// The rate limit page states how often a model may be called, twice for every
// model: once for the key an account is given on signing up and once for the
// key
// it is given on paying. It reaches a model two ways. The Chat models get a row
// each, headed by a product name that resolves through the same table the rate
// cards do, and everything else gets a row per endpoint, which reaches every
// model the overview answers there. A row Cohere answers with an invitation to
// contact its sales team states no ceiling and yields nothing, and the page's
// default row, which states what applies to everything not named above it, is
// not read as a statement about any model. An embedding model comes out of this
// with three ceilings that are not the same ceiling: inputs a minute, image
// inputs a minute, and batch jobs a minute, each under a key saying what it
// counts.
//
// The release notes are the only document dating anything. Cohere's overview
// has
// no date column and its model pages, bar the newest, state no release date, so
// the day a model arrived is read from the changelog: the index dates each
// entry
// and the entry names what it announces. An entry reaches a model only two
// ways,
// and both are the entry saying what it is about rather than mentioning a model
// in passing. Most entries carry a technical details list headed by the
// identifier. The rest announce a release, or an update to one, in prose and
// name the identifiers in it, and only those entries are read from, because
// every entry that withdraws a model names the models it recommends instead and
// dating a model from the first entry to mention it would put its release on
// the
// day it was deprecated. That is twelve of the thirty four. The same lists give
// the languages three models cover and the licence their weights are under.
//
// What Cohere does not publish:
//
//   - A rate for Aya Vision, for Tiny Aya, for the nightly builds, for the
//     third generation embedding and rerank models or for the Arabic
//     transcription model. Those models are served and
//     documented, and no price is stated for any of them on either pricing
//     page, on the model's own page or anywhere else Cohere publishes; the two
//     transcription pages answer the question with an invitation to contact
//     sales. The card headed Command A+ quotes nothing but zero for an API key
//     and a model download, which is the open weight licence rather than a
//     rate, and is not read as one, so what that model costs comes from the
//     dedicated deployment page alone.
//   - Anything about the two nightly Command builds beyond their existence.
//     They survive only in the table of platform identifiers, which has no
//     column for a context length, an output ceiling, a modality or an
//     endpoint, they are named in no announcement, and Cohere documents them
//     nowhere else. The changelog announcing the nightly build states which
//     sizes it came in and no quantity at all.
//   - A capability list against a model. Neither the overview nor either
//     pricing page has a capability column, and a Command model's own page
//     describes what it can do in paragraphs rather than enumerating it. The
//     three guides, the six model pages and the two audio pages are the whole
//     of what is enumerable, and none of them speaks about embedding or rerank
//     models, so those two families carry no capability at all. The catalog's
//     vocabulary has no value for what they do either: what a reranker offers
//     is relevance scoring, and there is no canonical word for it.
//   - A rate for a model on a cloud platform. The overview says which
//     identifier a model answers to on Bedrock, SageMaker, Azure and Oracle,
//     and stops there; what a call to it costs on any of them is published by
//     that platform and not by Cohere, and no rate here is read from anywhere
//     but Cohere. The only rate Cohere states for a platform is for its own,
//     Model Vault, and that one is recorded with the platform against it.
//   - Which models reason, as a list. The reasoning guide explains the feature
//     and calls one model in its examples, which is a worked example and not an
//     enumeration, and the Chat reference documents the parameter that turns it
//     on without saying which models accept it. Only the two model pages that
//     document turning their own thinking off are read, so a reasoning model
//     without a page of its own would be missed rather than guessed at.
//   - Which models can be held to a schema, beyond the six the structured
//     outputs guide lists. Those six are four models, because the guide names
//     two of them twice, once dated and once not. The Chat reference says the
//     parameter is supported on "Command R, Command R+ and newer models", and
//     says the same of two others in the same shape, but a model is newer than
//     another only by a release date Cohere states for a minority of them, so
//     the phrase is not turned into a list.
//   - A knowledge cutoff, for any model. No overview column, no model page, no
//     release note and no reference states when a model's training data ends,
//     and none of them states it in any other wording either.
//   - A display name for the older embedders and rerankers or for the nightly
//     builds. Their descriptions describe rather than name, "A model that
//     allows for text to be classified or turned into embeddings", the summary
//     does not link them, and no card or platform table sells them.
//   - An output ceiling for anything but a chat model, and a context length for
//     an audio model. The embedding and rerank tables have no such column,
//     because what those models return is a vector and a set of scores rather
//     than a run of tokens, and the audio table bounds a request by the size of
//     the file rather than by a count of tokens.
//   - An output modality for anything but a transcription model. The modality
//     column states what a model accepts and says nothing about what it
//     returns, and only the two audio pages answer both halves outright. Text
//     is recorded for every other model that has an input, because the medium
//     each family works in is text on both sides: Command generates text
//     following an instruction, and the embedding and rerank families vectorize
//     and score it. That is not a claim about the return value, which is a
//     vector for one and a set of scores for the other, and the catalog has a
//     word for neither. The two sides are set together, so the nightly builds,
//     which have no modality column, carry neither.
package cohere
