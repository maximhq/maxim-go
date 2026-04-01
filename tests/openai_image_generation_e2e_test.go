package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/maximhq/maxim-go/logging"
	"github.com/maximhq/maxim-go/schemas"
)

// openAIImageGeneration calls the real OpenAI images/generations endpoint and returns the raw response map.
func openAIImageGeneration(t *testing.T, apiKey string, prompt, model, responseFormat, size string) map[string]interface{} {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	reqBody := map[string]interface{}{
		"model":           model,
		"prompt":          prompt,
		"n":               1,
		"response_format": responseFormat,
		"size":            size,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal image generation request: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/images/generations", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("OpenAI image generation request failed: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode image generation response: %v", err)
	}
	if errObj, ok := result["error"]; ok {
		t.Fatalf("OpenAI image generation returned error: %v", errObj)
	}
	return result
}

// TestOpenAI_E2E_ImageGeneration_DallE2_B64JSON uses dall-e-2 with b64_json response format.
// b64_json causes the SDK to upload actual image bytes to Maxim (via FileDataAttachment),
// so the attachment contains persistent image data rather than an expiring URL reference.
func TestOpenAI_E2E_ImageGeneration_DallE2_B64JSON(t *testing.T) {
	requireMaximCredentials(t)
	openAIKey := getOpenAIKey(t)

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	const prompt = "a simple red circle on a white background"
	traceID := uuid.NewString()
	genID := uuid.NewString()

	trace := logger.Trace(&logging.TraceConfig{Id: traceID})
	logger.SetTraceInput(traceID, prompt)
	logger.AddGenerationToTrace(traceID, &logging.GenerationConfig{
		Id:       genID,
		Provider: logging.ProviderOpenAI,
		Model:    "dall-e-2",
		Messages: []schemas.CompletionRequest{
			{Role: "user", Content: prompt},
		},
		ModelParameters: map[string]interface{}{
			"n":               1,
			"size":            "256x256",
			"response_format": "b64_json",
		},
	})

	result := openAIImageGeneration(t, openAIKey, prompt, "dall-e-2", "b64_json", "256x256")

	data, ok := result["data"].([]interface{})
	if !ok || len(data) == 0 {
		t.Fatalf("expected non-empty data array, got: %v", result)
	}
	item, ok := data[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data[0] to be a map, got: %T", data[0])
	}
	b64, ok := item["b64_json"].(string)
	if !ok || b64 == "" {
		t.Fatalf("expected non-empty b64_json in data[0], got: %v", item)
	}
	t.Logf("dall-e-2 returned b64_json image (%d chars)", len(b64))

	logger.AddResultToGeneration(genID, result)
	logger.SetTraceOutput(traceID, prompt+" — image generated")
	trace.End()
	logger.Flush()

	t.Log("Successfully logged dall-e-2 b64_json image generation to Maxim")
}

// TestOpenAI_E2E_ImageGeneration_DallE3_B64JSON uses dall-e-3 with b64_json.
// dall-e-3 returns a revised_prompt in the response; the SDK uses it as the assistant message content.
func TestOpenAI_E2E_ImageGeneration_DallE3_B64JSON(t *testing.T) {
	requireMaximCredentials(t)
	openAIKey := getOpenAIKey(t)

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	const prompt = "a serene mountain landscape at sunset"
	traceID := uuid.NewString()
	genID := uuid.NewString()

	trace := logger.Trace(&logging.TraceConfig{Id: traceID})
	logger.SetTraceInput(traceID, prompt)
	logger.AddGenerationToTrace(traceID, &logging.GenerationConfig{
		Id:       genID,
		Provider: logging.ProviderOpenAI,
		Model:    "dall-e-3",
		Messages: []schemas.CompletionRequest{
			{Role: "user", Content: prompt},
		},
		ModelParameters: map[string]interface{}{
			"n":               1,
			"size":            "1024x1024",
			"response_format": "b64_json",
			"quality":         "standard",
		},
	})

	result := openAIImageGeneration(t, openAIKey, prompt, "dall-e-3", "b64_json", "1024x1024")

	data, ok := result["data"].([]interface{})
	if !ok || len(data) == 0 {
		t.Fatalf("expected non-empty data array, got: %v", result)
	}
	item, ok := data[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data[0] to be a map, got: %T", data[0])
	}
	b64, ok := item["b64_json"].(string)
	if !ok || b64 == "" {
		t.Fatalf("expected non-empty b64_json in data[0], got: %v", item)
	}
	revisedPrompt, ok := item["revised_prompt"].(string)
	if !ok || revisedPrompt == "" {
		t.Fatalf("expected non-empty revised_prompt in data[0], got: %v", item)
	}
	t.Logf("dall-e-3 b64_json: %d chars, revised_prompt: %q", len(b64), revisedPrompt)

	logger.AddResultToGeneration(genID, result)
	logger.SetTraceOutput(traceID, prompt+" — image generated")
	trace.End()
	logger.Flush()

	t.Log("Successfully logged dall-e-3 b64_json image generation to Maxim")
}

