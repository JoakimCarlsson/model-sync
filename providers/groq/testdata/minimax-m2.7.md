---
description: Model card for MiniMax-M2.7: a 229B-parameter MoE model (~10B active) with interleaved thinking, built for agentic workflows, tool use, and coding, with near-instant responses on Groq.
title: MiniMax M2.7 - GroqDocs
image: https://console.groq.com/og_cloudv5.jpg
---

# MiniMax M2.7

PreviewEnterprise

`minimaxai/minimax-m2.7`

[Try it in Playground](https://console.groq.com/playground?model=minimaxai/minimax-m2.7)

TOKEN SPEED

\~260 tps

Powered bygroq

INPUT

Text

OUTPUT

Text

CAPABILITIES

[Tool Use](https://console.groq.com/docs/tool-use), [JSON Object Mode](https://console.groq.com/docs/structured-outputs#json-object-mode), [Reasoning](https://console.groq.com/docs/reasoning)

![MiniMaxAI logo](https://console.groq.com/_next/image?url=%2Fminimax_logo.png&w=96&q=75)MiniMaxAI

[Model card](https://huggingface.co/MiniMaxAI/MiniMax-M2.7)

MiniMax M2.7 is a 229-billion-parameter Mixture-of-Experts model (\~10B active per token) from MiniMax, built for agentic workflows and real-world software engineering. It interleaves thinking with actions across multi-step tasks and is designed for building complex agent harnesses, leveraging agent teams, skills, and dynamic tool search, while its sparse activation keeps inference fast and efficient. On Groq, MiniMax M2.7 is available to Enterprise customers. [Contact sales](https://groq.com/enterprise-access) for access.

---

### LIMITS

CONTEXT WINDOW

196,608

---

MAX OUTPUT TOKENS

131,072

---

### QUANTIZATION

This uses Groq's TruePoint Numerics, which reduces precision only in areas that don't affect accuracy, preserving quality while delivering significant speedup over traditional approaches. [Learn more here](https://groq.com/blog/inside-the-lpu-deconstructing-groq-speed).

### [Key Technical Specifications](#key-technical-specifications)

### Model Architecture

A Mixture-of-Experts model with 229B total parameters and roughly 10B activated per token, built on a 62-layer decoder-only transformer with grouped-query attention (48 query / 8 key-value heads) and 256 experts, pre-trained on 29.2T tokens. Features interleaved thinking for agentic reasoning, with a 196K-token context window on Groq.

### Performance Metrics

MiniMax M2.7 demonstrates strong performance across agentic coding and tool-use benchmarks:

* SWE-Pro (Agentic Coding): 56.2%
* Terminal-Bench 2 (Agentic Terminal Use): 57.0%
* VIBE-Pro (App Building): 55.6%
* Toolathon (Tool Use): 46.3%
* MM Claw (Agentic Tasks): 62.7%
* MLE-Bench Lite (ML Engineering): 66.6% medal rate

### Use Cases

Agentic Coding and Software Engineering

Purpose-built for autonomous coding agents that plan, edit, run, and verify code across long multi-step sessions.
* Repository-scale code generation and refactoring
* Terminal and environment-driven workflows
* Bug fixing and multi-file edits
* Integration with coding assistants and agent scaffolds

Autonomous Agents and Tool Use

Builds and runs complex agent harnesses with dynamic tool selection across elaborate productivity tasks.
* Multi-agent and agent-team orchestration
* Dynamic tool search and function calling
* Long-horizon task planning and execution
* End-to-end productivity and data workflows

### Best Practices

* Sampling: use the officially recommended temperature=1.0, top\_p=0.95, and top\_k=40 for best performance.
* Interleaved thinking: in multi-turn agentic workflows, retain the model's thinking content in conversation history, per MiniMax's guidance for the M2 series.
* Tool use: provide clear, well-structured tool schemas with examples to take advantage of the model's dynamic tool-calling strengths.
* Use the full 196K context window for repository-scale code, long agent trajectories, and multi-document workflows.

### [Get Started with MiniMax M2.7](#get-started-with-minimax-m27)

Experience state-of-the-art agentic coding and tool use with MiniMax M2.7 at Groq speed:

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
    model="minimaxai/minimax-m2.7",
    messages=[
        {
            "role": "user",
            "content": "Explain why fast inference is critical for reasoning models"
        }
    ]
)
print(completion.choices[0].message.content)
```