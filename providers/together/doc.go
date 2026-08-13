// Package together parses Together AI's model catalog into the catalog model.
//
// Together is a host rather than a lab: it serves other people's models, so
// every entry carries the organization that made it and an API string that is
// a path rather than a name. It publishes one page holding eight tables of
// eight different shapes, one per modality, and the shape is the only thing
// saying what a rate means. An image model is priced per megapixel with a
// default step count, a video model per video at a fixed resolution and
// duration, an audio model per million characters or per minute of audio
// depending on which direction it runs, and an embedding model per million
// tokens beside the width of the vector it returns.
//
// The chat table also reports two capabilities and the quantization a model is
// served at. The capabilities are a yes-or-no column each, and only the yeses
// are recorded: a model answering no to both carries no capability list, which
// is as close as the catalog comes to saying it has neither.
//
// The third capability has no column. Together states which models reason on a
// page of its own, as a table of the models that do, so that page is read too
// and its rows matched to the catalog's by the identifier both state. That
// table says more than the fact: its type column separates a model that always
// reasons from one that can be switched and one that takes an effort setting,
// which is recorded alongside. A row naming a model the catalog does not carry
// is skipped rather than creating one, because the page closes by naming
// several more that run only on dedicated inference.
//
// What a model takes and returns is stated by which of the eight tables it is
// listed in, and nowhere else. A model in the vision table is a chat model that
// also takes images — every one of them is in the chat table too — so the two
// readings are merged rather than one replacing the other. The audio table is
// the only one whose models run in two directions, and it has a column saying
// which. An embedding model and a reranker are recorded as working in text on
// both sides, which names the medium and not the return value: one answers with
// a vector and the other with a set of scores, and the catalog has a word for
// neither. Both sides are always set together, since a consumer reading one
// alone cannot tell an unstated output from a model that returns nothing.
//
// What Together does not publish: a bound on output length, for any model. Its
// tables state a context length and nothing else about how much a model may
// generate, and its listing API, which might, needs an account.
//
// Nor does it state a context length for every model it sells. Two chat models,
// Qwen3.7-Max and Qwen3.8-2.4T-A95B, have a dash where the count goes, which is
// how the table writes a fact it does not have. Those two carry rates and no
// bound at all.
package together
