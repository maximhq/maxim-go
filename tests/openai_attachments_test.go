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

	"github.com/google/uuid"
	"github.com/maximhq/maxim-go/logging"
)

// getOpenAIKey returns the OpenAI API key from the environment.
func getOpenAIKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		t.Skip("OPENAI_API_KEY not set, skipping OpenAI integration test")
	}
	return key
}

// requireMaximCredentials skips the test if MAXIM_API_KEY or MAXIM_LOG_REPO_ID are not set.
func requireMaximCredentials(t *testing.T) {
	t.Helper()
	if os.Getenv("MAXIM_API_KEY") == "" || os.Getenv("MAXIM_LOG_REPO_ID") == "" {
		t.Skip("MAXIM_API_KEY and MAXIM_LOG_REPO_ID required for OpenAI integration tests")
	}
}

// openAIChatCompletion makes a real OpenAI chat completion request and returns
// the raw JSON response body as a map.
func openAIChatCompletion(t *testing.T, apiKey string, messages []map[string]interface{}, model string) map[string]interface{} {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	reqBody := map[string]interface{}{
		"model":    model,
		"messages": messages,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 60 * time.Second}
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

// parseAndValidateResult runs ParseResult for the openai provider and fails
// the test if parsing returns an error or a nil result.
func parseAndValidateResult(t *testing.T, result map[string]interface{}, model string) *logging.MaximLLMResult {
	t.Helper()
	parsed, err := logging.ParseResult("openai", model, result)
	if err != nil {
		t.Fatalf("ParseResult failed: %v", err)
	}
	if parsed == nil {
		t.Fatal("ParseResult returned nil")
	}
	if parsed.ID == "" {
		t.Error("ParseResult: expected non-empty ID")
	}
	if parsed.Model == "" {
		t.Error("ParseResult: expected non-empty Model")
	}
	if len(parsed.Choices) == 0 {
		t.Error("ParseResult: expected at least one choice")
	}
	t.Logf("ParseResult OK — id=%s model=%s tokens=%d", parsed.ID, parsed.Model, parsed.Usage.TotalTokens)
	return parsed
}

// TestOpenAI_VisionWithUrlAttachment sends an image URL to OpenAI's vision API,
// parses the result via ParseResult, and logs it with the same image as a URL attachment.
func TestOpenAI_VisionWithUrlAttachment(t *testing.T) {
	requireMaximCredentials(t)
	openAIKey := getOpenAIKey(t)

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	trace := logger.Trace(&logging.TraceConfig{Id: uuid.New().String()})
	trace.SetInput("Describe the image via URL")

	visionMessages := []map[string]interface{}{
		{
			"role": "user",
			"content": []map[string]interface{}{
				{"type": "text", "text": "Describe this image in one sentence."},
				{"type": "image_url", "image_url": map[string]string{"url": testImageURL}},
			},
		},
	}

	gen := trace.AddGeneration(&logging.GenerationConfig{
		Id:       uuid.New().String(),
		Provider: "openai",
		Model:    "gpt-4o-mini",
		Messages: []logging.CompletionRequest{
			{Role: "user", Content: visionMessages[0]["content"]},
		},
	})

	result := openAIChatCompletion(t, openAIKey, visionMessages, "gpt-4o-mini")
	parsed := parseAndValidateResult(t, result, "gpt-4o-mini")

	gen.SetResult(result)

	// Log the same image as an attachment on the generation
	gen.AddAttachment(&logging.UrlAttachment{
		BaseAttachmentProps: logging.BaseAttachmentProps{
			ID:   uuid.New().String(),
			Name: "input_image.jpg",
			Tags: map[string]string{"source": "openai_vision_input"},
		},
		Type: logging.AttachmentTypeURL,
		URL:  testImageURL,
	})

	gen.End()
	trace.SetOutput(parsed.Choices[0].Message.Content)
	trace.End()

	logger.Flush()
	t.Log("Successfully logged OpenAI vision call (URL) with URL attachment")
}

// TestOpenAI_VisionWithFileDataAttachment reads a local image, sends it as
// base64 to OpenAI's vision API, parses via ParseResult, and logs it as a FileDataAttachment.
func TestOpenAI_VisionWithFileDataAttachment(t *testing.T) {
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

	trace := logger.Trace(&logging.TraceConfig{Id: uuid.New().String()})
	trace.SetInput("Describe the image via base64")

	b64 := base64.StdEncoding.EncodeToString(imageData)
	dataURI := "data:image/jpeg;base64," + b64

	visionMessages := []map[string]interface{}{
		{
			"role": "user",
			"content": []map[string]interface{}{
				{"type": "text", "text": "Describe this image in one sentence."},
				{"type": "image_url", "image_url": map[string]string{"url": dataURI}},
			},
		},
	}

	gen := trace.AddGeneration(&logging.GenerationConfig{
		Id:       uuid.New().String(),
		Provider: "openai",
		Model:    "gpt-4o-mini",
		Messages: []logging.CompletionRequest{
			{Role: "user", Content: visionMessages[0]["content"]},
		},
	})

	result := openAIChatCompletion(t, openAIKey, visionMessages, "gpt-4o-mini")
	parsed := parseAndValidateResult(t, result, "gpt-4o-mini")

	gen.SetResult(result)

	// Log the same image bytes as a FileDataAttachment
	gen.AddAttachment(&logging.FileDataAttachment{
		BaseAttachmentProps: logging.BaseAttachmentProps{
			ID:   uuid.New().String(),
			Name: "input_image.jpg",
			Tags: map[string]string{"source": "openai_vision_input"},
		},
		Type: logging.AttachmentTypeFileData,
		Data: imageData,
	})

	gen.End()
	trace.SetOutput(parsed.Choices[0].Message.Content)
	trace.End()

	logger.Flush()
	t.Log("Successfully logged OpenAI vision call (base64) with FileData attachment")
}

// TestOpenAI_VisionWithMultipleAttachments sends an image to OpenAI vision and
// attaches it at trace, span, and generation levels. Parses result via ParseResult.
func TestOpenAI_VisionWithMultipleAttachments(t *testing.T) {
	requireMaximCredentials(t)
	openAIKey := getOpenAIKey(t)

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	trace := logger.Trace(&logging.TraceConfig{Id: uuid.New().String()})
	trace.SetInput("Multi-level attachment test with OpenAI vision")

	// Attach image at trace level
	trace.AddAttachment(&logging.UrlAttachment{
		BaseAttachmentProps: logging.BaseAttachmentProps{
			ID:   uuid.New().String(),
			Name: "trace_level_image.jpg",
			Tags: map[string]string{"level": "trace"},
		},
		Type: logging.AttachmentTypeURL,
		URL:  testImageURL,
	})

	// Create a span with its own attachment
	span := trace.AddSpan(&logging.SpanConfig{Id: uuid.New().String()})
	span.AddAttachment(&logging.UrlAttachment{
		BaseAttachmentProps: logging.BaseAttachmentProps{
			ID:   uuid.New().String(),
			Name: "span_level_image.jpg",
			Tags: map[string]string{"level": "span"},
		},
		Type: logging.AttachmentTypeURL,
		URL:  testImageURL,
	})

	// Send the image to OpenAI vision inside the span
	visionMessages := []map[string]interface{}{
		{
			"role": "user",
			"content": []map[string]interface{}{
				{"type": "text", "text": "What do you see in this image? Reply in one sentence."},
				{"type": "image_url", "image_url": map[string]string{"url": testImageURL}},
			},
		},
	}

	gen := span.AddGeneration(&logging.GenerationConfig{
		Id:       uuid.New().String(),
		Provider: "openai",
		Model:    "gpt-4o-mini",
		Messages: []logging.CompletionRequest{
			{Role: "user", Content: visionMessages[0]["content"]},
		},
	})

	result := openAIChatCompletion(t, openAIKey, visionMessages, "gpt-4o-mini")
	parsed := parseAndValidateResult(t, result, "gpt-4o-mini")

	gen.SetResult(result)

	// Attach the same image at generation level
	gen.AddAttachment(&logging.UrlAttachment{
		BaseAttachmentProps: logging.BaseAttachmentProps{
			ID:   uuid.New().String(),
			Name: "generation_level_image.jpg",
			Tags: map[string]string{"level": "generation"},
		},
		Type: logging.AttachmentTypeURL,
		URL:  testImageURL,
	})

	gen.End()
	span.End()

	trace.SetOutput(parsed.Choices[0].Message.Content)
	trace.End()

	logger.Flush()
	t.Log("Successfully logged OpenAI vision call with attachments at trace, span, and generation levels")
}

// TestOpenAI_VisionWithMapAttachment sends an image to OpenAI vision and logs it
// using a map-based attachment. Parses result via ParseResult.
func TestOpenAI_VisionWithMapAttachment(t *testing.T) {
	requireMaximCredentials(t)
	openAIKey := getOpenAIKey(t)

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	trace := logger.Trace(&logging.TraceConfig{Id: uuid.New().String()})
	trace.SetInput("Map attachment test with OpenAI vision")

	visionMessages := []map[string]interface{}{
		{
			"role": "user",
			"content": []map[string]interface{}{
				{"type": "text", "text": "What is shown in this image? One sentence."},
				{"type": "image_url", "image_url": map[string]string{"url": testImageURL}},
			},
		},
	}

	gen := trace.AddGeneration(&logging.GenerationConfig{
		Id:       uuid.New().String(),
		Provider: "openai",
		Model:    "gpt-4o-mini",
		Messages: []logging.CompletionRequest{
			{Role: "user", Content: visionMessages[0]["content"]},
		},
	})

	result := openAIChatCompletion(t, openAIKey, visionMessages, "gpt-4o-mini")
	parsed := parseAndValidateResult(t, result, "gpt-4o-mini")

	gen.SetResult(result)

	// Use map-based attachment
	gen.AddAttachment(map[string]interface{}{
		"id":   uuid.New().String(),
		"type": logging.AttachmentTypeURL,
		"url":  testImageURL,
		"name": "vision_input_image.jpg",
		"tags": map[string]string{"style": "map", "source": "openai_vision_input"},
	})

	gen.End()
	trace.SetOutput(parsed.Choices[0].Message.Content)
	trace.End()

	logger.Flush()
	t.Log("Successfully logged OpenAI vision call with map-based attachment")
}

// TestOpenAI_VisionAttachmentOnError attaches an image to a generation and
// then simulates an error to verify attachments persist on error scenarios.
func TestOpenAI_VisionAttachmentOnError(t *testing.T) {
	requireMaximCredentials(t)
	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	trace := logger.Trace(&logging.TraceConfig{Id: uuid.New().String()})
	trace.SetInput("Error scenario: vision call with attachment")

	gen := trace.AddGeneration(&logging.GenerationConfig{
		Id:       uuid.New().String(),
		Provider: "openai",
		Model:    "gpt-4o-mini",
		Messages: []logging.CompletionRequest{
			{
				Role: "user",
				Content: []map[string]interface{}{
					{"type": "text", "text": "Describe this image."},
					{"type": "image_url", "image_url": map[string]string{"url": testImageURL}},
				},
			},
		},
	})

	// Attach image before setting error
	gen.AddAttachment(&logging.UrlAttachment{
		BaseAttachmentProps: logging.BaseAttachmentProps{
			ID:   uuid.New().String(),
			Name: "error_context_image.jpg",
			Tags: map[string]string{"scenario": "error"},
		},
		Type: logging.AttachmentTypeURL,
		URL:  testImageURL,
	})

	errType := "api_error"
	gen.SetError(&logging.GenerationError{
		Message: "Simulated API error for testing",
		Type:    &errType,
	})

	trace.SetOutput("Error scenario completed")
	trace.End()

	logger.Flush()
	t.Log("Successfully logged generation error with image attachment")
}
