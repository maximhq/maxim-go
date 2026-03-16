package logging

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/maximhq/maxim-go/schemas"
)

func ParseGeminiResult(jsonData []byte) (*schemas.MaximLLMResult, error) {
	var geminiResp schemas.GeminiResp
	if err := json.Unmarshal(jsonData, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Gemini completion: %w", err)
	}
	resp := schemas.MaximLLMResult{}

	resp.ID = geminiResp.ResponseID
	resp.Model = geminiResp.ModelVersion
	resp.Created = time.Now().Unix()

	if len(geminiResp.Candidates) > 0 {
		resp.Choices = make([]schemas.MaximLLMChoice, len(geminiResp.Candidates))

		for i, candidate := range geminiResp.Candidates {
			resp.Choices[i].Message.Role = candidate.Content.Role
			resp.Choices[i].FinishReason = candidate.FinishReason

			var fullContent string
			var toolCalls []schemas.ChatCompletionToolCall
			for _, part := range candidate.Content.Parts {
				if part.FunctionCall != nil {
					if toolCalls == nil {
						toolCalls = make([]schemas.ChatCompletionToolCall, 0)
					}
					var args string
					if argsJSON, err := json.Marshal(part.FunctionCall.Args); err == nil {
						args = string(argsJSON)
					} else {
						args = "{}"
					}
					toolCalls = append(toolCalls, schemas.ChatCompletionToolCall{
						Type: "function",
						ID:   uuid.NewString(),
						Function: schemas.ToolCallFunction{
							Name:      part.FunctionCall.Name,
							Arguments: args,
						},
					})
				} else if part.Text != "" {
					fullContent += part.Text
				}
			}
			resp.Choices[i].Message.Content = fullContent
			if toolCalls != nil {
				resp.Choices[i].Message.ToolCalls = toolCalls
			}
		}
	}

	resp.Usage.PromptTokens = geminiResp.UsageMetadata.PromptTokenCount
	resp.Usage.CompletionTokens = geminiResp.UsageMetadata.CandidatesTokenCount
	resp.Usage.TotalTokens = geminiResp.UsageMetadata.TotalTokenCount

	return &resp, nil
}
