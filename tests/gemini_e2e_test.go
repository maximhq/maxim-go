package tests

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/maximhq/maxim-go/logging"
	"github.com/maximhq/maxim-go/middlewares"
	"github.com/maximhq/maxim-go/schemas"
)

const geminiAPIBaseURL = "https://generativelanguage.googleapis.com/v1beta"

// getGeminiAPIKey returns the Gemini API key from the environment.
func getGeminiAPIKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		key = os.Getenv("GOOGLE_API_KEY")
	}
	if key == "" {
		t.Skip("GEMINI_API_KEY or GOOGLE_API_KEY not set, skipping Gemini integration test")
	}
	return key
}

// geminiChatCompletion makes a real Gemini API request through MaximGeminiTransport
// and returns the raw JSON response body as a map.
func geminiChatCompletion(t *testing.T, ctx context.Context, transport *middlewares.MaximGeminiTransport, apiKey string, prompt string, model string) map[string]interface{} {
	t.Helper()

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role":  "user",
				"parts": []map[string]interface{}{{"text": prompt}},
			},
		},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	url := geminiAPIBaseURL + "/models/" + model + ":generateContent"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey)

	resp, err := transport.Do(req)
	if err != nil {
		t.Fatalf("Gemini request failed: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if errObj, ok := result["error"]; ok {
		t.Fatalf("Gemini returned error: %v", errObj)
	}

	return result
}

// geminiRequest makes a Gemini API request with a custom request body.
func geminiRequest(t *testing.T, ctx context.Context, transport *middlewares.MaximGeminiTransport, apiKey string, reqBody map[string]interface{}, model string) map[string]interface{} {
	t.Helper()

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	url := geminiAPIBaseURL + "/models/" + model + ":generateContent"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey)

	resp, err := transport.Do(req)
	if err != nil {
		t.Fatalf("Gemini request failed: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if errObj, ok := result["error"]; ok {
		t.Fatalf("Gemini returned error: %v", errObj)
	}

	return result
}

// parseAndValidateGeminiResult runs ParseResult for the gemini provider and fails
// the test if parsing returns an error or a nil result.
func parseAndValidateGeminiResult(t *testing.T, result map[string]interface{}, model string) *schemas.MaximLLMResult {
	t.Helper()
	parsed, err := logging.ParseResult(logging.ProviderGemini, model, result)
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

// TestGemini_E2E_SimpleCompletion makes a real Gemini API call through MaximGeminiTransport
// and verifies the middleware traces the request to Maxim.
func TestGemini_E2E_SimpleCompletion(t *testing.T) {
	requireMaximCredentials(t)
	geminiKey := getGeminiAPIKey(t)

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	transport := middlewares.NewMaximGeminiTransport(logger)

	result := geminiChatCompletion(t, context.Background(), transport, geminiKey, "Say hello in one word.", "gemini-2.5-flash")
	parsed := parseAndValidateGeminiResult(t, result, "gemini-2.5-flash")

	logger.Flush()
	t.Logf("Successfully completed Gemini e2e — response: %q", parsed.Choices[0].Message.Content)
}

// TestGemini_E2E_WithTraceContext makes a Gemini API call with custom trace context
// (traceId, traceName, generationName) and verifies the flow.
func TestGemini_E2E_WithTraceContext(t *testing.T) {
	requireMaximCredentials(t)
	geminiKey := getGeminiAPIKey(t)

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	transport := middlewares.NewMaximGeminiTransport(logger)

	ctx := context.Background()
	ctx = context.WithValue(ctx, middlewares.ContextKeyTraceName, "gemini-trace-context-test")
	ctx = context.WithValue(ctx, middlewares.ContextKeyGenerationName, "gemini-gen-context-test")

	result := geminiChatCompletion(t, ctx, transport, geminiKey, "What is 2+2? Reply with just the number.", "gemini-2.5-flash")
	parseAndValidateGeminiResult(t, result, "gemini-2.5-flash")

	logger.Flush()
	t.Log("Successfully completed Gemini e2e with trace context")
}

// TestGemini_E2E_WithSystemInstruction tests Gemini with system instruction
// (uses a different request format).
func TestGemini_E2E_WithSystemInstruction(t *testing.T) {
	requireMaximCredentials(t)
	geminiKey := getGeminiAPIKey(t)

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	transport := middlewares.NewMaximGeminiTransport(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role":  "user",
				"parts": []map[string]interface{}{{"text": "What color is the sky?"}},
			},
		},
		"systemInstruction": map[string]interface{}{
			"parts": []map[string]interface{}{{"text": "You are a helpful assistant. Keep answers brief."}},
		},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	url := geminiAPIBaseURL + "/models/gemini-2.5-flash:generateContent"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", geminiKey)

	resp, err := transport.Do(req)
	if err != nil {
		t.Fatalf("Gemini request failed: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if errObj, ok := result["error"]; ok {
		t.Fatalf("Gemini returned error: %v", errObj)
	}

	parseAndValidateGeminiResult(t, result, "gemini-2.5-flash")

	logger.Flush()
	t.Log("Successfully completed Gemini e2e with system instruction")
}

// TestGemini_E2E_WithInlineImage reads a local image, base64-encodes it as inline_data,
// and sends to Gemini. Skips if test image is not found.
func TestGemini_E2E_WithInlineImage(t *testing.T) {
	requireMaximCredentials(t)
	geminiKey := getGeminiAPIKey(t)

	imageData, err := os.ReadFile(testImagePath)
	if err != nil {
		t.Skipf("test image not found at %s: %v", testImagePath, err)
	}

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	transport := middlewares.NewMaximGeminiTransport(logger)

	b64 := base64.StdEncoding.EncodeToString(imageData)
	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]interface{}{
					{
						"inline_data": map[string]interface{}{
							"mime_type": "image/jpeg",
							"data":      b64,
						},
					},
					{"text": "Describe this image in one sentence."},
				},
			},
		},
	}

	result := geminiRequest(t, context.Background(), transport, geminiKey, reqBody, "gemini-2.5-flash")
	parsed := parseAndValidateGeminiResult(t, result, "gemini-2.5-flash")

	logger.Flush()
	t.Logf("Successfully completed Gemini vision e2e (inline) — response: %q", parsed.Choices[0].Message.Content)
}

