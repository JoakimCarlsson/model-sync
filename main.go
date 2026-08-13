package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/joakimcarlsson/model-sync/catalog"
	"github.com/joakimcarlsson/model-sync/providers/anthropic"
	"github.com/joakimcarlsson/model-sync/providers/assemblyai"
	"github.com/joakimcarlsson/model-sync/providers/azure"
	"github.com/joakimcarlsson/model-sync/providers/bedrock"
	"github.com/joakimcarlsson/model-sync/providers/berget"
	"github.com/joakimcarlsson/model-sync/providers/cerebras"
	"github.com/joakimcarlsson/model-sync/providers/cohere"
	"github.com/joakimcarlsson/model-sync/providers/deepgram"
	"github.com/joakimcarlsson/model-sync/providers/deepseek"
	"github.com/joakimcarlsson/model-sync/providers/elevenlabs"
	"github.com/joakimcarlsson/model-sync/providers/fireworks"
	"github.com/joakimcarlsson/model-sync/providers/google"
	"github.com/joakimcarlsson/model-sync/providers/groq"
	"github.com/joakimcarlsson/model-sync/providers/mistral"
	"github.com/joakimcarlsson/model-sync/providers/ollama"
	"github.com/joakimcarlsson/model-sync/providers/openai"
	"github.com/joakimcarlsson/model-sync/providers/openrouter"
	"github.com/joakimcarlsson/model-sync/providers/perplexity"
	"github.com/joakimcarlsson/model-sync/providers/together"
	"github.com/joakimcarlsson/model-sync/providers/vertexai"
	"github.com/joakimcarlsson/model-sync/providers/voyage"
	"github.com/joakimcarlsson/model-sync/providers/xai"
	"github.com/joakimcarlsson/model-sync/store"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "coverage" {
		if err := coverage(os.Args[2:], os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "model-sync:", err)
			os.Exit(1)
		}
		return
	}
	data := flag.String("data", "data", "directory holding one file per model")
	api := flag.String(
		"api",
		"api.json",
		"aggregate written for consumers, or - to skip",
	)
	cache := flag.String("cache", "", "directory to cache fetched documents in")
	only := flag.String(
		"provider",
		"",
		"sync only this provider, or none to rebuild the aggregate alone",
	)
	timeout := flag.Duration(
		"timeout",
		30*time.Minute,
		"overall time budget for fetching",
	)
	flag.Parse()

	if err := run(*data, *api, *cache, *only, *timeout); err != nil {
		fmt.Fprintln(os.Stderr, "model-sync:", err)
		os.Exit(1)
	}
}

