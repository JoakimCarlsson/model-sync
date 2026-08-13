> ## Documentation Index
> Fetch the complete documentation index at: https://docs.perplexity.ai/llms.txt
> Use this file to discover all available pages before exploring further.

# Sonar Pro

> Learn about Sonar Pro for advanced search, including pricing, API usage, and Pro Search support.

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
          backgroundImage: `var(--sonar-pro-banner)`,
          backgroundRepeat: 'no-repeat',
          backgroundSize: 'cover',
          backgroundPosition: 'center'
        />

        <div className="flex items-center justify-between">
          <div className="flex items-left gap-2 flex-col">
            <h1 className="text-3xl font-semibold text-foreground">Sonar Pro</h1>
            <p className="text-sm text-muted-foreground">Advanced search with enhanced search results</p>
          </div>
        </div>
      </div>

      <SonarDeprecationNotice />

      <div className="text-left w-full pt-8 border-b border-t border-border py-8">
        <p className="text-lg text-foreground leading-relaxed">
          An advanced search model designed for complex queries, delivering deeper content understanding with enhanced search result accuracy and 2x more search results than standard Sonar.
        </p>

        <p className="mt-4 text-sm text-muted-foreground">
          Use <a href="/docs/sonar/pro-search/quickstart">Pro Search</a> for enhanced search results with reasoning.
        </p>
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
            <span className="text-xl text-foreground font-mono">\$3</span>
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
            <span className="text-xl text-foreground font-mono">\$15</span>
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
            <h3 className="font-medium mb-2">Non-reasoning model</h3>
          </div>
        </div>

        <div className="flex items-start gap-4">
          <div className="w-8 h-8 flex items-center justify-center flex-shrink-0">
            <img src="https://mintlify-assets.b-cdn.net/perplexity/searchicon.svg" alt="Search" className="w-8 h-8" />
          </div>

          <div>
            <h3 className="font-medium mb-2">Optimized for complex, multi-step Q\&A tasks</h3>
          </div>
        </div>

        <div className="flex items-start gap-4">
          <div className="w-8 h-8 flex items-center justify-center flex-shrink-0">
            <img src="https://mintlify-assets.b-cdn.net/perplexity/lengthicon.svg" alt="Length" className="w-8 h-8" />
          </div>

          <div>
            <h3 className="font-medium mb-2">200K context length</h3>
          </div>
        </div>
      </div>

      <div className="space-y-6">
        <div className="flex items-start gap-4">
          <div className="w-8 h-8 flex items-center justify-center flex-shrink-0">
            <img src="https://mintlify-assets.b-cdn.net/perplexity/speedicon.svg" alt="Speed" className="w-8 h-8" />
          </div>

          <div>
            <h3 className="font-medium mb-2">Advanced information retrieval architecture</h3>
          </div>
        </div>

        <div className="flex items-start gap-4">
          <div className="w-8 h-8 flex items-center justify-center flex-shrink-0">
            <img src="https://mintlify-assets.b-cdn.net/perplexity/citationicon.svg" alt="Citation" className="w-8 h-8" />
          </div>

          <div>
            <h3 className="font-medium mb-2">2x more search results than standard Sonar</h3>
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
          <img src="https://mintlify-assets.b-cdn.net/perplexity/glasses.svg" alt="Research" className="w-8 h-8" />
        </div>

        <div>
          <h3 className="font-medium mb-2">Complex research questions requiring depth</h3>
        </div>
      </div>

      <div className="flex items-start gap-4">
        <div className="w-8 h-8 flex items-center justify-center flex-shrink-0">
          <img src="https://mintlify-assets.b-cdn.net/perplexity/search.svg" alt="Analysis" className="w-8 h-8" />
        </div>

        <div>
          <h3 className="font-medium mb-2">Comparative analysis across multiple sources</h3>
        </div>
      </div>

      <div className="flex items-start gap-4">
        <div className="w-8 h-8 flex items-center justify-center flex-shrink-0">
          <img src="https://mintlify-assets.b-cdn.net/perplexity/eye.svg" alt="Synthesis" className="w-8 h-8" />
        </div>

        <div>
          <h3 className="font-medium mb-2">Information synthesis and detailed reporting</h3>
        </div>
      </div>
    </div>
  </div>

  <div className="px-8 border-b border-border py-8">
    <CodeGroup>

    </CodeGroup>

    **Sample Response Metadata**

    <Accordion title="Cost Breakdown for Sample Request">
      <Info>
        **Token Usage**

        * Prompt Tokens: 26
        * Completion Tokens: 832
        * Search Context Size: Low
      </Info>

      <Steps>
        <Step title="Calculate Input Tokens Cost">
          26 tokens ÷ 1,000,000 × \$3 = \$0.000078
        </Step>

        <Step title="Calculate Output Tokens Cost">
          832 tokens ÷ 1,000,000 × \$15 = \$0.01248
        </Step>

        <Step title="Calculate Search Context Cost">
          1 request × \$6 ÷ 1,000 = \$0.006
        </Step>

        <Step title="Calculate Total Cost">
          \$0.000078 + \$0.01248 + \$0.006 = \$0.018558
        </Step>
      </Steps>

      <Check>
        **Total cost for this request: \$0.018558**
      </Check>
    </Accordion>
  </div>
</div>
