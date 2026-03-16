// Package maxim provides the Maxim SDK for Go, enabling LLM observability,
// tracing, and logging across OpenAI, Azure, Gemini, Bedrock, and other providers.
//
// # Breaking Change Notice (v0.2.0)
//
// Types such as MaximLLMResult, Usage, ChatCompletionToolCall, and related
// structs have been refactored from the logging package to the schemas package.
// If you were importing these types from github.com/maximhq/maxim-go/logging,
// update your imports to use github.com/maximhq/maxim-go/schemas instead.
package maxim