// TestGemini_E2E_WithImageUrl fetches an image from URL, base64-encodes it, and sends to Gemini.
func TestGemini_E2E_WithImageUrl(t *testing.T) {
	requireMaximCredentials(t)
	geminiKey := getGeminiAPIKey(t)

	// Fetch image from URL
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Get(testImageURL)
	if err != nil {
		t.Skipf("failed to fetch test image from URL: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skipf("test image URL returned status %d", resp.StatusCode)
	}
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Skipf("failed to read image from URL: %v", err)
	}

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	transport := middlewares.NewMaximGeminiTransport(logger)

	b64 := base64.StdEncoding.EncodeToString(imageData)
	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]interface{}{
					{
						"inline_data": map[string]interface{}{
							"mime_type": "image/jpeg",
							"data":      b64,
						},
					},
					{"text": "Describe this image in one sentence."},
				},
			},
		},
	}

	result := geminiRequest(t, context.Background(), transport, geminiKey, reqBody, "gemini-2.5-flash")
	parsed := parseAndValidateGeminiResult(t, result, "gemini-2.5-flash")

	logger.Flush()
	t.Logf("Successfully completed Gemini vision e2e (URL) — response: %q", parsed.Choices[0].Message.Content)
}

// TestGemini_E2E_WithTagsAndMetrics makes a Gemini API call with tags and metrics in context
// and verifies the flow completes successfully (tags/metrics are sent to Maxim).
func TestGemini_E2E_WithTagsAndMetrics(t *testing.T) {
	requireMaximCredentials(t)
	geminiKey := getGeminiAPIKey(t)

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	transport := middlewares.NewMaximGeminiTransport(logger)

	ctx := context.Background()
	ctx = context.WithValue(ctx, middlewares.ContextKeyTraceTags, map[string]string{"e2e_test": "tags_metrics", "env": "test"})
	ctx = context.WithValue(ctx, middlewares.ContextKeyGenerationTags, map[string]string{"model_type": "chat", "source": "e2e"})
	ctx = context.WithValue(ctx, middlewares.ContextKeyTraceMetrics, map[string]float64{"latency_ms": 0.69, "tool_calls_count": 0.12})
	ctx = context.WithValue(ctx, middlewares.ContextKeyGenerationMetrics, map[string]float64{"tokens_in": 100, "tokens_out": 50})

	result := geminiChatCompletion(t, ctx, transport, geminiKey, "Say hello in one word.", "gemini-2.5-flash")
	parsed := parseAndValidateGeminiResult(t, result, "gemini-2.5-flash")

	logger.Flush()
	t.Logf("Successfully completed Gemini e2e with tags and metrics — response: %q", parsed.Choices[0].Message.Content)
}
