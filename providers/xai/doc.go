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
// rather than as rates, and a withdrawn voice mode is marked by appending the
// word Deprecated to the cell naming it rather than by giving the table a
// column of its own.
//
// Documents read:
//
//   - the pricing page, whose four tables carry every rate and which is the
//     only place the full model list appears.
//   - the models page, for the knowledge cutoff stated in prose.
//   - one page per model, discovered from the identifiers found in the pricing
//     tables, carrying modalities, context window, capabilities, aliases, rate
//     limits and regions.
//   - one page per voice mode, named after the mode rather than the model.
//     xAI generates a page under a voice model's identifier too, but it states
//     a text-only pair of modalities, no rates and no rate limits; the mode's
//     page is where the modalities, the capabilities and the session bound are
//     written. The pricing table is what links the two, since it is the only
//     document naming the model a mode runs on.
//
// A voice page is also shaped differently from every other: it states the same
// facts a text model gives as bullets in a two-column table, and its
// capabilities as sentences rather than as a labelled yes. A sentence naming
// something the catalog has a word for becomes that word and the rest are left
// out, because a list of the audio formats a model accepts is not a list of
// what it can do.
//
// Three of the server-side tools have a rate cell holding words rather than an
// amount, and the words are kept as a note against the tool. Image generation
// is charged at the Imagine API's own rates, which the same page states per
// image against the models that serve them; image and video understanding are
// charged by the tokens the analysis costs, at the rate of whichever model made
// the call. None of the three has a rate of its own, and the note is what says
// so rather than the absence.
//
// What xAI does not publish:
//
//   - a bound on output length, for any model. There is not one to read: the
//     model registry the documentation site ships to the browser carries a
//     maxPromptLength and nothing that bounds a completion, and the Grok 4.6
//     overview answers the question outright with "No text output limit". The
//     Responses API's max_output_tokens defaults to 128,000 when a request
//     omits it, but the reference says a larger value is accepted, so that is
//     a default and not a ceiling.
//   - a context window for an image, video or voice model. The registry states
//     one only for the text models, and what bounds a voice request is not a
//     token count anyway: a speech to speech session is bounded in minutes,
//     which is recorded, and text to speech by characters per request, which
//     only the guide states and no model page does.
//   - a capability list for an image or video model. The registry carries
//     function calling, structured outputs and reasoning for the text models
//     and nothing at all for the generation models, whose pages state the
//     modalities, the rate and the rate limits and stop there.
//
// The server-side tools are carried as models so their rates have somewhere to
// live. A tool has no context window, no modalities and no capabilities of its
// own, and nothing in the tools documentation says otherwise.
package xai
