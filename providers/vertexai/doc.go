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
// Reading it needs a Google credential, which no other provider here does.
// A token is taken from GOOGLE_OAUTH_TOKEN when set, and otherwise from the
// gcloud command. Without either, the provider reports that it is unconfigured
// and the rest of the sync proceeds.
package vertexai
