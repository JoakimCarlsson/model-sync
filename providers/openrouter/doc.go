// Package openrouter parses OpenRouter's model API into the catalog model.
//
// OpenRouter is the first source here that is not a document. It publishes
// JSON endpoints describing every model it brokers, so there is no markdown,
// no HTML and no prose to read. That it still needs its own package is the
// point: the shape of the source changes nothing about where a provider's
// vocabulary lives.
//
// It is also a marketplace rather than a lab, so its catalog restates models
// belonging to other providers under its own identifiers. openai/gpt-5.6-sol
// here is the same model as gpt-5.6-sol under openai, priced as OpenRouter
// sells it rather than as OpenAI does. Both are recorded; neither is merged
// into the other, because the rates genuinely differ and the identifiers are
// OpenRouter's own.
//
// # The documents
//
// Four routes are read. The model listing names every model and states its
// context, its rates, its parameters and its dates. The provider listing names
// every upstream OpenRouter routes to and says where it is registered and
// where it serves from. One endpoint document per model names the upstreams
// that actually sell it, each with its own rates, its own ceilings, its own
// weight precision and its own parameters. Twelve subject listings say which
// models OpenRouter files under a subject.
//
// The listing is asked for all output modalities rather than for its default,
// and this is not a detail. Asked for nothing the endpoint answers with the
// models that return text or images and silently drops the rest, which is a
// third of the catalog: every embedding model, every reranker, every
// transcriber, and every speech and video model. The default listing is a view
// and not the catalog.
//
// The subject listings are the only route whose vocabulary is not published as
// prose anywhere. Asked for a subject it does not have, the models endpoint
// answers with the list of the ones it does, and that answer is where the
// twelve come from.
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
// The fronted rate is recorded unqualified, because it is what OpenRouter
// presents as the model's price on its own listing and its own model page, and
// it is what a caller who names the model and nothing else is charged. It is
// usually the cheapest seller and not always, and it moves as routing moves,
// so two catalogs built a week apart will disagree about a model whose default
// seller changed in between. That is a fact about the marketplace rather than
// an error in either of them.
//
// Every other seller's rates are recorded beside it, each carrying the seller's
// name as a dimension, along with the tag that says which of that seller's
// offerings it is and the weight precision it serves at. All three are needed
// to keep the rates apart: one seller sells the same model at three service
// levels and another from two regions, a third sells it at two precisions, and
// each of those is its own price. Without them the fan would read as a
// contradiction the source does not have. Qualified this way the fan is not a range and not a
// substitute for the model's own price: a consumer asking what the model costs
// reads the unqualified rate, and a consumer asking what a named seller
// charges reads that seller's.
//
// Ceilings are treated the other way round. A ceiling states what a seller
// will itself return or accept, so the largest any seller states is recorded,
// since it is the longest answer the model is published as able to give and
// any smaller figure is a fact about one seller. Capabilities are unioned for
// the same reason: a caller may route to any seller, so a capability one
// seller offers is one the model has here.
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
// a model charged nothing for anything is free. Such a model records a rate of
// zero on those two keys as well as an attribute, so a consumer reading prices
// can tell it apart from a model whose rate is unknown. The test is every
// published rate and not just those two, because a model billed per image is
// charged zero per token and is not free: there the zero says that tokens are
// not how it is billed.
//
// Two of the keys are published without being documented. A seller may state a
// discount beside its rates, as a bare fraction, and OpenRouter says neither
// what it is reduced from nor whether the published rate already has it
// applied; it is therefore carried as a dimension on that seller's rates and
// no arithmetic is done with it. And an image model may state image_token
// beside image_output, always with the same figure, one naming a token and the
// other not; both are recorded, at the denominator each name implies, rather
// than one being taken for a restatement of the other.
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
// the caller may turn the thinking up. A seller may also state that it caches
// without being asked to, which is the same capability said a fourth way.
//
// Only the parameter list is read conditionally, and only because OpenRouter
// forwards the same one to models that could not use it: an embedding model is
// published as accepting temperature and top_k, and a transcription model as
// accepting response_format, which there names the shape of the transcript and
// not a schema the model is held to. So the parameters are recorded for every
// model and read as capabilities only for the models that write an answer.
// The rates and the reasoning object are read for every model, since neither
// is boilerplate: both are stated per model and only where they apply.
//
// All of them are translated into the catalog's own vocabulary. The parameter
// names are kept as well, under their own key, because "accepts a
// response_format parameter" is a fact about the request and "supports
// structured output" is a fact about the model, and a consumer should be able
// to ask either question without the answer to one standing in for the other.
//
// # Scores
//
// OpenRouter attaches two kinds of third-party benchmark to a model. The
// Artificial Analysis indices are recorded, named for the house that published
// them, because each is one number about one model. The arena rows beside them
// are not: they are one row per arena and per category, and an elo and a rank
// state a standing against the other models rather than anything about this
// one.
//
// # Embedding widths
//
// The width of the vector an embedding model returns is in none of the
// structured fields, and OpenRouter states it in the sentence describing the
// model or not at all. Where the sentence states it, it is read: a width given
// as "1024-dimensional" is the width the model returns, and a list given as
// "in 2048, 1024, 512, and 256 dimensions" is a choice the caller has. Half
// the embedding models say neither, and for those the width is not recorded,
// because guessing it from the name of the model is not reading it from the
// source.
//
// # What is not published
//
// Whether a model's weights are open is not stated. Roughly a third of the
// entries carry a Hugging Face identifier and the rest do not, but the
// identifier is a link and not a licence, and OpenRouter never says that
// having one means the weights may be had. Nothing here records it.
//
// The per-request limits object is present on every entry and empty on all of
// them, and no route states a rate limit of any kind for a model or for a
// seller. The endpoint documents carry latency and throughput members, and
// both are null everywhere.
//
// Uptime and status are published per seller and are not recorded. They are
// telemetry over the last five minutes, the last half hour and the last day:
// they answer whether a seller is up now, which is a different question from
// what the seller sells, and recording them would rewrite most of the catalog
// on every run without any of it being news.
//
// The input modality "file" is left as OpenRouter writes it. The
// documentation lists the value and defines nothing by it, so calling it a PDF
// here would be this package's claim rather than the vendor's. The two output
// modalities named for a task rather than a medium are the exception:
// "speech" is audio and "transcription" is text by the plain meaning of the
// words, so each carries the medium alongside the vendor's own value.
//
// A little over fifty models publish a rate of zero on every key they carry,
// among them most of the video models. Nothing else about them is priced and
// no seller of them states a rate either, so they record as charged nothing,
// which is all the source says about what they cost.
//
// Thirty-one models have no seller at all: their endpoint document lists none,
// so the listing entry is everything there is to know about them. Some carry
// no completion ceiling for the same reason, and the model pages say no more
// than the API does. An empty list here is the source's answer, not a gap in
// the reading of it.
package openrouter
