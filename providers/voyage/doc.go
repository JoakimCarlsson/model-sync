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
//   - the pricing page, carrying every rate, the free allowances, and the
//     storage and batch terms stated in prose.
//   - the embeddings, multimodal, contextualized-chunk and reranker pages, for
//     context lengths, embedding dimensions and descriptions.
//   - MongoDB's model overview, Voyage now being part of MongoDB. It restates
//     the same tables under its own column headings, and it is read because it
//     is the only page still stating the context length and the embedding
//     width of voyage-code-2, which Voyage dropped from its own capability
//     pages while continuing to serve and to charge for it.
//   - the batch inference page, for the list of models the discount covers.
//
// The overview is read after Voyage's own pages, so that where the two
// disagree the primary document wins. They disagree about voyage-4-nano, whose
// widths Voyage gives as 1024, 256, 512 and 2048 and MongoDB as 512, 128 and
// 256; the first document to state a set of widths is the one recorded, since
// merging them would report a set neither of them states.
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
// Two capabilities are recorded, neither of them one of the catalog's canonical
// values, which describe what a model answering in words can do: that a model
// offers more than one embedding width, which its dimension column states by
// listing them, and that it can return a vector narrower than a 32-bit float,
// which one sentence of the embeddings page states by naming the models it
// applies to.
//
// What Voyage does not publish:
//
//   - a display name for any model. Every page, Voyage's own and MongoDB's,
//     names a model by its identifier alone, and the marketplace pages that
//     might carry a listing title state only the family, as in "Voyage 4
//     models".
//   - an output token ceiling, for any model, which is not an omission: an
//     embedding is a vector of a fixed width and a reranking is one score per
//     document, so neither has a token count to cap. The width is recorded
//     instead, under the embedding dimension keys.
//   - a rate for the model whose weights it publishes, which costs it nothing
//     to state because running it is the reader's own affair.
package voyage
