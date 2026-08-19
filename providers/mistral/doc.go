// Package mistral parses Mistral's model documentation into the catalog
// model.
//
// Mistral publishes one page per model and an index linking to all of them.
// The index describes nothing: the identifiers a model answers to, its context
// window, its rates, its modalities, its capabilities and the weights behind
// it are stated only on the model's own page, so every page is fetched and the
// index is used for the list of them and for its deprecation table.
//
// Six further documents are read, each for something no model page states. Two
// embedding guides state the width of the vector each embedding model returns.
// The OCR guide states what the OCR processor will accept as a document. The
// supported-language page states the languages Mistral vouches for. The
// reasoning guide names the models that reason when asked rather than always.
// The pricing page states the ratios a published rate may be adjusted by. None
// of the six can create a model: each adds to a model a page already
// established, and an identifier none of the pages named is one Mistral no
// longer serves under that name.
//
// The pages are a client-rendered application, but the data is not lost to
// that. React serves the page as a flight payload, the rendered tree and its
// values encoded, and only the styling of that tree needs a browser. The
// payload is what this reads. Asking for a page with the RSC header returns
// the payload alone, and a response that arrives rendered instead carries the
// same payload embedded in it, so either is readable. The pricing page is the
// exception: it is served by a different site, as ordinary markup, with every
// amount in an attribute rather than in the text beside it.
//
// Values are anchored on the presentation class of the element holding them
// rather than on the label beside them, because the page writes a labelled
// statistic two ways depending on whether a tooltip sits between the label and
// the value.
//
// A model is keyed by the first identifier its page names, which is the dated
// one, and the rest are recorded as aliases. Two pages naming the same
// identifier are one model: Mistral documents Voxtral Mini twice, once for
// chat and once for transcription, and both describe one served model.
//
// A model Mistral has retired is not carried at all. Its page stays up and the
// deprecation table goes on listing it, but there is nothing left to call or
// to bill. A deprecated model is kept: it serves until its retirement date and
// Mistral still sells it. The badge on a model's own page is what separates
// the two, and is the only statement of standing Mistral makes; the
// deprecation table gives dates and no standing at all. The drop happens after
// every document has been read, because the page and the table each name the
// same identifier and only the page says which side of the line it falls on.
//
// The undated names go with it. An identifier such as mistral-saba-latest is
// recorded as an alias of whichever model's page lists it among the names it
// answers to, so an alias Mistral has repointed at a successor is already on
// that successor's page, and one that disappears with a retired model is one
// Mistral has stopped serving rather than one left dangling here.
//
// A model page labels its modalities as inputs and outputs, so both sides are
// read as stated. The embedding and moderation pages label an input and stop,
// and those models are recorded as returning text: that is the medium they work
// in, not the return value, which is a vector for one and a set of category
// scores for the other. The two sides are never set apart, because a consumer
// reading one alone cannot tell an unstated output from a model that returns
// nothing.
//
// # The weights tab
//
// Every model page carries a weights tab, and it is the only place Mistral
// says what a model is made of. A model whose weights Mistral publishes gets a
// row per flavour of them, each leading with the repository holding the
// weights and the licence governing them and going on to the parameter count,
// the count that is active in a forward pass, the memory the weights need at
// three quantizations, and the context the weights were trained for. The
// flavours are named in the link: a model published once is labelled plainly
// and is the one the API serves, and a model published as base, instruct and
// reasoning weights is served as the instruct one, so that is what
// hugging_face_id records and the other two are recorded beside it.
//
// A model Mistral serves without publishing its weights carries the tab with
// the shelf named and no row under it. That is a statement rather than a
// silence, which is why open_weights is set for every model and not only for
// the ones that carry a repository. The shelf itself is recorded as
// distribution: open, premier or labs. It is not the same fact as
// open_weights, and the two come apart in both directions. Mistral publishes
// the weights of three premier models under its research licence, so those are
// premier and downloadable at once, and it files one third-party model on the
// open shelf without hosting a repository for it.
//
// # Rates
//
// Mistral prices a model on the model's own page, and the pricing page reprints
// those same amounts, so nothing is priced from the pricing page. What only the
// pricing page states is that an amount can be adjusted: an input rate marked
// as cacheable is billed a tenth of itself for a prompt already seen, any rate
// is billed a tenth more on a regional endpoint, and the batch tab halves the
// whole card. Those are recorded as the ratios Mistral prints, not multiplied
// out, because Mistral prints no product of them anywhere and a catalog
// quoting one would be quoting arithmetic rather than a rate.
//
// A card on the pricing page is matched to a model by the documentation page
// it links to, which is the page this parser already read the model from.
// Cards Mistral prints without a link, and cards for a product rather than a
// model, match nothing.
//
// What Mistral does not publish:
//
//   - A rate for a deprecated model. The price card is removed from the page
//     when a model is deprecated, though the model serves until it retires,
//     and the pricing page lists only what Mistral currently sells. Seventeen
//     chat models are in that state and carry no price, which is what Mistral
//     states rather than a gap in this parser.
//   - A rate for a model published as weights alone. Such a model has no page
//     here at all: it has no identifier to call and nothing to bill, and the
//     absence of a rate for it is the whole of what Mistral says about it.
//   - An output bound for most models. Two model pages state one; the rest
//     state only a context window, and the statistic exists nowhere else:
//     the index carries the label and its tooltip in its string table without
//     ever rendering the tile, the pricing page states no limit at all, and
//     the overview it links to is the index under another URL.
//   - A context window in full. The statistics tile writes the shorthand
//     Mistral uses everywhere else, "128k" on mistral-large-2411 and "256k" on
//     devstral-2512, and no page, tooltip or index states the figure any other
//     way. The suffix is read as a thousand, so those are recorded as 128000
//     and 256000. A catalog quoting 131072 or 262144 for the same model has
//     rounded the shorthand to the neighbouring power of two, which is a
//     conversion Mistral never writes and this does not make.
//   - A context window for a model that reads neither prompt nor document as
//     tokens. The OCR and text-to-speech pages carry no statistics tile at
//     all, and the transcription pages render the tile with "--" in it, which
//     is Mistral stating there is no such bound rather than omitting one. What
//     bounds an OCR request instead is the document, and the OCR guide states
//     that in its FAQ: fifty megabytes, a thousand pages, and a table of the
//     formats the processor reads.
//   - A rate limit of any size. Mistral's documentation says which limits
//     exist, per model and per organisation, and that they are shown in the
//     admin panel and raised by asking; it prints no figure for any of them.
//     The priority tier page says the same of itself, that its limits are
//     agreed per organisation and per model. There is nothing to read.
//   - A supported-language list per model. The one page that lists languages
//     carries two tables, one for the language models and one for OCR, and
//     names no model in either. Each table is therefore recorded against the
//     models of the kind its heading names, and the kinds neither table covers
//     carry no list. Mistral adds that a model can do well in languages the
//     tables leave out, so what is recorded is what Mistral vouches for rather
//     than the whole of what a model understands.
//   - A fine-tuning rate for a model. The pricing page prices training and
//     storage for the classifier API, which is a product built on Ministral
//     rather than a model with an identifier of its own, and prices no other
//     customization.
//   - A knowledge cutoff, anywhere, for any model.
//
// The index's deprecation table states two dates in one cell with nothing
// between them, so they are separated by position: the first is the
// deprecation, the second the retirement. It is read after the model pages
// because it is the only document giving the retirement date of a model that
// is deprecated but still serving, whose page states only the earlier date.
package mistral
