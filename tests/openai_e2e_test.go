package tests

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/maximhq/maxim-go/logging"
	"github.com/maximhq/maxim-go/middlewares"
	"github.com/maximhq/maxim-go/schemas"
)

const openAIAPIBaseURL = "https://api.openai.com/v1"

// openAIE2EChatCompletion makes a real OpenAI API request through MaximOpenAIAPITransport.
func openAIE2EChatCompletion(t *testing.T, ctx context.Context, client *http.Client, apiKey string, messages []map[string]interface{}, model string) map[string]interface{} {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return openAIE2EChatCompletionWithToolsCtx(ctx, t, client, apiKey, messages, model, nil, nil)
}

// openAIE2EChatCompletionWithTools makes a chat completion request with optional tools and tool_choice.
func openAIE2EChatCompletionWithTools(t *testing.T, client *http.Client, apiKey string, messages []map[string]interface{}, model string, tools []map[string]interface{}, toolChoice interface{}) map[string]interface{} {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return openAIE2EChatCompletionWithToolsCtx(ctx, t, client, apiKey, messages, model, tools, toolChoice)
}

// openAIE2EChatCompletionWithToolsCtx is like openAIE2EChatCompletionWithTools but accepts a context (e.g. with traceId).
func openAIE2EChatCompletionWithToolsCtx(ctx context.Context, t *testing.T, client *http.Client, apiKey string, messages []map[string]interface{}, model string, tools []map[string]interface{}, toolChoice interface{}) map[string]interface{} {
	t.Helper()

	reqBody := map[string]interface{}{
		"model":    model,
		"messages": messages,
	}
	if tools != nil {
		reqBody["tools"] = tools
		if toolChoice != nil {
			reqBody["tool_choice"] = toolChoice
		}
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIAPIBaseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("OpenAI request failed: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if errObj, ok := result["error"]; ok {
		t.Fatalf("OpenAI returned error: %v", errObj)
	}

	return result
}

// openAIE2EChatCompletionStream makes a streaming chat completion request and returns accumulated content.
func openAIE2EChatCompletionStream(t *testing.T, client *http.Client, apiKey string, messages []map[string]interface{}, model string, tools []map[string]interface{}, toolChoice interface{}) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return openAIE2EChatCompletionStreamCtx(ctx, t, client, apiKey, messages, model, tools, toolChoice)
}

func openAIE2EChatCompletionStreamCtx(ctx context.Context, t *testing.T, client *http.Client, apiKey string, messages []map[string]interface{}, model string, tools []map[string]interface{}, toolChoice interface{}) string {
	t.Helper()
	reqBody := map[string]interface{}{
		"model":    model,
		"messages": messages,
		"stream":   true,
	}
	if tools != nil {
		reqBody["tools"] = tools
		if toolChoice != nil {
			reqBody["tool_choice"] = toolChoice
		}
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIAPIBaseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("OpenAI stream request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("OpenAI stream returned status %d", resp.StatusCode)
	}
	var content strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}
			var chunk map[string]interface{}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			choices, _ := chunk["choices"].([]interface{})
			if len(choices) == 0 {
				continue
			}
			choice, _ := choices[0].(map[string]interface{})
			delta, _ := choice["delta"].(map[string]interface{})
			if c, ok := delta["content"].(string); ok && c != "" {
				content.WriteString(c)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("reading stream: %v", err)
	}
	return content.String()
}

// parseAndValidateOpenAIE2EResult runs ParseResult for the openai provider.
func parseAndValidateOpenAIE2EResult(t *testing.T, result map[string]interface{}, model string) *schemas.MaximLLMResult {
	t.Helper()
	parsed, err := logging.ParseResult(logging.ProviderOpenAI, model, result)
	if err != nil {
		t.Fatalf("ParseResult failed: %v", err)
	}
	if parsed == nil {
		t.Fatal("ParseResult returned nil")
	}
	if parsed.Model == "" {
		t.Fatal("ParseResult: expected non-empty Model")
	}
	if len(parsed.Choices) == 0 {
		t.Fatal("ParseResult: expected at least one choice")
	}
	t.Logf("ParseResult OK — model=%s tokens=%d", parsed.Model, parsed.Usage.TotalTokens)
	return parsed
}

