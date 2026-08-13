// Package azure parses Azure's retail price list into the catalog model.
//
// Azure resells other labs' models through Foundry, so its catalog restates
// models belonging to OpenAI, xAI, DeepSeek, Meta, Mistral, Cohere and others
// at Microsoft's prices. They are recorded as Azure's own entries, the same as
// any reseller's.
//
// Two documents are read, because a billing API returns meters and rates and
// never says what a model holds. Azure's model documentation says that, and
// states no rate, so the price list stays the authority on cost and the
// documentation becomes the authority on capability.
//
// Joining them is the hard part. A meter's name is a billing SKU and carries
// what is being charged for as a suffix, so one model has several of them:
// gpt-5-mini and gpt-5-mini-inpt are the same model billed two ways. The
// documentation names the model alone. A meter is therefore matched to the
// longest documented name it equals or extends, which attaches one document to
// every meter of a model and keeps gpt-5 from claiming gpt-5-mini.
//
// The documentation is one page holding two kinds of table, because it
// documents the OpenAI family one way and every other collection another.
//
// The OpenAI tables state the same two bounds two ways, because they were
// written at different times: the newer ones head a Context Window and a Max
// Output Tokens column, the older ones head one Max request column holding both
// as a labelled pair. Both are read. Where the newer column states the model's
// window followed by the lower ceilings particular deployments impose, the
// first number is the model's and the rest are the deployment's. The embedding
// tables head one bare count with that same Max request column, which is the
// window, and add the width of the vector they return.
//
// The other collections — Cohere, DeepSeek, Meta, Mistral, xAI, Moonshot, Black
// Forest Labs — are documented as a Model, a Type and a Capabilities cell,
// where everything a model holds and can do is written as bullets: what it
// takes and how many tokens of it, what it returns and how many, the languages
// it covers, whether it calls tools and which formats it answers in. Those
// bullets are the only place Azure states any of that for the models it does
// not own, and they are read for the window, the output ceiling, the
// modalities, the languages, the vector widths and two capabilities.
//
// Those tables name a model the way its vendor does and the meters name it with
// the vendor already stripped off, so DeepSeek-V4-Pro is metered as v4-pro and
// FLUX.2-flex as flex. Joining them by shape would marry Grok 4 to Grok 4.20,
// so the join is a stated table instead. A model Azure documents and that table
// does not name keeps no capabilities.
//
// A model Azure has stopped documenting can still name its window in the meter
// itself, as gpt-4-32k does, and that is read as well: it is Azure stating the
// window in the only place it still states it.
//
// The rates are public and need no credential. What Azure does not publish is
// a model catalog: a model is named only inside a meter's SKU, and named
// differently in every family. "5 mini pp cd Inp Dz" is GPT-5 mini cached
// input on a data zone deployment, "Command A Plus Outp Glbl" is Cohere's
// model on a global one, and "Az-Babbage-002-Fine Tuned-Input" is a fine tuned
// legacy model. The family that supplies the missing part of the name lives in
// the product, not the SKU: the GPT-5 meters never say "gpt" because their
// product is "Azure OpenAI GPT5".
//
// A SKU is therefore read against closed sets of abbreviations, of which Azure
// has many for the same thing: input is Inp, Inpt or Input, cached is cd, cchd
// or Cached, a global deployment is glbl, Gl or Global, and training is trng.
// Whatever survives is the model, and the product supplies its prefix.
//
// Only meters in their primary region are read. Azure repeats every meter in
// each region it sells in, which would multiply the catalog for rates that
// mostly agree.
//
// What Azure does not publish: anything at all for the models it meters and no
// longer documents, and anything but rates for the partner-hosted deployments
// of models it does document. The Fireworks meters serve DeepSeek, Kimi, GLM
// and MiniMax models that the collections tables cover, but they cover them as
// models Azure sells rather than as that deployment, so those meters keep the
// rate alone. The same is true of the MAI models, of Phi, of Qwen and of
// Codestral, which are metered with no capability stated anywhere outside the
// portal's own model cards, which need an account to read.
//
// Nor does Azure publish a display name for a model anywhere. Its tables state
// a Model ID and a description, and the headings above them name a family
// rather than a model, since GPT-5.6 heads three of them. No Azure model
// therefore carries a name.
package azure
