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
// any reseller's are, rather than merged into the labs they come from.
//
// The rate tables are markdown but most sit inside MDX tab elements, so the
// heading in force is read from the tab's own heading rather than from the
// page structure around it.
//
// Each Sonar model has a page of its own, addressed by the identifier the API
// answers to, and that page states the model's context window in a heading and
// says in a card whether the model reasons. It states the latter either way
// round — "Advanced reasoning model" or "Non-reasoning model" — so the negative
// has to be told from the positive rather than read as an absence.
//
// What a Sonar model takes and can do is documented for the API rather than for
// any one model, in two guides: one saying that streaming and structured
// outputs work, the other saying that images and files may be sent. Both are
// read onto every model the Sonar index lists, because that index is exactly
// what the Sonar API serves. The media guide's other half, on the images and
// videos a response carries back, is not read as an output modality: those are
// media the search found and linked, not media the model produced.
//
// What Perplexity does not publish: anything about the models it brokers
// beyond their rates. Its table of them gives an identifier, three amounts and
// a link to the lab that made the model, and it is that lab's documentation
// which says what the model holds and can do. The Agent API's own warning is
// that not all of those models support all of its features, so its guides say
// nothing that could be read onto them. Forty of the 44 chat models are
// brokered and carry no context window, no capability and no modality for that
// reason. Nor does Perplexity state an output bound for any model, its own
// included, or an input bound for its embedding models.
package perplexity