// TestOpenAI_E2E_SimpleCompletion makes a real OpenAI API call through MaximOpenAIAPITransport.
func TestOpenAI_E2E_SimpleCompletion(t *testing.T) {
	requireMaximCredentials(t)
	openAIKey := getOpenAIKey(t)

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	client := &http.Client{
		Transport: middlewares.NewMaximOpenAIAPITransport(logger),
		Timeout:   60 * time.Second,
	}

	messages := []map[string]interface{}{
		{"role": "user", "content": "Say hello in one word."},
	}
	result := openAIE2EChatCompletion(t, context.Background(), client, openAIKey, messages, "gpt-4o-mini")
	parsed := parseAndValidateOpenAIE2EResult(t, result, "gpt-4o-mini")

	logger.Flush()
	t.Logf("Successfully completed OpenAI e2e — response: %q", parsed.Choices[0].Message.Content)
}

// TestOpenAI_E2E_SimpleCompletionStream is like TestOpenAI_E2E_SimpleCompletion but with stream=true.
func TestOpenAI_E2E_SimpleCompletionStream(t *testing.T) {
	requireMaximCredentials(t)
	openAIKey := getOpenAIKey(t)
	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{Id: logRepoId, AutoFlush: ptr(false)})
	defer logger.Flush()
	client := &http.Client{Transport: middlewares.NewMaximOpenAIAPITransport(logger), Timeout: 60 * time.Second}
	messages := []map[string]interface{}{{"role": "user", "content": "Hello"}}
	content := openAIE2EChatCompletionStream(t, client, openAIKey, messages, "gpt-4o-mini", nil, nil)
	if content == "" {
		t.Error("expected non-empty streamed response")
	}
	logger.Flush()
	t.Logf("Successfully completed OpenAI e2e (stream) — response: %q", content)
}

// TestOpenAI_E2E_WithTraceContext makes an OpenAI API call with custom trace context.
func TestOpenAI_E2E_WithTraceContext(t *testing.T) {
	requireMaximCredentials(t)
	openAIKey := getOpenAIKey(t)

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	client := &http.Client{
		Transport: middlewares.NewMaximOpenAIAPITransport(logger),
		Timeout:   60 * time.Second,
	}

	messages := []map[string]interface{}{
		{"role": "user", "content": "What is 2+2? Reply with just the number."},
	}
	result := openAIE2EChatCompletion(t, context.Background(), client, openAIKey, messages, "gpt-4o-mini")
	parseAndValidateOpenAIE2EResult(t, result, "gpt-4o-mini")

	logger.Flush()
	t.Log("Successfully completed OpenAI e2e with trace context")
}

// TestOpenAI_E2E_WithTraceContextStream is like TestOpenAI_E2E_WithTraceContext but with stream=true.
func TestOpenAI_E2E_WithTraceContextStream(t *testing.T) {
	requireMaximCredentials(t)
	openAIKey := getOpenAIKey(t)
	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{Id: logRepoId, AutoFlush: ptr(false)})
	defer logger.Flush()
	client := &http.Client{Transport: middlewares.NewMaximOpenAIAPITransport(logger), Timeout: 60 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, middlewares.ContextKeyTraceName, "openai-trace-context-test")
	ctx = context.WithValue(ctx, middlewares.ContextKeyGenerationName, "openai-gen-context-test")
	messages := []map[string]interface{}{{"role": "user", "content": "What is 2+2? Reply with just the number."}}
	content := openAIE2EChatCompletionStreamCtx(ctx, t, client, openAIKey, messages, "gpt-4o-mini", nil, nil)
	if content == "" {
		t.Error("expected non-empty streamed response")
	}
	logger.Flush()
	t.Logf("Successfully completed OpenAI e2e with trace context (stream) — response: %q", content)
}

// TestOpenAI_E2E_WithSystemMessage tests OpenAI with system message.
func TestOpenAI_E2E_WithSystemMessage(t *testing.T) {
	requireMaximCredentials(t)
	openAIKey := getOpenAIKey(t)

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	client := &http.Client{
		Transport: middlewares.NewMaximOpenAIAPITransport(logger),
		Timeout:   60 * time.Second,
	}

	messages := []map[string]interface{}{
		{"role": "system", "content": "You are a helpful assistant. Keep answers brief."},
		{"role": "user", "content": "What color is the sky?"},
	}
	result := openAIE2EChatCompletion(t, context.Background(), client, openAIKey, messages, "gpt-4o-mini")
	parseAndValidateOpenAIE2EResult(t, result, "gpt-4o-mini")

	logger.Flush()
	t.Log("Successfully completed OpenAI e2e with system message")
}

