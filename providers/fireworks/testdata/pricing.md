> ## Documentation Index
> Fetch the complete documentation index at: https://docs.fireworks.ai/llms.txt
> Use this file to discover all available pages before exploring further.

# Serverless Pricing

> Per-token serverless pricing for text, vision, and embedding models, including Priority and Fast serving paths

## Overview

Serverless inference is priced per token. For how Standard, Priority, and Fast serving paths work and how to select one, see [Serverless Serving Paths](/serverless/serving-paths).

Every text or vision request is billed across three dimensions:

* **Input tokens** — what you send to the model.
* **Cached input tokens** — input tokens served from [prompt cache](/guides/prompt-caching), priced lower.
* **Output tokens** — what the model generates.

Embeddings are billed only on input tokens.

## How pricing works

* Prices below are **per 1 million tokens** in US dollars.
* **Batch inference** is billed at **50% of serverless pricing** on both input and output. See [Batch inference](/guides/batch-inference).

## Text and vision models

Per-model pricing for headline models. Fast variants appear as adjacent rows. In each **Standard** or **Priority** cell, prices are **input / cached input / output** (USD per 1M tokens), in that order.

| Model                                                                                                 | Standard                  | Priority                      |
| ----------------------------------------------------------------------------------------------------- | ------------------------- | ----------------------------- |
| [Kimi K3](https://app.fireworks.ai/models/fireworks/kimi-k3)                                          | \$3.00 / \$0.30 / \$15.00 | \$3.75 / \$0.375 / \$18.75    |
| [Kimi K3 Fast](https://app.fireworks.ai/models/fireworks/kimi-k3)                                     | \$4.50 / \$0.45 / \$22.50 | —                             |
| [Kimi K3 US](https://app.fireworks.ai/models/fireworks/kimi-k3)                                       | \$3.30 / \$0.33 / \$16.50 | \$4.125 / \$0.4125 / \$20.625 |
| [Kimi K2.7 Code](https://app.fireworks.ai/models/fireworks/kimi-k2p7-code)                            | \$0.95 / \$0.19 / \$4.00  | \$1.425 / \$0.285 / \$6.00    |
| [Kimi K2.7 Code Fast](https://app.fireworks.ai/models/fireworks/kimi-k2p7-code)                       | \$1.90 / \$0.38 / \$8.00  | —                             |
| [Kimi K2.6](https://app.fireworks.ai/models/fireworks/kimi-k2p6)                                      | \$0.95 / \$0.16 / \$4.00  | \$1.50 / \$0.22 / \$6.00      |
| [Kimi K2.6 Fast](https://app.fireworks.ai/models/fireworks/kimi-k2p6)                                 | \$2.00 / \$0.30 / \$8.00  | —                             |
| [DeepSeek V4 Pro](https://app.fireworks.ai/models/fireworks/deepseek-v4-pro)                          | \$1.74 / \$0.145 / \$3.48 | \$2.61 / \$0.218 / \$5.22     |
| [DeepSeek V4 Flash](https://app.fireworks.ai/models/fireworks/deepseek-v4-flash)                      | \$0.14 / \$0.028 / \$0.28 | \$0.21 / \$0.042 / \$0.42     |
| [DeepSeek V4 Flash (0731)](https://app.fireworks.ai/models/fireworks/deepseek-v4-flash-0731)          | \$0.14 / \$0.028 / \$0.28 | \$0.21 / \$0.042 / \$0.42     |
| [GLM 5.2](https://app.fireworks.ai/models/fireworks/glm-5p2)                                          | \$1.40 / \$0.14 / \$4.40  | \$1.75 / \$0.18 / \$5.50      |
| [GLM 5.2 Fast](https://app.fireworks.ai/models/fireworks/glm-5p2)                                     | \$2.10 / \$0.21 / \$6.60  | —                             |
| [GLM 5.2 Fast US](https://app.fireworks.ai/models/fireworks/glm-5p2)                                  | \$2.10 / \$0.21 / \$6.60  | —                             |
| [GLM 5.1](https://app.fireworks.ai/models/fireworks/glm-5p1)                                          | \$1.40 / \$0.26 / \$4.40  | \$2.10 / \$0.39 / \$6.60      |
| [GLM 5.1 Fast](https://app.fireworks.ai/models/fireworks/glm-5p1)                                     | \$2.80 / \$0.52 / \$8.80  | —                             |
| [Qwen 3.7 Plus](https://app.fireworks.ai/models/fireworks/qwen3p7-plus)                               | \$0.40 / \$0.08 / \$1.60  | —                             |
| [MiniMax M3](https://app.fireworks.ai/models/fireworks/minimax-m3)                                    | \$0.30 / \$0.06 / \$1.20  | \$0.45 / \$0.09 / \$1.80      |
| [MiniMax M2.7](https://app.fireworks.ai/models/fireworks/minimax-m2p7)                                | \$0.30 / \$0.06 / \$1.20  | \$0.45 / \$0.09 / \$1.80      |
| [OpenAI GPT OSS 120B](https://app.fireworks.ai/models/fireworks/gpt-oss-120b)                         | \$0.15 / \$0.015 / \$0.60 | \$0.18 / \$0.018 / \$0.72     |
| [OpenAI GPT OSS 20B](https://app.fireworks.ai/models/fireworks/gpt-oss-20b)                           | \$0.07 / \$0.035 / \$0.30 | —                             |
| [NVIDIA Nemotron 3 Ultra (Preview)](https://app.fireworks.ai/models/fireworks/nemotron-3-ultra-nvfp4) | \$0.60 / \$0.12 / \$2.40  | —                             |

**—** in the Priority column means Priority is not available for that model. This pricing table is the source of truth for Priority availability.

## Other base models — by size and architecture

For any text or vision model not listed individually, pricing is set by parameter count and architecture. These size-based prices apply uniformly to input and output (no separate cached-input rate):

| Model                                                  | \$ / 1M tokens |
| ------------------------------------------------------ | -------------- |
| Less than 4B parameters                                | \$0.10         |
| 4B – 16B parameters                                    | \$0.20         |
| More than 16B parameters                               | \$0.90         |
| MoE up to 56B parameters (e.g. Mixtral 8x7B)           | \$0.50         |
| MoE 56.1B – 176B parameters (e.g. DBRX, Mixtral 8x22B) | \$1.20         |

## Embeddings

Embeddings are billed per 1M input tokens.

| Base model parameter count | \$ / 1M input tokens |
| -------------------------- | -------------------- |
| up to 150M                 | \$0.008              |
| 150M – 350M                | \$0.016              |
| Qwen3 8B                   | \$0.10               |

## Notes

* [US-only Serverless](/serverless/us-only-serverless) endpoints are priced at a 10% premium to the base model serverless prices, except GLM 5.2 Fast US, which matches global GLM 5.2 Fast pricing.
* For account-level controls (spend tiers, monthly spend limits, on-demand GPU quotas), see [Account quotas](/guides/quotas_usage/account-quotas).
