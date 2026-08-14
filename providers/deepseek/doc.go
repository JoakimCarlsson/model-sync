// Package deepseek parses DeepSeek's pricing page, change log and Responses
// API guide into the catalog model.
//
// DeepSeek publishes two models and describes them in a transposed table:
// models are columns and each row is one fact about both. The table also uses
// spanning cells, so a row can carry a section label ahead of its own label,
// and a fact shared by both models appears once rather than twice. Rows are
// therefore read from the right: the last cells are the per-model values, and
// what precedes them names the row. A row with fewer values than there are
// models states one value that applies to all of them.
//
// It separates a cache hit from a cache miss rather than charging for cache
// writes, so its cheapest input rate is fifty times below its standard one.
//
// Every URL read here needs its trailing slash. Without it the site answers
// with its home page at a 200 and with no redirect, so the fetch succeeds and
// yields a document holding one table of base URLs and no model at all.
//
// A second table follows the first, and it is a schedule rather than a rate. A
// footnote dates it: from 16:00 UTC on 16 August 2026 the rates become peak and
// off-peak, with off-peak at half of peak, peak being 01:00-04:00 and
// 06:00-10:00 UTC. Those amounts are not recorded, because recording a rate that
// is not yet charged as though it were would misprice every call made before
// that date, and the six figures involved are between one and a half and four
// times the current ones. What is charged today is the first table, and after
// that date the first table is what DeepSeek will restate.
//
// One row is prose where the rest are ticks. The thinking mode row says the
// models support thinking and non-thinking modes and which they default to,
// which is more than a tick can carry, so the sentence is kept whole and the
// capability in it recorded as well.
//
// The pricing table states neither a name nor a modality, and two other pages
// do.
//
// The change log heads the entry for a release with the model's name as
// DeepSeek writes it, "DeepSeek-V4-Pro" against the identifier deepseek-v4-pro,
// and it is read for that. A heading is taken only where it is the identifier
// of a model the pricing page stated, which is what keeps "DeepSeek-V4", the
// heading of the release that announced both models, and the headings of every
// model withdrawn before them, from naming anything. The model version on the
// pricing table is not that name: it is the dated build the identifier
// currently resolves to.
//
// The Responses API guide states the modalities, and states them of the API
// rather than of a model, which is right here because both models answer the
// one API. Its table of input items says a message carries input_text and
// output_text content parts and then, in the sentence after, that image and
// file inputs are not supported. So the row is read a sentence at a time and a
// sentence denying support is skipped, since the one denying images names
// input_image inside itself. The Anthropic API guide says the same in its own
// table, marking type="text" supported and type="image" not supported, and is
// not read because it would add nothing.
//
// What DeepSeek does not publish: anything beyond text. Neither guide offers a
// modality its models accept other than text, and the chat completion
// reference types every message's content as text.
package deepseek
