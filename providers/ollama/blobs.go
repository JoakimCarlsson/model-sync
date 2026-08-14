package ollama

import (
	"regexp"
	"strings"

	"github.com/joakimcarlsson/model-sync/catalog"
)

// AttrEmbeddingDimension is the width of the vector an embedding model
// returns. Every model returns one width and offers no choice of others, which
// is why it is stated here rather than enumerated.
const AttrEmbeddingDimension = "default_embedding_dimension"

// blobsPath is what the layers of a build are filed under. The model layer's
// page is where Ollama publishes the metadata the weights were built with.
const blobsPath = "/blobs/"

// modelBlobRe matches the link a build's page gives its model layer, which is
// the one layer of the several whose page carries the metadata.
var modelBlobRe = regexp.MustCompile(
	`(?is)href="(/library/[^"]+` + blobsPath + `[0-9a-f]+)"[^>]*>\s*model\s*<`,
)

// metadataRe matches one key and value of the model layer's page. The keys are
// the model's own, so the architecture prefixes most of them: the width is
// "bert.embedding_length" on one model and "qwen3.embedding_length" on the
// next, which is why the key is matched by its ending.
var metadataRe = regexp.MustCompile(
	`(?is)sm:text-black">\s*([A-Za-z0-9_.\-]+)\s*</div>\s*` +
		`<div[^>]*>\s*([^<]*?)\s*</div>`,
)

// embeddingLengthSuffix ends the key stating the width.
const embeddingLengthSuffix = ".embedding_length"

// applyBlob reads the metadata page of a build's model layer.
//
// It is fetched for the embedding models alone, because the width of the
// vector they return is the one fact about them no page of the library states
// and this page does. What else it carries the library already states, and in
// the words Ollama publishes rather than the words the weights were built
// with.
func (b *builder) applyBlob(doc catalog.Document) {
	m, ok := b.models[blobModelID(doc.URL)]
	if !ok {
		return
	}
	for _, entry := range metadataRe.FindAllStringSubmatch(
		string(doc.Body),
		-1,
	) {
		if !strings.HasSuffix(entry[1], embeddingLengthSuffix) {
			continue
		}
		m.SetAttr(AttrEmbeddingDimension, entry[2])
		m.AddSource(doc.URL)
		return
	}
}

// blobModelID names the model a layer belongs to, which its URL states as the
// build it was pulled from.
func blobModelID(url string) string {
	build, _, ok := strings.Cut(url, blobsPath)
	if !ok {
		return ""
	}
	id := build[strings.LastIndex(build, "/")+1:]
	if name, _, ok := strings.Cut(id, ":"); ok {
		return name
	}
	return id
}