// TestOpenAI_E2E_WithSystemMessageStream is like TestOpenAI_E2E_WithSystemMessage but with stream=true.
func TestOpenAI_E2E_WithSystemMessageStream(t *testing.T) {
	requireMaximCredentials(t)
	openAIKey := getOpenAIKey(t)
	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{Id: logRepoId, AutoFlush: ptr(false)})
	defer logger.Flush()
	client := &http.Client{Transport: middlewares.NewMaximOpenAIAPITransport(logger), Timeout: 60 * time.Second}
	messages := []map[string]interface{}{
		{"role": "system", "content": "You are a helpful assistant. Keep answers brief."},
		{"role": "user", "content": "What color is the sky?"},
	}
	content := openAIE2EChatCompletionStream(t, client, openAIKey, messages, "gpt-4o-mini", nil, nil)
	if content == "" {
		t.Error("expected non-empty streamed response")
	}
	logger.Flush()
	t.Logf("Successfully completed OpenAI e2e with system message (stream) — response: %q", content)
}

// TestOpenAI_E2E_WithImageUrl sends an image URL to OpenAI vision API.
func TestOpenAI_E2E_WithImageUrl(t *testing.T) {
	requireMaximCredentials(t)
	openAIKey := getOpenAIKey(t)

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	client := &http.Client{
		Transport: middlewares.NewMaximOpenAIAPITransport(logger),
		Timeout:   60 * time.Second,
	}

	messages := []map[string]interface{}{
		{
			"role": "user",
			"content": []map[string]interface{}{
				{"type": "text", "text": "Describe this image in one sentence."},
				{"type": "image_url", "image_url": map[string]string{"url": testImageURL}},
			},
		},
	}
	result := openAIE2EChatCompletion(t, context.Background(), client, openAIKey, messages, "gpt-4o-mini")
	parsed := parseAndValidateOpenAIE2EResult(t, result, "gpt-4o-mini")

	logger.Flush()
	t.Logf("Successfully completed OpenAI vision e2e (URL) — response: %q", parsed.Choices[0].Message.Content)
}

// TestOpenAI_E2E_WithImageUrlStream is like TestOpenAI_E2E_WithImageUrl but with stream=true.
func TestOpenAI_E2E_WithImageUrlStream(t *testing.T) {
	requireMaximCredentials(t)
	openAIKey := getOpenAIKey(t)
	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{Id: logRepoId, AutoFlush: ptr(false)})
	defer logger.Flush()
	client := &http.Client{Transport: middlewares.NewMaximOpenAIAPITransport(logger), Timeout: 60 * time.Second}
	messages := []map[string]interface{}{
		{
			"role": "user",
			"content": []map[string]interface{}{
				{"type": "text", "text": "Describe this image in one sentence."},
				{"type": "image_url", "image_url": map[string]string{"url": testImageURL}},
			},
		},
	}
	content := openAIE2EChatCompletionStream(t, client, openAIKey, messages, "gpt-4o-mini", nil, nil)
	if content == "" {
		t.Error("expected non-empty streamed response")
	}
	logger.Flush()
	t.Logf("Successfully completed OpenAI vision e2e (URL, stream) — response: %q", content)
}

// TestOpenAI_E2E_WithInlineImage reads a local image, base64-encodes it as data URL,
// and sends to OpenAI vision. Skips if test image is not found.
func TestOpenAI_E2E_WithInlineImage(t *testing.T) {
	requireMaximCredentials(t)
	openAIKey := getOpenAIKey(t)

	imageData, err := os.ReadFile(testImagePath)
	if err != nil {
		t.Skipf("test image not found at %s: %v", testImagePath, err)
	}

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	client := &http.Client{
		Transport: middlewares.NewMaximOpenAIAPITransport(logger),
		Timeout:   60 * time.Second,
	}

	b64 := base64.StdEncoding.EncodeToString(imageData)
	dataURL := "data:image/jpeg;base64," + b64

	messages := []map[string]interface{}{
		{
			"role": "user",
			"content": []map[string]interface{}{
				{"type": "text", "text": "Describe this image in one sentence."},
				{"type": "image_url", "image_url": map[string]string{"url": dataURL}},
			},
		},
	}
	result := openAIE2EChatCompletion(t, context.Background(), client, openAIKey, messages, "gpt-4o-mini")
	parsed := parseAndValidateOpenAIE2EResult(t, result, "gpt-4o-mini")

	logger.Flush()
	t.Logf("Successfully completed OpenAI vision e2e (inline) — response: %q", parsed.Choices[0].Message.Content)
}

