package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/joakimcarlsson/model-sync/catalog"
	"github.com/joakimcarlsson/model-sync/store"
)

// The keys counted here. They are the vocabulary provider packages already
// declare; a consumer keying on a synonym would silently miss the field, so
// these strings are the ones to grep for when adding a provider. The three
// capability columns count the canonical values in catalog, and count them
// exactly: a provider still spelling one its vendor's way reads as zero here,
// which is what makes the column a measurement of convergence rather than a
// count of however many synonyms happen to exist.
const (
	limitContextWindow   = "context_window"
	limitMaxOutputTokens = "max_output_tokens"
	listInputModalities  = "input_modalities"
	listOutputModalities = "output_modalities"
	listDimensions       = "embedding_dimensions"
)

// stateAttrs are the two keys providers record a lifecycle under.
var stateAttrs = []string{"state", "lifecycle_state"}

// retiredStates are the values that mean a model is no longer served. A model
// in one of these is excluded from coverage, because its absent price is
// correct and counting it would report a permanent shortfall.
var retiredStates = []string{"retired", "deprecated", "shutdown"}

// row is one provider-and-kind bucket of the coverage table.
type row struct {
	provider   string
	kind       string
	live       int
	priced     int
	context    int
	maxOut     int
	features   int
	inMod      int
	outMod     int
	named      int
	dims       int
	embed      int
	reason     int
	structured int
	tools      int
}

func (r *row) add(other row) {
	r.live += other.live
	r.priced += other.priced
	r.context += other.context
	r.maxOut += other.maxOut
	r.features += other.features
	r.inMod += other.inMod
	r.outMod += other.outMod
	r.named += other.named
	r.dims += other.dims
	r.embed += other.embed
	r.reason += other.reason
	r.structured += other.structured
	r.tools += other.tools
}

