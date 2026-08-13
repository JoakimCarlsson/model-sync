> For clean Markdown of any page, append .md to the page URL.
> For a complete documentation index, see https://docs.cohere.com/llms.txt.
> For AI client integration (Claude Code, Cursor, etc.), connect to the MCP server at https://docs.cohere.com/_mcp/server.

# Basic usage of tool use (function calling)

> An overview of using Cohere's tool use capabilities, enabling developers to build agentic workflows (API v2).

## Overview

Tool use is a technique which allows developers to connect Cohere’s Command family of models to external tools like search engines, APIs, functions, databases, etc.

This opens up a richer set of behaviors by leveraging tools to access external data sources, taking actions through APIs, interacting with a vector database, querying a search engine, etc., and is particularly valuable for enterprise developers, since a lot of enterprise data lives in external sources.

The Chat endpoint comes with built-in tool use capabilities such as function calling, multi-step reasoning, and citation generation.

