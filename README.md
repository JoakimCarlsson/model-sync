# model-sync

A machine-readable catalog of AI models and what they cost, built by reading
what each provider publishes.

It exists because the catalogs that already do this are narrow: they cover chat
models and flatten pricing to an input and an output rate. That loses whole
categories — rerankers, embeddings, transcription, speech, image and video —
and it loses the shape of real pricing, where an image costs a different amount
at each size and quality, video is billed per second per resolution, and the
same model has different rates on batch, priority and long-context requests.

Twenty-three providers, refreshed by running `make sync`.

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
      "dims": { "modality": "image", "tier": "standard" } }
  ],
  "attrs": { "api_id": "gpt-image-2" },
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

`metric`, `unit`, `amount` and `currency` are always present, and `dims` values
are always lower case, so a dimension is something to match on rather than
something to normalize first. One `(metric, unit, dims)` key carries one
amount: where a provider publishes two, the parser adds the dimension that
tells them apart, and the build refuses to publish a model where it has not.

A rate that exists without a number carries `"amount": null` and
`"variable": true`, which is what OpenRouter's routers are: the request is
billed at whatever the model it was routed to charges. Nothing in the data is
ever a negative amount or a sentinel.

`attrs`, `limits` and `lists` follow the same rule — open maps keyed by the
provider's own words, so context windows, capabilities, deprecation dates and
rate limits land somewhere without the schema growing.

Two keys are not the provider's words and hold for every model:

- **`attrs.api_id`** is the exact string to send as the model in an API
  request. It equals `id` wherever the two coincide, and does not where a
  provider addresses a model by something else: Fireworks by a path
  (`accounts/fireworks/models/…`), Bedrock by a versioned identifier
  (`amazon.nova-2-multimodal-embeddings-v1:0`), Ollama by a tag
  (`llama3:latest`). The provider's own name for it — `model_path`,
  `model_id`, `api_identifier`, `default_snapshot` — is kept alongside.
- **`limits.max_audio_output_tokens`** bounds what a speech model generates,
  where the provider states that bound in audio tokens rather than in text.
  Google's TTS models publish an 8,192 token input window and a 16,384 token
  output ceiling, and the second counts slices of sound; recorded as
  `max_output_tokens` it read as a ceiling no request could ask for.

`name` is optional. It holds the name the provider publishes, and is absent
where the provider publishes none: an Ollama library entry, an Azure meter and
a Voyage model are named by their identifier and nothing else, so deriving a
display name from the slug is the consumer's call rather than this catalog's
guess.

`kind` separates the three things a model can do with sound, because a consumer
asking for one cannot use the others: **speech** reads text out,
**transcription** writes speech down, and **audio** and **realtime** converse.

Every model records the URLs it was read from.

## What the build refuses to publish

`api.json` is written only if every model passes:

- no negative amount, and no price missing `metric`, `unit`, `amount` or
  `currency`
- no `max_output_tokens` above the model's own `context_window`
- no `(metric, unit, dims)` key holding two different amounts
- every `dims` value lower case
- every model carrying an `attrs.api_id`

A failing rule prints the provider, the model and the rule, and leaves the
previous `api.json` in place: a figure no request can ask for or a cost that
comes out negative is worse for a consumer than yesterday's file. Warnings are
printed and published; a chat model whose only rates are qualified by fine
tuning is one, since that is a gap in what the vendor states rather than a
statement that is wrong.

## Running it

```sh
make sync                    # fetch everything, rewrite data/ and api.json
go run . -provider cohere    # sync one provider, leaving the rest of the tree
make validate                # re-check the committed tree, fetching nothing
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
validate/            the rules a catalog has to pass to be published
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
