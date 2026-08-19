// Package google parses Google's Gemini pricing and model documentation into
// the catalog model.
//
// Three documents carry the bulk of it because none is enough. The pricing
// page is the only one naming every model that costs anything, and states no
// context window, no capability and no modality. Each model's own page states
// all three and no rate at all. The index is the only one pairing the name a
// model is sold under with the endpoint the API answers to, the only one
// saying which models Google has withdrawn, and the only one saying which of
// the rest are still in preview.
//
// Six more are read because Google states a fact per model in each of them and
// nowhere else. The deprecation schedule dates every model's release and its
// earliest shutdown and names what to move to. The rate limit page states what
// a project may enqueue for batch, per model and per usage tier. The thinking
// guide states how much each model reasons by default and which levels it
// accepts. The Veo guide compares the video models parameter by parameter and
// carries a property table for the two Veo 3 endpoints that have no model page
// at all. The Imagen guide states in prose what the Veo guide states in a
// table. The image generation guide states which aspect ratios an image model
// answers in and at what sizes. A model card, which the model pages link to on
// DeepMind's site rather than Google's own, is the only document stating a
// knowledge cutoff.
//
// A model is held under the endpoint it answers to, which the pricing page
// states beneath each heading as one <code> per endpoint. That is what settles
// the identity question, and it is worth reading rather than deriving: the name
// a heading carries and the identifier the API takes drift, so what pricing
// heads "Gemini 3.1 Flash Image (Nano Banana 2)" the API calls
// gemini-3.1-flash-image, and matching the two on their spelling attached one
// model's page to another. It also settles how many models a heading is: Google
// prices three sizes of Imagen and three builds of Veo 3.1 under one heading
// apiece, naming the quality level in the row label and the endpoints below the
// heading, so each row goes to the endpoint whose identifier carries its level
// and the level Google leaves out of an identifier, its standard, goes to the
// endpoint carrying none. A row naming no level prices every endpoint under the
// heading, which is how the custom-tools endpoint of Gemini 3.1 Pro is priced.
// The one heading stating no endpoint at all, Gemma, is held under its name.
//
// A model Google has stopped serving is not held at all. The index says so as
// an aside on the model's name, "(Shut down)" or "(Retired)", and an endpoint
// marked either way is never begun, so no row of its tables is read and no page
// is attached to it. The rates outlive the model: the pricing page still heads
// Gemini 2.0 Flash and still tabulates what it charged, months after the index
// marked it shut down, and a rate for something that no longer answers is a
// leftover row rather than an offer. Deprecated is deliberately not dropped,
// because Google goes on serving a deprecated model and goes on publishing its
// rates until it retires; the three Imagen 4 endpoints are marked that way and
// are kept, carrying the state.
//
// The index does not link every page it should. Four models have a page all
// the same: the two second-generation robotics previews, which the index lists
// without a link, and the flash-lite preview and Veo 2, which it does not list.
// A page is addressed by an endpoint identifier, so every identifier the
// pricing page states is tried, and a 404 among those is read as Google having
// no page rather than as a failed fetch.
//
// A page attaches to a model in three rounds. Its own address comes first,
// because a page also states the identifiers it covers and a few state the
// wrong one: the streaming robotics page and the Lyria Pro page each name their
// sibling's endpoint, and taking that at face value would give one model the
// other's page. The identifiers a page states come second, which is what lets
// the one page Google publishes for a family reach every endpoint in it. Last,
// an endpoint still unclaimed takes the page describing the rest of the family
// the pricing page groups it with, and where two pages already split a family
// nothing is done, so Veo 3.1 Lite keeps its own page.
//
// A page that does exist states its bounds in a row headed "Token limits" or,
// on the video and omni pages, "Limits", and names the input bound three ways
// inside it: "Input token limit" almost everywhere, "Context window" on the omni
// model and "Text input" on the video models, where it bounds the prompt. All
// are read, being one fact under several of Google's own names. Reading only the
// first heading skipped those rows whole.
//
// What a page states is recorded as it stands, which is where this parser and a
// third-party catalog part company. Google states 128,000 for the computer-use
// preview and not the 131,072 a power of two would round it to. It states the
// Veo 3.1 prompt bound as "1,024 tokens", a token count rather than the
// characters a prompt bound is elsewhere published in, so it belongs in the
// same field as every other window. It states 1,048,576 on the omni page
// itself, so that figure is the model's own and not a family default. The image
// models read backwards beside their names and do not: Gemini 3 Pro Image
// states 65,536 and Gemini 3.1 Flash Image 131,072, each on the page whose
// model code row names that same model, so the larger window really is the
// flash one and no page has been attached to the wrong model.
//
// Lyria states a bound as well. Both music pages carry "Input token limit:
// 131,072", so the figure is published rather than filled in from a sibling.
// The Pro page carries it inside a table copied from the Clip page down to the
// model code, which is the reason a page is attached by its address before
// anything it states about itself.
//
// The deprecation schedule is the document that dates a model. It states a
// release date to the day for almost every model and to the month for the
// newest, and neither is rounded to the other: gemini-3.7-flash is dated
// "August 2026" and carries a release date of that precision. A shutdown date
// is the earliest Google may retire the model, which is what it says in as many
// words, and the cell saying none has been announced states no date and yields
// none. The schedule is also the only document distinguishing a model that is
// generally available from one in preview, which it does with a divider row
// rather than a column, so each of its tables is read in order.
//
// Two documents disagree about two models. The schedule greys the rows of the
// models it has already shut down, and among them are Gemini 2.5 Pro TTS and
// the September 2025 flash-lite preview, both of which the index still lists
// without the aside it hangs off a withdrawn model and both of which the
// pricing page still prices. The index is followed, because it is the document
// that says what the API answers to; the greyed row is still read for its
// dates. A model the index does mark withdrawn is never begun at all, which is
// unchanged.
//
// The index states a lifecycle twice over and only one of the two is a field.
// The aside on a model's name is the withdrawal, and the grid of cards above
// the tables marks each model Stable or Preview under its description. A card
// names the page rather than the model, and a page is addressed by an
// endpoint, so the card resolves to an endpoint without going through a name.
// The badge saying a model is new sits in the same line and is a fact about
// the model's age rather than its availability, so only the last word of that
// line is read.
//
// Google publishes no thinking budget in tokens. The parameter is
// thinking_level and the guide tabulates, per endpoint, the level a model
// thinks at when it is not told and the levels it accepts, so the bound is a
// set of names and is recorded as the list it is. A model page states the same
// set in the parenthesis after "Supported" on its thinking row, which is why
// more models carry the levels than the guide tabulates.
//
// The rate limit page states one figure per model and the rest per project.
// The enqueued token ceiling is per model and per usage tier and is recorded;
// the spend ceiling, the tier qualifications, the concurrent batch count, the
// batch file sizes and the priority multiplier belong to the account. Its rows
// name models the way the index does rather than by endpoint, with a few
// naming the endpoint instead, so both are tried and a row matching neither is
// skipped. Google does not publish a requests-per-minute or tokens-per-minute
// table at all any more: the page says those depend on the account and directs
// the reader to AI Studio to see their own.
//
// The Veo guide compares models in two tables, one of request parameters and
// one of what comes back, both with one column per model and neither naming an
// endpoint anywhere. A column heads two models where they differ in nothing
// the table states, "Veo 3.1 & Veo 3.1 Fast", so a heading is split on the
// ampersand and each half brought to the same form as an endpoint: the part
// before the verb, with the trailing zero of a major-only version dropped, so
// that veo-3.0-fast-generate-001 meets the heading "Veo 3 Fast". A parameter's
// accepted values are the ones the cell quotes; an unquoted code span in the
// same cell names a type. The output table writes its sizes in prose beside an
// italic rider saying which durations they allow, and the rider's own numbers
// are not sizes, so it is dropped before the prose is read.
//
// That comparison is also where the video models get a capability and an input
// they would otherwise lack. Its audio row marks with a tick what the Gemini
// model pages call audio generation, and it is recorded under that name rather
// than under a second word for the same thing; Veo 2 is marked silent and
// carries no such capability. Its input row names each input as the conversion
// it drives, "Video-to-Video", which is how the one model taking a video for
// input says so at all, the model page stating "Text, Image" for every one of
// them.
//
// The Imagen guide restricts one of its values to some of the models it
// describes, saying the two image sizes are for the standard and ultra models.
// It names quality levels where the property table names endpoints, and the
// level Google leaves out of an identifier is its standard, so the restriction
// resolves and the fast endpoint carries the parameter list without the sizes.
//
// A pricing cell states more than one amount for four different reasons, and
// reading the amounts alone left a model carrying two rates for one thing with
// nothing to tell them apart. Each amount therefore carries the clause running
// from it to the next: the output size it buys, the prompt length band it
// applies to, the day an introductory rate runs through or the day its
// successor starts, and whether it buys storage rather than a read. The
// denominator comes from the column heading unless the clause states one,
// because a grounded search request is not a token, an hour of cache storage
// is not either, and the embedding models quote a second amount per image, per
// second and per frame beside the amount per million tokens. Two rows of one
// table price a tool rather than the model, at the same rate against the same
// denominator, so the tool is part of what identifies the rate.
//
// One clause had to be rescued before it could be read. "prompts <= 200k
// tokens" is not markup, but everything from its angle bracket to the next one
// reads as a tag, so stripping tags took the clause with it and left the cell
// stating two amounts and no reason for the difference.
//
// A model page writes its modalities as prose rather than as a list, and the
// wordings matter. "Video with audio" names two modalities and not a quality of
// one, so the video models return both. "Text embeddings" is the return value
// of an embedding model, and since the catalog has no word for a vector it is
// read as the text those models work in; dropping it left them stating what
// they take and nothing about what they give back, which a consumer cannot tell
// from a model that returns nothing.
//
// Google states rates twice over: a model has one table per serving tier, and
// each table has one column per plan. A single rate therefore needs both a tier
// and a plan to identify it, and the free plan's "Free of charge" is a real
// rate of zero rather than an absence. Only one column of a table states the
// denominator its rates are quoted against, "Paid Tier, per 1M tokens in USD"
// beside a bare "Free Tier", and since both price the same rows, the one
// denominator stated is the denominator of all of them. Without that the free
// plan's rates have no unit and are dropped, which is what left the open Gemma
// models, whose only rate is a free one, reading as unpriced.
//
// Neither the model nor the tier appears inside its table, both being headings
// above it, so the pricing page is read as a running state of which model and
// which tier are in force rather than as a list of self-describing tables.
//
// A model page is the opposite: one table of properties, each row a labelled
// fact, with the capabilities enumerated and marked supported or not. Only the
// supported ones are recorded, and one marked supported in preview counts as
// supported. An embedding model's page labels the same field in the singular,
// "Input" where a chat model's says "Inputs", and adds the width of the vector
// it returns: a range followed by the widths Google recommends. Only the named
// widths are recorded, since a range is not a list of values to pick from.
//
// What Google does not publish, checked against the live documents rather than
// taken on trust:
//
// A rate limit in requests or tokens per minute. The rate limit page once
// tabulated them per model and per tier and now states that they vary with the
// account and are to be read in AI Studio. What it still tabulates per model,
// the batch enqueued token ceiling, is read.
//
// A thinking budget in tokens, for the same reason: the parameter it bounded
// no longer exists.
//
// A knowledge cutoff for most models. It is stated in one sentence among a
// model card's known limitations rather than as a field, and most cards do not
// state it: of the sixteen models whose page links a card, four say when the
// model's knowledge ends. The rest say nothing, and the sentence that does
// appear names a second, earlier date for the domains the model was not
// refreshed in, which is not the cutoff and is not recorded as one.
//
// A model card for the video, image, music and embedding models. DeepMind
// publishes cards for some of them, but the Gemini model pages carry the model
// card row only for the Gemini models, and DeepMind's own index of cards
// identifies them by a title rather than by an endpoint, so nothing links a
// card to an endpoint for the rest.
//
// A release date for eight of the models held. The deprecation schedule dates
// every model it lists, and it lists none of the newest previews: the
// computer-use model, the custom-tools endpoint, both robotics previews, the
// omni preview and Gemma.
//
// A capability table for the Imagen models, and a size for the aspect ratios
// of two of the image models. The image generation guide tabulates them for
// the 3.1 Flash and 2.5 Flash image models under headings that resolve to an
// endpoint, and for a third under the heading "3.1 Pro Image", which names no
// endpoint Google serves. The table is not attached on a guess about which
// model was meant.
//
// A page for Gemma. The pricing page names no endpoint under that heading and
// /gemini-api/docs/models/gemma-4 is a 404. The Gemma overview at
// /gemma/docs/core states a context window in prose, and per size class rather
// than per model: small models hold 128K and medium ones 256K. Which size the
// Gemini API serves under one undifferentiated rate is not said, so Gemma
// carries its rates, its name and no bounds.
//
// A page for either build of Veo 3. Both endpoints are priced and both
// addresses are 404s, while Veo 3.1 and Veo 2 have pages, so the omission is
// Google retiring the documentation ahead of the rates.
//
// A prompt bound for Veo 2. Its page states "Text input: N/A" where the other
// video pages state a token count.
//
// An output token limit for anything that does not answer in tokens. The
// embedding pages state a vector width instead, the image and video pages an
// output count, "1 to 4" images or "1" video, the omni page a duration, and
// the music pages nothing at all beyond the prompt bound. Those are the models
// with no max_output_tokens, and the field would be meaningless on them.
//
// A capability table for the embedding, Imagen, Veo, omni and computer-use
// pages. Every other page enumerates a dozen capabilities and marks each
// supported or not; those carry no such row at all, so the features of those
// models are unstated rather than stated as absent. Where a page does
// carry the row, a capability it marks "Not supported" is recorded as absent
// rather than as unknown, which is why the reasoning, structured-output and
// tool columns sit below the model count for the speech and image models.
//
// A rate of its own for the agent models. Deep Research, Deep Research Max and
// the Antigravity agent have pages and appear on the pricing page only under
// "Pricing for agents", which says their inference is "charged at standard
// Gemini list rates" rather than at a rate of theirs. Lyria RealTime has a
// page and no rate anywhere on the pricing page. Having nothing to price, none
// of them is held.
package google
