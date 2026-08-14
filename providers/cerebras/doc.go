// Package cerebras parses Cerebras' model catalog into the catalog model.
//
// Cerebras publishes a catalog naming every model it serves and, for each of
// them, a page of its own. The catalog says what a model holds and under which
// standing it is offered; the page says what it costs, what it can do and what
// it accepts, none of which the catalog repeats. Both are read, and so is the
// public model list its API answers with, which is Cerebras stating for itself
// which models it currently sells rather than a page describing them.
//
// A model page ends with one call carrying every fact about the model as an
// attribute, so the page is read as attributes rather than as prose: the
// rates, the output ceilings, the capabilities, the endpoints and the formats
// are all values of that call.
//
// Two of its fields are unusual. A context window and an output ceiling differ
// by plan, written as "65k / 131k" for free and paid in the catalog and as two
// named tiers on the page. Both are kept, because the free ceiling is what a
// reader on the free plan actually gets. And one page writes its rates with
// the denominator, "$2.25 / M tokens", while another writes only the amount;
// the missing denominator is read as the per-million-token one, which is the
// only denominator Cerebras quotes anything against.
//
// The paid ceilings are the two facts the public list is read before either
// document for. Both documents round them to "131k" and "40k"; the list states
// 131072 and 40960. The free ceilings have no such statement and stay as the
// documents round them.
//
// The catalog opens with a notice when a model has a date to go by, naming it
// as its own tables name it. That date is recorded as a date and not as a
// state: the model is served until it arrives, and recording it as withdrawn
// would drop from the catalog something Cerebras is still selling.
//
// What Cerebras does not publish: rates for its plans in a form tied to a
// model. It sells a monthly coding subscription by a daily token allowance,
// and that is a plan rather than a model's price, so it is not recorded
// against one.
//
// Nor does it publish a model for its dedicated endpoints to be entered as.
// Those pages list a dozen families of weights, by their Hugging Face
// repositories, that Cerebras will host on reserved capacity, with no
// identifier to call one by, no rate, and no ceiling, since the endpoint is
// provisioned by talking to Cerebras. What it sells self-serve is what the
// public list answers with, and that is what this package carries.
package cerebras
