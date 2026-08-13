// Package xai parses xAI's published documentation into the catalog model.
//
// xAI's own vocabulary again shares nothing with the other providers. It bills
// text by a prompt-size band rather than a service tier, so the same model has
// two input rates depending on how large the request is, and that band is
// written into the model cell as "(< 200k prompt tokens)". It quotes image
// generation per image, video per second, speech per hour and per minute with
// an equivalent rate in parentheses, text to speech per million characters,
// and server-side tools per thousand calls, all on one page in four tables of
// four different shapes. Batch discounts are stated as a percentage in prose
// rather than as rates.
//
// Documents read:
//
//   - the pricing page, whose four tables carry every rate and which is the
//     only place the full model list appears.
//   - the models page, for the knowledge cutoff stated in prose.
//   - one page per model, discovered from the identifiers found in the pricing
//     tables, carrying modalities, context window, capabilities, aliases, rate
//     limits and regions.
//
// Three of the server-side tools have a rate cell holding words rather than an
// amount, and the words are kept as a note against the tool. Image generation
// is charged at the Imagine API's own rates, which the same page states per
// image against the models that serve them; image and video understanding are
// charged by the tokens the analysis costs, at the rate of whichever model made
// the call. None of the three has a rate of its own, and the note is what says
// so rather than the absence.
//
// What xAI does not publish: a bound on output length. A model page states a
// context window, the rate limits and the regions, and stops there, so no
// model carries a maximum output.
package xai
