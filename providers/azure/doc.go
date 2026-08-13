// Package azure parses Azure's retail price list into the catalog model.
//
// Azure resells other labs' models through Foundry, so its catalog restates
// models belonging to OpenAI, xAI, DeepSeek, Meta, Mistral, Cohere and others
// at Microsoft's prices. They are recorded as Azure's own entries, the same as
// any reseller's.
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
package azure
