---
description: Overview, technical specs, use cases, and best practices for the compound agentic tool system on Groq.
title: Compound - GroqDocs
image: https://console.groq.com/og_cloudv5.jpg
---

# Compound

`groq/compound`

[Try it in Playground](https://console.groq.com/playground?model=groq/compound)

TOKEN SPEED

\~450 tps

Powered bygroq

INPUT

Text

OUTPUT

Text

CAPABILITIES

[Web Search](https://console.groq.com/docs/web-search), [Code Execution](https://console.groq.com/docs/code-execution), [Visit Website](https://console.groq.com/docs/visit-website), [Wolfram Alpha](https://console.groq.com/docs/wolfram-alpha), [JSON Object Mode](https://console.groq.com/docs/structured-outputs#json-object-mode)

![Groq logo](https://console.groq.com/_next/image?url=%2Fgroq-circle.png&w=96&q=75)Groq

Groq's Compound system integrates OpenAI's GPT-OSS 120B and Llama 4 models with external tools like web search and code execution. This allows applications to access real-time data and interact with external environments, providing more accurate and current responses than standalone LLMs. Instead of managing separate tools and APIs, Compound systems offer a unified interface that handles tool integration and orchestration, letting you focus on application logic rather than infrastructure complexity.

Rate limits for `groq/compound` are determined by the rate limits of the individual models that comprise them.

This system should not be used by customers for processing protected health information as it is not a HIPAA Covered Cloud Service under Groq's Business Associate Addendum at this time. This system is also not available currently for use with regional / sovereign endpoints.

---

### PRICING

Underlying Model Pricing (per 1M tokens)

Pricing (GPT-OSS-120B)

Input

$0.15

Output

$0.60

Pricing (Llama 4 Scout)

Input

Pending

Output

Pending

Built-in Tool Pricing

Basic Web Search

$5 / 1000 requests

Advanced Web Search

$8 / 1000 requests

Visit Website

$1 / 1000 requests

Code Execution

$0.18 / hour

Wolfram Alpha

Based on your API key from Wolfram, not billed by Groq

Final pricing depends on which underlying models and tools are used for your specific query. See the [Pricing page](https://groq.com/pricing) for more details or the [Compound page](https://console.groq.com/docs/compound#model-usage-details) for usage breakdowns.

---

### LIMITS

CONTEXT WINDOW

131,072

---

MAX OUTPUT TOKENS

8,192

---

### QUANTIZATION

This uses Groq's TruePoint Numerics, which reduces precision only in areas that don't affect accuracy, preserving quality while delivering significant speedup over traditional approaches. [Learn more here](https://groq.com/blog/inside-the-lpu-deconstructing-groq-speed).

### [Key Technical Specifications](#key-technical-specifications)

### Model Architecture

Compound is powered by [Llama 4 Scout](https://console.groq.com/docs/model/meta-llama/llama-4-scout-17b-16e-instruct) and [GPT-OSS 120B](https://console.groq.com/docs/model/openai/gpt-oss-120b) for intelligent reasoning and tool use.

### Performance Metrics

Groq developed a new evaluation benchmark for measuring search capabilities called [RealtimeEval](https://github.com/groq/realtime-eval). This benchmark is designed to evaluate tool-using systems on current events and live data. On the benchmark, Compound outperformed GPT-4o-search-preview and GPT-4o-mini-search-preview significantly.

[Learn More About Agentic ToolingDiscover how to build powerful applications with real-time web search and code execution](https://console.groq.com/docs/compound) 

### Use Cases

Realtime Web Search

Automatically access up-to-date information from the web using the built-in web search tool.

Code Execution

Execute Python code automatically using the code execution tool powered by [E2B](https://e2b.dev/).

Code Generation and Technical Tasks

Create AI tools for code generation, debugging, and technical problem-solving with high-quality multilingual support.

### Best Practices

* Use system prompts to improve steerability and reduce false refusals. Compound is designed to be highly steerable with appropriate system prompts.
* Consider implementing system-level protections like Llama Guard for input filtering and response validation.
* Deploy with appropriate safeguards when working in specialized domains or with critical content.
* Compound should not be used by customers for processing protected health information. It is not a HIPAA Covered Cloud Service under Groq's Business Associate Addendum for customers at this time.

### [Quick Start](#quick-start)

Experience the capabilities of `groq/compound` on Groq:

curlJavaScriptPythonJSON

shell

```
pip install groq
```

Python

```
from groq import Groq
client = Groq()
completion = client.chat.completions.create(
    model="groq/compound",
    messages=[
        {
            "role": "user",
            "content": "Explain why fast inference is critical for reasoning models"
        }
    ]
)
print(completion.choices[0].message.content)
```