package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/joakimcarlsson/model-sync/catalog"
	"github.com/joakimcarlsson/model-sync/providers/openai"
)

func main() {
	out := flag.String("out", "catalog.json", "file to write the catalog to, or - for stdout")
	cache := flag.String("cache", "", "directory to cache fetched documents in")
	timeout := flag.Duration("timeout", 2*time.Minute, "overall time budget for fetching")
	flag.Parse()

	if err := run(*out, *cache, *timeout); err != nil {
		fmt.Fprintln(os.Stderr, "model-sync:", err)
		os.Exit(1)
	}
}

// run fetches and parses every source, then writes the merged catalog.
func run(out, cache string, timeout time.Duration) error {
	openaiSource := openai.New()
	openaiSource.CacheDir = cache
	sources := []catalog.Source{openaiSource}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var cat catalog.Catalog
	for _, source := range sources {
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
		cat.Add(source.ID(), source.Name(), models)
		fmt.Fprintf(os.Stderr, "model-sync: %s: %d documents, %d models\n", source.ID(), len(docs), len(models))
	}
	cat.Normalize()
	return write(out, &cat)
}

// write encodes the catalog as indented JSON.
func write(out string, cat *catalog.Catalog) error {
	file := os.Stdout
	if out != "-" {
		f, err := os.Create(out)
		if err != nil {
			return err
		}
		defer f.Close()
		file = f
	}
	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(cat)
}
