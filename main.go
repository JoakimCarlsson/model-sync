package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/joakimcarlsson/model-sync/catalog"
	"github.com/joakimcarlsson/model-sync/providers/anthropic"
	"github.com/joakimcarlsson/model-sync/providers/assemblyai"
	"github.com/joakimcarlsson/model-sync/providers/groq"
	"github.com/joakimcarlsson/model-sync/providers/openai"
	"github.com/joakimcarlsson/model-sync/providers/openrouter"
	"github.com/joakimcarlsson/model-sync/providers/together"
	"github.com/joakimcarlsson/model-sync/providers/voyage"
	"github.com/joakimcarlsson/model-sync/providers/xai"
	"github.com/joakimcarlsson/model-sync/store"
)

func main() {
	data := flag.String("data", "data", "directory holding one file per model")
	api := flag.String(
		"api",
		"api.json",
		"aggregate written for consumers, or - to skip",
	)
	cache := flag.String("cache", "", "directory to cache fetched documents in")
	timeout := flag.Duration(
		"timeout",
		2*time.Minute,
		"overall time budget for fetching",
	)
	flag.Parse()

	if err := run(*data, *api, *cache, *timeout); err != nil {
		fmt.Fprintln(os.Stderr, "model-sync:", err)
		os.Exit(1)
	}
}

// run syncs every source into the data tree, then rebuilds the aggregate from
// the tree so that providers not synced by this run survive in it.
func run(data, api, cache string, timeout time.Duration) error {
	openaiSource := openai.New()
	openaiSource.CacheDir = cache
	anthropicSource := anthropic.New()
	anthropicSource.CacheDir = cache
	xaiSource := xai.New()
	xaiSource.CacheDir = cache
	voyageSource := voyage.New()
	voyageSource.CacheDir = cache
	openrouterSource := openrouter.New()
	openrouterSource.CacheDir = cache
	togetherSource := together.New()
	togetherSource.CacheDir = cache
	assemblyaiSource := assemblyai.New()
	assemblyaiSource.CacheDir = cache
	groqSource := groq.New()
	groqSource.CacheDir = cache
	sources := []catalog.Source{
		assemblyaiSource,
		groqSource,
		openaiSource,
		anthropicSource,
		xaiSource,
		voyageSource,
		openrouterSource,
		togetherSource,
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for _, source := range sources {
		if err := sync(ctx, data, source); err != nil {
			return err
		}
	}
	if api == "-" {
		return nil
	}
	cat, err := store.Load(data)
	if err != nil {
		return err
	}
	if err := store.WriteAggregate(api, cat); err != nil {
		return err
	}
	fmt.Fprintf(
		os.Stderr,
		"model-sync: %s: %d providers, %d models\n",
		api,
		len(cat.Providers),
		cat.Count(),
	)
	return nil
}

// sync fetches and parses one source and writes its models to the tree. A
// fetch that only partly succeeded is reported and then used, because losing
// one document should not withdraw every model the others describe.
func sync(ctx context.Context, data string, source catalog.Source) error {
	docs, err := source.Fetch(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "model-sync: %s: %v\n", source.ID(), err)
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
