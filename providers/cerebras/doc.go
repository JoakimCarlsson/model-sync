// Package cerebras parses Cerebras' model catalog into the catalog model.
//
// Cerebras publishes no per-model rate. It sells monthly plans and a credit
// balance rather than a price per token, so its models carry limits and
// capabilities but no prices, and that absence is the accurate record of what
// Cerebras charges rather than a gap in this parser.
//
// Its one unusual field is a context window that differs by plan, written as
// "65k / 131k" for free and paid. Both are kept, because the free ceiling is
// what a reader on the free plan actually gets.
package cerebras