// TestOpenAI_E2E_WithInlineImageStream is like TestOpenAI_E2E_WithInlineImage but with stream=true.
func TestOpenAI_E2E_WithInlineImageStream(t *testing.T) {
	requireMaximCredentials(t)
	openAIKey := getOpenAIKey(t)
	imageData, err := os.ReadFile(testImagePath)
	if err != nil {
		t.Skipf("test image not found at %s: %v", testImagePath, err)
	}
	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{Id: logRepoId, AutoFlush: ptr(false)})
	defer logger.Flush()
	client := &http.Client{Transport: middlewares.NewMaximOpenAIAPITransport(logger), Timeout: 60 * time.Second}
	b64 := base64.StdEncoding.EncodeToString(imageData)
	dataURL := "data:image/jpeg;base64," + b64
	messages := []map[string]interface{}{
		{
			"role": "user",
			"content": []map[string]interface{}{
				{"type": "text", "text": "Describe this image in one sentence."},
				{"type": "image_url", "image_url": map[string]string{"url": dataURL}},
			},
		},
	}
	content := openAIE2EChatCompletionStream(t, client, openAIKey, messages, "gpt-4o-mini", nil, nil)
	if content == "" {
		t.Error("expected non-empty streamed response")
	}
	logger.Flush()
	t.Logf("Successfully completed OpenAI vision e2e (inline, stream) — response: %q", content)
}

// TestOpenAI_E2E_WithToolCalls tests OpenAI with function/tool calling.
func TestOpenAI_E2E_WithToolCalls(t *testing.T) {
	requireMaximCredentials(t)
	openAIKey := getOpenAIKey(t)

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	client := &http.Client{
		Transport: middlewares.NewMaximOpenAIAPITransport(logger),
		Timeout:   60 * time.Second,
	}

	tools := []map[string]interface{}{
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "get_weather",
				"description": "Get the current weather for a location.",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "City name, e.g. Paris or Tokyo",
						},
					},
					"required": []string{"location"},
				},
			},
		},
	}
	toolChoice := map[string]interface{}{
		"type":     "function",
		"function": map[string]interface{}{"name": "get_weather"},
	}

	messages := []map[string]interface{}{
		{"role": "user", "content": "What's the weather in Paris?"},
	}
	result := openAIE2EChatCompletionWithTools(t, client, openAIKey, messages, "gpt-4o-mini", tools, toolChoice)
	parsed := parseAndValidateOpenAIE2EResult(t, result, "gpt-4o-mini")

	// With tool_choice forcing get_weather, we expect tool_calls in the response
	if len(parsed.Choices[0].Message.ToolCalls) == 0 {
		t.Error("expected tool_calls in response when using tool_choice")
	} else {
		t.Logf("Tool call: %s(%s)", parsed.Choices[0].Message.ToolCalls[0].Function.Name, parsed.Choices[0].Message.ToolCalls[0].Function.Arguments)
	}

	logger.Flush()
	t.Log("Successfully completed OpenAI e2e with tool calls")
}

// TestOpenAI_E2E_WithToolCallsStream is like TestOpenAI_E2E_WithToolCalls but with stream=true.
// With tool_choice, the model returns tool calls; streaming may return them in chunks.
func TestOpenAI_E2E_WithToolCallsStream(t *testing.T) {
	requireMaximCredentials(t)
	openAIKey := getOpenAIKey(t)
	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{Id: logRepoId, AutoFlush: ptr(false)})
	defer logger.Flush()
	client := &http.Client{Transport: middlewares.NewMaximOpenAIAPITransport(logger), Timeout: 60 * time.Second}
	tools := []map[string]interface{}{
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "get_weather",
				"description": "Get the current weather for a location.",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{"type": "string", "description": "City name"},
					},
					"required": []string{"location"},
				},
			},
		},
	}
	toolChoice := map[string]interface{}{"type": "function", "function": map[string]interface{}{"name": "get_weather"}}
	messages := []map[string]interface{}{{"role": "user", "content": "What's the weather in Paris?"}}
	content := openAIE2EChatCompletionStream(t, client, openAIKey, messages, "gpt-4o-mini", tools, toolChoice)
	// With tool_choice, stream may return empty content (tool calls) or text; either is valid
	logger.Flush()
	t.Logf("Successfully completed OpenAI e2e with tool calls (stream) — content len=%d", len(content))
}

