package logging

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/maximhq/maxim-go/schemas"
)

func ParseBedrockResult(model string, jsonData []byte) (*schemas.MaximLLMResult, error) {
	var bedrockResp schemas.BedrockConverseResp
	if err := json.Unmarshal(jsonData, &bedrockResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Bedrock completion: %w", err)
	}
	resp := schemas.MaximLLMResult{}

	// Set the fields
	// Strip "global." prefix if present (e.g. cross-region inference profile)
	resp.Model = strings.TrimPrefix(model, "global.") // Bedrock doesn't return model info in the response
	// Concatenate all content values
	var fullContent string
	var toolCalls []schemas.ChatCompletionToolCall
	// Checking for Output.Value else check in Output.Message
	if bedrockResp.Output.Value != nil {
		for _, content := range bedrockResp.Output.Value.Content {
			if str, ok := content.Value.(string); ok {
				fullContent += str
			} else if m, ok := content.Value.(map[string]interface{}); ok {
				if toolCalls == nil {
					toolCalls = make([]schemas.ChatCompletionToolCall, 0)
				}
				var args string
				if input, inputFound := m["Input"].(map[string]interface{}); inputFound {
					if inputJSON, err := json.Marshal(input); err == nil {
						args = string(inputJSON)
					} else {
						args = "{}"
					}
				} else {
					args = "{}"
				}
				toolCalls = append(toolCalls, schemas.ChatCompletionToolCall{
					Type: "function",
					ID:   fmt.Sprintf("%v", m["ToolUseId"]),
					Function: schemas.ToolCallFunction{
						Name:      fmt.Sprintf("%v", m["Name"]),
						Arguments: args,
					},
				})
			}
		}
	} else if bedrockResp.Output.Message != nil {
		for _, content := range bedrockResp.Output.Message.Content {
			if content.ToolUse != nil {
				if toolCalls == nil {
					toolCalls = make([]schemas.ChatCompletionToolCall, 0)
				}
				var args string
				if inputJSON, err := json.Marshal(content.ToolUse.Input); err == nil {
					args = string(inputJSON)
				} else {
					// Fallback to empty JSON object if marshaling fails
					args = "{}"
				}
				toolCalls = append(toolCalls, schemas.ChatCompletionToolCall{
					Type: "function",
					ID:   content.ToolUse.ID,
					Function: schemas.ToolCallFunction{
						Name:      content.ToolUse.Name,
						Arguments: args,
					},
				})
				continue
			}
			fullContent += content.Text
		}
	}
	// Set the choice with content
	resp.Choices = make([]schemas.MaximLLMChoice, 1)
	if bedrockResp.Output.Value != nil {
		resp.Choices[0].Message.Role = bedrockResp.Output.Value.Role
	} else if bedrockResp.Output.Message != nil {
		resp.Choices[0].Message.Role = bedrockResp.Output.Message.Role
	}
	if toolCalls != nil {
		resp.Choices[0].Message.ToolCalls = toolCalls
		resp.Choices[0].FinishReason = "tool_use"
	}
	resp.Choices[0].Message.Content = fullContent
	resp.Choices[0].FinishReason = bedrockResp.StopReason

	// Set usage information
	resp.Usage.PromptTokens = bedrockResp.Usage.InputTokens
	resp.Usage.CompletionTokens = bedrockResp.Usage.OutputTokens
	resp.Usage.TotalTokens = bedrockResp.Usage.TotalTokens

	// Set creation timestamp
	resp.Created = time.Now().Unix()

	// Generate an ID if one isn't available
	resp.ID = fmt.Sprintf("bedrock-%d", time.Now().UnixNano())

	return &resp, nil
}
