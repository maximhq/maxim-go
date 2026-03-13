package logging

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/maximhq/maxim-go/schemas"
)

// ParseAnthropicResult parses a JSON response from Anthropic's API and returns a MaximLLMResult.
func ParseAnthropicResult(jsonData []byte) (*schemas.MaximLLMResult, error) {
	var anthropicResp struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason   string `json:"stop_reason"`
		StopSequence string `json:"stop_sequence"`
		Usage        struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(jsonData, &anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Anthropic completion: %w", err)
	}
	resp := schemas.MaximLLMResult{}
	// Set the fields
	resp.ID = anthropicResp.ID
	resp.Model = anthropicResp.Model

	// Concatenate all text content
	var fullContent string
	for _, content := range anthropicResp.Content {
		if content.Type == "text" {
			fullContent += content.Text
		}
	}

	// Set the choice with content
	if len(resp.Choices) == 0 {
		resp.Choices = make([]schemas.MaximLLMChoice, 1)
	}
	resp.Choices[0].Message.Role = anthropicResp.Role
	resp.Choices[0].Message.Content = fullContent
	resp.Choices[0].FinishReason = anthropicResp.StopReason

	// Set usage information
	resp.Usage.PromptTokens = anthropicResp.Usage.InputTokens
	resp.Usage.CompletionTokens = anthropicResp.Usage.OutputTokens
	resp.Usage.TotalTokens = anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens

	// Set creation timestamp
	resp.Created = time.Now().Unix()
	return &resp, nil
}
