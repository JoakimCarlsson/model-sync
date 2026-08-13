> For clean Markdown of any page, append .md to the page URL.
> For a complete documentation index, see https://docs.cohere.com/llms.txt.
> For AI client integration (Claude Code, Cursor, etc.), connect to the MCP server at https://docs.cohere.com/_mcp/server.

# How do Structured Outputs Work?

> This page describes how to get Cohere models to create outputs in a certain format, such as JSON, TOOLS, using parameters such as `response_format`.

## Overview

Structured Outputs is a feature that forces the LLM’s response to strictly follow a schema specified by the user. When Structured Outputs is turned on, the LLM will generate structured data that follows the desired schema, provided by the user, 100% of the time. This increases the reliability of LLMs in enterprise applications where downstream applications expect the LLM output to be correctly formatted. With Structured Outputs, hallucinated fields and entries in structured data can be reliably eliminated.

Compatible models:

* Command A+
* Command A
* Command R+ 08 2024
* Command R+
* Command R 08 2024
* Command R

## How to Use Structured Outputs

There are two ways to use Structured Outputs:

