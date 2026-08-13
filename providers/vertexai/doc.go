// Package vertexai parses Google Cloud's billing catalog into the catalog
// model.
//
// Vertex resells Google's own Gemini models and a set of open models at GCP
// rates, which differ from the rates the Gemini API charges, so it is recorded
// as its own provider rather than merged into Google's.
//
// The source is the Cloud Billing Catalog API rather than the pricing page.
// The page states the same model in several tables with nothing to tell the
// statements apart; the catalog gives each rate its own SKU carrying the
// region, the usage type and the unit as fields, so a rate is self describing.
// Amounts arrive as an integer part and a nanos remainder and are combined
// exactly, then multiplied by the quantity the SKU is quoted for.
//
// What a SKU is for lives in its description, which follows one of two forms:
// a Gemini rate reads "Gemini 3 Flash Text Input Flex - Predictions", and a
// Model Garden rate reads "Cloud Vertex AI Model Garden Model as a Service
// Gemma-4 Input Token". Both are taken apart against closed sets of words, and
// whatever is left is the model.
//
// A billing catalog says what a model costs and never what it holds, so a
// second set of documents is read: the page Google publishes for each model
// Vertex serves. Those state the identifier the API answers to, the context
// window, the output ceiling, the modalities and the capabilities, and no
// rate. They need no credential, but the catalog does, so a run still turns on
// having one.
//
// A page addresses a model by the same identifier a SKU is read as, once two
// differences are taken out. The documentation appends how a model is served
// or which tuning is offered — the catalog's deepseek-v3.2 is the page's
// deepseek-v3.2-maas and its llama-3.3-70b the page's
// llama-3.3-70b-instruct-maas — and those suffixes are stripped. And a SKU can
// be finer than a model, since Vertex prices Gemini 2.5 Flash differently with
// thinking on and again above a length, so a SKU naming a condition as well as
// a model reaches the model by the longest name it extends.
//
// The pages state the same two bounds two ways. Google's own models carry
// token-limit rows; the models it serves for other labs carry none and state
// the figures among their per-region quotas instead, as "65,536 maximum
// output, 163,840 context length". Both are read.
//
// An embedding model's page heads its rows differently from a generative one.
// The bound on what it accepts is a maximum sequence length rather than a
// context window, and it states the width of the vector it returns as a ceiling,
// "Up to 1,024", which is the only place Vertex publishes one. Both are read
// under the keys the rest of the catalog uses, so a consumer keying on
// context_window finds an embedding model's input bound where it finds every
// other model's.
//
// What Google does not publish: a page for every model Vertex bills for. The
// Gemma variants offered for self-deployment and the older Llama releases are
// metered with none, and carry rates alone. Nor does it state an output modality
// for an embedding model: the table marks embeddings as the output, which is
// what the model returns rather than a modality it returns it in.
//
// Reading the catalog needs a Google credential, which no other provider here
// does.
// A token is taken from GOOGLE_OAUTH_TOKEN when set, and otherwise from the
// gcloud command. Without either, the provider reports that it is unconfigured
// and the rest of the sync proceeds.
package vertexai
