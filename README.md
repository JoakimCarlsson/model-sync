# model-sync

A machine-readable catalog of AI models and what they cost, built by reading
what each provider publishes.

It exists because the catalogs that already do this are narrow: they cover chat
models and flatten pricing to an input and an output rate. That loses whole
categories — rerankers, embeddings, transcription, speech, image and video —
and it loses the shape of real pricing, where an image costs a different amount
at each size and quality, video is billed per second per resolution, and the
same model has different rates on batch, priority and long-context requests.

Twenty providers, refreshed by running `make sync`.

## The data

One file per model under `data/<provider>/models/<id>.json`, and the whole
catalog in `api.json`.

```json
{
  "id": "gpt-image-2",
  "provider": "openai",
  "kind": "image",
  "prices": [
    { "metric": "image_output", "unit": "per_image", "amount": 0.006,
      "currency": "USD",
      "dims": { "quality": "low", "size": "1024x1024", "tier": "standard" } },
    { "metric": "input_tokens", "unit": "per_1m_tokens", "amount": 8,
      "currency": "USD",
      "dims": { "modality": "Image", "tier": "standard" } }
  ],
  "limits": { "context_window": 1050000 },
  "lists": { "features": ["function_calling", "streaming"] },
  "sources": ["https://developers.openai.com/api/docs/pricing.md"]
}
```

A price is a tuple — **metric, unit, amount, dims** — rather than a named
field. That is what lets one schema hold a per-token chat rate, a per-second
video rate at 1080p, a per-GB-month storage fee, and a rate that only applies
above 200k prompt tokens. `dims` says when a rate applies; without it the model
would need a new field for every provider's idea of a discount.

`attrs`, `limits` and `lists` follow the same rule — open maps keyed by the
provider's own words, so context windows, capabilities, deprecation dates and
rate limits land somewhere without the schema growing.

Every model records the URLs it was read from.

## Running it

```sh
make sync                    # fetch everything, rewrite data/ and api.json
go run . -provider cohere    # sync one provider, leaving the rest of the tree
make fmt
make lint
```

A full sync fetches something over six hundred documents, since several
providers publish a page per model, and takes a good few minutes; `-timeout`
bounds the whole run and defaults to thirty minutes. `-provider` syncs one and
still rebuilds `api.json` from the whole tree, which is what keeps a change to
one parser reviewable as its own diff.

A source that fails is reported and the rest still sync, and the run exits
non-zero. A source that fetched nothing, parsed nothing, or ran out of time
writes nothing, so its existing files stay as they are: a vendor moving a page
costs that provider a refresh rather than corrupting it.

The generated data is committed. Output is byte-identical for unchanged input,
so a sync produces a diff only where a provider actually changed something.

## Layout

```
catalog/             the types. shared by everything, owned by nobody.
providers/<name>/    one package per provider
store/               reads and writes the data tree
main.go              wires providers together
```

## Adding a provider

Write a package under `providers/` that satisfies `catalog.Source`:

```go
type Source interface {
    ID() string
    Name() string
    Fetch(ctx context.Context) ([]Document, error)
    Parse(docs []Document) ([]Model, error)
}
```

Then add it to the list in `main.go`.

The rule that matters: **`catalog` holds no vendor vocabulary.** `Metric`,
`Unit` and `Kind` are bare string types with no constants declared in them.
Every value — `per_gb_month`, `per_megapixel`, `cache_write_tokens` — is
declared in the package that uses it. Providers share no parsing code either,
because no two of these documents have the same shape.

If adding a provider requires editing `catalog`, something is wrong with the
change.

Each provider's `doc.go` describes what that provider publishes and the
peculiarities of reading it.
