// Package mistral parses Mistral's model documentation into the catalog
// model.
//
// Mistral publishes one page per model and an index linking to all of them.
// The index describes nothing: the identifiers a model answers to, its context
// window, its rates, its modalities and its capabilities are stated only on
// the model's own page, so every page is fetched and the index is used for the
// list of them and for its deprecation table.
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
// What Mistral does not publish:
//
//   - A rate for a deprecated model. The price card is removed from the page
//     when a model is deprecated, though the model serves until it retires.
//     Seventeen chat models are in that state and carry no price, which is
//     what Mistral states rather than a gap in this parser.
//   - A rate for a retired model, correctly, since it no longer serves.
//   - An output bound for most models. Two state one; the rest state only a
//     context window.
//   - Capabilities for a retired model. The features tab is removed with the
//     price card.
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
