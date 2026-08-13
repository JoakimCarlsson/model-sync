> ## Documentation Index
> Fetch the complete documentation index at: https://docs.perplexity.ai/llms.txt
> Use this file to discover all available pages before exploring further.

# Models

> Compare Sonar, Sonar Pro, Sonar Reasoning Pro, and Sonar Deep Research models for the Sonar API.

    More models, tools, and research-backed presets: Sonar Chat Completions is now <a href="/docs/agent-api/quickstart">Agent API.</a> Migration guide <a href="/docs/agent-api/migrate-from-sonar/overview">here</a>.
  </Warning>;

    <hr className="border-t border-border mb-16" />
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
      <div className="space-y-6">
        <div>
          <h2 className="text-xl font-semibold text-foreground mb-4">Search</h2>
          <p className="text-muted-foreground text-m leading-relaxed">
            Models designed to retrieve and synthesize information efficiently.
          </p>
        </div>
        
        <div className="space-y-4">
          <div className="flex items-start gap-3">
            <div className="w-2 h-2 rounded-full bg-primary mt-2 flex-shrink-0"></div>
            <div className="text-muted-foreground text-xs leading-relaxed">
              Best suited for quick factual queries, topic summaries, product comparisons, and current events where simple information retrieval and synthesis is needed without complex reasoning.
            </div>
          </div>
          <div className="flex items-start gap-3">
            <div className="w-2 h-2 rounded-full bg-red-400 mt-2 flex-shrink-0"></div>
            <div className="text-muted-foreground text-xs leading-relaxed">
              Not ideal for multi-step analyses, exhaustive research on broad topics, or projects requiring detailed instructions or comprehensive reports across multiple sources.
            </div>
          </div>
        </div>
      </div>

      <div className="space-y-6 mt-10">
        <a href="/docs/sonar/models/sonar" className="model-card-link bg-card border border-border rounded-lg p-4 flex items-center gap-4 transition-all duration-300 hover:border-primary hover:bg-card hover:scale-[1.02] cursor-pointer">
          <div className="w-28 h-28 bg-card border border-border rounded-lg flex items-center justify-center flex-shrink-0" style={{
  backgroundImage: `var(--sonar-sq)`,
  backgroundColor: '#22808C'
            <span className="text-foreground text-xs font-medium">sonar</span>
          </div>
          <div className="flex flex-col">
            <div className="font-semibold text-foreground text-m mb-2">Sonar</div>
            <div className="text-xs text-[#B8C4B0] leading-relaxed">
              Lightweight, cost-effective search model with grounding.
            </div>
          </div>
        </a>

        <a href="/docs/sonar/models/sonar-pro" className="model-card-link bg-card border border-border rounded-lg p-4 flex items-center gap-4 transition-all duration-300 hover:border-primary hover:bg-card hover:scale-[1.02] cursor-pointer">
          <div className="w-28 h-28 bg-card border border-border rounded-lg flex items-center justify-center flex-shrink-0" style={{
  backgroundImage: `var(--sonar-pro-sq)`,
  backgroundColor: '#22808C'
            <span className="text-foreground text-xs font-medium">sonar pro</span>
          </div>
          <div className="flex-grow">
            <div className="font-semibold text-foreground text-m mb-2">Sonar Pro</div>
            <div className="text-xs text-[#B8C4B0] leading-relaxed">
              Advanced search offering with grounding, supporting complex queries and follow-ups.
            </div>
          </div>
        </a>
      </div>
    </div>

    <hr className="border-t border-border my-16" />

    <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
      <div className="space-y-6">
        <div>
          <h2 className="text-xl font-semibold text-foreground mb-4">Reasoning</h2>
          <p className="text-muted-foreground text-m leading-relaxed">
            Models that excel at complex, multi-step tasks.
          </p>
        </div>
        
        <div className="space-y-4">
          <div className="flex items-start gap-3">
            <div className="w-2 h-2 rounded-full bg-primary mt-2 flex-shrink-0"></div>
            <div className="text-muted-foreground text-xs leading-relaxed">
              Excellent for complex analyses requiring step-by-step thinking, tasks needing strict adherence to instructions, information synthesis across sources, and logical problem-solving that demands informed recommendations.
            </div>
          </div>
          <div className="flex items-start gap-3">
            <div className="w-2 h-2 rounded-full bg-red-400 mt-2 flex-shrink-0"></div>
            <div className="text-muted-foreground text-xs leading-relaxed">
              Not recommended for simple factual queries, basic information retrieval, exhaustive research projects (use Research models instead), or when speed takes priority over reasoning quality.
            </div>
          </div>
        </div>
      </div>

      <div className="space-y-6 mt-10">
        <a href="/docs/sonar/models/sonar-reasoning-pro" className="model-card-link bg-card border border-border rounded-lg p-4 flex items-center gap-4 transition-all duration-300 hover:border-primary hover:bg-card hover:scale-[1.02] cursor-pointer">
          <div className="w-28 h-28 bg-card border border-border rounded-lg flex items-center justify-center flex-shrink-0" style={{
  backgroundImage: `var(--reasoning-pro-sq)`,
  backgroundColor: '#21808D'
            <span className="text-foreground text-xs font-medium text-center">sonar reasoning pro</span>
          </div>
          <div className="flex-grow">
            <div className="font-semibold text-foreground text-m mb-2">Sonar Reasoning Pro</div>
            <div className="text-xs text-[#B8C4B0] leading-relaxed">
              Precise reasoning offering with Chain of Thought (CoT).
            </div>
          </div>
        </a>
      </div>
    </div>

    <hr className="border-t border-border my-16" />

    <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
      <div className="space-y-6">
        <div>
          <h2 className="text-xl font-semibold text-foreground mb-4">Research</h2>
          <p className="text-muted-foreground text-m leading-relaxed">
            Models that conduct in-depth analysis and generate detailed reports.
          </p>
        </div>
        
        <div className="space-y-4">
          <div className="flex items-start gap-3">
            <div className="w-2 h-2 rounded-full bg-primary mt-2 flex-shrink-0"></div>
            <div className="text-muted-foreground text-xs leading-relaxed">
              Ideal for comprehensive topic reports, in-depth analysis with exhaustive web research, and projects requiring synthesis of multiple information sources into cohesive reports like market analyses or literature reviews.
            </div>
          </div>
          
        </div>
      </div>

      <div className="space-y-6 mt-10">
        <a href="/docs/sonar/models/sonar-deep-research" className="model-card-link bg-card border border-border rounded-lg p-4 flex items-center gap-4 transition-all duration-300 hover:border-primary hover:bg-card hover:scale-[1.02] cursor-pointer">
          <div className="w-28 h-28 bg-card border border-border rounded-lg flex items-center justify-center flex-shrink-0" style={{
  backgroundImage: `var(--deep-sq)`,
  backgroundColor: '#9C452B'
            <span className="text-foreground text-xs font-medium text-center">sonar deep research</span>
          </div>
          <div className="flex flex-col">
            <div className="font-semibold text-foreground text-m mb-2">Sonar Deep Research</div>
            <div className="text-xs text-muted-foreground leading-relaxed">
              Expert-level research model conducting exhaustive searches and generating comprehensive reports.
            </div>
          </div>
        </a>
      </div>
    </div>
  </div>;

  return <svg xmlns="http://www.w3.org/2000/svg" width="80" height="30" viewBox="0 0 219 47" fill="none" className="text-foreground">
  <path d="M194.057 1.28212H201.77V7.26314C201.77 7.66217 201.968 7.86059 202.367 7.86059C202.766 7.86059 203.052 7.63382 203.224 7.17592C204.194 5.01072 205.535 3.47349 207.251 2.56205C208.965 1.59392 211.338 1.10986 214.364 1.10986H218.049V8.80037H213.165C209.338 8.80037 206.482 9.71181 204.598 11.5347C202.712 13.3576 201.77 16.3492 201.77 20.5073V45.7157H194.057V1.28212Z" fill="currentColor" />
  <path d="M145.653 32.4715C145.653 28.5402 146.996 25.4635 149.68 23.2416C152.421 20.963 156.505 19.7659 161.933 19.6526L178.212 19.3102V17.5157C178.212 14.1556 177.242 11.5347 175.299 9.65511C173.356 7.77555 170.329 6.83577 166.217 6.83577C162.563 6.83577 159.791 7.63382 157.907 9.22774C156.023 10.8238 155.079 12.8735 155.079 15.381H146.94C147.112 12.4766 147.969 9.85572 149.51 7.52044C151.052 5.18515 153.252 3.36228 156.106 2.05182C158.963 0.684667 162.332 0 166.217 0C172.728 0 177.641 1.56776 180.953 4.69891C184.265 7.83224 185.922 12.2477 185.922 17.9431V45.7157H178.212V41.1018C178.212 40.7595 178.04 40.5894 177.697 40.5894C177.412 40.5894 177.126 40.7595 176.84 41.1018C173.755 45.0332 169.215 46.9978 163.217 46.9978C157.733 46.9978 153.422 45.659 150.28 42.9814C147.195 40.3038 145.653 36.7998 145.653 32.4715ZM153.365 32.4715C153.365 34.8635 154.307 36.7431 156.194 38.1102C158.077 39.4774 160.705 40.1621 164.076 40.1621C168.474 40.1621 171.93 38.965 174.442 36.573C176.956 34.181 178.212 30.677 178.212 26.0631V25.7208L161.933 26.0631C159.019 26.1198 156.85 26.7173 155.422 27.8577C154.05 28.9392 153.365 30.4786 153.365 32.4715Z" fill="currentColor" />
  <path d="M105.794 1.28212V6.15328C105.794 6.55231 105.995 6.75073 106.394 6.75073C106.566 6.75073 106.708 6.69404 106.821 6.58065C106.993 6.41058 107.163 6.23832 107.336 6.06824C110.362 2.02347 114.989 0.00218047 121.214 0.00218047C126.812 0.00218047 131.21 1.56994 134.408 4.70109C137.607 7.83442 139.205 12.2499 139.205 17.9453V45.7179H131.495V19.2274C131.495 15.1826 130.466 12.106 128.41 9.99745C126.354 7.88894 123.384 6.83577 119.501 6.83577C115.103 6.83577 111.703 8.17458 109.305 10.8522C106.963 13.4731 105.792 17.4023 105.792 22.6442V45.7157H98.0818V1.28212H105.794Z" fill="currentColor" />
  <path d="M68.9421 0C73.2834 0 77.1951 0.911436 80.6795 2.73431C84.2206 4.55718 86.992 7.23479 88.9893 10.7672C91.0455 14.2995 92.0746 18.5427 92.0746 23.4989C92.0746 28.4551 91.0455 32.6983 88.9893 36.2307C86.9898 39.763 84.2206 42.4406 80.6795 44.2635C77.1951 46.0864 73.2834 46.9978 68.9421 46.9978C64.6007 46.9978 60.6606 46.0864 57.1196 44.2635C53.6352 42.4406 50.866 39.763 48.8098 36.2307C46.8103 32.6983 45.8116 28.4551 45.8116 23.4989C45.8116 18.5427 46.8103 14.2995 48.8098 10.7672C50.866 7.23479 53.6373 4.55718 57.1196 2.73431C60.6606 0.911436 64.6029 0 68.9421 0ZM68.9421 6.83577C64.3151 6.83577 60.5734 8.31631 57.7192 11.2796C54.9195 14.1839 53.5218 18.2571 53.5218 23.4989C53.5218 28.7408 54.9216 32.8422 57.7192 35.8033C60.5756 38.7077 64.3151 40.1621 68.9421 40.1621C73.569 40.1621 77.2823 38.7099 80.0799 35.8033C82.9363 32.84 84.3645 28.7386 84.3645 23.4989C84.3645 18.2592 82.9363 14.1839 80.0799 11.2796C77.2802 8.31631 73.569 6.83577 68.9421 6.83577Z" fill="currentColor" />
  <path d="M22.2757 0C25.989 0 29.3011 0.625794 32.2142 1.87956C35.184 3.13333 37.5259 4.92786 39.2397 7.26314C40.9536 9.59842 41.8672 12.3044 41.9806 15.381H33.8409C33.383 9.68346 29.3862 6.83577 21.8461 6.83577C18.5907 6.83577 16.0482 7.46156 14.221 8.71533C12.4505 9.9691 11.5652 11.563 11.5652 13.5015C11.5652 15.1543 12.1648 16.3492 13.3641 17.0905C14.62 17.8319 16.7351 18.5144 19.7049 19.1423L27.8446 20.8518C32.2426 21.7633 35.6136 23.0737 37.9554 24.7832C40.3539 26.4927 41.5532 29.1987 41.5532 32.9011C41.5532 35.6354 40.753 38.0841 39.1547 40.2493C37.5564 42.3578 35.2996 44.0084 32.3865 45.2055C29.5301 46.4026 26.218 47 22.4479 47C15.6514 47 10.3965 45.4911 6.68314 42.4712C2.9698 39.3945 0.74354 35.3498 0 30.3347H8.13969C9.22556 36.8848 13.9942 40.1621 22.4479 40.1621C26.1613 40.1621 28.9872 39.5079 30.9299 38.1974C32.8727 36.8303 33.8431 35.2058 33.8431 33.3263C33.8431 32.0725 33.5007 31.1327 32.8139 30.5069C32.1859 29.8223 31.3006 29.3099 30.1581 28.9697C29.0155 28.6274 27.1032 28.1717 24.4169 27.6026L16.2772 25.8931C12.3371 25.095 9.28007 23.7562 7.11051 21.8766C4.94094 19.9971 3.85507 17.2628 3.85507 13.6737C3.85507 9.45887 5.56892 6.12494 8.99661 3.67627C12.4221 1.22542 16.8507 0 22.2757 0Z" fill="currentColor" />
</svg>;

<div className="w-[80%] max-w-5xl mx-auto px-8 sm:px-8 lg:px-12 xl:px-16">
  <div className="flex flex-col md:flex-row justify-between items-start md:items-center mt-8 mb-8">
    <div className="flex items-center gap-2">
      <SonarLogo />

      <h1 className="text-2xl font-bold text-foreground"><span className="font-thin">Models</span></h1>
    </div>
  </div>

  <SonarDeprecationNotice />

  <div className="flex flex-col md:flex-row justify-between items-start md:items-center mt-8 mb-8">
    <div className="flex items-center gap-2">
      <h3 className="text-xl font-bold text-foreground"><span className="font-thin">Explore the Sonar range and compare models</span></h3>
    </div>
  </div>

  <Frame>
    <img src="https://mintcdn.com/perplexity/qLnd99sJyTqU60Ni/docs/assets/models/modelgraph.png?fit=max&auto=format&n=qLnd99sJyTqU60Ni&q=85&s=7783c052f9684bfd9edf7026b49aee26" alt="Sonar Range" className="mx-auto" width="1880" height="972" data-path="docs/assets/models/modelgraph.png" />
  </Frame>

  <ModelOverviewCards />
</div>
