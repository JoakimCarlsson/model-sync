package vertexai

import (
	"slices"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Scalar keys the lifecycle tables populate.
const (
	// AttrDeprecatedOn is when Google gave notice that a model is planned for
	// retirement. The deprecation page defines the two words apart: a
	// deprecated model still answers for the workloads already on it and gains
	// nothing further, and a retired one has been switched off.
	AttrDeprecatedOn = "deprecated_on"
	// AttrReplacement is the model Google names as the upgrade path.
	AttrReplacement = "recommended_replacement"
	// AttrSelfDeploy is what Google offers instead of a managed open model it
	// is withdrawing. It is not a replacement model but a way of running the
	// same weights, so it is recorded apart from one.
	AttrSelfDeploy = "self_deploy_alternative"
)

// lifecycle is what a lifecycle table states about one model.
type lifecycle struct {
	State       string
	Retires     string
	Qualifier   string
	Released    string
	Replacement string
	Deprecated  string
	SelfDeploy  string
	Source      string
}

// lifecyclePages are the tables stating when a model was released, when notice
// of its withdrawal was given and when it stops answering.
//
// Google keeps two, and neither names what the other does. The versions page
// covers the models Google made, states a release and a retirement date for
// each and lists the ones already gone. The open model deprecation page covers
// the models it serves for other labs as a managed API, and states the date
// notice was given as well, which the versions page never does: every managed
// open model was deprecated on one day with a retirement three months later,
// and until this was read the catalog said only that they were on sale.
var lifecyclePages = []string{versionsURL, deprecationsURL}

// readLifecycles reads both tables, keyed by the identifier the billing
// catalog would call the model.
//
// The identifier is matched exactly rather than by prefix, unlike a model
// page: a page describes a family and a row describes one release, and a live
// variant of a withdrawn model is one Google has not said it withdrew.
func readLifecycles(docs []catalog.Document) map[string]lifecycle {
	out := map[string]lifecycle{}
	for _, doc := range docs {
		if !slices.Contains(lifecyclePages, doc.URL) {
			continue
		}
		body := string(doc.Body)
		retired := versionsRetiredRe.FindString(body)
		for _, row := range versionsRowRe.FindAllStringSubmatch(body, -1) {
			id := servedName(firstField(specText(row[1])))
			if id == "" {
				continue
			}
			out[id] = readLifecycleRow(doc.URL, row, retired)
		}
	}
	return out
}

// readLifecycleRow reads one row, which the two pages head differently: the
// versions page states a release date and then a retirement date and names the
// model to move to, and the deprecation page states when notice was given and
// then the same retirement date and names a way to keep running the weights.
func readLifecycleRow(
	url string,
	row []string,
	retired string,
) lifecycle {
	life := lifecycle{
		Retires:   isoDate(specText(row[3])),
		Qualifier: qualifierOf(specText(row[3])),
		Source:    url,
	}
	if url == deprecationsURL {
		life.Deprecated = isoDate(specText(row[2]))
		life.SelfDeploy = specText(row[4])
		return life
	}
	life.Released = isoDate(specText(row[2]))
	life.Replacement = specText(row[4])
	if strings.Contains(retired, row[0]) {
		life.State = StateRetired
	}
	return life
}
