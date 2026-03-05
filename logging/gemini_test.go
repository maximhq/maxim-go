package logging_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/joho/godotenv"
	"github.com/maximhq/maxim-go/logging"
)

// TestParseGeminiNativeResponse tests parsing of native Gemini API response format
func TestParseGeminiNativeResponse(t *testing.T) {
	// Sample native Gemini response
	geminiResponse := map[string]interface{}{
		"candidates": []map[string]interface{}{
			{
				"content": map[string]interface{}{
					"parts": []map[string]interface{}{
						{
							"text": "Hello! How can I help you today?",
						},
					},
					"role": "model",
				},
				"finishReason": "STOP",
				"index":        0,
			},
		},
		"usageMetadata": map[string]interface{}{
			"promptTokenCount":     10,
			"candidatesTokenCount": 20,
			"totalTokenCount":      30,
		},
		"modelVersion": "gemini-pro",
	}

	jsonData, err := json.Marshal(geminiResponse)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	result, err := logging.ParseGeminiResult(jsonData)
	if err != nil {
		t.Fatalf("ParseGeminiResult failed: %v", err)
	}

	// Validate result
	if result.Model != "gemini-pro" {
		t.Errorf("Expected model 'gemini-pro', got '%s'", result.Model)
	}

	if len(result.Choices) != 1 {
		t.Fatalf("Expected 1 choice, got %d", len(result.Choices))
	}

	if result.Choices[0].Message.Content != "Hello! How can I help you today?" {
		t.Errorf("Expected content 'Hello! How can I help you today?', got '%s'", result.Choices[0].Message.Content)
	}

	if result.Choices[0].Message.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", result.Choices[0].Message.Role)
	}

	if result.Choices[0].FinishReason != "stop" {
		t.Errorf("Expected finish reason 'stop', got '%s'", result.Choices[0].FinishReason)
	}

	if result.Usage.PromptTokens != 10 {
		t.Errorf("Expected 10 prompt tokens, got %d", result.Usage.PromptTokens)
	}

	if result.Usage.CompletionTokens != 20 {
		t.Errorf("Expected 20 completion tokens, got %d", result.Usage.CompletionTokens)
	}

	if result.Usage.TotalTokens != 30 {
		t.Errorf("Expected 30 total tokens, got %d", result.Usage.TotalTokens)
	}
}

// TestParseGeminiOpenAICompatible tests parsing of OpenAI-compatible Gemini response
func TestParseGeminiOpenAICompatible(t *testing.T) {
	// Sample OpenAI-compatible response
	openAIResponse := map[string]interface{}{
		"id":      "chatcmpl-123",
		"model":   "gemini-pro",
		"created": 1677652288,
		"choices": []map[string]interface{}{
			{
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": "This is a test response",
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     15,
			"completion_tokens": 25,
			"total_tokens":      40,
		},
	}

	jsonData, err := json.Marshal(openAIResponse)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	result, err := logging.ParseGeminiResult(jsonData)
	if err != nil {
		t.Fatalf("ParseGeminiResult failed: %v", err)
	}

	// Validate result
	if result.ID != "chatcmpl-123" {
		t.Errorf("Expected ID 'chatcmpl-123', got '%s'", result.ID)
	}

	if result.Model != "gemini-pro" {
		t.Errorf("Expected model 'gemini-pro', got '%s'", result.Model)
	}

	if len(result.Choices) != 1 {
		t.Fatalf("Expected 1 choice, got %d", len(result.Choices))
	}

	if result.Choices[0].Message.Content != "This is a test response" {
		t.Errorf("Expected content 'This is a test response', got '%s'", result.Choices[0].Message.Content)
	}
}

// TestParseGeminiWithFunctionCall tests parsing of Gemini response with function calls
func TestParseGeminiWithFunctionCall(t *testing.T) {
	// Sample Gemini response with function call
	geminiResponse := map[string]interface{}{
		"candidates": []map[string]interface{}{
			{
				"content": map[string]interface{}{
					"parts": []map[string]interface{}{
						{
							"text": "Let me check the weather for you.",
						},
						{
							"functionCall": map[string]interface{}{
								"name": "get_weather",
								"args": map[string]interface{}{
									"location": "San Francisco",
									"unit":     "celsius",
								},
							},
						},
					},
					"role": "model",
				},
				"finishReason": "STOP",
				"index":        0,
			},
		},
		"usageMetadata": map[string]interface{}{
			"promptTokenCount":     15,
			"candidatesTokenCount": 30,
			"totalTokenCount":      45,
		},
	}

	jsonData, err := json.Marshal(geminiResponse)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	result, err := logging.ParseGeminiResult(jsonData)
	if err != nil {
		t.Fatalf("ParseGeminiResult failed: %v", err)
	}

	// Validate result
	if len(result.Choices) != 1 {
		t.Fatalf("Expected 1 choice, got %d", len(result.Choices))
	}

	if result.Choices[0].Message.Content != "Let me check the weather for you." {
		t.Errorf("Expected text content, got '%s'", result.Choices[0].Message.Content)
	}

	if len(result.Choices[0].Message.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(result.Choices[0].Message.ToolCalls))
	}

	toolCall := result.Choices[0].Message.ToolCalls[0]
	if toolCall.Function.Name != "get_weather" {
		t.Errorf("Expected function name 'get_weather', got '%s'", toolCall.Function.Name)
	}

	if toolCall.Type != "function" {
		t.Errorf("Expected type 'function', got '%s'", toolCall.Type)
	}
}

