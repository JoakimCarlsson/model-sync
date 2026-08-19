package together

import (
	"regexp"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// DedicatedURL is the catalog of models Together will deploy on hardware
// reserved for one account, which is a different product from the per-token
// one the serverless catalog prices.
const DedicatedURL = "https://docs.together.ai/docs/" +
	"dedicated-endpoints/models.md"

// dedicatedEntryRe matches one entry of that catalog. The page renders its
// table from a literal array in the page source rather than writing the table
// out, so the array is what is read. The hardware is optional: Together writes
// a dash in the column for a model it publishes no deployment profile for.
var dedicatedEntryRe = regexp.MustCompile(
	`apiName: "([^"]+)"(?:[^{}]*?minHardware: "([^"]+)")?`,
)

// deploymentDedicated is what the model library calls this deployment option,
// and it is recorded in the library's word so that the two documents do not
// each contribute a spelling.
const deploymentDedicated = "Dedicated"

// applyDedicated reads the dedicated inference catalog onto the models the
// serverless catalog established.
//
// Together publishes two catalogs and they overlap without agreeing: a model
// may be sold per token, deployed on reserved hardware, both, or neither, and
// only this page says which models are in the second set. For a model in both
// it also states the smallest instance type it publishes a deployment profile
// for, which is the closest thing Together states to a model's size in
// hardware.
//
// A model listed only here creates nothing. It has no rate on any page, and a
// catalog entry whose every price field is empty says less than the absence of
// the entry does.
func (b *builder) applyDedicated(doc catalog.Document) {
	for _, entry := range dedicatedEntryRe.FindAllStringSubmatch(
		string(doc.Body),
		-1,
	) {
		m, ok := b.models[entry[1]]
		if !ok {
			continue
		}
		m.AddList(ListDeployments, deploymentDedicated)
		m.SetAttr(AttrDedicatedHardware, entry[2])
		m.AddSource(doc.URL)
	}
}