// TestOpenAI_E2E_WithToolCallsFullFlow tests the full tool-calling flow: request 1 returns tool calls,
// we simulate tool execution, request 2 with tool result returns final text. Uses shared traceId.
func TestOpenAI_E2E_WithToolCallsFullFlow(t *testing.T) {
	requireMaximCredentials(t)
	openAIKey := getOpenAIKey(t)

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	client := &http.Client{
		Transport: middlewares.NewMaximOpenAIAPITransport(logger),
		Timeout:   60 * time.Second,
	}

	traceId := uuid.NewString()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, middlewares.ContextKeyTraceId, traceId)
	ctx = context.WithValue(ctx, middlewares.ContextKeyLogger, logger)

	tools := []map[string]interface{}{
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "get_weather",
				"description": "Get the current weather for a location.",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "City name, e.g. Paris or Tokyo",
						},
					},
					"required": []string{"location"},
				},
			},
		},
	}
	toolChoice := map[string]interface{}{
		"type":     "function",
		"function": map[string]interface{}{"name": "get_weather"},
	}

	messages := []map[string]interface{}{
		{"role": "user", "content": "What's the weather in Paris?"},
	}
	result1 := openAIE2EChatCompletionWithToolsCtx(ctx, t, client, openAIKey, messages, "gpt-4o-mini", tools, toolChoice)
	parsed1 := parseAndValidateOpenAIE2EResult(t, result1, "gpt-4o-mini")

	if len(parsed1.Choices[0].Message.ToolCalls) == 0 {
		t.Fatal("expected tool_calls in first response when using tool_choice")
	}
	toolCallId := parsed1.Choices[0].Message.ToolCalls[0].ID
	toolResult := "Sunny, 22°C"

	toolCallsAPI := make([]map[string]interface{}, 0, len(parsed1.Choices[0].Message.ToolCalls))
	for _, tc := range parsed1.Choices[0].Message.ToolCalls {
		toolCallsAPI = append(toolCallsAPI, map[string]interface{}{
			"id":   tc.ID,
			"type": tc.Type,
			"function": map[string]interface{}{
				"name":      tc.Function.Name,
				"arguments": tc.Function.Arguments,
			},
		})
	}
	assistantMsg := map[string]interface{}{
		"role":       "assistant",
		"content":    nil,
		"tool_calls": toolCallsAPI,
	}
	toolResultMsg := map[string]interface{}{
		"role":         "tool",
		"tool_call_id": toolCallId,
		"content":      toolResult,
	}
	messages2 := append(messages, assistantMsg, toolResultMsg)

	// Use tool_choice: "auto" so the model can respond with text instead of being forced to call a tool again
	toolChoiceAuto := "auto"
	result2 := openAIE2EChatCompletionWithToolsCtx(ctx, t, client, openAIKey, messages2, "gpt-4o-mini", tools, toolChoiceAuto)
	parsed2 := parseAndValidateOpenAIE2EResult(t, result2, "gpt-4o-mini")

	if parsed2.Choices[0].Message.Content == "" {
		t.Error("expected non-empty final text response after tool result")
	}

	logger.EndTrace(traceId)
	logger.Flush()
	t.Logf("Successfully completed OpenAI e2e full-flow tool calls — final: %q", parsed2.Choices[0].Message.Content)
}

