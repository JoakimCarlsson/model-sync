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
package voyage
