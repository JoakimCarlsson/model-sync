> ## Documentation Index
> Fetch the complete documentation index at: https://docs.together.ai/llms.txt
> Use this file to discover all available pages before exploring further.

# Reasoning

> Use reasoning models that think step-by-step before answering.

Reasoning models are trained to think step-by-step before responding with an answer. Given an input prompt, they first produce a chain of thought, visible as tokens in the `reasoning` output field, and then output a final answer in the `content` field.

## Supported models

Reasoning models fall into a few behavioral types:

* **Reasoning only:** Always produces reasoning tokens. Cannot be toggled off.
* **Hybrid:** Supports both reasoning and non-reasoning modes via `reasoning={"enabled": True/False}`.
* **Adjustable effort:** Supports the `reasoning_effort` parameter to control reasoning depth (`"low"`, `"medium"`, or `"high"`).

The following models support reasoning on [serverless inference](/docs/serverless/models):

| Model                      | API string                          | Type                   | Context length |
| :------------------------- | :---------------------------------- | :--------------------- | :------------- |
| MiniMax M3                 | `MiniMaxAI/MiniMax-M3`              | Hybrid (on by default) | 512K           |
| DeepSeek-V4-Pro            | `deepseek-ai/DeepSeek-V4-Pro`       | Hybrid (on by default) | 512K           |
| GLM-5                      | `zai-org/GLM-5`                     | Hybrid (on by default) | 200K           |
| Kimi K2.6                  | `moonshotai/Kimi-K2.6`              | Hybrid (on by default) | 262K           |
| Qwen3.6 Plus               | `Qwen/Qwen3.6-Plus`                 | Hybrid (on by default) | 1M             |
| Qwen3.5 9B                 | `Qwen/Qwen3.5-9B`                   | Hybrid (on by default) | 262K           |
| Cogito v2.1 671B           | `deepcogito/cogito-v2-1-671b`       | Hybrid (on by default) | 164K           |
| Nemotron 3 Ultra 550B A55B | `nvidia/nemotron-3-ultra-550b-a55b` | Hybrid (on by default) | 512K           |
| GPT-OSS 120B               | `openai/gpt-oss-120b`               | Adjustable effort      | 128K           |
| GPT-OSS 20B                | `openai/gpt-oss-20b`                | Adjustable effort      | 128K           |

Additional reasoning models, including DeepSeek-R1 and its distillations, Qwen QwQ-32B, and DeepSeek V3.1 (hybrid), are available for [dedicated model inference](/docs/dedicated-endpoints/models).

