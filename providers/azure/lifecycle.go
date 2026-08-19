package azure

import (
	"maps"
	"slices"
	"strings"
)

// ScheduleURL is where Azure publishes, per model and per version, the
// lifecycle stage it is in, the day it stops answering and what to move to.
// The gallery states a stage for every model and a retirement date for almost
// none, so the two documents complement rather than repeat each other. It also
// covers models the gallery has dropped, which is the only place a retired
// model is still named.
const ScheduleURL = "https://learn.microsoft.com/en-us/azure/foundry/" +
	"openai/concepts/model-retirement-schedule"

// LifecycleURL defines the stages the schedule's Lifecycle column names and
// the order a model passes through them. It states nothing about any
// particular model, so it is not fetched; it is what the stage vocabulary
// this package translates is read against.
const LifecycleURL = "https://learn.microsoft.com/en-us/azure/foundry/" +
	"openai/concepts/model-retirements"

// Attributes the retirement schedule states.
const (
	AttrRetirementDate = "retirement_date"
	AttrReplacement    = "recommended_replacement"
	// AttrTrainingRetirement and AttrDeploymentRetirement are the two phases a
	// fine tuned model retires in, which Azure schedules separately from the
	// base model and from each other: training stops first and the models
	// already trained keep answering until the second date.
	AttrTrainingRetirement   = "fine_tuning_training_retirement_date"
	AttrDeploymentRetirement = "fine_tuning_deployment_retirement_date"
)

// Columns of the retirement schedule's tables, named as Azure heads them.
// They are matched exactly, because "Retirement date" is a prefix of neither
// of the two headings the fine tuning table uses and both tables head a Model
// and a Version column.
const (
	colVersion      = "version"
	colLifecycle    = "lifecycle"
	colRetirement   = "retirement date"
	colReplacement  = "replacement"
	colTrainRetire  = "training retirement date"
	colDeployRetire = "deployment retirement date"
)

// catalogModels name the models the gallery and the retirement schedule name
// differently from the concept pages collectionModels already covers, or that
// only these two documents name at all.
//
// The join is stated for the same reason collectionModels is stated: a meter's
// name has had the vendor stripped off it and abbreviated by the time it is an
// identifier, so DeepSeek-V3.1 is metered as v3.1 and gpt-35-turbo-16k as
// gpt-35-trb16k. Where a document names two variants of one metered model,
// both names map to it and it keeps whatever they agree on.
var catalogModels = map[string][]string{
	"dall-e-3":            {"image-dall-e-3"},
	"deepseek-r1":         {"r1"},
	"deepseek-v3-0324":    {"v3-0324"},
	"deepseek-v3.1":       {"v3.1"},
	"fw-kimi-k2-thinking": {"kimi-k2-thinking"},
	"gpt-35-turbo":        {"gpt-35-turbo4k", "gpt-3.5-turbo"},
	"gpt-35-turbo-16k": {
		"gpt-35-turbo16k",
		"gpt-35-trb16k",
		"gpt35-turbo-16k",
	},
	"grok-4-1-fast-non-reasoning": {"grok-4.1"},
	"grok-4-1-fast-reasoning":     {"grok-4.1"},
	"mistral-document-ai-2505":    {"doc-ai-2505"},
	"phi-3.5-mini-instruct":       {"phi-3.5-mini-128k-instruct"},
	"phi-3.5-vision-instruct":     {"phi-3.5-v-128k-instruct"},
}

// catalogKeys returns every key a document's reading of one named model is
// recorded under: the name itself, so that a meter reaches it by the same
// prefix rule every other document is reached by, and each identifier a stated
// join says the meters call it.
func catalogKeys(name string) []string {
	keys := []string{name}
	for _, id := range collectionModels[name] {
		keys = appendNew(keys, id)
	}
	for _, id := range catalogModels[name] {
		keys = appendNew(keys, id)
	}
	return keys
}

