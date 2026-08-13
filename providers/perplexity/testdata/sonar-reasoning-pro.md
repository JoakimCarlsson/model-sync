> ## Documentation Index
> Fetch the complete documentation index at: https://docs.perplexity.ai/llms.txt
> Use this file to discover all available pages before exploring further.

# Sonar Reasoning Pro

> Learn about Sonar Reasoning Pro for advanced multi-step reasoning, including pricing and API usage.

    More models, tools, and research-backed presets: Sonar Chat Completions is now <a href="/docs/agent-api/quickstart">Agent API.</a> Migration guide <a href="/docs/agent-api/migrate-from-sonar/overview">here</a>.
  </Warning>;

<div className="px-8 pt-8">
  <a href="/docs/sonar/models" className="inline-flex items-center text-muted-foreground hover:text-foreground transition-colors text-sm">
    <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
    </svg>

    Models
  </a>
</div>

<div className="w-[80%] max-w-5xl mx-auto px-8 sm:px-8 lg:px-12 xl:px-16">
  <div className="text-white px-8 pt-8 rounded-xl mb-8">
    <div className="max-w-6xl mx-auto">
      <div className="mb-8">
        <div
          className="w-full h-48 rounded-lg mb-6"
          style={{ 
          backgroundImage: `var(--sonar-reasoning-pro-banner)`,
          backgroundRepeat: 'no-repeat',
          backgroundSize: 'cover',
          backgroundPosition: 'center'
        />

        <div className="flex items-center justify-between">
          <div className="flex items-left gap-2 flex-col">
            <h1 className="text-3xl font-semibold text-foreground">Sonar Reasoning Pro</h1>
            <p className="text-sm text-muted-foreground">Advanced reasoning with enhanced multi-step analysis</p>
          </div>
        </div>
      </div>

      <SonarDeprecationNotice />

      <div className="text-left w-full pt-8 border-b border-t border-border py-8">
        <p className="text-lg text-foreground leading-relaxed">
          A high-performance reasoning model leveraging advanced multi-step Chain-of-Thought (CoT) reasoning and enhanced information retrieval for complex problem-solving.
        </p>

        <p className="mt-4 text-sm text-muted-foreground">
          <code>sonar-reasoning</code> was deprecated on December 15, 2025. Use <code>sonar-reasoning-pro</code> for improved reasoning capabilities.
        </p>

        <div className="mt-6">
          <Warning>
            Using image input with structured outputs is not supported in thinking models.
          </Warning>
        </div>
      </div>
    </div>
  </div>

  <div className="px-8 border-b border-border pb-8 max-w-6xl mx-auto">
    <div className="flex items-left gap-2 flex-col">
      <h2 className="text-lg font-semibold text-left text-foreground">Pricing</h2>
      <a className="text-left text-muted-foreground mb-8 text-sm" href="/docs/getting-started/pricing" style={{ textDecoration: 'underline' }}>See the full pricing and search context size guide.</a>
    </div>

    <div className="grid grid-cols-1 md:grid-cols-3 gap-2">
      <div className="bg-card p-4 rounded-lg border border-border">
        <div className="text-left">
          <div className="mb-6">
            <h3 className="text-sm font-semibold text-foreground">Input Tokens</h3>
          </div>

          <div className="flex justify-center items-center">
            <span className="text-xl text-foreground font-mono">\$2</span>
          </div>

          <div className="text-center mt-2">
            <span className="text-xs text-muted-foreground">Per 1M Tokens</span>
          </div>
        </div>
      </div>

      <div className="bg-card p-4 rounded-lg border border-border">
        <div className="text-left">
          <div className="mb-6">
            <h3 className="text-sm font-semibold text-foreground">Output Tokens</h3>
          </div>

          <div className="flex justify-center items-center">
            <span className="text-xl text-foreground font-mono">\$8</span>
          </div>

          <div className="text-center mt-2">
            <span className="text-xs text-muted-foreground">Per 1M Tokens</span>
          </div>
        </div>
      </div>

      <div className="bg-card p-4 rounded-lg border border-border">
        <div className="text-left">
          <h3 className="text-sm font-semibold text-foreground mb-2">Price Per 1K Requests</h3>

          <p className="text-xs text-muted-foreground mb-6" />

          <div className="space-y-2">
            <div className="flex justify-between items-center">
              <span className="text-xl text-foreground font-mono">\$14</span>
              <span className="text-xl text-foreground font-mono">\$10</span>
              <span className="text-xl text-foreground font-mono">\$6</span>
            </div>

            <div className="flex justify-between items-center">
              <span className="text-xs text-muted-foreground">High</span>
              <span className="text-xs text-muted-foreground">Medium</span>
              <span className="text-xs text-muted-foreground">Low</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>

  <div className="px-8 border-b border-border py-8">
    <h2 className="text-lg font-semibold text-left text-foreground mb-8">Features</h2>

    <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
      <div className="space-y-6">
        <div className="flex items-start gap-4">
          <div className="w-8 h-8 flex items-center justify-center flex-shrink-0">
            <img src="https://mintlify-assets.b-cdn.net/perplexity/arrow.svg" alt="Arrow" className="w-8 h-8" />
          </div>

          <div>
            <h3 className="font-medium mb-2">Advanced reasoning model</h3>
          </div>
        </div>

        <div className="flex items-start gap-4">
          <div className="w-8 h-8 flex items-center justify-center flex-shrink-0">
            <img src="https://mintlify-assets.b-cdn.net/perplexity/searchicon.svg" alt="Search" className="w-8 h-8" />
          </div>

          <div>
            <h3 className="font-medium mb-2">Enhanced Chain-of-Thought (CoT) reasoning</h3>
          </div>
        </div>

        <div className="flex items-start gap-4">
          <div className="w-8 h-8 flex items-center justify-center flex-shrink-0">
            <img src="https://mintlify-assets.b-cdn.net/perplexity/lengthicon.svg" alt="Length" className="w-8 h-8" />
          </div>

          <div>
            <h3 className="font-medium mb-2">128K context length</h3>
          </div>
        </div>
      </div>

      <div className="space-y-6">
        <div className="flex items-start gap-4">
          <div className="w-8 h-8 flex items-center justify-center flex-shrink-0">
            <img src="https://mintlify-assets.b-cdn.net/perplexity/speedicon.svg" alt="Speed" className="w-8 h-8" />
          </div>

          <div>
            <h3 className="font-medium mb-2">Best for complex multi-step reasoning tasks</h3>
          </div>
        </div>

        <div className="flex items-start gap-4">
          <div className="w-8 h-8 flex items-center justify-center flex-shrink-0">
            <img src="https://mintlify-assets.b-cdn.net/perplexity/lockicon.svg" alt="Lock" className="w-8 h-8" />
          </div>

          <div>
            <h3 className="font-medium mb-2">No training on customer data</h3>
          </div>
        </div>
      </div>
    </div>
  </div>

  <div className="px-8 border-b border-border py-8">
    <h2 className="text-lg font-semibold text-left text-foreground mb-8">Real World Use Cases</h2>

    <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
      <div className="flex items-start gap-4">
        <div className="w-8 h-8 flex items-center justify-center flex-shrink-0">
          <img src="https://mintlify-assets.b-cdn.net/perplexity/glasses.svg" alt="Complex Analysis" className="w-8 h-8" />
        </div>

        <div>
          <h3 className="font-medium mb-2">Complex multi-step analysis and reasoning</h3>
        </div>
      </div>

      <div className="flex items-start gap-4">
        <div className="w-8 h-8 flex items-center justify-center flex-shrink-0">
          <img src="https://mintlify-assets.b-cdn.net/perplexity/search.svg" alt="Research" className="w-8 h-8" />
        </div>

        <div>
          <h3 className="font-medium mb-2">Advanced research with deep reasoning</h3>
        </div>
      </div>

      <div className="flex items-start gap-4">
        <div className="w-8 h-8 flex items-center justify-center flex-shrink-0">
          <img src="https://mintlify-assets.b-cdn.net/perplexity/eye.svg" alt="Decision Making" className="w-8 h-8" />
        </div>

        <div>
          <h3 className="font-medium mb-2">Strategic decision making with comprehensive analysis</h3>
        </div>
      </div>
    </div>
  </div>

  <div className="px-8 border-b border-border py-8">
    <CodeGroup>

    </CodeGroup>

    <Note>
      * The `sonar-reasoning-pro` model is designed to output a `<think>` section containing reasoning tokens, immediately followed by a valid JSON object. As a result, the `response_format` parameter does not remove these reasoning tokens from the output. We recommend using a custom parser to extract the valid JSON portion, and an example implementation can be found [here](https://github.com/ppl-ai/api-discussion/blob/main/utils/extract_json_reasoning_models.py).
    </Note>

    **Sample Response Metadata**

    <Accordion title="Cost Breakdown for Sample Request">
      <Info>
        **Token Usage**

        * Prompt Tokens: 17
        * Completion Tokens: 1152
        * Search Context Size: Low
      </Info>

      <Steps>
        <Step title="Calculate Input Tokens Cost">
          17 tokens ÷ 1,000,000 × $2 = $0.000034
        </Step>

        <Step title="Calculate Output Tokens Cost">
          1152 tokens ÷ 1,000,000 × $8 = $0.009216
        </Step>

        <Step title="Calculate Search Context Cost">
          1 request × $6 ÷ 1,000 = $0.006
        </Step>

        <Step title="Calculate Total Cost">
          $0.000034 + $0.009216 + $0.006 = $0.015250
        </Step>
      </Steps>

      <Check>
        **Total cost for this request: \$0.015250**
      </Check>
    </Accordion>
  </div>
</div>
