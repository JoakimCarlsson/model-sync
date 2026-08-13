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
package bedrock
