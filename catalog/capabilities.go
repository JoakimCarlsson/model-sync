package catalog

// ListFeatures is the enumeration a model's capabilities are recorded in. Its
// values name what a model can do, never what parameter an API accepts: a
// provider whose document lists request parameters records those under
// ListParameters instead, so that a consumer can tell "supports structured
// output" from "accepts a response_format parameter".
const ListFeatures = "features"

// ListParameters is the enumeration of request parameter names a provider
// states its API accepts for a model.
const ListParameters = "parameters"

// The capability values a consumer derives a boolean from. Vendors spell these
// several ways, and a provider package translates its vendor's wording to the
// spelling here rather than passing it through: a consumer keying on
// CapabilityStructuredOutputs would otherwise silently miss every provider
// whose document says "JSON mode" instead.
//
// A vendor's own wording is kept alongside these, never instead of them, and
// only where it says something the canonical value does not.
const (
	// CapabilityReasoning is set where the model produces reasoning before its
	// answer.
	CapabilityReasoning = "reasoning"
	// CapabilityStructuredOutputs is set where the model can be constrained to
	// a caller-supplied shape, however the vendor spells it.
	CapabilityStructuredOutputs = "structured_outputs"
	// CapabilityFunctionCalling is set where the model can call tools the
	// caller declares.
	CapabilityFunctionCalling = "function_calling"
	// CapabilityJSONMode narrows CapabilityStructuredOutputs and never
	// replaces it. Some vendors document two strengths of the same capability,
	// one constraining the answer to a caller-supplied schema and one only
	// requiring it to be JSON, and a model offering the weaker one carries
	// both values: CapabilityStructuredOutputs so that the consumer asking
	// whether the answer can be constrained finds it, and this so that the
	// consumer asking how far does not have to guess.
	CapabilityJSONMode = "json_mode"
)