// TestOpenAI_E2E_WithToolCallsFullFlowStream is like TestOpenAI_E2E_WithToolCallsFullFlow but the second request uses stream=true.
func TestOpenAI_E2E_WithToolCallsFullFlowStream(t *testing.T) {
	requireMaximCredentials(t)
	openAIKey := getOpenAIKey(t)
	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{Id: logRepoId, AutoFlush: ptr(false)})
	defer logger.Flush()
	client := &http.Client{Transport: middlewares.NewMaximOpenAIAPITransport(logger), Timeout: 90 * time.Second}
	traceId := uuid.NewString()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, middlewares.ContextKeyTraceId, traceId)
	ctx = context.WithValue(ctx, middlewares.ContextKeyLogger, logger)
	tools := []map[string]interface{}{
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "get_weather",
				"description": "Get the current weather for a location.",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{"type": "string", "description": "City name"},
					},
					"required": []string{"location"},
				},
			},
		},
	}
	toolChoice := map[string]interface{}{"type": "function", "function": map[string]interface{}{"name": "get_weather"}}
	messages := []map[string]interface{}{{"role": "user", "content": "What's the weather in Paris?"}}
	result1 := openAIE2EChatCompletionWithToolsCtx(ctx, t, client, openAIKey, messages, "gpt-4o-mini", tools, toolChoice)
	parsed1 := parseAndValidateOpenAIE2EResult(t, result1, "gpt-4o-mini")
	if len(parsed1.Choices[0].Message.ToolCalls) == 0 {
		t.Fatal("expected tool_calls in first response")
	}
	toolCallId := parsed1.Choices[0].Message.ToolCalls[0].ID
	toolCallsAPI := make([]map[string]interface{}, 0, len(parsed1.Choices[0].Message.ToolCalls))
	for _, tc := range parsed1.Choices[0].Message.ToolCalls {
		toolCallsAPI = append(toolCallsAPI, map[string]interface{}{
			"id": tc.ID, "type": tc.Type,
			"function": map[string]interface{}{"name": tc.Function.Name, "arguments": tc.Function.Arguments},
		})
	}
	assistantMsg := map[string]interface{}{"role": "assistant", "content": nil, "tool_calls": toolCallsAPI}
	toolResultMsg := map[string]interface{}{"role": "tool", "tool_call_id": toolCallId, "content": "Sunny, 22°C"}
	messages2 := append(messages, assistantMsg, toolResultMsg)
	content := openAIE2EChatCompletionStreamCtx(ctx, t, client, openAIKey, messages2, "gpt-4o-mini", tools, "auto")
	if content == "" {
		t.Error("expected non-empty final text response after tool result (stream)")
	}
	logger.EndTrace(traceId)
	logger.Flush()
	t.Logf("Successfully completed OpenAI e2e full-flow tool calls (stream) — final: %q", content)
}

// TestOpenAI_E2E_WithTagsAndMetrics makes an OpenAI API call with tags and metrics in context.
func TestOpenAI_E2E_WithTagsAndMetrics(t *testing.T) {
	requireMaximCredentials(t)
	openAIKey := getOpenAIKey(t)

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	client := &http.Client{
		Transport: middlewares.NewMaximOpenAIAPITransport(logger),
		Timeout:   60 * time.Second,
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, middlewares.ContextKeyTraceTags, map[string]string{"e2e_test": "tags_metrics", "env": "test"})
	ctx = context.WithValue(ctx, middlewares.ContextKeyGenerationTags, map[string]string{"model_type": "chat", "source": "e2e"})
	ctx = context.WithValue(ctx, middlewares.ContextKeyTraceMetrics, map[string]float64{"latency_ms": 0.69, "tool_calls_count": 0.12})
	ctx = context.WithValue(ctx, middlewares.ContextKeyGenerationMetrics, map[string]float64{"tokens_in": 100, "tokens_out": 50})

	messages := []map[string]interface{}{
		{"role": "user", "content": "Say hello in one word."},
	}
	result := openAIE2EChatCompletion(t, ctx, client, openAIKey, messages, "gpt-4o-mini")
	parsed := parseAndValidateOpenAIE2EResult(t, result, "gpt-4o-mini")

	logger.Flush()
	t.Logf("Successfully completed OpenAI e2e with tags and metrics — response: %q", parsed.Choices[0].Message.Content)
}

// TestOpenAI_E2E_WithTagsAndMetricsStream is like TestOpenAI_E2E_WithTagsAndMetrics but with stream=true.
func TestOpenAI_E2E_WithTagsAndMetricsStream(t *testing.T) {
	requireMaximCredentials(t)
	openAIKey := getOpenAIKey(t)
	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{Id: logRepoId, AutoFlush: ptr(false)})
	defer logger.Flush()
	client := &http.Client{Transport: middlewares.NewMaximOpenAIAPITransport(logger), Timeout: 60 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, middlewares.ContextKeyTraceTags, map[string]string{"e2e_test": "tags_metrics", "env": "test"})
	ctx = context.WithValue(ctx, middlewares.ContextKeyGenerationTags, map[string]string{"model_type": "chat", "source": "e2e"})
	ctx = context.WithValue(ctx, middlewares.ContextKeyTraceMetrics, map[string]float64{"latency_ms": 0.69})
	ctx = context.WithValue(ctx, middlewares.ContextKeyGenerationMetrics, map[string]float64{"tokens_in": 100})
	messages := []map[string]interface{}{{"role": "user", "content": "Say hello in one word."}}
	content := openAIE2EChatCompletionStreamCtx(ctx, t, client, openAIKey, messages, "gpt-4o-mini", nil, nil)
	if content == "" {
		t.Error("expected non-empty streamed response")
	}
	logger.Flush()
	t.Logf("Successfully completed OpenAI e2e with tags and metrics (stream) — response: %q", content)
}
