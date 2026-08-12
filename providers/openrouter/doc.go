// Package openrouter parses OpenRouter's model API into the catalog model.
//
// OpenRouter is the first source here that is not a document. It publishes a
// JSON endpoint listing every model it brokers, so there is no markdown, no
// HTML and no prose to read. That it still needs its own package is the point:
// the shape of the source changes nothing about where a provider's vocabulary
// lives.
//
// It is also a marketplace rather than a lab, so its catalog restates models
// belonging to other providers under its own identifiers. openai/gpt-5.6-sol
// here is the same model as gpt-5.6-sol under openai, priced as OpenRouter
// sells it rather than as OpenAI does. Both are recorded; neither is merged
// into the other, because the rates genuinely differ and the identifiers are
// OpenRouter's own.
//
// Rates are published per single unit, as a decimal string of dollars per
// token. They are scaled here to the denominators the rest of the catalog
// uses, per million tokens and per thousand calls. The scaling is exact
// rational arithmetic rather than floating point, so a rate of "0.000002"
// records as 2 and not as 1.9999999999999998.
package openrouter