// run syncs every source into the data tree, then rebuilds the aggregate from
// the tree so that providers not synced by this run survive in it.
//
// A named provider syncs alone, and naming one that does not exist syncs
// nothing and rebuilds the aggregate. That the aggregate comes from the tree is
// what makes both useful: reviewing a change to one parser means reading the
// diff of that parser's models, and a run that refetched the other twenty-one
// would bury it under whatever those vendors had changed in the meantime.
//
// The time budget covers the whole run, and a full run fetches something over
// six hundred documents: Ollama alone reads a tag listing for each of its 233
// models, OpenAI a page per model, and Vertex and Mistral likewise. Two minutes
// could not finish one, which is why the default is half an hour. Syncing a
// single provider needs a small fraction of it.
//
// A source that fails is reported and the rest still sync. One vendor moving a
// page must not stop the other twenty-one from refreshing, and a source that
// parsed nothing writes nothing, so its files stay as they were and the
// aggregate is rebuilt with its previous models still in it. The failures are
// returned at the end, so a run that lost a provider still exits non-zero rather
// than passing quietly with stale data.
func run(data, api, cache, only string, timeout time.Duration) error {
	mistralSource := mistral.New()
	mistralSource.CacheDir = cache
	ollamaSource := ollama.New()
	ollamaSource.CacheDir = cache
	openaiSource := openai.New()
	openaiSource.CacheDir = cache
	anthropicSource := anthropic.New()
	anthropicSource.CacheDir = cache
	xaiSource := xai.New()
	xaiSource.CacheDir = cache
	vertexaiSource := vertexai.New()
	vertexaiSource.CacheDir = cache
	voyageSource := voyage.New()
	voyageSource.CacheDir = cache
	openrouterSource := openrouter.New()
	openrouterSource.CacheDir = cache
	togetherSource := together.New()
	togetherSource.CacheDir = cache
	assemblyaiSource := assemblyai.New()
	assemblyaiSource.CacheDir = cache
	perplexitySource := perplexity.New()
	perplexitySource.CacheDir = cache
	azureSource := azure.New()
	azureSource.CacheDir = cache
	bedrockSource := bedrock.New()
	bedrockSource.CacheDir = cache
	bergetSource := berget.New()
	bergetSource.CacheDir = cache
	cerebrasSource := cerebras.New()
	cerebrasSource.CacheDir = cache
	cohereSource := cohere.New()
	cohereSource.CacheDir = cache
	deepgramSource := deepgram.New()
	deepgramSource.CacheDir = cache
	deepseekSource := deepseek.New()
	deepseekSource.CacheDir = cache
	elevenlabsSource := elevenlabs.New()
	elevenlabsSource.CacheDir = cache
	fireworksSource := fireworks.New()
	fireworksSource.CacheDir = cache
	googleSource := google.New()
	googleSource.CacheDir = cache
	groqSource := groq.New()
	groqSource.CacheDir = cache
	sources := []catalog.Source{
		assemblyaiSource,
		azureSource,
		bedrockSource,
		bergetSource,
		cerebrasSource,
		cohereSource,
		deepgramSource,
		deepseekSource,
		elevenlabsSource,
		fireworksSource,
		googleSource,
		groqSource,
		perplexitySource,
		mistralSource,
		ollamaSource,
		openaiSource,
		anthropicSource,
		xaiSource,
		vertexaiSource,
		voyageSource,
		openrouterSource,
		togetherSource,
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var failures []error
	for _, source := range sources {
		if only != "" && source.ID() != only {
			continue
		}
		if err := sync(ctx, data, source); err != nil {
			fmt.Fprintf(os.Stderr, "model-sync: %v\n", err)
			failures = append(failures, err)
		}
	}
	if api == "-" {
		return errors.Join(failures...)
	}
	cat, err := store.Load(data)
	if err != nil {
		return errors.Join(append(failures, err)...)
	}
	if err := store.WriteAggregate(api, cat); err != nil {
		return errors.Join(append(failures, err)...)
	}
	fmt.Fprintf(
		os.Stderr,
		"model-sync: %s: %d providers, %d models\n",
		api,
		len(cat.Providers),
		cat.Count(),
	)
	return errors.Join(failures...)
}

// sync fetches and parses one source and writes its models to the tree. A
// fetch that only partly succeeded is reported and then used, because losing
// one document should not withdraw every model the others describe.
//
// A fetch cut short by the time budget is the exception, and it does not write.
// Tolerating a lost document assumes the vendor stopped publishing it; a expired
// deadline means every document still to come will fail too, so what would be
// written is not what the vendor publishes but however much of it the clock
// allowed. A model whose own page timed out would keep its rate and lose the
// context window, the capabilities and the modalities that page states, and that
// overwrites good data with worse.
func sync(ctx context.Context, data string, source catalog.Source) error {
	docs, err := source.Fetch(ctx)
	if errors.Is(err, catalog.ErrUnconfigured) {
		fmt.Fprintf(
			os.Stderr,
			"model-sync: %s: skipped, %v\n",
			source.ID(),
			err,
		)
		return nil
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "model-sync: %s: %v\n", source.ID(), err)
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Errorf(
			"%s: out of time, nothing written: %w",
			source.ID(),
			ctxErr,
		)
	}
	if len(docs) == 0 {
		return fmt.Errorf("%s: no documents fetched", source.ID())
	}
	models, err := source.Parse(docs)
	if err != nil {
		return fmt.Errorf("%s: %w", source.ID(), err)
	}
	if len(models) == 0 {
		return fmt.Errorf(
			"%s: no models parsed from %d documents",
			source.ID(),
			len(docs),
		)
	}
	provider := catalog.Provider{
		ID:     source.ID(),
		Name:   source.Name(),
		Models: models,
	}
	if err := store.WriteProvider(data, provider); err != nil {
		return fmt.Errorf("%s: %w", source.ID(), err)
	}
	fmt.Fprintf(
		os.Stderr,
		"model-sync: %s: %d documents, %d models\n",
		source.ID(),
		len(docs),
		len(models),
	)
	return nil
}
