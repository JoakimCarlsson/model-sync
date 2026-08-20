package google

import (
	"regexp"
	"slices"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// Keys the media guides populate.
const (
	ListAspectRatios     = "aspect_ratios"
	ListResolutions      = "resolutions"
	ListDurationsSeconds = "durations_seconds"
	ListPersonGeneration = "person_generation"
	ListImageSizes       = "image_sizes"
	ListParameters       = catalog.ListParameters
)

// Numeric bounds the media guides state.
const (
	LimitFramesPerSecond = "frames_per_second"
	LimitMaxOutputVideos = "max_output_videos"
	LimitMaxOutputImages = "max_output_images"
)

// Rows of the Veo comparison tables this parser reads, named as Google labels
// them. The first four are request parameters and the rest are properties of
// the output.
const (
	paramAspectRatio      = "aspectratio"
	paramDurationSeconds  = "durationseconds"
	paramPersonGeneration = "persongeneration"
	paramResolution       = "resolution"
	paramImageSize        = "imagesize"
	featureAudio          = "audio"
	// modalityAudio is the catalog's word for sound, which is also the word
	// the guides name the feature with.
	modalityAudio    = "audio"
	featureInputs    = "input modalities"
	featureFrameRate = "frame rate"
	featureVideosPer = "videos per request"
	featureStatus    = "status"
	// headParameter is what the comparison of request parameters heads its
	// first column with, and what tells it from the comparison of the output,
	// whose rows name a property rather than something a caller may send.
	headParameter = "parameter"
)

// present is the mark Google puts in a comparison cell for a capability the
// model has, against a cross for one it lacks.
const present = "✔"

// featureAudioGeneration is what the Gemini model pages call the capability
// the Veo comparison heads "Audio", and is the name it is recorded under so
// that one vendor's two words for the same thing do not become two features.
const featureAudioGeneration = "audio_generation"

// generationRe matches how the Veo comparison names an input, which it writes
// as the conversion the input is used for rather than as the input itself.
var generationRe = regexp.MustCompile(`(?i)^(\w+)-to-video$`)

// unsupported are the two ways a Veo column says a model does not take a
// parameter at all.
var unsupported = []string{"n/a", "unsupported"}

// mediaStates map the availability Google states in the Veo comparison onto
// the catalog's vocabulary.
var mediaStates = map[string]string{
	"stable":  StateActive,
	"preview": StatePreview,
}

var (
	// comparisonRe matches a table with one column per model. The media guides
	// carry several, and the two that compare models are told from the rest by
	// what their first heading cell says.
	comparisonRe = regexp.MustCompile(
		`(?is)<table>\s*<thead>(.*?)</thead>(.*?)</table>`,
	)
	headCellRe = regexp.MustCompile(`(?is)<th[^>]*>(.*?)</th\s*>`)
	// quotedRe matches an accepted value, which Google quotes inside the code
	// span it writes it in. An unquoted span in the same cell is a type name
	// rather than a value.
	quotedRe = regexp.MustCompile(`^&quot;(.*)&quot;$|^"(.*)"$`)
	// asideRe matches the italic rider Google hangs off a cell to say when a
	// value applies, whose own numbers are not values.
	asideRe = regexp.MustCompile(`(?is)<i>.*?</i\s*>`)
	// resolutionRe matches an output size named in prose.
	resolutionRe = regexp.MustCompile(`(?i)\b(\d+p|\d+k)\b`)
	// ratioRe matches an aspect ratio stated as a row label.
	ratioRe = regexp.MustCompile(`^\d+:\d+$`)
	// sizeHeadRe matches the column heading of an image size, "1K resolution"
	// or "512px resolution".
	sizeHeadRe = regexp.MustCompile(`(?i)^(\d+px|\d+k)\s+resolution$`)
	// headingStartRe matches the opening of a heading, which is where one
	// section of prose ends.
	headingStartRe = regexp.MustCompile(`(?i)<h[1-6][\s>]`)
	// generateRe matches the part of a video endpoint that says what it does
	// rather than which model it is.
	generateRe = regexp.MustCompile(`-generate.*$`)
)

// familyKey reduces a video endpoint to the name the comparison tables head
// its column with. The tables name a model and the API answers to an endpoint,
// and neither document states the other, so the two are brought to a common
// form: the part of the endpoint before the verb, with the trailing zero of a
// major-only version dropped, which is exactly how Google writes the heading.
// veo-3.0-fast-generate-001 becomes veo-3-fast and its column is headed "Veo 3
// Fast".
func familyKey(id string) string {
	key := generateRe.ReplaceAllString(id, "")
	return strings.ReplaceAll(key, ".0", "")
}

// columnModels pairs each column of a comparison table with the models it
// prices. Google heads one column with two models where they differ in
// nothing the table states, writing "Veo 3.1 & Veo 3.1 Fast".
func (b *builder) columnModels(head string) [][]*catalog.Model {
	cells := headCellRe.FindAllStringSubmatch(head, -1)
	if len(cells) < 2 {
		return nil
	}
	byFamily := map[string][]*catalog.Model{}
	for _, id := range b.order {
		key := familyKey(id)
		byFamily[key] = append(byFamily[key], b.models[id])
	}
	out := make([][]*catalog.Model, 0, len(cells)-1)
	for _, cell := range cells[1:] {
		var models []*catalog.Model
		for _, name := range strings.Split(text(cell[1]), "&") {
			models = append(models, byFamily[slugID(name)]...)
		}
		out = append(out, models)
	}
	return out
}

// applyVideoGuide reads the Veo guide, which states per model what the model
// pages do not: which aspect ratios, resolutions, durations and person
// settings a request may ask for, what frame rate comes back, how many videos
// one request yields and whether the model is generally available.
//
// The guide also carries a property table per endpoint, including the two Veo
// 3 endpoints Google publishes no model page for, so it is read for those as
// well.
func (b *builder) applyVideoGuide(doc catalog.Document) {
	body := string(doc.Body)
	b.applyGuideTables(doc)
	for _, table := range comparisonRe.FindAllStringSubmatch(body, -1) {
		columns := b.columnModels(table[1])
		if len(columns) == 0 {
			continue
		}
		params := comparesParameters(table[1])
		for _, row := range pageRowRe.FindAllStringSubmatch(table[2], -1) {
			cells := pageCellRe.FindAllStringSubmatch(row[1], -1)
			if len(cells) != len(columns)+1 {
				continue
			}
			applyComparison(columns, cells, params, doc.URL)
		}
	}
}

// comparesParameters reports whether a table's rows name request parameters
// rather than properties of the output.
func comparesParameters(head string) bool {
	cells := headCellRe.FindAllStringSubmatch(head, -1)
	if len(cells) == 0 {
		return false
	}
	return strings.EqualFold(text(cells[0][1]), headParameter)
}

// applyComparison records one row of a comparison table against each column's
// models.
func applyComparison(
	columns [][]*catalog.Model,
	cells [][]string,
	params bool,
	src string,
) {
	name := comparisonLabel(cells[0][1])
	for i, models := range columns {
		for _, m := range models {
			if applyMediaField(m, name, cells[i+1][1], params) {
				m.AddSource(src)
			}
		}
	}
}

// comparisonLabel reads the name a comparison row states, which Google writes
// as a code span for a request parameter and as bold text for a property of
// the output, in both cases followed by a colon and a sentence explaining it.
// A parameter keeps the spelling the API takes, since that is what a caller
// sends, and is matched case-insensitively.
func comparisonLabel(cell string) string {
	name, _, _ := strings.Cut(text(cell), ":")
	return strings.TrimSpace(name)
}

// applyMediaField records one cell of a comparison table, reporting whether it
// said anything. A cell of the parameter comparison also says that the model
// takes the parameter its row names, which is a fact of its own and is
// recorded for the rows stating a type rather than a set of values too.
func applyMediaField(m *catalog.Model, name, cell string, params bool) bool {
	if slices.Contains(unsupported, strings.ToLower(text(cell))) {
		return false
	}
	if params {
		m.AddList(ListParameters, name)
	}
	switch strings.ToLower(name) {
	case paramAspectRatio:
		return addValues(m, ListAspectRatios, cell) || params
	case paramDurationSeconds:
		return addValues(m, ListDurationsSeconds, cell) || params
	case paramPersonGeneration:
		return addValues(m, ListPersonGeneration, cell) || params
	case paramResolution:
		return addResolutions(m, cell)
	case featureAudio:
		if strings.Contains(cell, present) {
			m.AddList(ListFeatures, featureAudioGeneration)
		}
		return true
	case featureInputs:
		addGenerationInputs(m, cell)
		return true
	case featureFrameRate:
		m.SetLimit(LimitFramesPerSecond, parseCount(text(cell)))
		return true
	case featureVideosPer:
		m.SetLimit(LimitMaxOutputVideos, largestCount(text(cell)))
		return true
	case featureStatus:
		m.SetAttr(AttrState, mediaStates[strings.ToLower(text(cell))])
		return true
	}
	return params
}

// addGenerationInputs records the inputs a comparison cell names. Google names
// each as the conversion it drives, "Image-to-Video", so the modality is the
// half before the conversion. It is worth reading beside the model page, which
// states the same models' inputs as "Text, Image" and leaves out the video a
// video-to-video model takes.
func addGenerationInputs(m *catalog.Model, cell string) {
	for _, part := range splitProse(text(cell)) {
		match := generationRe.FindStringSubmatch(strings.TrimSpace(part))
		if match == nil {
			continue
		}
		m.AddList(ListInputModalities, modalityNames[strings.ToLower(match[1])])
	}
}

// addResolutions records the output sizes a cell names. The parameter table
// quotes them and the output table writes them in prose beside a rider saying
// which durations they allow, whose own numbers are not sizes, so the rider is
// dropped before the prose is read.
func addResolutions(m *catalog.Model, cell string) bool {
	if addValues(m, ListResolutions, cell) {
		return true
	}
	added := false
	for _, size := range resolutionRe.FindAllString(
		text(asideRe.ReplaceAllString(cell, "")),
		-1,
	) {
		m.AddList(ListResolutions, strings.ToLower(size))
		added = true
	}
	return added
}

// addValues records the values a cell quotes, which is how the Veo comparison
// states what a parameter accepts. A code span the cell leaves unquoted names
// a type rather than a value, and is not one of them.
func addValues(m *catalog.Model, key, cell string) bool {
	added := false
	for _, code := range codeRe.FindAllStringSubmatch(cell, -1) {
		value := quotedValue(strings.TrimSpace(code[1]))
		if value == "" {
			continue
		}
		m.AddList(key, value)
		added = true
	}
	return added
}

// quotedValue returns what a code span quotes, or nothing where it quotes
// nothing.
func quotedValue(code string) string {
	match := quotedRe.FindStringSubmatch(code)
	if match == nil {
		return ""
	}
	if match[1] != "" {
		return match[1]
	}
	return match[2]
}

// largestCount returns the largest quantity a cell states, which is the bound
// where Google writes a choice, as the "1 or 2" videos one Veo 2 request
// returns.
func largestCount(value string) int64 {
	var largest int64
	for _, digits := range widthRe.FindAllString(value, -1) {
		if n := parseCount(digits); n > largest {
			largest = n
		}
	}
	return largest
}

// applyImageGuide reads the image generation guide's tables of the sizes each
// aspect ratio comes back in.
//
// The tables are headed by the model rather than labelled with its endpoint,
// and one heading names an endpoint that does not exist, so a heading is used
// only where it resolves to a model the pricing page priced. The guide is read
// as a running state of which heading is in force, the tables having nothing
// inside them that says which model they describe.
func (b *builder) applyImageGuide(doc catalog.Document) {
	var (
		current *catalog.Model
		sizes   []string
	)
	for _, at := range marks(string(doc.Body)) {
		if at.heading != "" {
			current, sizes = b.headed(at.heading), nil
			continue
		}
		if current == nil {
			continue
		}
		cells := rowCells(at.row)
		if len(cells) < 2 {
			continue
		}
		if named := imageSizes(cells); named != nil {
			sizes = named
			continue
		}
		if !ratioRe.MatchString(cells[0]) {
			continue
		}
		current.AddSource(doc.URL)
		current.AddList(ListAspectRatios, cells[0])
		current.AddList(ListImageSizes, sizes...)
	}
}

// headed returns the model a guide heading names, which is nothing for the
// headings that name a section rather than a model. Google drops the family
// from these headings, writing "3.1 Flash Image" for gemini-3.1-flash-image,
// so the family is put back before the heading is looked up.
func (b *builder) headed(heading string) *catalog.Model {
	slug := slugID(heading)
	if m, ok := b.models[slug]; ok {
		return m
	}
	return b.models["gemini-"+slug]
}

// imageSizes reads the sizes a table's heading row names, and reports nothing
// for a row that is not one.
func imageSizes(cells []string) []string {
	var out []string
	for _, cell := range cells[1:] {
		if match := sizeHeadRe.FindStringSubmatch(
			strings.TrimSpace(cell),
		); match != nil {
			out = append(out, match[1])
		}
	}
	return out
}

// imagenParamRe matches the name of one parameter the Imagen guide states,
// which it writes as a bare code span opening a list item. A span the guide
// quotes is a value of the parameter above it rather than a parameter.
var imagenParamRe = regexp.MustCompile(
	`(?is)<li>(?:<p>)?\s*<code[^>]*>([a-zA-Z]+)</code>`,
)

// imagenSizeRe matches an image size the guide names, which it writes without
// quotes.
var imagenSizeRe = regexp.MustCompile(`(?i)^\d+k$`)

// applyImagenGuide reads the Imagen guide, which states in prose what the Veo
// guide states in a table: the parameters a request may carry and the values
// each accepts.
//
// The guide describes the three Imagen endpoints together and names them in
// its own property table, so that table says which models the prose is about.
// Where a sentence restricts a value to some of them it names the quality
// levels rather than the endpoints, and the restriction is honoured: the two
// image sizes are stated for the standard and ultra models, and recording them
// against the fast one would be recording something Google did not say.
func (b *builder) applyImagenGuide(doc catalog.Document) {
	body := string(doc.Body)
	b.applyGuideTables(doc)
	ids := codesIn(tableCodes(first(tableRe, body)))
	found := imagenParamRe.FindAllStringSubmatchIndex(body, -1)
	for i, at := range found {
		end := len(body)
		if i+1 < len(found) {
			end = found[i+1][0]
		}
		b.applyImagenParam(
			ids,
			body[at[2]:at[3]],
			body[at[1]:sectionEnd(body, at[1], end)],
			doc.URL,
		)
	}
}

// applyImagenParam records one parameter of the Imagen guide against every
// endpoint the guide covers, or against the quality levels its sentence names.
func (b *builder) applyImagenParam(ids []string, name, segment, src string) {
	levels := levelsIn(segment)
	for _, id := range ids {
		m := b.models[id]
		if m == nil || !addressed(id, levels) {
			continue
		}
		m.AddSource(src)
		m.AddList(ListParameters, name)
		switch strings.ToLower(name) {
		case paramAspectRatio:
			addValues(m, ListAspectRatios, segment)
		case paramPersonGeneration:
			addValues(m, ListPersonGeneration, segment)
		case paramImageSize:
			addSizes(m, segment)
		}
	}
}

// sectionEnd bounds one parameter's prose. The last parameter of a list has no
// next parameter to end at, and the sentences restricting a value to some of
// the models are per parameter, so a segment running to the end of the
// document would take a later section's words for its own.
func sectionEnd(body string, from, next int) int {
	at := headingStartRe.FindStringIndex(body[from:])
	if at == nil || from+at[0] > next {
		return next
	}
	return from + at[0]
}

// levelsIn returns the quality levels a sentence names, which is nothing where
// it names none and the sentence therefore covers every endpoint.
func levelsIn(segment string) []string {
	var out []string
	lower := strings.ToLower(text(segment))
	for _, level := range variants {
		if strings.Contains(lower, level+" ") {
			out = append(out, level)
		}
	}
	return out
}

// addressed reports whether one endpoint is among the quality levels a
// sentence names. Google leaves the standard level out of an identifier, so an
// identifier carrying no level is the standard one.
func addressed(id string, levels []string) bool {
	if len(levels) == 0 {
		return true
	}
	level := nameVariant(id)
	if level == "" {
		level = "standard"
	}
	return slices.Contains(levels, level)
}

// addSizes records the image sizes a sentence names, which the guide writes
// unquoted where it quotes every other value.
func addSizes(m *catalog.Model, segment string) {
	for _, code := range codeRe.FindAllStringSubmatch(segment, -1) {
		size := strings.TrimSpace(text(code[1]))
		if imagenSizeRe.MatchString(size) {
			m.AddList(ListImageSizes, size)
		}
	}
}
