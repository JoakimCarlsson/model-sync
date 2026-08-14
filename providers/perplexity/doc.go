// Package perplexity parses Perplexity's pricing documentation into the
// catalog model.
//
// Perplexity sells four different things and prices each on its own terms. Its
// Sonar models are charged per token and again per thousand requests, with the
// request fee varying by how much of the web the query is allowed to read.
// Its tools are charged per invocation, its sandbox per session, and its
// search API per thousand requests. Its embeddings are charged per token and
// publish the width of the vector beside the rate.
//
// It also brokers other labs' models through its Agent API, listing them under
// namespaced identifiers at what it states are first-party rates with no
// markup. Those are recorded here as Perplexity's own catalog entries, the way
// any reseller's are, rather than merged into the labs they come from. A
// brokered model that costs more on a long prompt has both rates in the one
// cell, each with the bound it applies under, so each is recorded against the
// prompt length it belongs to; a cache rate written as a reduction of the
// input rate is recorded as that reduction and not as an amount, since the
// amount it reduces is itself two amounts.
//
// The rate tables are markdown but most sit inside MDX tab elements, so the
// heading in force is read from the tab's own heading rather than from the
// page structure around it.
//
// Each Sonar model has a page of its own, addressed by the identifier the API
// answers to, and that page states the model's context window in a heading and
// says in a card whether the model reasons. It states the latter either way
// round, as "Advanced reasoning model" or as "Non-reasoning model", so the
// negative has to be told from the positive rather than read as an absence.
// What a Sonar
// model may answer with is stated once for the endpoint, as the ceiling the
// chat completions reference puts on max_tokens, and that reference enumerates
// the models it takes, which is what says who the ceiling binds.
//
// What a Sonar model takes and can do is documented for the API rather than for
// any one model, in two guides: one saying that streaming and structured
// outputs work, the other saying that images and files may be sent. Both are
// read onto every model the Sonar index lists, because that index is exactly
// what the Sonar API serves. The media guide's other half, on the images and
// videos a response carries back, is not read as an output modality: those are
// media the search found and linked, not media the model produced.
//
// The Agent API is documented the same way, and what it states of every model
// it serves is read onto every model its own model page tabulates. Its output
// guide says outright that streaming works across all of them, and its request
// reference gives the content parts a message may hold, which is where the
// text on both sides comes from. Its image part is not read as a modality: it
// says what the endpoint accepts, and the model page warns that the models
// behind it differ. That page's prose occasionally says more about a model than
// its row does, so a line of it is read where it names exactly one model the
// tables listed; a card introducing a whole family, or two models it then
// describes differently, is left alone.
//
// The Embeddings guide tabulates the same models as the pricing page with
// three more columns: the input bound, whether the vector may be truncated,
// and what it is quantized to. An embedding model is recorded as working in
// text on both sides, the medium rather than the return value, since the
// vector it answers with is not something the catalog has a word for and
// recording the input alone would read as an unstated output.
//
// What Perplexity does not publish. Almost nothing about the models it brokers
// beyond their rates: its table of them gives an identifier, three amounts and
// a link to the lab that made the model, and it is that lab's documentation
// which says what the model holds. Asked directly what the context window of
// each model is, its FAQ answers that it varies and to see the models page and
// the provider documentation linked from it, so 39 of the 40 brokered models
// carry no context window; the one that does is the one whose card states it
// in prose. Nor is there an output bound for any of them: max_output_tokens is
// a request parameter the Agent API bounds from below and not above. The same
// page warns that not all brokered models support all features, naming
// reasoning and tools, so a capability is recorded for one only where
// something states it of that model, and no document states function calling
// of any model at all: the Sonar API has no parameter for it, and the Agent
// API's tools are a property of the run rather than of the model chosen for
// it. Nothing bounds what an embedding model answers with, since the width of
// the vector is the answer's size and is already stated, and nothing at all is
// stated of the tools, which are billed products rather than models.
package perplexity
