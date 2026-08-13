// Package groq parses Groq's supported-models page into the catalog model.
//
// Groq publishes one table per standing: production, systems and preview. Its
// cells hold several values run together with no separator, because they are
// rendered as stacked lines rather than written as text: a price reads
// "$0.05 input$0.08 output" and a rate limit reads "250K TPM1K RPM". Both are
// read by matching each value and its label rather than by splitting, since
// there is nothing to split on.
//
// Groq serves speech as well as text, so the same column states dollars per
// million tokens, per million characters, and per hour of audio, and only the
// label after the amount says which.
//
// The table says nothing about what a model takes or can do. Each row links to
// the model's own page, which states both under headings written in capitals
// with the value on the line below, and that page names the identifier the
// API answers to, so the two need no matching. Vision is listed there as a
// capability, and is recorded as an image input instead, which is what every
// provider stating modalities calls it.
//
// A system is a model in the table and a page like any other, but Groq files
// its page under a path of its own rather than beside the models, so both are
// followed. That page closes with the sentence saying a system has no rate:
// what a query costs is whatever the underlying models and built-in tools it
// reached for cost. The sentence is kept as a note, which is why the two
// compound systems carry no price and do not read as free.
//
// What Groq does not publish: a rate for a model sold only to enterprises. Its
// table writes "ContactSales" where the amount goes, and the badge beside the
// model's name is recorded instead, so the model carries an access note rather
// than a price.
package groq
