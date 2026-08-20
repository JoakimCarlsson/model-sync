// Package catalog defines the provider-independent representation of a model.
//
// A price is a tuple: (metric, unit, amount, dimensions). Capabilities follow
// the same rule: they are open maps whose keys are provider vocabulary, not
// struct fields. Metric, Unit and Kind are bare string types with no constants
// declared here, so a provider that bills or behaves in a way no other
// provider does adds constants to its own package and never edits this one.
//
// Two things are the catalog's own words rather than a provider's, because a
// consumer cannot read the data without them. APIID is the attribute holding
// the exact string to send as the model in a request, which every model
// carries: without it a consumer needs one rule per provider for whether to
// read model_path, model_id or api_identifier. And a Price may be Variable,
// which serializes its amount as null: a rate that exists without a number is
// then read from the field the number would be in, rather than from a sentinel
// a consumer would have to know to recognize.
//
// This package holds no vendor vocabulary, parses no vendor text, performs no
// I/O, and compiles with zero providers linked.
package catalog