// TestOpenAI_E2E_ImageGeneration_DallE3_URL uses dall-e-3 with url response format.
// NOTE: DALL-E pre-signed URLs expire in ~1 hour. Maxim stores the URL reference but
// the image will not be viewable after expiry. Use b64_json for persistent image data.
func TestOpenAI_E2E_ImageGeneration_DallE3_URL(t *testing.T) {
	requireMaximCredentials(t)
	openAIKey := getOpenAIKey(t)

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	const prompt = "a futuristic city skyline at night"
	traceID := uuid.NewString()
	genID := uuid.NewString()

	trace := logger.Trace(&logging.TraceConfig{Id: traceID})
	logger.SetTraceInput(traceID, prompt)
	logger.AddGenerationToTrace(traceID, &logging.GenerationConfig{
		Id:       genID,
		Provider: logging.ProviderOpenAI,
		Model:    "dall-e-3",
		Messages: []schemas.CompletionRequest{
			{Role: "user", Content: prompt},
		},
		ModelParameters: map[string]interface{}{
			"n":               1,
			"size":            "1024x1024",
			"response_format": "url",
			"quality":         "standard",
		},
	})

	result := openAIImageGeneration(t, openAIKey, prompt, "dall-e-3", "url", "1024x1024")

	data, ok := result["data"].([]interface{})
	if !ok || len(data) == 0 {
		t.Fatalf("expected non-empty data array, got: %v", result)
	}
	item, ok := data[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data[0] to be a map, got: %T", data[0])
	}
	imageURL, ok := item["url"].(string)
	if !ok || imageURL == "" {
		t.Fatalf("expected non-empty url in data[0], got: %v", item)
	}
	revisedPrompt, ok := item["revised_prompt"].(string)
	if !ok || revisedPrompt == "" {
		t.Fatalf("expected non-empty revised_prompt in data[0], got: %v", item)
	}
	t.Logf("dall-e-3 url: %s, revised_prompt: %q", imageURL, revisedPrompt)

	logger.AddResultToGeneration(genID, result)
	logger.SetTraceOutput(traceID, prompt+" — image generated")
	trace.End()
	logger.Flush()

	t.Log("Successfully logged dall-e-3 URL image generation to Maxim")
}

// TestOpenAI_E2E_ImageGeneration_DallE2_MultipleImages generates 2 images with dall-e-2
// and verifies that each produces its own attachment entry in Maxim.
func TestOpenAI_E2E_ImageGeneration_DallE2_MultipleImages(t *testing.T) {
	requireMaximCredentials(t)
	openAIKey := getOpenAIKey(t)

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	const prompt = "a simple geometric pattern"
	traceID := uuid.NewString()
	genID := uuid.NewString()

	trace := logger.Trace(&logging.TraceConfig{Id: traceID})
	logger.SetTraceInput(traceID, prompt)
	logger.AddGenerationToTrace(traceID, &logging.GenerationConfig{
		Id:       genID,
		Provider: logging.ProviderOpenAI,
		Model:    "dall-e-2",
		Messages: []schemas.CompletionRequest{
			{Role: "user", Content: prompt},
		},
		ModelParameters: map[string]interface{}{
			"n":               2,
			"size":            "256x256",
			"response_format": "b64_json",
		},
	})

	// Request 2 images from dall-e-2
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	reqBody := map[string]interface{}{
		"model":           "dall-e-2",
		"prompt":          prompt,
		"n":               2,
		"response_format": "b64_json",
		"size":            "256x256",
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/images/generations", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+openAIKey)

	client := &http.Client{Timeout: 120 * time.Second}
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

	data, ok := result["data"].([]interface{})
	if !ok || len(data) < 2 {
		t.Fatalf("expected at least 2 images in data array, got: %v", result)
	}
	t.Logf("dall-e-2 returned %d images", len(data))

	logger.AddResultToGeneration(genID, result)
	logger.SetTraceOutput(traceID, prompt+" — 2 images generated")
	trace.End()
	logger.Flush()

	t.Logf("Successfully logged dall-e-2 multi-image generation (%d images) to Maxim", len(data))
}
