// Package validate refuses to publish a catalog a consumer cannot read.
//
// Every rule here answers a question a consumer of api.json has to be able to
// answer from the data alone. A negative amount cannot be multiplied by a
// token count. A price without a unit cannot be normalized against any other
// price. An output ceiling above the context window is not a value any request
// can ask for. Two amounts under one (metric, unit, dims) key leave a consumer
// choosing between them arbitrarily. A dimension value whose casing varies by
// provider breaks the match a consumer uses it for. A model without an API
// identifier cannot be called.
//
// The rules are checked against the assembled catalog rather than inside each
// parser, because that is where a contradiction between two providers, or
// between two documents one provider publishes, becomes visible. They run in
// the build before the aggregate is written, so a regression fails the sync
// instead of shipping.
//
// A Problem is either an error, which stops the aggregate being written, or a
// warning, which is reported and does not. The split is by whether a consumer
// is misled or merely underserved: a chat model priced only under a fine
// tuning dimension has no ordinary inference rate to show, which is a gap in
// what was scraped rather than a statement that is wrong, so it warns.
//
// This package holds no vendor vocabulary except the names of the pricing
// dimensions that qualify a rate as not being ordinary inference, which is
// stated once, in fineTuningDims.
package validate
