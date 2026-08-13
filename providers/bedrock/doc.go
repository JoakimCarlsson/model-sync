// Package bedrock parses AWS Bedrock's price list into the catalog model.
//
// Bedrock resells other labs' models, so its catalog restates models belonging
// to Anthropic, Meta and others at AWS's prices in AWS's regions. Those are
// recorded as Bedrock's own entries rather than merged into the labs they come
// from, the same as any reseller's.
//
// AWS publishes the machine-readable price list its billing runs on rather
// than a pricing page, which is the best source any provider here offers: no
// markup to parse, and every rate carries the region and serving path it
// applies to. It is also the largest, listing every model against every region
// it is offered in, so one model can carry a hundred rates that differ only by
// where the request lands.
//
// The list states the metric and the serving path in one field: an
// "Output tokens priority" rate is the output metric on the priority path.
// Rates are recorded at the denominator AWS publishes, which is usually per
// thousand tokens rather than per million.
//
// It says nothing else. A billing document names a model and charges for it,
// and never states a context window, a modality or a capability. AWS states
// those on a model card per model in the user guide, one of which is fetched
// for every card the guide's contents index, in the markdown AWS serves beside
// each rendered page.
//
// A card states support as a picture rather than as a word: every modality,
// capability and endpoint AWS knows of is listed and each carries a tick or a
// cross, so what a model lacks is named as plainly as what it has. Only the
// ticks are read, and the heading above a marked entry decides which list it
// joins, because the same word stands under Input Modalities and under Output
// Modalities and the column beside them holds the endpoints.
//
// Joining a card to a meter is done on the name, since neither document
// carries the other's identifier: the price list names a model in prose and so
// does a card, and the two disagree in small ways. The list writes "Llama 3.1
// 70B" where the card writes "Llama 3.1 70B Instruct", and "Nova 2.0 Lite"
// where the card writes "Nova 2 Lite". Both are reduced to letters and digits
// with the serving words dropped and a one-place version's trailing zero taken
// off, and either may then be a prefix of the other, because the list names a
// serving variant the card does not have and the card names a tuning the list
// does not.
//
// What AWS does not publish: a card for every model it bills for. Twenty-five
// of the 76 chat models are metered with no card — the withdrawn Claude 2
// releases, the newest additions the guide has not caught up with, and the
// latency-optimized variants, which are the same model on a faster path and
// are carded under the model's plain name rather than their own. Those carry
// rates and a display name and nothing more.
package bedrock
