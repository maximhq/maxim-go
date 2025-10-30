// Package logging provides parsers for various LLM provider API responses.
package logging

import (
	"encoding/json"
	"fmt"
	"time"
)

// ParseGeminiResult parses a JSON response from Google's Gemini API and returns a MaximLLMResult.
// It handles both native Gemini format and OpenAI-compatible format.
func ParseGeminiResult(jsonData []byte) (*MaximLLMResult, error) {
	// First, try to parse as OpenAI-compatible format
	var openAIResp MaximLLMResult
	if err := json.Unmarshal(jsonData, &openAIResp); err == nil {
		// Check if it looks like an OpenAI format (has choices array)
		if len(openAIResp.Choices) > 0 {
			return &openAIResp, nil
		}
	}

	// If not OpenAI format, parse as native Gemini format
	return parseNativeGeminiResult(jsonData)
}

// parseNativeGeminiResult parses the native Gemini API response format
func parseNativeGeminiResult(jsonData []byte) (*MaximLLMResult, error) {
	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text         string                  `json:"text"`
					FunctionCall *map[string]interface{} `json:"functionCall,omitempty"`
				} `json:"parts"`
				Role string `json:"role"`
			} `json:"content"`
			FinishReason  string `json:"finishReason"`
			Index         int    `json:"index"`
			SafetyRatings []struct {
				Category    string `json:"category"`
				Probability string `json:"probability"`
			} `json:"safetyRatings,omitempty"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
		ModelVersion string `json:"modelVersion,omitempty"`
	}

	if err := json.Unmarshal(jsonData, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Gemini completion: %w", err)
	}

	// Validate response has candidates
	if len(geminiResp.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in Gemini response")
	}

	resp := MaximLLMResult{}

	// Generate an ID (Gemini doesn't provide one in native format)
	resp.ID = fmt.Sprintf("gemini-%d", time.Now().UnixNano())

	// Set model from modelVersion or use a default
	if geminiResp.ModelVersion != "" {
		resp.Model = geminiResp.ModelVersion
	} else {
		resp.Model = "gemini"
	}

	// Set creation timestamp
	resp.Created = time.Now().Unix()

	// Initialize choices array
	resp.Choices = make([]struct {
		Message struct {
			Role      string                   `json:"role"`
			Content   string                   `json:"content"`
			ToolCalls []ChatCompletionToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	}, len(geminiResp.Candidates))

	// Process each candidate
	for i, candidate := range geminiResp.Candidates {
		// Map Gemini role to OpenAI role format
		role := "assistant"
		if candidate.Content.Role == "model" {
			role = "assistant"
		} else if candidate.Content.Role == "user" {
			role = "user"
		}
		resp.Choices[i].Message.Role = role

		// Concatenate all text parts
		var fullContent string
		var toolCalls []ChatCompletionToolCall

		for partIdx, part := range candidate.Content.Parts {
			if part.Text != "" {
				fullContent += part.Text
			}

			// Handle function calls as tool calls
			if part.FunctionCall != nil {
				functionCall := *part.FunctionCall
				if name, ok := functionCall["name"].(string); ok {
					// Convert args to JSON string
					var argsJSON string
					if args, ok := functionCall["args"]; ok {
						argsBytes, err := json.Marshal(args)
						if err == nil {
							argsJSON = string(argsBytes)
						}
					}

					toolCall := ChatCompletionToolCall{
						ID:   fmt.Sprintf("call_%d_%d", i, partIdx),
						Type: "function",
						Function: ToolCallFunction{
							Name:      name,
							Arguments: argsJSON,
						},
					}
					toolCalls = append(toolCalls, toolCall)
				}
			}
		}

		resp.Choices[i].Message.Content = fullContent
		if len(toolCalls) > 0 {
			resp.Choices[i].Message.ToolCalls = toolCalls
		}

		// Map finish reason
		finishReason := mapGeminiFinishReason(candidate.FinishReason)
		resp.Choices[i].FinishReason = finishReason
	}

	// Set usage information
	resp.Usage.PromptTokens = geminiResp.UsageMetadata.PromptTokenCount
	resp.Usage.CompletionTokens = geminiResp.UsageMetadata.CandidatesTokenCount
	resp.Usage.TotalTokens = geminiResp.UsageMetadata.TotalTokenCount

	return &resp, nil
}

// mapGeminiFinishReason maps Gemini finish reasons to OpenAI format
func mapGeminiFinishReason(reason string) string {
	switch reason {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY":
		return "content_filter"
	case "RECITATION":
		return "content_filter"
	case "OTHER":
		return "stop"
	default:
		return "stop"
	}
}
