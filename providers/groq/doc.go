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
// The page also prices the model again, and prices one thing more than the
// table does: the table's price column holds an input and an output amount and
// has no column for the cached input rate, which the page states under a label
// of its own. The two rates that agree with the table are dropped as
// duplicates. The page leaves the denominator of a token rate unwritten, and it
// is the one Groq quotes token rates against everywhere it does write it: per
// 1M tokens, as the table's column and the systems' pricing heading both say.
//
// A system is a model in the table and a page like any other, but Groq files
// its page under a path of its own rather than beside the models, so both are
// followed. The table's price column is empty for a system and its page says
// why: a system has no rate of its own, what a query costs being whatever the
// underlying models and built-in tools it reached for cost. It then states
// those rates, one per model it reaches for and one per tool it can call, and
// they are recorded against the system with the model or the tool as a
// dimension. The sentence is kept as a note as well, since the rates are a set
// to be drawn from rather than a total.
//
// What Groq does not publish: a rate for a model sold only to enterprises. Its
// table writes "ContactSales" where the amount goes, its page states no pricing
// section at all, and the table's cell is kept as a note saying the model is
// sold by arrangement so it does not read as free. The badge beside the model's
// name is recorded as well, but it marks the plan the model belongs to and
// several priced models carry one, so it is the empty amount and not the badge
// that says there is no rate. Where a rate exists but is not settled, the page
// writes "Pending" instead of an amount, and that is kept the same way.
//
// Nor a context window or an output ceiling for the two Whisper models: the
// table writes a dash in both columns, and the limits section of each page
// states a maximum file size or nothing at all. Groq bounds a transcription by
// the size and length of the audio, which the speech-to-text guide states, and
// never by a count of tokens.
package groq
