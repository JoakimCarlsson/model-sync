package azure

import (
	"encoding/json"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// GalleryURL is the catalog the Foundry portal browses. It answers without a
// credential and is the only document Azure publishes that states a display
// name, a description, a publisher, a licence and a lifecycle stage for every
// model at once. Everything the concept pages state about a model is stated
// here as well, in fields rather than as prose in a table cell.
const GalleryURL = "https://api.catalog.azureml.ms/asset-gallery/v1.0/models"

// galleryExcluded is the publisher whose entries are left out of the listing.
//
// The catalog holds fifteen thousand models, of which thirteen thousand are
// Hugging Face repositories mirrored for deployment onto a virtual machine.
// Azure meters none of them: they are billed as compute, and the meter listing
// this package reads holds no rate for any of them. Excluding one publisher is
// what turns a listing nobody can page through into two thousand entries.
const galleryExcluded = "Hugging Face"

// galleryPageSize is the largest page the listing accepts.
const galleryPageSize = 100

// galleryMaxPages bounds the walk, as the meter listing's own bound does.
const galleryMaxPages = 60

// Attributes the gallery and the retirement schedule state.
const (
	AttrSummary      = "summary"
	AttrState        = "state"
	AttrAuthor       = "author"
	AttrPublisher    = "publisher"
	AttrLicense      = "license"
	AttrVersion      = "version"
	AttrReleaseDate  = "release_date"
	AttrCatalogAdded = "catalog_added_on"
	// AttrKnowledgeCutoff is AttrTrainingCutoff under the catalog's own name,
	// normalized to an ISO date. Azure writes the fact as "October 2023" or
	// "May 31, 2024"; AttrTrainingCutoff keeps Azure's wording and this states
	// the same fact in the form a consumer can compare.
	AttrKnowledgeCutoff = "knowledge_cutoff"
)

// Enumerations only the gallery states.
const (
	// ListKeywords is what the catalog tags a model with: the words a reader
	// filters the portal's model list by.
	ListKeywords = "keywords"
	// ListTasks is what the catalog says a model is for, which is a task name
	// rather than an API route, so it is not ListEndpoints.
	ListTasks = "tasks"
	// ListDeployments is the deployment types a model can be served on, and
	// ListRegions the Azure regions those deployments exist in. Both are
	// stated per deployment type; the regions are flattened, because a
	// consumer asking whether a model runs in a region does not want to ask it
	// once per deployment type.
	ListDeployments = "deployment_types"
	ListRegions     = "regions"
)

// galleryLatest is the label the catalog marks the current version of a model
// with. A model is listed once per version it has had.
const galleryLatest = "latest"

// galleryFeatures translate the catalog's capability names into the catalog
// vocabulary. Azure states these as a list of names rather than as prose,
// which is why they cover far more models than the concept pages' bullets do.
var galleryFeatures = map[string][]string{
	"reasoning":        {catalog.CapabilityReasoning},
	"tool-calling":     {catalog.CapabilityFunctionCalling},
	"function-calling": {catalog.CapabilityFunctionCalling},
	"streaming":        {"streaming"},
	"fine-tuning":      {"fine_tuning"},
	"agents":           {"agents"},
	"agent":            {"agents"},
	"agentsv2":         {"agents", "agents_v2"},
	"assistants":       {"assistants"},
	"assistant":        {"assistants"},
	"background-mode":  {"background_mode"},
	"image-input":      {"image_input"},
	"routing":          {"routing"},
}

// galleryModalities translate the catalog's modality names into the catalog
// vocabulary. Azure names a PDF as its own modality and this package records
// it as a file, which is the word the rest of the catalog uses. Anything else
// the gallery names is kept as Azure writes it, since a modality nobody else
// states is still a fact Azure states.
var galleryModalities = map[string]string{"pdf": "file"}

// galleryStates translate a lifecycle stage into the catalog's vocabulary.
// Azure defines the five stages on its lifecycle page and spells the third of
// them two ways in the gallery's own data.
var galleryStates = map[string]string{
	"generally available": "active",
	"stable":              "active",
	"ga":                  "active",
	"preview":             "preview",
	"legacy":              "legacy",
	"deprecated":          "deprecated",
	"retired":             "retired",
}

// isoDateRe matches a version written as a dated release. Azure's lifecycle
// page defines a model version as "a dated release within a family", so a
// version of this shape is the day the model was released and a version that
// counts revisions is not.
var isoDateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// galleryRequest is one page of the listing's query.
type galleryRequest struct {
	Filters           []galleryFilter `json:"filters"`
	PageSize          int             `json:"pageSize"`
	ContinuationToken string          `json:"continuationToken,omitempty"`
}

// galleryFilter is one condition on the listing.
type galleryFilter struct {
	Field    string   `json:"field"`
	Values   []string `json:"values"`
	Operator string   `json:"operator"`
}

// galleryPage is one response from the listing.
type galleryPage struct {
	Summaries         []gallerySummary `json:"summaries"`
	ContinuationToken string           `json:"continuationToken"`
}

// gallerySummary is one version of one model, as the catalog states it.
type gallerySummary struct {
	AssetID           string        `json:"assetId"`
	Name              string        `json:"name"`
	DisplayName       string        `json:"displayName"`
	Version           string        `json:"version"`
	Publisher         string        `json:"publisher"`
	Author            string        `json:"author"`
	License           string        `json:"license"`
	Summary           string        `json:"summary"`
	Lifecycle         string        `json:"lifecycle"`
	CreatedTime       string        `json:"createdTime"`
	Labels            []string      `json:"labels"`
	Keywords          []string      `json:"keywords"`
	InferenceTasks    []string      `json:"inferenceTasks"`
	ModelCapabilities []string      `json:"modelCapabilities"`
	Deprecation       galleryRetire `json:"deprecation"`
	ModelLimits       galleryLimits `json:"modelLimits"`
	DeploymentSKU     []gallerySKU  `json:"deploymentSku"`
}

// galleryRetire is when the catalog says a model stops answering.
type galleryRetire struct {
	InferenceRetirementDate string `json:"inferenceRetirementDate"`
}

// galleryLimits is what the catalog says a model holds.
type galleryLimits struct {
	TextLimits                galleryText `json:"textLimits"`
	SupportedLanguages        []string    `json:"supportedLanguages"`
	SupportedInputModalities  []string    `json:"supportedInputModalities"`
	SupportedOutputModalities []string    `json:"supportedOutputModalities"`
}

// galleryText is the pair of token bounds.
type galleryText struct {
	InputContextWindow int64 `json:"inputContextWindow"`
	MaxOutputTokens    int64 `json:"maxOutputTokens"`
}

// gallerySKU is one deployment type and the regions it exists in.
type gallerySKU struct {
	Name      string   `json:"name"`
	Locations []string `json:"locations"`
}

// readGallery reads the catalog listing, keyed the same way the concept pages
// are read: by the name the model is documented under, and additionally by
// every identifier the meters call it, so that a name the price list has
// already stripped its vendor off still reaches its entry.
func readGallery(body []byte) map[string]documented {
	var page galleryPage
	if err := json.Unmarshal(body, &page); err != nil {
		return nil
	}
	byName := map[string][]gallerySummary{}
	for _, s := range page.Summaries {
		name := strings.ToLower(strings.TrimSpace(s.Name))
		if name == "" {
			continue
		}
		byName[name] = append(byName[name], s)
	}
	out := map[string]documented{}
	for _, name := range slices.Sorted(maps.Keys(byName)) {
		record(out, name, readGallerySummary(current(byName[name])))
	}
	return out
}

// current returns the version of a model the catalog presents as its current
// one, which is the one it labels latest. A model whose versions have all
// retired carries no such label, and the highest version is then the last one
// Azure published.
func current(versions []gallerySummary) gallerySummary {
	slices.SortFunc(versions, func(a, b gallerySummary) int {
		if latest(a) != latest(b) {
			if latest(a) {
				return -1
			}
			return 1
		}
		if a.Version != b.Version {
			return strings.Compare(b.Version, a.Version)
		}
		if a.CreatedTime != b.CreatedTime {
			return strings.Compare(b.CreatedTime, a.CreatedTime)
		}
		return strings.Compare(a.AssetID, b.AssetID)
	})
	return versions[0]
}

// latest reports whether the catalog labels this version the current one.
func latest(s gallerySummary) bool {
	return slices.Contains(s.Labels, galleryLatest)
}

// readGallerySummary records what the catalog states about one model.
func readGallerySummary(s gallerySummary) documented {
	d := documented{
		Name:      strings.TrimSpace(s.DisplayName),
		Summary:   strings.TrimSpace(s.Summary),
		State:     galleryStates[strings.ToLower(strings.TrimSpace(s.Lifecycle))],
		Publisher: strings.TrimSpace(s.Publisher),
		Author:    strings.TrimSpace(s.Author),
		License:   strings.TrimSpace(s.License),
		Version:   strings.TrimSpace(s.Version),
		Added:     day(s.CreatedTime),
		Retire:    day(s.Deprecation.InferenceRetirementDate),
		Context:   s.ModelLimits.TextLimits.InputContextWindow,
		MaxOut:    s.ModelLimits.TextLimits.MaxOutputTokens,
		Languages: s.ModelLimits.SupportedLanguages,
		Keywords:  trimmed(s.Keywords),
		Tasks:     trimmed(s.InferenceTasks),
	}
	if d.Author == "" {
		d.Author = d.Publisher
	}
	if isoDateRe.MatchString(d.Version) {
		d.Release = d.Version
	}
	d.InputMod = galleryFlow(s.ModelLimits.SupportedInputModalities)
	d.OutMod = galleryFlow(s.ModelLimits.SupportedOutputModalities)
	for _, name := range s.ModelCapabilities {
		d.Features = appendNew(
			d.Features,
			galleryFeatures[strings.ToLower(strings.TrimSpace(name))]...,
		)
	}
	for _, sku := range s.DeploymentSKU {
		d.Deployments = appendNew(d.Deployments, strings.TrimSpace(sku.Name))
		for _, region := range sku.Locations {
			d.Regions = appendNew(
				d.Regions,
				strings.ToLower(strings.TrimSpace(region)),
			)
		}
	}
	return d
}

// galleryFlow translates the modality names one side of the catalog's flow
// states.
func galleryFlow(names []string) []string {
	var out []string
	for _, name := range names {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			continue
		}
		if mapped, ok := galleryModalities[name]; ok {
			name = mapped
		}
		out = appendNew(out, name)
	}
	return out
}

// day reduces a timestamp to the date it falls on, which is the precision
// Azure states the fact at.
func day(stamp string) string {
	date, _, _ := strings.Cut(strings.TrimSpace(stamp), "T")
	if !isoDateRe.MatchString(date) {
		return ""
	}
	return date
}

// trimmed drops the blanks and the surrounding spaces from a list of values.
// The catalog writes a leading space into some of its keywords.
func trimmed(values []string) []string {
	var out []string
	for _, value := range values {
		out = appendNew(out, strings.TrimSpace(value))
	}
	return out
}
