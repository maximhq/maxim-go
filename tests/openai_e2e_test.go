package tests

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

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

	reqBody := map[string]interface{}{
		"model":    model,
		"messages": messages,
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
