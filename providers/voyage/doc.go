// Package voyage parses Voyage AI's published documentation into the catalog
// model.
//
// Voyage is the provider that most obviously cannot share a parser with the
// others. It sells embeddings and rerankers rather than chat, so it has no
// output tokens at all; it bills multimodal work per billion pixels, a
// denominator nobody else uses; it charges file storage per gigabyte per
// month; it grants a free token allowance per account that is part of the
// price table; and it states the same rate twice per row, per thousand tokens
// and per million tokens, in columns that do not always agree.
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
//   - the batch inference page, for the list of models the discount covers.
//
// Which of those pages a model is documented on is also the only thing Voyage
// says about modality: the multimodal page is the one whose models vectorize
// text and pictures together, and the rest are text. No page states an output
// modality, and none is recorded, because an embedding model returns a vector
// and a reranker returns a score rather than a modality.
//
// What Voyage does not publish: a capability list, for any model; a modality
// for a model that has fallen off the capability pages and survives only in the
// rate tables; and a rate for the model whose weights it publishes, which costs
// it nothing to state because running it is the reader's own affair.
package voyage
