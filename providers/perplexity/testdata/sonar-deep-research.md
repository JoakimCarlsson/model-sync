> ## Documentation Index
> Fetch the complete documentation index at: https://docs.perplexity.ai/llms.txt
> Use this file to discover all available pages before exploring further.

# Sonar Deep Research

> Learn about Sonar Deep Research for exhaustive research workflows, including pricing and API usage.

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
          backgroundImage: `var(--sonar-deep-research-banner)`,
          backgroundRepeat: 'no-repeat',
          backgroundSize: 'cover',
          backgroundPosition: 'center'
        />

        <div className="flex items-center justify-between">
          <div className="flex items-left gap-2 flex-col">
            <h1 className="text-3xl font-semibold text-foreground">Sonar Deep Research</h1>
            <p className="text-sm text-muted-foreground">Exhaustive research with expert-level insights</p>
          </div>
        </div>
      </div>

      <SonarDeprecationNotice />

      <div className="text-left w-full pt-8 border-b border-t border-border py-8">
        <p className="text-lg text-foreground leading-relaxed">
          A powerful research model capable of conducting exhaustive searches across hundreds of sources, synthesizing expert-level insights, and generating detailed reports with comprehensive analysis.
        </p>
      </div>
    </div>
  </div>

  <div className="px-8 border-b border-border pb-8 max-w-6xl mx-auto">
    <div className="flex items-left gap-2 flex-col">
      <h2 className="text-lg font-semibold text-left text-foreground">Pricing</h2>
      <a className="text-left text-muted-foreground mb-8 text-sm" href="/docs/getting-started/pricing" style={{ textDecoration: 'underline' }}>See the full pricing and search context size guide.</a>
    </div>

    <div className="grid grid-cols-1 md:grid-cols-5 gap-2">
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
          <div className="mb-6">
            <h3 className="text-sm font-semibold text-foreground">Citation Tokens</h3>
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
            <h3 className="text-sm font-semibold text-foreground">Search Queries</h3>
          </div>

          <div className="flex justify-center items-center">
            <span className="text-xl text-foreground font-mono">\$5</span>
          </div>

          <div className="text-center mt-2">
            <span className="text-xs text-muted-foreground">Per 1K Requests</span>
          </div>
        </div>
      </div>

      <div className="bg-card p-4 rounded-lg border border-border">
        <div className="text-left">
          <div className="mb-6">
            <h3 className="text-sm font-semibold text-foreground">Reasoning Tokens</h3>
          </div>

          <div className="flex justify-center items-center">
            <span className="text-xl text-foreground font-mono">\$3</span>
          </div>

          <div className="text-center mt-2">
            <span className="text-xs text-muted-foreground">Per 1M Tokens</span>
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
            <h3 className="font-medium mb-2">Deep research / Reasoning model</h3>
          </div>
        </div>

        <div className="flex items-start gap-4">
          <div className="w-8 h-8 flex items-center justify-center flex-shrink-0">
            <img src="https://mintlify-assets.b-cdn.net/perplexity/searchicon.svg" alt="Search" className="w-8 h-8" />
          </div>

          <div>
            <h3 className="font-medium mb-2">Exhaustive research across hundreds of sources</h3>
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
            <h3 className="font-medium mb-2">Expert-level subject analysis</h3>
          </div>
        </div>

        <div className="flex items-start gap-4">
          <div className="w-8 h-8 flex items-center justify-center flex-shrink-0">
            <img src="https://mintlify-assets.b-cdn.net/perplexity/citationicon.svg" alt="Citation" className="w-8 h-8" />
          </div>

          <div>
            <h3 className="font-medium mb-2">Detailed report generation</h3>
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
          <img src="https://mintlify-assets.b-cdn.net/perplexity/glasses.svg" alt="Academic Research" className="w-8 h-8" />
        </div>

        <div>
          <h3 className="font-medium mb-2">Academic research and comprehensive reports</h3>
        </div>
      </div>

      <div className="flex items-start gap-4">
        <div className="w-8 h-8 flex items-center justify-center flex-shrink-0">
          <img src="https://mintlify-assets.b-cdn.net/perplexity/search.svg" alt="Market Analysis" className="w-8 h-8" />
        </div>

        <div>
          <h3 className="font-medium mb-2">Market analysis and competitive intelligence</h3>
        </div>
      </div>

      <div className="flex items-start gap-4">
        <div className="w-8 h-8 flex items-center justify-center flex-shrink-0">
          <img src="https://mintlify-assets.b-cdn.net/perplexity/eye.svg" alt="Due Diligence" className="w-8 h-8" />
        </div>

        <div>
          <h3 className="font-medium mb-2">Due diligence and investigative research</h3>
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

        * Prompt Tokens: 33
        * Completion Tokens: 11,395
        * Citation Tokens: 19,028
        * Search Queries: 21
        * Reasoning Tokens: 193,947
      </Info>

      <Steps>
        <Step title="Calculate Input Tokens Cost">
          33 tokens ÷ 1,000,000 × \$2 = \$0.000066
        </Step>

        <Step title="Calculate Output Tokens Cost">
          11,395  tokens ÷ 1,000,000 × \$8 = \$0.09116
        </Step>

        <Step title="Calculate Citation Tokens Cost">
          19,028 tokens ÷ 1,000,000 × \$2 = \$0.038056
        </Step>

        <Step title="Calculate Search Queries Cost">
          21 queries × \$5 ÷ 1,000 = \$0.105
        </Step>

        <Step title="Calculate Reasoning Tokens Cost">
          193,947 tokens ÷ 1,000,000 × \$3 = \$0.581841
        </Step>

        <Step title="Calculate Total Cost">
          \$0.000066 + \$0.09116 + \$0.038056 + \$0.105 + \$0.581841 = \$0.816123
        </Step>
      </Steps>

      <Check>
        **Total cost for this request: \$0.816123**
      </Check>
    </Accordion>
  </div>

  <div className="px-8 border-b border-border py-8">
    <h2 className="text-lg font-semibold text-left text-foreground mb-8">Advanced Features</h2>

    <Tabs>
      <Tab title="Reasoning Effort">
        Control the computational effort dedicated to each query with the `reasoning_effort` parameter. This allows you to balance between speed and thoroughness while managing costs by directly impacting the amount of reasoning tokens consumed.

        **Options:**

        * `"low"`: Faster, simpler answers with reduced token usage
        * `"medium"`: Balanced approach (default)
        * `"high"`: Deeper, more thorough responses with increased token usage

        <CodeGroup>

        </CodeGroup>

        <CodeGroup>
        </CodeGroup>
      </Tab>

      <Tab title="Async API">
        For research-intensive tasks that may take longer to process, you can use the async API. This allows you to submit a request and retrieve the results later.

        <Steps>
          <Step title="Submit Request">
            <CodeGroup>

            </CodeGroup>

            **Sample Response (Request Submission)**

          </Step>

          <Step title="Retrieve Results">
            <CodeGroup>

            </CodeGroup>
          </Step>
        </Steps>

        <Warning>
          Async requests have a time-to-live (TTL) of 7 days. After this period, the request and its results will no longer be accessible.
        </Warning>
      </Tab>
    </Tabs>
  </div>
</div>
