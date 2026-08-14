// Package openrouter parses OpenRouter's model API into the catalog model.
//
// OpenRouter is the first source here that is not a document. It publishes a
// JSON endpoint listing every model it brokers, so there is no markdown, no
// HTML and no prose to read. That it still needs its own package is the point:
// the shape of the source changes nothing about where a provider's vocabulary
// lives.
//
// It is also a marketplace rather than a lab, so its catalog restates models
// belonging to other providers under its own identifiers. openai/gpt-5.6-sol
// here is the same model as gpt-5.6-sol under openai, priced as OpenRouter
// sells it rather than as OpenAI does. Both are recorded; neither is merged
// into the other, because the rates genuinely differ and the identifiers are
// OpenRouter's own.
//
// # One model, many sellers
//
// Every entry in the listing describes the model through one upstream: the
// endpoint OpenRouter currently fronts for it, out of the several that may
// serve it. Its rate, its completion ceiling and the parameters it accepts are
// that seller's, not an average and not a range. The others are a document
// away, linked from the entry, and they disagree with each other freely:
// twenty-one sellers of one model quote prompt rates spanning a factor of two
// and completion ceilings spanning two orders of magnitude.
//
// The rate recorded here is the fronted one, unchanged, because it is what
// OpenRouter presents as the model's price on its own listing and its own model
// page, and it is what a caller who names the model and nothing else is
// charged. It is usually the cheapest seller and not always, and it moves as
// routing moves, so two catalogs built a week apart will disagree about a
// model whose default seller changed in between. That is a fact about the
// marketplace rather than an error in either of them. The per-seller spread is
// deliberately not published here: recording a range would say the model has a
// price it does not have, and recording the cheapest would say the caller pays
// something it does not pay.
//
// The endpoint documents are read for one thing only, and only where the
// listing left a hole: a model whose fronted seller states no completion
// ceiling, or forwards no parameter implying any capability, is looked up so
// the other sellers can answer. Where the listing answered, the fronted
// seller's answer stands. The ceiling taken is the largest any seller will
// return, since each states what it will itself produce and the largest is the
// longest answer the model is published as able to give.
//
// # Rates
//
// Rates are published per single unit, as a decimal string of dollars per
// token. They are scaled here to the denominators the rest of the catalog
// uses, per million tokens and per thousand calls. The scaling is exact
// rational arithmetic rather than floating point, so a rate of "0.000002"
// records as 2 and not as 1.9999999999999998.
//
// A zero rate is ambiguous in this source: OpenRouter writes zero both for a
// model that costs nothing and for a charge that does not apply to the model at
// all, which is why a zero is otherwise dropped rather than recorded as a rate
// of nothing. On the prompt and completion keys the ambiguity is gone, because
// every model is billed on both. A model charged zero for both is free, and
// that is recorded as a rate of zero as well as an attribute, so a consumer
// reading prices can tell it apart from a model whose rate is unknown.
//
// # Capabilities
//
// OpenRouter publishes no capability list, and states capabilities three ways
// instead, none of them by name. It states the parameters its API will forward
// for a model, and accepting a parameter implies the capability that parameter
// drives. It states the charges it levies, and a rate for reading a cache or
// for running a search is a statement that the model does the thing being
// billed, since nothing is charged for a capability the model lacks. And it
// attaches a reasoning object to a model that thinks before answering and to no
// other, so the object's presence is the capability and its members say how far
// the caller may turn the thinking up.
//
// All three are translated into the catalog's own vocabulary. The parameter
// names are kept as well, under their own key, because "accepts a
// response_format parameter" is a fact about the request and "supports
// structured output" is a fact about the model, and a consumer should be able
// to ask either question without the answer to one standing in for the other.
//
// # What is not published
//
// Thirty-eight models carry no completion ceiling, and it is not being missed:
// their entry states none, and every seller in their endpoint document states
// none either. Every xAI, Mistral and Perplexity model here is in this group,
// along with OpenRouter's own routing pseudo-models, whose ceiling depends on
// whichever model the router picks. The model pages say no more than the API
// does.
//
// Eleven models carry no capability at all, for the same reason: OpenRouter
// forwards them no parameter that implies one, bills them for nothing but their
// tokens, and gives them no reasoning object. Their model pages state nothing
// further. An empty list here is the source's answer, not a gap in the reading
// of it.
package openrouter
