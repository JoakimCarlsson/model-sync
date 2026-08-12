// Package openai parses OpenAI's published documentation into the catalog
// model.
//
// Every string OpenAI uses lives here: its metric and unit names, its
// dimension keys, its column headers, its section and tier labels, and the
// shapes of its documents. OpenAI publishes markdown by appending .md to a doc
// URL, which makes it cheap to parse; that is a fact about OpenAI and not a
// pattern other providers follow, so the readers in this package are local to
// it and shared with nobody.
//
// Three documents shapes are read, because one provider already publishes in
// more than one:
//
//   - pricing.md, whose rates live in markdown pipe tables qualified by the
//     bare prose lines above them.
//   - the per-model pages under /api/docs/models/, which carry capabilities,
//     modalities, limits, endpoints, snapshots and rate limits.
//   - the guides, whose per-image dollar tables are raw JSX with rowSpan and
//     are invisible to a markdown table reader.
package openai