// readSchedule reads the retirement schedule's tables, keyed the same way the
// gallery is.
//
// A model is listed once per version, and the versions of one model are all
// written the same way, so the highest of them is the newest and its row is
// the one that describes the model as it stands. An older version's row states
// the stage that version reached, which is not the model's.
func readSchedule(body string) map[string]documented {
	stages := map[string][]scheduled{}
	tuned := map[string][]scheduled{}
	for _, table := range docTableRe.FindAllStringSubmatch(body, -1) {
		lines := docRowRe.FindAllStringSubmatch(table[1], -1)
		if len(lines) < 2 {
			continue
		}
		at := scheduleColumns(lines[0][1])
		if at[colModel] < 0 || at[colVersion] < 0 {
			continue
		}
		rows := stages
		switch {
		case at[colRetirement] >= 0:
		case at[colDeployRetire] >= 0:
			rows = tuned
		default:
			continue
		}
		for _, line := range lines[1:] {
			readScheduleRow(
				rows,
				at,
				docCellRe.FindAllStringSubmatch(line[1], -1),
			)
		}
	}
	out := map[string]documented{}
	for _, name := range slices.Sorted(maps.Keys(stages)) {
		record(out, name, newest(stages[name]))
	}
	for _, name := range slices.Sorted(maps.Keys(tuned)) {
		record(out, name, newest(tuned[name]))
	}
	return out
}

// record folds one reading into every key the named model is reached by.
//
// A key more than one name reaches keeps no name of its own, since Azure lists
// a reasoning and a non-reasoning variant of one metered model and neither
// name is the metered model's. Everything else it keeps is the first of those
// names to state it, in name order, because the variants of one metered model
// are one model as far as the price list is concerned and they agree on what
// this reads.
func record(out map[string]documented, name string, d documented) {
	for _, key := range catalogKeys(name) {
		held := out[key]
		before := held.Name
		fold(&held, d)
		if d.Name != "" {
			held.Name = soleName(before, d.Name)
		}
		out[key] = held
	}
}

// scheduled is one row of the schedule.
type scheduled struct {
	version string
	read    documented
}

// scheduleColumns locates the columns the schedule's two kinds of table are
// read by.
func scheduleColumns(row string) map[string]int {
	at := map[string]int{
		colModel:        -1,
		colVersion:      -1,
		colLifecycle:    -1,
		colRetirement:   -1,
		colReplacement:  -1,
		colTrainRetire:  -1,
		colDeployRetire: -1,
	}
	for i, cell := range docCellRe.FindAllStringSubmatch(row, -1) {
		header := strings.ToLower(docText(cell[1]))
		for name := range at {
			if at[name] < 0 && header == name {
				at[name] = i
			}
		}
	}
	return at
}

// readScheduleRow records one row against the model it names.
func readScheduleRow(
	rows map[string][]scheduled,
	at map[string]int,
	cells [][]string,
) {
	name := strings.ToLower(stated(cellText(cells, at[colModel])))
	if name == "" {
		return
	}
	version := stated(cellText(cells, at[colVersion]))
	stage := strings.ToLower(stated(cellText(cells, at[colLifecycle])))
	d := documented{
		Name:         displayName(stated(cellText(cells, at[colModel]))),
		Version:      version,
		State:        galleryStates[stage],
		Retire:       stated(cellText(cells, at[colRetirement])),
		Replacement:  stated(cellText(cells, at[colReplacement])),
		TrainRetire:  footnoted(cellText(cells, at[colTrainRetire])),
		DeployRetire: footnoted(cellText(cells, at[colDeployRetire])),
	}
	if isoDateRe.MatchString(version) {
		d.Release = version
	}
	rows[name] = append(rows[name], scheduled{version: version, read: d})
}

// newest returns the reading of a model's highest version, which is the one
// describing the model as it stands.
//
// An older version's row is not folded into it. Every value in a row belongs
// to the version the row is about: the stage that version reached, the day it
// stops answering and what to move off it onto, which for sora-2 is a later
// version of sora-2. Filling a gap in the newest row from an older one would
// state the older version's fact about the model.
func newest(versions []scheduled) documented {
	best := versions[0]
	for _, v := range versions[1:] {
		if v.version > best.version {
			best = v
		}
	}
	return best.read
}

// footnoted returns a cell's text with the footnote marker Azure appends to it
// dropped, which is how "No earlier than 2027-04-14 1" is the date it states
// and not the date and a marker.
func footnoted(cell string) string {
	return strings.TrimSpace(annotationRe.ReplaceAllString(stated(cell), ""))
}

// stated returns a cell's text, or nothing where the cell holds the dash Azure
// writes to say there is no value. The dash is matched by the absence of a
// letter or a digit rather than by its shape, since Azure writes several.
func stated(cell string) string {
	cell = strings.TrimSpace(cell)
	for _, r := range cell {
		if r >= '0' && r <= '9' {
			return cell
		}
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			return cell
		}
	}
	return ""
}