// TestGeminiAPIIntegration tests real Gemini API integration
// This test requires GEMINI_API_KEY to be set in .env file
func TestGeminiAPIIntegration(t *testing.T) {
	// Load .env file
	err := godotenv.Load("../.env")
	if err != nil {
		t.Skip("Skipping integration test: .env file not found")
	}

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: GEMINI_API_KEY not set in .env")
	}

	// Make a real API call to Gemini
	model := "gemini-pro"
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, apiKey)

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{
						"text": "Say hello in one sentence",
					},
				},
			},
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("API returned non-200 status: %d, body: %s", resp.StatusCode, string(body))
	}

	// Parse the response using our parser
	result, err := logging.ParseGeminiResult(body)
	if err != nil {
		t.Fatalf("ParseGeminiResult failed: %v, response: %s", err, string(body))
	}

	// Validate the parsed result
	if result == nil {
		t.Fatal("Result is nil")
	}

	if len(result.Choices) == 0 {
		t.Fatal("No choices in result")
	}

	if result.Choices[0].Message.Content == "" {
		t.Error("Content is empty")
	}

	if result.Usage.TotalTokens == 0 {
		t.Error("Total tokens is 0")
	}

	// Log the result for inspection
	t.Logf("Successfully parsed Gemini response:")
	t.Logf("Model: %s", result.Model)
	t.Logf("Content: %s", result.Choices[0].Message.Content)
	t.Logf("Prompt tokens: %d", result.Usage.PromptTokens)
	t.Logf("Completion tokens: %d", result.Usage.CompletionTokens)
	t.Logf("Total tokens: %d", result.Usage.TotalTokens)
}

// TestGeminiFinishReasonMapping tests the mapping of Gemini finish reasons
func TestGeminiFinishReasonMapping(t *testing.T) {
	testCases := []struct {
		geminiReason   string
		expectedReason string
	}{
		{"STOP", "stop"},
		{"MAX_TOKENS", "length"},
		{"SAFETY", "content_filter"},
		{"RECITATION", "content_filter"},
		{"OTHER", "stop"},
	}

	for _, tc := range testCases {
		t.Run(tc.geminiReason, func(t *testing.T) {
			geminiResponse := map[string]interface{}{
				"candidates": []map[string]interface{}{
					{
						"content": map[string]interface{}{
							"parts": []map[string]interface{}{
								{"text": "test"},
							},
							"role": "model",
						},
						"finishReason": tc.geminiReason,
						"index":        0,
					},
				},
				"usageMetadata": map[string]interface{}{
					"promptTokenCount":     1,
					"candidatesTokenCount": 1,
					"totalTokenCount":      2,
				},
			}

			jsonData, err := json.Marshal(geminiResponse)
			if err != nil {
				t.Fatalf("Failed to marshal test data: %v", err)
			}

			result, err := logging.ParseGeminiResult(jsonData)
			if err != nil {
				t.Fatalf("ParseGeminiResult failed: %v", err)
			}

			if result.Choices[0].FinishReason != tc.expectedReason {
				t.Errorf("Expected finish reason '%s', got '%s'", tc.expectedReason, result.Choices[0].FinishReason)
			}
		})
	}
}