// coverage prints how much of each field is populated across the catalog.
func coverage(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("coverage", flag.ExitOnError)
	api := fs.String("api", "api.json", "aggregate to read")
	data := fs.String(
		"data",
		"",
		"read the data tree instead of the aggregate",
	)
	kind := fs.String("kind", "", "report only this kind, such as chat")
	provider := fs.String("provider", "", "report only this provider")
	all := fs.Bool(
		"all",
		false,
		"count retired, deprecated and shutdown models too",
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	cat, err := loadCatalog(*api, *data)
	if err != nil {
		return err
	}
	rows := measure(cat, *kind, *provider, *all)
	return render(out, rows, *kind, *all)
}

// loadCatalog reads the aggregate, or the tree when one is asked for.
func loadCatalog(api, data string) (*catalog.Catalog, error) {
	if data != "" {
		return store.Load(data)
	}
	body, err := os.ReadFile(api)
	if err != nil {
		return nil, err
	}
	return store.DecodeAggregate(body)
}

// measure buckets every model by provider and kind and counts what is set.
func measure(
	cat *catalog.Catalog,
	kind, provider string,
	all bool,
) []row {
	buckets := map[string]*row{}
	order := []string{}
	for _, p := range cat.Providers {
		if provider != "" && p.ID != provider {
			continue
		}
		for _, m := range p.Models {
			if kind != "" && string(m.Kind) != kind {
				continue
			}
			if !all && !live(m) {
				continue
			}
			key := p.ID + "\x00" + string(m.Kind)
			bucket, ok := buckets[key]
			if !ok {
				bucket = &row{provider: p.ID, kind: kindLabel(m.Kind)}
				buckets[key] = bucket
				order = append(order, key)
			}
			bucket.add(count(m))
		}
	}
	slices.Sort(order)
	rows := make([]row, 0, len(order))
	for _, key := range order {
		rows = append(rows, *buckets[key])
	}
	return rows
}

// count reduces one model to the fields it populates.
func count(m catalog.Model) row {
	r := row{live: 1}
	if len(m.Prices) > 0 {
		r.priced = 1
	}
	if m.Limits[limitContextWindow] > 0 {
		r.context = 1
	}
	if m.Limits[limitMaxOutputTokens] > 0 {
		r.maxOut = 1
	}
	features := m.Lists[catalog.ListFeatures]
	if len(features) > 0 {
		r.features = 1
	}
	if slices.Contains(features, catalog.CapabilityReasoning) {
		r.reason = 1
	}
	if slices.Contains(features, catalog.CapabilityStructuredOutputs) {
		r.structured = 1
	}
	if slices.Contains(features, catalog.CapabilityFunctionCalling) {
		r.tools = 1
	}
	if len(m.Lists[listInputModalities]) > 0 {
		r.inMod = 1
	}
	if len(m.Lists[listOutputModalities]) > 0 {
		r.outMod = 1
	}
	if m.Name != "" {
		r.named = 1
	}
	if m.Kind == "embedding" {
		r.embed = 1
		if len(m.Lists[listDimensions]) > 0 {
			r.dims = 1
		}
	}
	return r
}

// live reports whether a model is still served. A model stating no lifecycle
// at all is live: most providers document only the models they serve and say
// nothing until one is withdrawn.
func live(m catalog.Model) bool {
	for _, key := range stateAttrs {
		value := strings.ToLower(strings.TrimSpace(m.Attrs[key]))
		if value == "" {
			continue
		}
		for _, dead := range retiredStates {
			if strings.HasPrefix(value, dead) {
				return false
			}
		}
	}
	return true
}

// kindLabel names the bucket of a model whose kind no document stated.
func kindLabel(k catalog.Kind) string {
	if k == "" {
		return "(none)"
	}
	return string(k)
}

// render writes the table, a subtotal per provider and a grand total. Counts
// are shown against the bucket size so a shortfall is visible without holding
// the live count of every provider in mind.
func render(out io.Writer, rows []row, kind string, all bool) error {
	tab := tabwriter.NewWriter(out, 0, 0, 2, ' ', tabwriter.AlignRight)
	w := &errWriter{w: tab}
	scope := "live models"
	if all {
		scope = "all models, including retired"
	}
	if kind != "" {
		scope = kind + " " + scope
	}
	header := &errWriter{w: out}
	header.printf("coverage of %s\n\n", scope)
	if header.err != nil {
		return header.err
	}
	w.printf(
		"provider\tkind\tlive\tpriced\tctx\tmaxout\tfeats\treason\tstruct\ttools\tinmod\toutmod\tname\tembdim\t\n",
	)
	var total row
	groups := byProvider(rows)
	for _, group := range groups {
		var subtotal row
		for _, r := range group {
			subtotal.add(r)
			total.add(r)
			writeRow(w, r.provider, r.kind, r)
		}
		if len(group) > 1 {
			writeRow(w, group[0].provider, "all kinds", subtotal)
		}
		if len(groups) > 1 {
			w.printf("\t\t\t\t\t\t\t\t\t\t\t\t\t\t\n")
		}
	}
	if len(groups) > 1 {
		writeRow(w, "all", "all kinds", total)
	}
	if w.err != nil {
		return w.err
	}
	return tab.Flush()
}

// errWriter holds the first write error so a run of formatted writes can be
// checked once at the end rather than after every line.
type errWriter struct {
	w   io.Writer
	err error
}

func (e *errWriter) printf(format string, args ...any) {
	if e.err != nil {
		return
	}
	_, e.err = fmt.Fprintf(e.w, format, args...)
}

// byProvider splits the sorted rows into one run per provider.
func byProvider(rows []row) [][]row {
	groups := [][]row{}
	for i, r := range rows {
		if i > 0 && rows[i-1].provider == r.provider {
			groups[len(groups)-1] = append(groups[len(groups)-1], r)
			continue
		}
		groups = append(groups, []row{r})
	}
	return groups
}

// writeRow writes one line, blanking a count that cannot apply.
func writeRow(w *errWriter, provider, kind string, r row) {
	w.printf(
		"%s\t%s\t%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t\n",
		provider,
		kind,
		r.live,
		gap(r.priced, r.live),
		gap(r.context, r.live),
		gap(r.maxOut, r.live),
		gap(r.features, r.live),
		gap(r.reason, r.live),
		gap(r.structured, r.live),
		gap(r.tools, r.live),
		gap(r.inMod, r.live),
		gap(r.outMod, r.live),
		gap(r.named, r.live),
		gap(r.dims, r.embed),
	)
}

// gap renders a count, marking a full bucket so the eye finds the shortfalls.
func gap(have, of int) string {
	if of == 0 {
		return "-"
	}
	if have == of {
		return fmt.Sprintf("%d", have)
	}
	return fmt.Sprintf("%d!", have)
}
