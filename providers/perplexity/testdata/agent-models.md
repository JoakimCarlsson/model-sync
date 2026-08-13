> ## Documentation Index
> Fetch the complete documentation index at: https://docs.perplexity.ai/llms.txt
> Use this file to discover all available pages before exploring further.

# Agent API Models

> Compare third-party and Perplexity models available through the Agent API, including token pricing and provider documentation.

## Available Models

The Agent API supports direct access to models from multiple providers. All models are accessed directly from first-party providers with transparent token-based pricing.

Pricing rates are updated monthly and **reflect direct first-party provider pricing with no markup**. All charges are based on actual token consumption, and every API response includes exact token counts so you know your costs per request.

<Tip>
  Looking for pre-configured model setups? See [**Presets**](/docs/agent-api/presets) — optimized for specific use cases.
</Tip>

<Tabs>
  <Tab title="Anthropic">
    <Card title="Anthropic">
      Claude Opus (highest reasoning), Sonnet (balanced), and Haiku (fastest, cheapest).
    </Card>

    | Model                         | Input (\$/1M) | Output (\$/1M) | Cache (\$/1M) | Docs                                                                                                                     |
    | ----------------------------- | ------------- | -------------- | ------------- | ------------------------------------------------------------------------------------------------------------------------ |
    | `anthropic/claude-opus-5`     | 5             | 25             | 0.50          | [Claude Opus 5](https://platform.claude.com/docs/en/about-claude/models/overview)                                        |
    | `anthropic/claude-opus-4-8`   | 5             | 25             | 0.50          | [Claude Opus 4.8](https://platform.claude.com/docs/en/about-claude/models/overview)                                      |
    | `anthropic/claude-opus-4-7`   | 5             | 25             | 0.50          | [Claude Opus 4.7](https://www.anthropic.com/news/claude-opus-4-7)                                                        |
    | `anthropic/claude-opus-4-6`   | 5             | 25             | 0.50          | [Claude Opus 4.6](https://www.anthropic.com/news/claude-opus-4-6)                                                        |
    | `anthropic/claude-opus-4-5`   | 5             | 25             | 0.50          | [Claude Opus 4.5](https://www.anthropic.com/news/claude-opus-4-5)                                                        |
    | `anthropic/claude-sonnet-5`   | 2             | 10             | 0.20          | [Claude Sonnet 5](https://platform.claude.com/docs/en/about-claude/models/whats-new-sonnet-5)                            |
    | `anthropic/claude-fable-5`    | 10            | 50             | 1.00          | [Claude Fable 5](https://platform.claude.com/docs/en/about-claude/models/introducing-claude-fable-5-and-claude-mythos-5) |
    | `anthropic/claude-sonnet-4-6` | 3             | 15             | 0.30          | [Claude Sonnet 4.6](https://www.anthropic.com/news/claude-sonnet-4-6)                                                    |
    | `anthropic/claude-sonnet-4-5` | 3             | 15             | 0.30          | [Claude Sonnet 4.5](https://www.anthropic.com/news/claude-sonnet-4-5)                                                    |
    | `anthropic/claude-haiku-4-5`  | 1             | 5              | 0.10          | [Claude Haiku 4.5](https://www.anthropic.com/news/claude-haiku-4-5)                                                      |

    <Warning>
      Requests that use an `anthropic/*` model must include `max_output_tokens`. If omitted, the API returns HTTP 400 with `validation failed: max_output_tokens is required when using Anthropic models`. `max_output_tokens` is a shared Agent API parameter, but this required condition applies only to Anthropic models.
    </Warning>
  </Tab>

  <Tab title="OpenAI">
    <Card title="OpenAI">
      GPT-5 family — flagship, mini, and nano variants.
    </Card>

    | Model                  | Input (\$/1M)                   | Output (\$/1M)                   | Cache (\$/1M) | Docs                                                                 |
    | ---------------------- | ------------------------------- | -------------------------------- | ------------- | -------------------------------------------------------------------- |
    | `openai/gpt-5.6-sol`   | 5.00 (≤272k)<br />10.00 (>272k) | 30.00 (≤272k)<br />45.00 (>272k) | 90% off input | [GPT-5.6](https://openai.com/index/gpt-5-6/)                         |
    | `openai/gpt-5.6-terra` | 2.00 (≤272k)<br />4.00 (>272k)  | 12.00 (≤272k)<br />18.00 (>272k) | 90% off input | [GPT-5.6](https://openai.com/index/gpt-5-6/)                         |
    | `openai/gpt-5.6-luna`  | 0.20 (≤272k)<br />0.40 (>272k)  | 1.20 (≤272k)<br />1.80 (>272k)   | 90% off input | [GPT-5.6](https://openai.com/index/gpt-5-6/)                         |
    | `openai/gpt-5.5`       | 5.00 (≤272k)<br />10.00 (>272k) | 30.00 (≤272k)<br />45.00 (>272k) | 0.50          | [GPT-5.5](https://developers.openai.com/api/docs/models/gpt-5.5)     |
    | `openai/gpt-5.4`       | 2.50 (≤272k)<br />5.00 (>272k)  | 15.00 (≤272k)<br />22.50 (>272k) | 0.25          | [GPT-5.4](https://platform.openai.com/docs/models/gpt-5.4)           |
    | `openai/gpt-5.4-mini`  | 0.75                            | 4.50                             | 0.075         | [GPT-5.4 Mini](https://platform.openai.com/docs/models/gpt-5.4-mini) |
    | `openai/gpt-5.4-nano`  | 0.20                            | 1.25                             | 0.02          | [GPT-5.4 Nano](https://platform.openai.com/docs/models/gpt-5.4-nano) |
    | `openai/gpt-5.2`       | 1.75                            | 14                               | 0.175         | [GPT-5.2](https://platform.openai.com/docs/models/gpt-5.2)           |
    | `openai/gpt-5.1`       | 1.25                            | 10                               | 0.125         | [GPT-5.1](https://platform.openai.com/docs/models/gpt-5.1)           |
    | `openai/gpt-5`         | 1.25                            | 10                               | 0.125         | [GPT-5](https://platform.openai.com/docs/models/gpt-5)               |
    | `openai/gpt-5-mini`    | 0.25                            | 2                                | 0.025         | [GPT-5 Mini](https://platform.openai.com/docs/models/gpt-5-mini)     |
  </Tab>

  <Tab title="Google">
    <Card title="Google">
      Gemini 3 family — Pro for long-context, Flash and Flash Lite for speed.
    </Card>

    | Model                           | Input (\$/1M)                  | Output (\$/1M)                   | Cache (\$/1M) | Docs                                                                                        |
    | ------------------------------- | ------------------------------ | -------------------------------- | ------------- | ------------------------------------------------------------------------------------------- |
    | `google/gemini-3.1-pro-preview` | 2.00 (≤200k)<br />4.00 (>200k) | 12.00 (≤200k)<br />18.00 (>200k) | 90% off input | [Gemini 3.1 Pro](https://ai.google.dev/gemini-api/docs/models#gemini-3.1-pro-preview)       |
    | `google/gemini-3.1-flash-lite`  | 0.25                           | 1.50                             | 90% off input | [Gemini 3.1 Flash Lite](https://ai.google.dev/gemini-api/docs/models/gemini-3.1-flash-lite) |
    | `google/gemini-3.5-flash`       | 1.50                           | 9.00                             | 0.15          | [Gemini 3.5 Flash](https://ai.google.dev/gemini-api/docs/models/gemini-3.5-flash)           |
    | `google/gemini-3.5-flash-lite`  | 0.30                           | 2.50                             | 0.03          | [Gemini 3.5 Flash Lite](https://ai.google.dev/gemini-api/docs/models/gemini-3.5-flash-lite) |
    | `google/gemini-3.6-flash`       | 1.50                           | 7.50                             | 0.15          | [Gemini 3.6 Flash](https://ai.google.dev/gemini-api/docs/models/gemini-3.6-flash)           |
    | `google/gemini-3-flash-preview` | 0.50                           | 3.00                             | 90% off input | [Gemini 3.0 Flash](https://ai.google.dev/gemini-api/docs/models#gemini-3-flash-preview)     |
  </Tab>

  <Tab title="xAI">
    <Card title="xAI">
      Grok 4.6, 4.5, 4.3, and 4.20 variants: flagship, reasoning, non-reasoning, and multi-agent.
    </Card>

    | Model                         | Input (\$/1M)                  | Output (\$/1M)                  | Cache (\$/1M)                  | Docs                                                           |
    | ----------------------------- | ------------------------------ | ------------------------------- | ------------------------------ | -------------------------------------------------------------- |
    | `xai/grok-4.6`                | 2.00 (≤200k)<br />4.00 (>200k) | 6.00 (≤200k)<br />12.00 (>200k) | 0.50                           | [Grok 4.6](https://docs.x.ai/developers/models/grok-4.6)       |
    | `xai/grok-4.5`                | 2.00 (≤200k)<br />4.00 (>200k) | 6.00 (≤200k)<br />12.00 (>200k) | 0.30 (≤200k)<br />0.60 (>200k) | [Grok 4.5](https://docs.x.ai/developers/models)                |
    | `xai/grok-4.3`                | 1.25 (≤200k)<br />2.50 (>200k) | 2.50 (≤200k)<br />5.00 (>200k)  | 0.20                           | [Grok 4.3](https://docs.x.ai/developers/models)                |
    | `xai/grok-4.20-reasoning`     | 1.25 (≤200k)<br />2.50 (>200k) | 2.50 (≤200k)<br />5.00 (>200k)  | 0.20                           | [Grok 4.20 Reasoning](https://docs.x.ai/developers/models)     |
    | `xai/grok-4.20-non-reasoning` | 1.25 (≤200k)<br />2.50 (>200k) | 2.50 (≤200k)<br />5.00 (>200k)  | 0.20                           | [Grok 4.20 Non Reasoning](https://docs.x.ai/developers/models) |
    | `xai/grok-4.20-multi-agent`   | 1.25 (≤200k)<br />2.50 (>200k) | 2.50 (≤200k)<br />5.00 (>200k)  | 0.20                           | [Grok 4.20 Multi-Agent](https://docs.x.ai/developers/models)   |
  </Tab>

  <Tab title="DeepSeek">
    <Card title="DeepSeek">
      DeepSeek V4 Flash 0731 — a fast, efficient open reasoning model with a 1M-token context window.
    </Card>

    | Model                               | Input (\$/1M) | Output (\$/1M) | Cache (\$/1M) | Docs                                           |
    | ----------------------------------- | ------------- | -------------- | ------------- | ---------------------------------------------- |
    | `perplexity/deepseek-v4-flash-0731` | 0.13          | 0.26           | 0.028         | [DeepSeek](https://huggingface.co/deepseek-ai) |
  </Tab>

  <Tab title="Z.AI">
    <Card title="Z.AI">
      GLM 5.2 — Z.AI's flagship reasoning model.
    </Card>

    | Model                | Input (\$/1M) | Output (\$/1M) | Cache (\$/1M) | Docs                     |
    | -------------------- | ------------- | -------------- | ------------- | ------------------------ |
    | `perplexity/glm-5.2` | 1.40          | 4.40           | 0.26          | [GLM](https://docs.z.ai) |
  </Tab>

  <Tab title="Moonshot AI">
    <Card title="Moonshot AI">
      Kimi K3 — Moonshot AI's flagship reasoning model — and Kimi K2.7 Code for coding and agentic tasks.
    </Card>

    | Model                       | Input (\$/1M) | Output (\$/1M) | Cache (\$/1M) | Docs                                                 |
    | --------------------------- | ------------- | -------------- | ------------- | ---------------------------------------------------- |
    | `perplexity/kimi-k3`        | 3.00          | 15.00          | 0.30          | [Kimi K3](https://huggingface.co/moonshotai/Kimi-K3) |
    | `perplexity/kimi-k2.7-code` | 0.95          | 4.00           | 0.19          | [Kimi K2](https://platform.moonshot.ai/docs)         |

    <Info>
      Kimi K3 accepts `minimal`, `low`, `medium`, `high`, `xhigh`, and `max` reasoning effort. `minimal` uses low effort, while `xhigh` and `max` use maximum effort. Reasoning tokens are billed at the output-token rate.
    </Info>
  </Tab>

  <Tab title="NVIDIA">
    <Card title="NVIDIA">
      Nemotron open-weight reasoning models.
    </Card>

    | Model                                       | Input (\$/1M) | Output (\$/1M) | Cache (\$/1M) | Docs                                                                                                |
    | ------------------------------------------- | ------------- | -------------- | ------------- | --------------------------------------------------------------------------------------------------- |
    | `perplexity/nemotron-3.5-lightning-30b-a3b` | 0.0115        | 0.17           | 0.00115       | [Nemotron 3.5 Lightning](https://huggingface.co/nvidia/NVIDIA-Nemotron-3.5-Lightning-30B-A3B-NVFP4) |
    | `perplexity/nemotron-3-ultra-550b-a55b`     | 0.25          | 2.50           | 0.25          | [Nemotron 3 Ultra](https://huggingface.co/nvidia/NVIDIA-Nemotron-3-Ultra-550B-A55B-BF16)            |
  </Tab>

  <Tab title="Perplexity">
    <Card title="Perplexity">
      Sonar — Perplexity's grounded search model.
    </Card>

    | Model              | Input (\$/1M) | Output (\$/1M) | Cache (\$/1M) | Docs                                                        |
    | ------------------ | ------------- | -------------- | ------------- | ----------------------------------------------------------- |
    | `perplexity/sonar` | 0.25          | 2.50           | 0.0625        | [Sonar](https://docs.perplexity.ai/docs/sonar/models/sonar) |
  </Tab>
</Tabs>

GPT-5.6 Sol supports Fast mode at 2× the listed token prices. Set `service_tier` to `priority`; the response includes the processing tier that served the request.

<Warning>
  Not all third-party models support all features (e.g., reasoning, tools). Check model documentation for specific capabilities.
</Warning>

## Estimate your cost

<PricingCalculator product="agent" data={PRICING} />

## Using a Model

<CodeGroup>

</CodeGroup>

<Accordion title="Response">
</Accordion>

<Tip>
  **See Your Costs in Real-Time:** Every response includes a `usage` field with exact input tokens, output tokens, and cache read tokens. Calculate your cost instantly using the pricing table above.

</Tip>

## Model Fallback

For high-availability applications, you can specify multiple models in a fallback chain. When one model fails or is unavailable, the API automatically tries the next model in the chain.

<Card title="Model Fallback Chain" icon="square-rounded-arrow-down" href="/docs/agent-api/model-fallback">
  Learn how to use model fallback chains to ensure high availability and reliability by automatically trying multiple models when one fails.
</Card>

<Info>
  **Example:**

  For detailed examples, pricing information, and best practices, see the [Model Fallback documentation](/docs/agent-api/model-fallback).
</Info>

## Next Steps

<CardGroup cols={2}>
  <Card title="Web Search" icon="screwdriver-wrench" href="/docs/agent-api/tools/web-search">
    Equip your model with web search for source-grounded context.
  </Card>

  <Card title="Prompt Guide" icon="lightbulb" href="/docs/agent-api/prompt-guide">
    Write prompts that get the most out of the Agent API.
  </Card>

  <Card title="Output Control" icon="wand-magic-sparkles" href="/docs/agent-api/output-control">
    Shape responses with structured outputs and JSON schemas.
  </Card>

  <Card title="Finance Search" icon="chart-line" href="/docs/agent-api/tools/finance-search">
    Query market data, filings, and ticker-level information.
  </Card>
</CardGroup>
