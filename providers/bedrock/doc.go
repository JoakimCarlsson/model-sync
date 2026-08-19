// Package bedrock parses AWS Bedrock's price list and user guide into the
// catalog model.
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
// A third shape of meter states neither field. Those are what AWS bills for
// besides inference: an hour of provisioned throughput, bought outright or
// against a term of one or six months, which is the only thing separating
// three rates quoted per model unit hour; the tokens and the images a
// customization job consumes and the month of storing what it produced; an
// hour of reinforcement fine-tuning; and a request whose answer was grounded.
// The usage type is the only field naming those, so it is read for them, with
// the region it opens with and the model's own name after it taken off. It is
// also where the one metric no other field states is read from, the cached
// prompt read back through global routing.
//
// The models Amazon built itself before Nova are metered under a field of
// their own, so a product naming no model in the usual field is read from
// that one. Their meters name no lab either, as do the meters billing for the
// customization of another lab's model, and a meter naming no lab is folded
// into the model already carrying exactly that name where one model does. The
// products are read in the order of the identifiers AWS keys them by, in two
// passes, so that the folding does not depend on which product was read first.
//
// A picture is metered apart from a picture. An image model is billed per
// image at a rate depending on whether the prompt was text or another image,
// how large the result is and which of two qualities was asked for, and a
// video model on how many frames a second it runs at, all of it stated inside
// the metric field. Those are recorded as dimensions of the rate, since
// without them a model carries four rates per region that differ only in
// amount.
//
// The list says nothing else. A billing document names a model and charges for
// it, and never states a context window, a modality or a capability. AWS
// states those on a model card per model in the user guide, one of which is
// fetched for every card the guide's contents index, in the markdown AWS
// serves beside each rendered page. The pages that once carried the same facts
// in one table, models-supported and models-features, now say only that the
// cards have them.
//
// The cards are also where most of the models come from. The guide cards every
// model Bedrock serves and the price list meters only some of them, so a card
// no model claims describes a model of its own: without them the catalog holds
// no Stability image model, no Cohere or Titan embedding model, no reranker,
// and no Anthropic model past Claude 3. Such a model is named by the lab the
// card credits it to and the title the card carries, and it carries every
// fact the card states and no rate, because a card usually states none. A
// model AWS documents and does not publish a price for is still a model AWS
// serves.
//
// A card states support as a picture rather than as a word: every modality,
// capability and endpoint AWS knows of is listed and each carries a tick or a
// cross, so what a model lacks is named as plainly as what it has. Only the
// ticks are read, and the heading above a marked entry decides which list it
// joins, because the same word stands under Input Modalities and under Output
// Modalities and the column beside them holds the endpoints. Everything else
// on a card is a table too, and which table it is stands in its headings
// rather than in a heading of its own, so the headings are what a reader
// dispatches on: the Regions the model is offered in and whether each is
// reached directly or only across Regions, the identifiers it answers to on
// each endpoint together with the inference profiles routing to it, the
// service tiers it is offered on, and, for the models that cache prompts, how
// large a prompt has to be to be cached, how many checkpoints a request may
// carry, how long they live and which fields accept them.
//
// Eight cards state a table of rates as well, and for five of those models the
// price list carries no meter at all, so the card is the only place AWS states
// what they cost. Those rates vary by how a request is routed rather than by
// Region, which is a dimension of their own and not one the price list uses,
// so a model carrying both kinds keeps them apart. Nothing is read from such a
// table unless the sentence under it states the denominator and the tier the
// rates are for, because a column of amounts says what a thing costs and never
// what it is counted in.
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
// serving words dropped and a one-place version's trailing zero taken off.
//
// The names are compared in two rounds. Every model naming a card outright
// takes it first, and only the cards left over are offered to a model that
// merely begins one or is begun by one, because the exact match is the surer
// reading: Nova Sonic 2.0 begins the Nova Sonic card while naming the Nova 2
// Sonic one, and the list's Mistral Large 2407 and Titan Image Generator G1
// would otherwise take the cards of the Mistral Large and the Titan Image
// Generator G1 v2 that the list meters separately. A name matching nothing is
// tried again without the author's words and then with them, since the list
// writes "R1" for what the card calls DeepSeek-R1 and "google.gemma-4-31b" for
// its Gemma 4 31B, and last of all without the release it ends in, which is
// only accepted where one card is left: the list's Voxtral Mini 1.0 is the
// card's Voxtral Mini 3B 2507.
//
// Six pages of the guide are read besides the cards, each stating one fact
// about many models where a card states many about one. They are read last,
// because every one of them keys on the model identifier that only a card
// supplies.
//
// The model lifecycle page dates the retirement of the models AWS is
// withdrawing. A card states an end of life AWS undertakes not to come before,
// and states none at all for the four models in five that are not being
// withdrawn; this page names the day a model stops being offered to new
// callers, the day its price may rise as it enters public extended access, and
// the day it stops answering. Its entries are read defensively, one of them
// having its labels and its values out of step, so a value not shaped like the
// thing its label names is dropped rather than recorded.
//
// Two pages name, per model, the Regions a serving path may be used in: one
// for batch inference, which distinguishes the Regions a model may be batched
// in directly from those reached through an inference profile, and one for
// latency-optimized inference. Both list the Regions rather than answering
// yes, so a model named with no Region against it is one the page mentions and
// does not offer the path in.
//
// The quota page for the bedrock-mantle endpoint states a default for the
// tokens a model may be sent and may generate per minute. It names the model
// in prose rather than by identifier, so it is joined on the name and only
// where exactly one model is named by those words.
//
// Three pages state the specification of one of the models Amazon built
// itself as a labelled list, and they are the only pages saying how wide a
// vector an embedding model returns. Where the widths are a set the caller
// chooses from, all of them are recorded and the one AWS marks as the default
// is recorded again on its own.
//
// What AWS does not publish:
//
// A card for every model it bills for. Claude 2.0, Claude 2.1, Claude Instant
// and Claude 3 Sonnet are metered and carded nowhere, being past the end of
// the life the guide's model lifecycle page tracks, which lists only models
// still short of theirs. Nova 2.0 Pro and Nova 2.0 Omni are metered and not
// yet carded, being in preview. Mistral Large 2407 is metered and carded only
// in the release the list meters as plain Mistral Large. The Titan text models
// are metered and were never carded at all, and three more Titan meters name
// models the cards name differently enough that no reading of the two names
// would join them: the list's Titan Image Generator G1 and Titan Embeddings G1
// Image and its TitanEmbeddingsV2-Text-input. Those thirteen carry rates and a
// display name and nothing more.
//
// A capability table on every card it does write. The cards for Claude 3
// Haiku, Nova Canvas, Nova Sonic and the rerankers state the model's
// modalities and stop, and some of them state no context window or output
// ceiling either.
//
// An output ceiling for the models whose cards state a context window and
// leave the line off, or write it as N/A, which is what the Gemma 4 releases,
// the Voxtral releases, several of the GPT-5.6 releases and Grok 4.3 do. No
// context window at all for an image model: not one Stability card states one.
//
// Reasoning for a model it does not name as reasoning. The detail is stated
// only where AWS chose to state it, so DeepSeek V3.1, the gpt-oss releases and
// the GLM releases carry no such line, and nothing else on the site supplies
// one: the guide's own reasoning page refers the reader back to the cards.
//
// A quota per model. The guide describes the quotas that exist and refers the
// reader to the Service Quotas console for their values, except for the one
// table of defaults on the bedrock-mantle page, which today names a single
// model. The burndown rate at which an output token is charged against a quota
// is stated in prose against families rather than models, naming "Anthropic
// Claude models version 4.8" and "all other Anthropic models version 4.7 and
// below", which is not a join to a model and is not read.
//
// How wide a vector its resellers' embedding models return. The three pages
// stating a width state it for Amazon's own models. The inference parameter
// pages beside them are written per family rather than per model, so the
// widths Cohere's Embed v4 accepts stand on a page describing a request shape
// and are reachable only through a name in a code sample, and are not read.
//
// The remaining gaps in the capability lists are not silence but refusal. A
// card marks structured outputs and client-side tool calling with a cross as
// readily as with a tick, and it is a cross that stands against structured
// outputs on Llama 3.1 8B and against both on Llama 3.2 1B. Those models are
// documented as not having the capability rather than undocumented, so the
// count of models carrying it can never reach the count of models served.
package bedrock
