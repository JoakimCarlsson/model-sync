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
package groq
