package catalog

import (
	"context"
	"maps"
	"slices"
	"strings"
)

// Kind classifies what a model does. Values are declared by provider packages.
type Kind string

// Metric is the thing being counted and billed. Values are declared by
// provider packages.
type Metric string

// Unit is the denominator an amount is quoted against. Values are declared by
// provider packages.
type Unit string

// Dims are the axes along which a price varies. Both keys and values are
// provider vocabulary. An empty Dims means the price is unconditional.
type Dims map[string]string

// Key returns a stable encoding of the dimensions, used for deduplication and
// for deterministic ordering.
func (d Dims) Key() string {
	if len(d) == 0 {
		return ""
	}
	keys := slices.Sorted(maps.Keys(d))
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+d[k])
	}
	return strings.Join(parts, ";")
}

// With returns a copy of d with k set to v, leaving the receiver untouched so
// a caller can layer per-row values onto a shared base. An empty v is dropped.
func (d Dims) With(k, v string) Dims {
	out := make(Dims, len(d)+1)
	maps.Copy(out, d)
	if v != "" {
		out[k] = v
	}
	return out
}

// Merge returns a copy of d with all of other's keys layered on top.
func (d Dims) Merge(other Dims) Dims {
	out := make(Dims, len(d)+len(other))
	maps.Copy(out, d)
	maps.Copy(out, other)
	return out
}

// Price is one billable rate for one model. A model priced both per token and
// per generated image holds both, distinguished by Unit.
type Price struct {
	Metric   Metric  `json:"metric"`
	Unit     Unit    `json:"unit,omitempty"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency,omitempty"`
	Dims     Dims    `json:"dims,omitempty"`
	// Note carries pricing text that does not reduce to the amount, such as a
	// free allowance or a rider billed separately.
	Note string `json:"note,omitempty"`
}

func (p Price) key() string {
	return string(p.Metric) + "|" + string(p.Unit) + "|" + p.Dims.Key()
}

// Model is one model from one provider, with every rate and every capability
// known for it.
//
// Attrs, Limits and Lists are open the same way Dims is: their keys are
// provider vocabulary. A provider exposing something no other provider has
// adds a key, never a struct field.
type Model struct {
	ID       string  `json:"id"`
	Provider string  `json:"provider"`
	Name     string  `json:"name,omitempty"`
	Kind     Kind    `json:"kind,omitempty"`
	Prices   []Price `json:"prices,omitempty"`
	// Attrs holds scalar facts: a description, a knowledge cutoff, a default
	// snapshot.
	Attrs map[string]string `json:"attrs,omitempty"`
	// Limits holds numeric bounds: context window, max output tokens,
	// per-tier rate limits.
	Limits map[string]int64 `json:"limits,omitempty"`
	// Lists holds enumerations: supported features, tools, endpoints, input
	// and output modalities, snapshots.
	Lists   map[string][]string `json:"lists,omitempty"`
	Notes   []string            `json:"notes,omitempty"`
	Sources []string            `json:"sources,omitempty"`
}

// AddPrice appends a price, dropping it only when an identical tuple with an
// identical amount and note is already present. Two rates sharing a tuple but
// differing in amount are both kept, so a contradiction between two documents
// survives into the output instead of being silently resolved here.
func (m *Model) AddPrice(p Price) {
	for _, existing := range m.Prices {
		if existing.key() == p.key() && existing.Amount == p.Amount && existing.Note == p.Note {
			return
		}
	}
	m.Prices = append(m.Prices, p)
}

// SetAttr records a scalar fact. Empty values are ignored and an existing
// value is never overwritten, so the first document to state something wins.
func (m *Model) SetAttr(key, value string) {
	if key == "" || value == "" {
		return
	}
	if m.Attrs == nil {
		m.Attrs = map[string]string{}
	}
	if _, ok := m.Attrs[key]; !ok {
		m.Attrs[key] = value
	}
}

// SetLimit records a numeric bound. Zero is ignored and an existing value is
// never overwritten.
func (m *Model) SetLimit(key string, value int64) {
	if key == "" || value == 0 {
		return
	}
	if m.Limits == nil {
		m.Limits = map[string]int64{}
	}
	if _, ok := m.Limits[key]; !ok {
		m.Limits[key] = value
	}
}

// AddList appends to an enumeration, dropping empties and duplicates.
func (m *Model) AddList(key string, values ...string) {
	if key == "" {
		return
	}
	for _, v := range values {
		if v == "" || slices.Contains(m.Lists[key], v) {
			continue
		}
		if m.Lists == nil {
			m.Lists = map[string][]string{}
		}
		m.Lists[key] = append(m.Lists[key], v)
	}
}

// AddNote appends a note if not already present.
func (m *Model) AddNote(note string) {
	if note == "" || slices.Contains(m.Notes, note) {
		return
	}
	m.Notes = append(m.Notes, note)
}

// AddSource records a URL this model's data came from.
func (m *Model) AddSource(src string) {
	if src == "" || slices.Contains(m.Sources, src) {
		return
	}
	m.Sources = append(m.Sources, src)
}

func (m *Model) sort() {
	slices.SortStableFunc(m.Prices, func(a, b Price) int {
		if a.Metric != b.Metric {
			return strings.Compare(string(a.Metric), string(b.Metric))
		}
		if ka, kb := a.Dims.Key(), b.Dims.Key(); ka != kb {
			return strings.Compare(ka, kb)
		}
		if a.Unit != b.Unit {
			return strings.Compare(string(a.Unit), string(b.Unit))
		}
		return compareFloat(a.Amount, b.Amount)
	})
	for _, values := range m.Lists {
		slices.Sort(values)
	}
	slices.Sort(m.Notes)
	slices.Sort(m.Sources)
}

func compareFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// Document is one fetched artifact, kept with the URL it came from so a parser
// can attribute every fact to its source.
type Document struct {
	URL  string
	Body []byte
}

// Source is one vendor. Fetch and Parse are separate so a parser can be driven
// from documents on disk without touching the network.
type Source interface {
	ID() string
	Name() string
	Fetch(ctx context.Context) ([]Document, error)
	Parse(docs []Document) ([]Model, error)
}

// Provider is one vendor's slice of the catalog.
type Provider struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Models []Model `json:"models"`
}

// Catalog is the whole dataset.
type Catalog struct {
	Providers []Provider `json:"providers"`
}

// Add appends a provider's models to the catalog.
func (c *Catalog) Add(id, name string, models []Model) {
	c.Providers = append(c.Providers, Provider{ID: id, Name: name, Models: models})
}

// Normalize orders providers, models, prices and enumerations so regenerating
// the catalog from unchanged input produces byte-identical output.
func (c *Catalog) Normalize() {
	slices.SortStableFunc(c.Providers, func(a, b Provider) int {
		return strings.Compare(a.ID, b.ID)
	})
	for i := range c.Providers {
		models := c.Providers[i].Models
		for j := range models {
			models[j].sort()
		}
		slices.SortStableFunc(models, func(a, b Model) int {
			return strings.Compare(a.ID, b.ID)
		})
	}
}

// Count returns the number of models across all providers.
func (c *Catalog) Count() int {
	n := 0
	for _, p := range c.Providers {
		n += len(p.Models)
	}
	return n
}
