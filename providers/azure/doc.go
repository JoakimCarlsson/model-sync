// Package azure parses Azure's retail price list into the catalog model.
//
// Azure resells other labs' models through Foundry, so its catalog restates
// models belonging to OpenAI, xAI, DeepSeek, Meta, Mistral, Cohere and others
// at Microsoft's prices. They are recorded as Azure's own entries, the same as
// any reseller's.
//
// Seven documents are read, because a billing API returns meters and rates and
// never says what a model holds. Azure's model documentation says that, and
// states no rate, so the price list stays the authority on cost and the
// documentation becomes the authority on capability. The documentation is four
// concept pages, the models Azure sells, the models it resells and a guide each
// for image and video generation; a fifth page scheduling every model's
// retirement; and the catalog listing the Foundry portal browses, which states
// in fields what the pages state in prose and covers models no page does.
//
// Joining them is the hard part. A meter's name is a billing SKU and carries
// what is being charged for as a suffix, so one model has several of them:
// gpt-5-mini and gpt-5-mini-inpt are the same model billed two ways. The
// documentation names the model alone. A meter is therefore matched to the
// longest documented name it equals or extends, which attaches one document to
// every meter of a model and keeps gpt-5 from claiming gpt-5-mini.
//
// # What the price list publishes
//
// The rates are public and need no credential. What Azure does not publish is
// a model catalog: a model is named only inside a meter's SKU, and named
// differently in every family. "5 mini pp cd Inp Dz" is GPT-5 mini cached
// input on a data zone deployment, "Command A Plus Outp Glbl" is Cohere's
// model on a global one, and "Az-Babbage-002-Fine Tuned-Input" is a fine tuned
// legacy model. The family that supplies the missing part of the name lives in
// the product, not the SKU: the GPT-5 meters never say "gpt" because their
// product is "Azure OpenAI GPT5", and a SKU states as much of that family as
// it feels like, so "gpt oss 120b", "oss 20b" and "20b" are all one model.
//
// A SKU is therefore read against closed sets of abbreviations, of which Azure
// has many for the same thing: input is in, Inp, Inpt or Input, cached is cd,
// cchd, ccchd, cched or Cached, a global deployment is glbl, Gl, glb or
// Global, a regional one rg, rgnl, regn or regnl, and training is trng. It
// also states one fact twice and in either order, as "Batch Inpt cchd" does,
// so whatever a vocabulary still matches after the first reading is taken out
// as well. Whatever survives is the model, and the product supplies its
// prefix.
//
// The hard word is the one naming a modality, because Azure names several
// models after one: gpt-audio is metered as "gpt aud", gpt-image-1 mini as
// "gpt img 1 mini" and DALL-E's family as "Image 2". A modality word is the
// rate's own only where it is the last of them, is neither of the first two
// words of the SKU, and either sits beside the word naming the direction or
// the deployment or is not the only one in the SKU. That reading is what makes
// "gpt aud mini Inp" and "gpt aud mini txt Inp" the audio and the text rate of
// one model rather than two models.
//
// What is left over after that is a rate's dimensions rather than a model.
// Azure meters a video at one length at one frame size at one shape, an image
// at one quality and resolution, and the tokens a fine tuning grader spends
// apart from the tokens of a request; all of them are recorded as dimensions,
// which is what collapses eighteen Sora meters into three models and eight
// DALL-E meters into one.
//
// Some meters are not models at all: a provisioned throughput reservation, an
// hour of the managed compute a model is deployed on, and the calls a hosted
// tool such as the code interpreter makes. Those are dropped, since there is
// no model for them to be a fact about.
//
// Only meters in their primary region are read. Azure repeats every meter in
// each region it sells in, which would multiply the catalog for rates that
// mostly agree.
//
// # What the documentation publishes
//
// The models Azure sells are documented on one page holding two kinds of
// table, because it documents the OpenAI family one way and every other
// collection another.
//
// The OpenAI tables state the same two bounds two ways, because they were
// written at different times: the newer ones head a Context Window and a Max
// Output Tokens column, the older ones head one Max request column holding
// both as a labelled pair. Both are read. Where the newer column states the
// model's window followed by the lower ceilings particular deployments impose,
// the first number is the model's and the rest are the deployment's. The
// embedding tables head one bare count with that same Max request column,
// which is the window, and add the width of the vector they return.
//
// Those tables write the identifier in code style, and everything else in the
// same cell in plain text: the release the row documents, the preview marker,
// the footnote number, and a display name where the model has one. The code
// spans are therefore the models, and a cell holds more than one where a table
// covers a model and its mini variant in one row, as the audio tables do for
// gpt-audio and gpt-audio-mini.
//
// The description column is where those tables state what a model can do, as
// bullets: reasoning, structured outputs, JSON mode, streaming, computer use,
// the tool calling it supports and the APIs it answers on. The audio, speech
// and transcription tables describe the model in a sentence instead, and the
// sentence names the modalities: "Audio model for real-time low-latency
// transcription" takes audio and returns text.
//
// The other collections, on both the page for the models Azure sells and the
// page for the ones it resells, are documented as a Model, a Type and a
// Capabilities cell. The bullets in the capabilities cell say what a model
// takes and how many tokens of it, what it returns and how many, the languages
// it covers, whether it calls tools and which formats it answers in. The Type
// cell says what the model is, and is the only place these collections state
// that one reasons: DeepSeek-V4-Pro is typed "chat-completion (with reasoning
// content)" and DeepSeek-V3 is not.
//
// Those tables name a model the way its vendor does and the meters name it
// with the vendor already stripped off, so DeepSeek-V4-Pro is metered as
// v4-pro and FLUX.2-flex as flex. Joining them by shape would marry Grok 4 to
// Grok 4.20, so the join is a stated table instead. A model Azure documents
// and that table does not name keeps no capabilities.
//
// A token bound is recorded as the count Azure prints and never as a rounding
// of it, because Azure prints an exact count and its families do not share
// one. The partner page writes Phi-4 as "text (16,384 tokens)",
// Phi-4-mini-instruct and Ministral-3B as 131,072, Phi-4-reasoning as 32,768
// and Phi-4-mini-reasoning as 128,000, and the embedding table on the page for
// the models Azure sells heads a Max request of 8,192 for both
// text-embedding-3 models. Third party catalogs round four of those to 128,000
// and 32,000 and shorten the embedding one to 8,191; Microsoft states none of
// those figures, so none of them is recorded.
//
// Phi-4-reasoning-plus is on no concept page, and the catalog listing is where
// Azure states its bounds, its licence and its publisher. What the listing
// leaves empty it takes from Phi-4-reasoning's row, by the same prefix rule
// that attaches any meter to a document naming less of it than the meter does.
//
// The fine tuning table heads a Modality column with the flow through the
// model, "Text and vision to text", and that is the only place Azure states
// what some of the models it fine tunes take and return, Qwen-32B among them.
//
// The image and video guides compare their models in one table laid out the
// other way round, a column per model and a row per fact, and their modality
// row is the only place Azure says what those models take and return. The
// model tables state how long a prompt may be, in characters, and nothing
// else.
//
// # What the catalog listing publishes
//
// The listing behind the Foundry portal's model catalog answers without a
// credential, and states as fields what the concept pages state as prose: a
// display name, a description, the publisher selling the model and the lab that
// wrote it, a licence, a lifecycle stage, a version, the two token bounds, the
// modalities either side of the model, the languages it covers, the tasks it
// answers, the words the portal filters on, and the deployment types and Azure
// regions it can be served in. That is why it carries capabilities for two
// models for every one a concept page's bullets reach: the pages describe a
// model in a sentence and the listing names what it does.
//
// It lists a model once per version, and labels one of them the current one.
// That version is the model, and the others are what it used to be. A model
// whose versions have all retired carries no such label, and its highest
// version is then the last one Azure published.
//
// Thirteen of its fifteen thousand entries are Hugging Face repositories
// mirrored for deployment onto a virtual machine, which Azure bills as compute
// and meters nowhere in the listing this package reads. Excluding that one
// publisher is what makes the listing walkable.
//
// A version written as a date is a release date, because Azure's lifecycle page
// defines a model version as "a dated release within a family". A version that
// counts revisions of a registry entry is not, and no date is read out of one.
//
// # What the retirement schedule publishes
//
// One page holds, per model and per version, the lifecycle stage it is in, the
// day it stops answering and the model to move to. It is the only document
// stating a retirement date for more than a handful of models, and the only one
// still naming a model Azure has retired: the concept pages drop a model when
// it goes and the catalog listing drops most of them, while the schedule keeps
// the row and marks it retired.
//
// Only the highest version's row is read. Every value in a row belongs to the
// version the row is about, so filling a gap in the current version's row from
// an older one would state the older version's fact about the model: sora-2's
// first version names its own second version as the replacement, which is not
// what a reader of the model wants told about it.
//
// The same page schedules a fine tuned model's two retirements, of training and
// of deployment, which happen on different days from each other and from the
// base model's.
//
// # How the documents are joined
//
// A meter reaches every name it equals or extends, most specific first, and the
// first document to state a fact keeps it. That is what lets gpt-4-32k take its
// publisher, its licence and its lifecycle from its own catalog entry, which
// states no token bound, and its window from the concept page's row for the
// gpt-4 family, which does.
//
// Where two documents state the same fact, the concept pages win, because they
// are written about the model and the listing's fields are generated from the
// registry entry. Where they do not overlap, which is most of what the listing
// carries, there is nothing to resolve.
//
// A model Azure has stopped documenting can still name its window in the meter
// itself, as gpt-4-32k does, and that is read as well: it is Azure stating the
// window in the only place it still states it. It is read before the
// documentation, so that a meter naming a smaller deployment of a documented
// model keeps its own window rather than the family's.
//
// # Names
//
// A display name is published in two places. A capability table heads its rows
// with the name the lab that made the model uses, and the meters carry the
// same model with the vendor stripped off, so the heading is what restores it:
// v4-pro is DeepSeek-V4-Pro and flex is FLUX.2-flex. An OpenAI table writes
// one under the identifier where the model has one, which is how gpt-4o is
// "GPT-4o (Omni)" and gpt-4 turbo "GPT-4 Turbo with Vision".
//
// The preview marker and the footnote number Azure appends to a heading are
// not part of the name and are dropped. Where two documented rows reach one
// meter naming it differently, as the reasoning and non-reasoning variants of
// Grok 4.20 do, neither is the name of the model they are metered as and the
// model keeps none.
//
// The catalog listing states a third, which is what the portal shows above a
// model card: "OpenAI gpt-5", "Cohere Embed 4", "FLUX.2-flex". It is stated for
// every model the listing holds, which is why a display name reaches most of
// the catalog rather than the quarter of it the tables reach. Where the listing
// writes the identifier back as the name, as it does for gpt-4-32k, that is
// still the name Azure publishes and it is recorded.
//
// The retirement schedule states a fourth, in the column heading each row,
// which is how a model the listing has dropped still has one.
//
// Everywhere else Azure states no name. The GPT-5 and o-series tables head a
// Model ID column and a description, and the headings above them name a family
// rather than a model, since GPT-5.6 heads three of them. Those models carry
// the identifier alone, and so do the meters no document names.
//
// # What Azure does not publish
//
// Anything at all for the models it meters and neither the catalog listing nor
// the retirement schedule carries. The concept pages drop a model when it
// retires, and the page for the models Azure resells says it lists them
// "excluding deprecated and retired models", so the two later documents are
// where a retired model survives: GPT-3.5 Turbo, DALL-E 3, the Phi-3 family,
// Grok 3, DeepSeek R1, V3 and V3.1, Kimi K2 Thinking, MAI-Image-2 and the 2505
// Mistral Document AI release are named by one of them and nothing else.
// GPT-4.5, DALL-E 2, MAI-DS-R1, Grok 4 Fast and the 2503 and 2512 Mistral OCR
// releases are named by none. A window survives for some of those inside the
// meter's own name, and nothing else does.
//
// Anything for the models it meters under a codename. The MAI product carries
// twelve meters named "MAI o" through "MAI z", which name no model that any
// document names.
//
// A per-deployment rate limit, and the day a model was last changed. Azure
// states requests and tokens per minute against a subscription's quota rather
// than against a model, and its Models API, which returns a model's
// lifecycleStatus and per-SKU deprecation dates, is reached through Azure
// Resource Manager and needs a subscription. The lifecycle stage and the
// retirement date this package records are the public statements of the same
// two facts, on the schedule page and in the catalog listing.
//
// Anything but a rate for a partner-hosted model no page covers. The Fireworks
// meters serve GLM and MiniMax models that neither page documents; the
// DeepSeek, Kimi and gpt-oss models on the same deployment are documented as
// models Azure sells, and are read as the same model, since a deployment is
// not a different model.
//
// A token bound for the models that have none. Transcription and speech are
// bounded by the size of the audio file, at 25 MB, and image generation by the
// length of the prompt in characters; Azure states no context window or output
// ceiling for either, nor for the rerank and OCR models, whose capability
// bullets state a page count and a file size instead.
package azure
