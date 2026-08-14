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
// The Model Garden form has a third ending, which stops at the direction:
// "Cloud Vertex AI Model Garden Model as a Service Llama3.1 405B Input", where
// the same form elsewhere carries on to the word the rate is counted in. A
// direction is only read as an ending under that prefix, and there it is
// unambiguous, because the Model Garden never writes the " - Predictions"
// suffix that Gemini's own meters end in, so nothing under the prefix can be a
// prediction rate with its suffix left off. Read without the prefix the two
// forms cannot be told apart, and the same rule would take Google's
// per-product meters, "CodeMender Gemini 3 Flash Global Text Input", for models
// of their own. Llama 3.1 405B is metered this way and no other, so until this
// was read it was billed for and absent from the catalog; Google publishes no
// page for it, so it carries its two rates and nothing else.
//
// Two of those words have to be read together. A description names the
// modality beside the side of the request it falls on, "Text Input" or "Input
// Text", and a model can be named for a modality itself, so the modality is
// taken from beside the direction rather than from wherever it first appears:
// "Gemini 3.1 Flash Image Global Video Input" prices video input on the image
// model, and taking the first modality invented a Gemini 3.1 Flash Video that
// Vertex does not serve. The Model Garden also spells a lab against its
// version both ways, metering Llama 3.3 70B for tuning and Llama3.3 70B for
// inference, and counts in "Token" or "Tokens" indifferently; reading either
// spelling as its own thing left one identifier priced for tuning alone.
//
// A billing catalog says what a model costs and never what it holds, so a
// second set of documents is read: the page Google publishes for each model
// Vertex serves. Those state the identifier the API answers to, the context
// window, the output ceiling, the modalities and the capabilities, and no
// rate. They need no credential, but the catalog does, so a run still turns on
// having one.
//
// A page addresses a model by the same identifier a SKU is read as, once three
// differences are taken out. The documentation appends how a model is served
// or which tuning is offered, so that the catalog's deepseek-v3.2 is the page's
// deepseek-v3.2-maas and its llama-3.3-70b the page's
// llama-3.3-70b-instruct-maas, and those suffixes are stripped. The catalog
// puts the lab in front of a model where the page does not, "OpenAI
// gpt-oss-120b" against gpt-oss-120b. And either document can be the coarser:
// a SKU naming a condition as well as a model, since Vertex prices Gemini 2.5
// Flash differently with thinking on and again above a length, reaches the
// model by the longest name it extends; a SKU naming a model more briefly than
// its page does, "Llama 4 Maverick" against
// llama-4-maverick-17b-128e-instruct-maas, reaches the one page whose
// identifier it begins, and reaches none where it begins two.
//
// The pages state the same two bounds several ways. Google's own models carry
// token-limit rows, headed "Context window" or, on the models that answer in
// an image, "Maximum input tokens". The models it serves for other labs carry
// no such rows and state the figures among their per-region quotas, in either
// order: as "65,536 maximum output, 163,840 context length" where the model is
// sold as a service, and as "Max output: 8,192" and "Context length: 524,288"
// where it is sold through a partner. All of them are read, and reading only
// the first quota form took the figure beside the wrong label, which recorded
// a Llama page's output ceiling as its context window. A quantity may also be
// abbreviated, "128K" for what another page writes as 1,048,576.
//
// The pages come in two shapes throughout, and both are read: Google's own lay
// the modalities against a grid and mark each capability supported or not,
// while the models served for other labs list "Inputs: Text, Code, Images" and
// group the capabilities into the ones the model has and the ones it lacks.
//
// An embedding model's page heads its rows differently from a generative one.
// The bound on what it accepts is a maximum sequence length rather than a
// context window, and it states the width of the vector it returns as a ceiling,
// "Up to 1,024", which is the only place Vertex publishes one. Both are read
// under the keys the rest of the catalog uses, so a consumer keying on
// context_window finds an embedding model's input bound where it finds every
// other model's.
//
// Three more documents are read, none of which the index links, because each
// answers something no model page does:
//
// The capability pages, one for thinking, one for structured output and one
// for function calling, enumerate the models supporting them. A Gemini page
// never mentions function calling at all, so this is the only document that
// says whether a Gemini model can call a tool. Only the list under the page's
// expandable heading is read: the navigation links every model Google
// documents, and reading the whole page would say they all support it.
//
// The Gemma page tabulates the variants and what each takes and returns. Gemma
// is sold for self deployment rather than as a service, so all but the one
// variant offered as a service have no page, and this is the only document
// naming their modalities.
//
// The versions page states when each model retires and which have retired
// already. Vertex goes on metering a model after it stops serving it, so this
// is the only document telling a rate on sale from a rate left behind: Gemini
// 2.0 Flash and 2.0 Flash-Lite retired on 1 June 2026 and are metered still.
// A model the page lists as retired is not held at all, a rate for something
// that no longer answers being a leftover rather than an offer. A model still
// served keeps the date it is due to retire on, which is a fact about something
// that can still be bought, and the page states one for every model it
// documents rather than only for the ones already gone. A row is matched on the
// identifier exactly, unlike a model page: the row describes one release, and
// a live variant of a withdrawn model is one Google has not said it withdrew.
//
// What Google does not publish. A self-deployed open model has no context
// window to publish, because the length it serves is an argument of the
// deployment rather than a property of the model, and the pages for deploying
// with vLLM and Hex-LLM say as much by making it a flag. So the Gemma 3 and
// Gemma 4 variants, MedGemma, Qwen 3 and Qwen 3.5 and the Llama 3.1 and 3.2
// releases carry modalities where the Gemma table names them and no bounds and
// no capabilities at all; there is no page for any of them, and the paths
// where one would sit answer 404. The models index states a family-wide "128K
// context window" for Gemma 3 in prose, which is not read: it is a sentence
// about a family whose smallest member holds a quarter of it.
//
// Several SKUs name no single model. "Gemini 3.0 / 3.1 Pro" is one meter
// covering two releases, so there is no page it could be joined to, and Gemini
// 3.1 Flash is metered for use over the Live API while only its Lite and Image
// variants have pages. Gemini 2.0 Flash Live is metered and is named in no
// lifecycle table and on no page, so unlike the two 2.0 models beside it, which
// the versions page names as retired and which are therefore dropped, nothing
// says whether Vertex still serves it. It is kept, because a withdrawal has to
// be stated somewhere to be acted on.
//
// Where a page states a capability is absent, that absence is the answer: the
// two image models are documented as not supporting structured output and are
// named on no capability page, and DeepSeek OCR's page lists the three
// capabilities it might have and marks all three unsupported. An embedding
// model has no output ceiling to state, and the E5 pages carry no capability
// row at all, only the ways the model may be bought.
//
// A model whose page is published after a sync stays without one until the
// next sync. That is not a gap in the join, which matches on the identifier
// both documents state, and the fix for it is to sync again.
//
// An embedding page marks "Embeddings" as its output, which is what the model
// returns rather than a modality it returns it in. The catalog has no word for a
// vector, so it is read as the text the model works in: skipping it left those
// models stating what they take and nothing about what they give back, and a
// consumer cannot tell that from a model that returns nothing.
//
// Reading the catalog needs a Google credential, which no other provider here
// does.
// A token is taken from GOOGLE_OAUTH_TOKEN when set, and otherwise from the
// gcloud command. Without either, the provider reports that it is unconfigured
// and the rest of the sync proceeds.
package vertexai
