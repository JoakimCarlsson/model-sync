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
// The list states a rate two ways, because AWS has added a second endpoint
// without rewriting the meters of the first. The older meter names the metric
// and the serving path in one field, so that an "Output tokens priority" rate
// is the output metric on the priority path, and takes the path from the
// product's feature where that field names none. The newer meter, for the
// models reached through bedrock-mantle, leaves both empty and states the
// metric as an identifier, input_tokens_mantle, beside a service tier of its
// own. Both are read, and the service tier only where nothing else states the
// path: a model offered on both endpoints carries a tier in each shape, and
// reading the newer field for the older meters would price one path twice.
// Rates are recorded at the denominator AWS publishes, which is usually per
// thousand tokens rather than per million.
//
// It says nothing else. A billing document names a model and charges for it,
// and never states a context window, a modality or a capability. AWS states
// those on a model card per model in the user guide, one of which is fetched
// for every card the guide's contents index, in the markdown AWS serves beside
// each rendered page. The pages that once carried the same facts in one table,
// models-supported and models-features, now say only that the cards have them.
//
// A card states support as a picture rather than as a word: every modality,
// capability and endpoint AWS knows of is listed and each carries a tick or a
// cross, so what a model lacks is named as plainly as what it has. Only the
// ticks are read, and the heading above a marked entry decides which list it
// joins, because the same word stands under Input Modalities and under Output
// Modalities and the column beside them holds the endpoints.
//
// Joining a card to a meter is done on the identifier where both documents
// give one. A meter reaching a model through bedrock-mantle names it in the
// usage type it bills under, and every card names the same identifier in the
// table of endpoints the model answers on. That join is exact, and it is what
// says the list's "NVIDIA Nemotron Nano 2" is the card's "NVIDIA Nemotron Nano
// 9B v2", which no reading of the two names would.
//
// The meters of the older shape carry no identifier, and those are joined on
// the prose name, which the two documents write differently: the list writes
// "Llama 3.1 70B" where the card writes "Llama 3.1 70B Instruct", "Nova 2.0
// Lite" where the card writes "Nova 2 Lite", and "Nova Sonic 2.0" where the
// card writes "Nova 2 Sonic". Both are reduced to letters and digits with the
// serving words dropped and a one-place version's trailing zero taken off, and
// a card naming exactly those words in any order is taken before one that
// merely begins with them, because Nova Sonic 2.0 begins the Nova Sonic card
// while naming the Nova 2 Sonic one. A name matching nothing is tried again
// without the author's words and then with them, since the list writes "R1"
// for what the card calls DeepSeek-R1 and "google.gemma-4-31b" for its Gemma 4
// 31B, and last of all without the release it ends in, which is only accepted
// where one card is left: the list's Voxtral Mini 1.0 is the card's Voxtral
// Mini 3B 2507.
//
// What AWS does not publish:
//
// A card for every model it bills for. Claude 2.0, Claude 2.1, Claude Instant
// and Claude 3 Sonnet are metered and carded nowhere, being past the end of
// life the guide's model lifecycle page tracks, which lists only models still
// short of theirs. Nova 2.0 Pro and Nova 2.0 Omni are metered and not yet
// carded, being in preview. Those six carry rates and a display name and
// nothing more.
//
// A capability table on every card it does write. The cards for Claude 3
// Haiku, Nova Canvas and Nova Sonic state the model's modalities and stop,
// and the last two state no context window or output ceiling either.
//
// An output ceiling for eight models whose cards state a context window: the
// three Gemma 4 releases, the two Voxtral releases, GPT-5.6 Luna, GPT-5.6
// Terra and Grok 4.3 leave the line off, and GPT-5.4 writes it as N/A.
//
// Reasoning for a model it does not name as reasoning. The detail is stated
// only where AWS chose to state it, so DeepSeek V3.1, the gpt-oss releases and
// the GLM releases carry no such line, and nothing else on the site supplies
// one: the guide's own reasoning page refers the reader back to the cards.
//
// The remaining gaps in the capability lists are not silence but refusal. A
// card marks structured outputs and client-side tool calling with a cross as
// readily as with a tick, and it is a cross that stands against structured
// outputs on Llama 3.1 8B and against both on Llama 3.2 1B. Those models are
// documented as not having the capability rather than undocumented, so the
// count of models carrying it can never reach the count of models served.
package bedrock
