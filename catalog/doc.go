// Package catalog defines the provider-independent representation of a model.
//
// A price is a tuple: (metric, unit, amount, dimensions). Capabilities follow
// the same rule: they are open maps whose keys are provider vocabulary, not
// struct fields. Metric, Unit and Kind are bare string types with no constants
// declared here, so a provider that bills or behaves in a way no other
// provider does adds constants to its own package and never edits this one.
//
// This package holds no vendor vocabulary, parses no vendor text, performs no
// I/O, and compiles with zero providers linked.
package catalog
