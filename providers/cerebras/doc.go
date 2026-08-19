// Package cerebras parses Cerebras' model catalog into the catalog model.
//
// Cerebras publishes a catalog naming every model it serves and, for each of
// them, a page of its own. The catalog says what a model holds and how fast it
// runs; the page says what it costs, what it can do, what it accepts and what
// it is known to get wrong, none of which the catalog repeats. Both are read,
// and so is the public model list its API answers with, which is Cerebras
// stating for itself which models it currently sells rather than a page
// describing them.
//
// The public list is read before every document, and is the richest of them.
// It states the two ceilings to the token where the pages round them, the two
// rates against a single token where the pages quote them against the million,
// the repository the weights come from, the tokenizer, the instruct format,
// the request parameters the model accepts, the capabilities as flags, and two
// flags saying whether the model is in preview and whether it is on its way
// out. The standing of a model is read from those two flags, because they are
// the only statement of it that does not depend on how the catalog page
// happens to be laid out this month.
//
// A model page ends with one call carrying every fact about the model as an
// attribute, so the page is read as attributes rather than as prose: the
// rates, the output ceilings, the capabilities, the endpoints, the formats,
// the per-plan rate limits and the caveats are all values of that call. The
// paragraph of prose above that call summarizes the same facts and is not
// read, because it goes stale: it currently states a rate and a speed for
// Gemma 4 that the call beside it, the catalog and the public list all
// contradict.
//
// Three of its fields are unusual. A context window and an output ceiling
// differ by plan, written as "65k / 131k" for free and paid in the catalog and
// as two named tiers on the page. Both are kept, because the free ceiling is
// what a reader on the free plan actually gets, and the rate limits are kept
// the same way, under a bare key for what a paying caller gets and a suffixed
// one for the free trial. And one page writes its rates with the denominator,
// "$2.25 / M tokens", while another writes only the amount; the missing
// denominator is read as the per-million-token one, which is the only
// denominator Cerebras quotes anything against.
//
// The rate limit page states the same per-plan tables as the model pages and
// adds the hourly bound they leave out; the model pages state the one bound it
// leaves out, how many images a single request may carry. Both are read and
// they agree. Cerebras heads the same number TPM on the one and an input token
// rate on the other, and it is recorded once, under the canonical key.
//
// The change log and the deprecation record are read for dates. A model's
// release date is the creation date the public list states for it, and where
// the list states none, the day the change log announced the model was now
// available. The deprecation record dates what Cerebras has withdrawn and says
// what to move to, naming the model by identifier where the notice on the
// catalog page names it by title. All three of those pages reach back years
// and name models nobody can call any more, so none of them may create a
// model: they only add to one the catalog or the public list has named.
//
// A deprecation date is recorded as a date and not as a state, because the
// model is served until it arrives and recording it as withdrawn would drop
// from the catalog something Cerebras is still selling.
//
// The licence is the one document read that Cerebras did not write. Cerebras
// publishes which weights it serves, by naming a public repository per model
// in its own model list, and it is that repository that states what the
// weights may be used for. The repository is fetched and its licence taken,
// and nothing else is taken from it. Naming a public repository is also what
// is read as the weights being open, since Cerebras states of its public
// endpoints that it hosts open-source models from the community and serves
// them unmodified.
//
// What Cerebras does not publish: a knowledge cutoff for any model, on the
// catalog, on a model page or in the list. Nor an amount for its plans. It
// sells credits, and it sells a monthly subscription per model in several
// tiers, but the console page that describes those tiers names no rate for any
// of them, so there is nothing to record against a model. The free trial is
// credits with an expiry and not an allowance per model, which the rate limit
// page says of itself when it says there is no permanently free tier.
//
// Nor does it publish a model for its dedicated endpoints to be entered as.
// Those pages list a dozen families of weights, by their Hugging Face
// repositories, that Cerebras will host on reserved capacity, with no
// identifier to call one by, no rate, and no ceiling, since the endpoint is
// provisioned by talking to Cerebras. What it sells self-serve is what the
// public list answers with, and that is what this package carries.
package cerebras
