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
package perplexity
