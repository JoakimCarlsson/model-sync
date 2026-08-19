// Package voyage parses Voyage AI's published documentation into the catalog
// model.
//
// Voyage is the provider that most obviously cannot share a parser with the
// others. It sells embeddings and rerankers rather than chat, so it has no
// output tokens at all; it bills multimodal work per billion pixels, a
// denominator nobody else uses; it charges file storage per gigabyte per
// month; it grants a free token allowance per account that is part of the
// price table; and it states the same rate twice per row, per thousand tokens
// and per million tokens, in columns that have disagreed and so are both
// recorded.
//
// Its own documents are not even consistent with each other: the reranker page
// states its model table as HTML while every other page uses markdown, so this
// package reads both.
//
// Documents read:
//
//   - the pricing page, carrying every rate, the free allowances, the storage
//     and batch terms stated in prose, and the pixel band an image is billed
//     in.
//   - the embeddings, multimodal, contextualized-chunk and reranker pages, for
//     context lengths, embedding dimensions and descriptions.
//   - MongoDB's model overview, Voyage now being part of MongoDB. It restates
//     the same tables under its own column headings, and it is read because it
//     is the only page still stating the context length and the embedding
//     width of voyage-code-2, which Voyage dropped from its own capability
//     pages while continuing to serve and to charge for it.
//   - the batch inference page, for the list of models the discount covers and
//     the window a batch is completed in.
//   - the rate limit page, for the requests and the tokens a minute each model
//     allows.
//   - the tokenization page, for the tokenizer a model uses.
//   - the OpenAPI definition of each of the four endpoints, published on the
//     API reference pages, for the parameters a request may carry and the
//     bounds it may not exceed.
//   - the AWS Marketplace listing of each model MongoDB sells there, for the
//     only display name anyone publishes.
//   - the post announcing each model, for the day it was announced.
//
// The overview is read after Voyage's own pages, so that where the two
// disagree the primary document wins. They disagree about voyage-4-nano, whose
// widths Voyage gives as 1024, 256, 512 and 2048 and MongoDB as 512, 128 and
// 256; the first document to state a set of widths is the one recorded, since
// merging them would report a set neither of them states.
//
// # What an endpoint says about a model
//
// Voyage documents a request rather than a model. The parameters a call may
// carry, the number of items it may hold and the tokens it may spend are
// stated once per endpoint, in the OpenAPI definition of that endpoint, and
// nowhere against the models themselves. They are recorded against every model
// the endpoint serves, which is the set of models the matching guide page
// tables, and the exceptions inside those descriptions name their own models:
// a token budget that differs between voyage-4-lite and voyage-4-large is
// written as a number followed by the models it holds for, and is read that
// way.
//
// A model whose weights Voyage publishes is left out of that, because a bound
// on a request to Voyage's API says nothing about a model Voyage does not
// host.
//
// The token budget is read from the guide page as well as from the definition,
// because the guide still names two models the definition has stopped naming
// while Voyage continues to serve them. The two agree wherever both speak.
//
// The models on MongoDB's overview are placed by the headings above their
// table rather than by the address of the page, which is why the heading chain
// is kept and not only the nearest heading: the overview says which family a
// table belongs to two or three levels up, with headings in between saying
// only that what follows is the older models.
//
// # Capabilities
//
// None of the catalog's canonical capability values applies to anything Voyage
// sells. They describe what a model answering in words can do, and nothing
// here answers in words. What Voyage does publish per model is recorded under
// this package's own names: that a model offers more than one embedding width,
// which its dimension column states by listing them; that it can return a
// vector narrower than a 32-bit float, which the output type parameter states
// by naming the models it applies to; that it distinguishes a query from a
// document, Voyage prepending a different retrieval prompt to each; that an
// over-length input is cut to fit rather than refused; that it will chunk a
// whole document itself; and that its relevance can be steered by an
// instruction written into the query, which the reranker page grants to two of
// its six models.
//
// The set of vector encodings itself is recorded alongside, since the
// capability says only that something narrower than a float is available and
// the caller still has to know which of int8, uint8, binary and ubinary to
// ask for. Every model the embedding endpoints serve carries at least the
// float that Voyage says all of them support.
//
// # Modality
//
// Which of those pages a model is documented on is the main thing Voyage says
// about modality: the multimodal page, and the multimodal heading of the
// overview, are where the models that vectorize text and pictures together are
// listed, and the rest are text. The one exception is video, which one
// sentence of the multimodal page grants to voyage-multimodal-3.5 while
// withholding it from the older model in the same table. No page states an
// output modality. Text is recorded as the output of every model that has an
// input, because the medium each of them works in is text on both sides, and
// the two sides are always set together: a consumer reading one alone cannot
// tell an unstated modality from a model that takes or returns nothing.
//
// What that does not record is the shape of the return value. An embedding is a
// vector and a reranking is a set of scores, and neither is a modality the
// catalog has a word for, so neither is stated here.
//
// # Standing and dates
//
// Voyage sorts its models into the current ones and, under a heading saying a
// newer model is better, the older ones, which are recorded as active and
// legacy. Five of the older ones carry a stronger statement: their own
// description opens with the word deprecated, and that overrides the standing
// the rate table gave them. It is the only thing Voyage says about a model
// being on the way out. There is no retirement date, no deprecation date and
// no shutdown notice for any model, and the deprecated five are still served
// and still priced.
//
// There is no changelog either. What Voyage dates is the post announcing a
// model, linked from the model's own row, and that is what release_date holds:
// the day Voyage announced the model, not a separate day it began serving it,
// which no document states. The post states its day twice, as an instant in
// UTC and as the day the post carries, and they disagree whenever a post went
// up in the Pacific evening; the day recorded is the one Voyage prints on the
// post and puts in its address.
//
// # Rate limits
//
// Voyage states one pair of limits per model for the first usage tier and
// gives the higher tiers as multiples of it, which is also how its own worked
// example states them, so the multiples are recorded rather than left for a
// consumer to compute. One row of that table names a generation instead of its
// members, "voyage 1 & 2 Series embedding models", and is expanded to the
// embedding models whose identifiers carry those generation numbers, which is
// the only reading of the phrase the identifiers admit.
//
// # What Voyage does not publish
//
//   - a display name of its own for any model. Every page, Voyage's own and
//     MongoDB's, names a model by its identifier alone. The name recorded here
//     comes from the AWS Marketplace listing MongoDB sells the model through,
//     which titles one model rather than a family. A listing is not told which
//     model it belongs to: its title is reduced to the identifier it spells and
//     is recorded only against a model already known from Voyage's own pages,
//     so a listing that is renamed or withdrawn drops out rather than naming
//     the wrong thing. Twelve models are sold that way; the listings for the
//     models Voyage sold before the acquisition now serve a page with no
//     listing in it, and those models stay unnamed.
//   - an output token ceiling, for any model, which is not an omission: an
//     embedding is a vector of a fixed width and a reranking is one score per
//     document, so neither has a token count to cap. The width is recorded
//     instead, under the embedding dimension keys.
//   - a rate for the model whose weights it publishes, which costs it nothing
//     to state because running it is the reader's own affair.
//   - a tokenizer name for any model past the first two generations. It names
//     Llama 2 as the tokenizer its earlier models share and says only that the
//     newer ones have their own, publishing each as a repository rather than
//     as a name.
package voyage
