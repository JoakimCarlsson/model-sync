// Package google parses Google's Gemini API pricing into the catalog model.
//
// Google states rates only in the rendered pricing page, and states them
// twice over: a model has one table per serving tier, and each table has one
// column per plan. A single rate therefore needs both a tier and a plan to
// identify it, and the free plan's "Free of charge" is a real rate of zero
// rather than an absence.
//
// Neither the model nor the tier appears inside its table. Both are headings
// above it, so the page is read as a running state of which model and which
// tier are in force rather than as a list of self-describing tables.
package google
