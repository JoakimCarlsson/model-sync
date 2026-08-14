// Package mistral parses Mistral's model documentation into the catalog
// model.
//
// Mistral publishes one page per model and an index linking to all of them.
// The index describes nothing: the identifiers a model answers to, its context
// window, its rates, its modalities and its capabilities are stated only on
// the model's own page, so every page is fetched and the index is used for the
// list of them and for its deprecation table.
//
// One fact a model page never states is the width of the vector an embedding
// model returns. Mistral states that in the guide to each embedding model
// instead, in the prose of the section that walks through a call, naming the
// model in the same sentence, so the two guides are fetched for it. Where the
// width can be asked for, the guide states a default and a maximum and no set
// of options in between, so the default is recorded as the width and the
// maximum beside it, rather than a list of choices Mistral does not publish.
//
// The pages are a client-rendered application, but the data is not lost to
// that. React serves the page as a flight payload — the rendered tree and its
// values, encoded — and only the styling of that tree needs a browser. The
// payload is what this reads. Asking for a page with the RSC header returns
// the payload alone, and a response that arrives rendered instead carries the
// same payload embedded in it, so either is readable.
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
// What Mistral does not publish:
//
//   - A rate for a deprecated model. The price card is removed from the page
//     when a model is deprecated, though the model serves until it retires.
//     Seventeen chat models are in that state and carry no price, which is
//     what Mistral states rather than a gap in this parser.
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
//     is Mistral stating there is no such bound rather than omitting one.
//   - An API identifier for a weights-only release. Mistral publishes pages
//     for models it has released the weights of without serving them, and
//     those are skipped: they describe a download, with nothing to key or bill
//     a catalog entry by.
//   - Batch, priority and regional rates. The pricing page states them, but it
//     renders them in the browser from data the document does not carry, so
//     only the standard rate on each model page can be read.
//
// The index's deprecation table states two dates in one cell with nothing
// between them, so they are separated by position: the first is the
// deprecation, the second the retirement. It is read after the model pages
// because it is the only document giving the retirement date of a model that
// is deprecated but still serving, whose page states only the earlier date.
package mistral
