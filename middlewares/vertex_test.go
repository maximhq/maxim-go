package middlewares

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/maximhq/maxim-go/logging"
)

const vertexBaseTestURL = "https://us-central1-aiplatform.googleapis.com/v1/projects/my-project/locations/us-central1/publishers/google/models"

func vertexReq(t *testing.T, model, action string, body map[string]interface{}, logger *logging.Logger) *http.Request {
	t.Helper()
	bodyBytes, _ := json.Marshal(body)
	url := vertexBaseTestURL + "/" + model + ":" + action
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")
	ctx := context.WithValue(req.Context(), ContextKeyLogger, logger)
	ctx = context.WithValue(ctx, ContextKeyProvider, logging.ProviderVertex)
	return req.WithContext(ctx)
}

func vertexNext(body string, status int) func(*http.Request) (*http.Response, error) {
	return func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(bytes.NewBufferString(body)),
		}, nil
	}
}

// --- Model extraction ---

func TestExtractVertexModel(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "standard generateContent path",
			path:     "/v1/projects/my-project/locations/us-central1/publishers/google/models/gemini-2.0-flash:generateContent",
			expected: "publishers/google/models/gemini-2.0-flash",
		},
		{
			name:     "streamGenerateContent path",
			path:     "/v1/projects/proj/locations/europe-west1/publishers/google/models/gemini-1.5-pro:streamGenerateContent",
			expected: "publishers/google/models/gemini-1.5-pro",
		},
		{
			name:     "model with preview suffix",
			path:     "/v1/projects/proj/locations/us-central1/publishers/google/models/gemini-2.5-pro-preview-0325:generateContent",
			expected: "publishers/google/models/gemini-2.5-pro-preview-0325",
		},
		{
			name:     "non-google publisher",
			path:     "/v1/projects/proj/locations/us-central1/publishers/anthropic/models/claude-3-5-sonnet:generateContent",
			expected: "publishers/anthropic/models/claude-3-5-sonnet",
		},
		{
			name:     "empty path",
			path:     "",
			expected: "",
		},
		{
			name:     "path without models/",
			path:     "/v1/projects/proj/locations/us-central1/publishers/google",
			expected: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractVertexModel(tt.path)
			if got != tt.expected {
				t.Errorf("extractVertexModel(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

// --- Transport ---

func TestMaximVertexTransport_BasicRequest(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	transport := NewMaximVertexTransport(logger)

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hello"}}},
		},
	}
	req := vertexReq(t, "gemini-2.0-flash", "generateContent", body, logger)

	nextCalled := false
	next := func(r *http.Request) (*http.Response, error) {
		nextCalled = true
		if p, ok := r.Context().Value(ContextKeyProvider).(string); !ok || p != logging.ProviderVertex {
			t.Errorf("expected provider %q, got %q", logging.ProviderVertex, p)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"candidates":[{"content":{"parts":[{"text":"world"}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1,"totalTokenCount":2},"modelVersion":"gemini-2.0-flash-001"}`)),
		}, nil
	}

	resp, err := MaximGeminiMiddleware(req, next)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !nextCalled {
		t.Error("next was not called")
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	_ = transport
	logger.Flush()
}

func TestMaximVertexTransport_NilLogger(t *testing.T) {
	transport := NewMaximVertexTransport(nil)

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hello"}}},
		},
	}
	bodyBytes, _ := json.Marshal(body)
	url := vertexBaseTestURL + "/gemini-2.0-flash:generateContent"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}

	nextCalled := false
	next := func(r *http.Request) (*http.Response, error) {
		nextCalled = true
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"candidates":[]}`)),
		}, nil
	}

	resp, err := MaximGeminiMiddleware(req, next)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !nextCalled {
		t.Error("next was not called")
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	_ = transport
}

func TestMaximVertexTransport_ResponseBodyRestoredAfterRead(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	req := vertexReq(t, "gemini-2.0-flash", "generateContent", map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hi"}}},
		},
	}, logger)

	const respJSON = `{"candidates":[{"content":{"parts":[{"text":"hi back"}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":2,"candidatesTokenCount":3,"totalTokenCount":5}}`

	resp, err := MaximGeminiMiddleware(req, vertexNext(respJSON, http.StatusOK))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Body must still be readable downstream
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("response body not valid JSON after middleware: %v", err)
	}
	if m["candidates"] == nil {
		t.Error("candidates missing from restored response body")
	}
	logger.Flush()
}

// --- Streaming ---

func TestMaximVertexTransport_Streaming_AggregatesChunks(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	req := vertexReq(t, "gemini-2.0-flash", "streamGenerateContent", map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "count"}}},
		},
	}, logger)

	// Vertex streams usageMetadata only on the last chunk.
	sseBody := strings.Join([]string{
		`data: {"candidates":[{"content":{"parts":[{"text":"1"}],"role":"model"},"finishReason":""}]}`,
		`data: {"candidates":[{"content":{"parts":[{"text":", 2"}],"role":"model"},"finishReason":""}]}`,
		`data: {"candidates":[{"content":{"parts":[{"text":", 3"}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":6,"totalTokenCount":9}}`,
		``,
	}, "\n")

	resp, err := MaximGeminiMiddleware(req, vertexNext(sseBody, http.StatusOK))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	// Body is the original SSE bytes restored by the middleware.
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "1, 2, 3") && !strings.Contains(string(body), `"text":"1"`) {
		t.Errorf("unexpected restored body: %s", body)
	}
	logger.Flush()
}

func TestMaximVertexTransport_Streaming_UsageMetadataOnAllChunks(t *testing.T) {
	// Some Vertex models send usageMetadata on every chunk with running totals.
	// The middleware must use the LAST chunk's values (highest counts).
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	req := vertexReq(t, "gemini-2.0-flash", "streamGenerateContent", map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hello"}}},
		},
	}, logger)

	sseBody := strings.Join([]string{
		`data: {"candidates":[{"content":{"parts":[{"text":"Hello"}],"role":"model"}}],"usageMetadata":{"promptTokenCount":2,"candidatesTokenCount":1,"totalTokenCount":3}}`,
		`data: {"candidates":[{"content":{"parts":[{"text":"!"}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":2,"candidatesTokenCount":2,"totalTokenCount":4}}`,
		``,
	}, "\n")

	resp, err := MaximGeminiMiddleware(req, vertexNext(sseBody, http.StatusOK))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	logger.Flush()
}

func TestMaximVertexTransport_Streaming_WithAltSSEQueryParam(t *testing.T) {
	// Streaming URLs typically include ?alt=sse; model extraction must still work.
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	bodyBytes, _ := json.Marshal(map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hi"}}},
		},
	})
	url := vertexBaseTestURL + "/gemini-2.5-flash:streamGenerateContent?alt=sse"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer test-token")
	ctx := context.WithValue(req.Context(), ContextKeyLogger, logger)
	ctx = context.WithValue(ctx, ContextKeyProvider, logging.ProviderVertex)
	req = req.WithContext(ctx)

	sseBody := "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"hi\"}],\"role\":\"model\"},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":1,\"candidatesTokenCount\":1,\"totalTokenCount\":2}}\n"

	resp, err := MaximGeminiMiddleware(req, vertexNext(sseBody, http.StatusOK))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	logger.Flush()
}

func TestMaximVertexTransport_Streaming_NetworkError(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	req := vertexReq(t, "gemini-2.0-flash", "streamGenerateContent", map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hi"}}},
		},
	}, logger)

	networkErr := errors.New("connection reset by peer")
	next := func(r *http.Request) (*http.Response, error) {
		return nil, networkErr
	}

	resp, err := MaximGeminiMiddleware(req, next)
	if resp != nil {
		t.Error("expected nil response on network error")
	}
	if err != networkErr {
		t.Errorf("expected networkErr, got %v", err)
	}
	logger.Flush()
}

// --- Tool calls ---

func TestMaximVertexTransport_ToolCall_SingleFunction(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	req := vertexReq(t, "gemini-2.0-flash", "generateContent", map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "What is the weather in London?"}}},
		},
		"tools": []map[string]interface{}{
			{
				"functionDeclarations": []map[string]interface{}{
					{
						"name":        "get_weather",
						"description": "Get current weather for a city",
						"parameters": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"city": map[string]interface{}{"type": "string"},
							},
						},
					},
				},
			},
		},
	}, logger)

	toolCallResp := `{
		"candidates":[{
			"content":{
				"parts":[{
					"functionCall":{
						"name":"get_weather",
						"args":{"city":"London"}
					}
				}],
				"role":"model"
			},
			"finishReason":"STOP"
		}],
		"usageMetadata":{"promptTokenCount":20,"candidatesTokenCount":10,"totalTokenCount":30}
	}`

	resp, err := MaximGeminiMiddleware(req, vertexNext(toolCallResp, http.StatusOK))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	logger.Flush()
}

func TestMaximVertexTransport_ToolCall_MultipleTools(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	req := vertexReq(t, "gemini-2.5-flash", "generateContent", map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "Book a flight and hotel to Paris"}}},
		},
		"tools": []map[string]interface{}{
			{
				"functionDeclarations": []map[string]interface{}{
					{"name": "search_flights", "description": "Search available flights"},
					{"name": "search_hotels", "description": "Search available hotels"},
				},
			},
		},
	}, logger)

	resp := `{
		"candidates":[{
			"content":{
				"parts":[
					{"functionCall":{"name":"search_flights","args":{"destination":"Paris","date":"2026-05-01"}}},
					{"functionCall":{"name":"search_hotels","args":{"city":"Paris","checkin":"2026-05-01"}}}
				],
				"role":"model"
			},
			"finishReason":"STOP"
		}],
		"usageMetadata":{"promptTokenCount":30,"candidatesTokenCount":25,"totalTokenCount":55}
	}`

	httpResp, err := MaximGeminiMiddleware(req, vertexNext(resp, http.StatusOK))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if httpResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", httpResp.StatusCode)
	}
	logger.Flush()
}

func TestMaximVertexTransport_ToolCall_Streaming(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	req := vertexReq(t, "gemini-2.0-flash", "streamGenerateContent", map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "Get weather for Tokyo"}}},
		},
		"tools": []map[string]interface{}{
			{"functionDeclarations": []map[string]interface{}{
				{"name": "get_weather", "description": "Get weather"},
			}},
		},
	}, logger)

	sseBody := strings.Join([]string{
		`data: {"candidates":[{"content":{"parts":[{"functionCall":{"name":"get_weather","args":{"city":"Tokyo"}}}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":15,"candidatesTokenCount":8,"totalTokenCount":23}}`,
		``,
	}, "\n")

	httpResp, err := MaximGeminiMiddleware(req, vertexNext(sseBody, http.StatusOK))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if httpResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", httpResp.StatusCode)
	}
	logger.Flush()
}

func TestMaximVertexTransport_ToolCall_WithGenerationConfig(t *testing.T) {
	// generationConfig fields (temperature, etc.) are stored as model parameters alongside tools.
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	req := vertexReq(t, "gemini-2.0-flash", "generateContent", map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "Calculate 2+2"}}},
		},
		"tools": []map[string]interface{}{
			{"functionDeclarations": []map[string]interface{}{
				{"name": "calculator", "description": "Perform arithmetic"},
			}},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     0.0,
			"maxOutputTokens": 256,
		},
	}, logger)

	toolResp := `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"calculator","args":{"op":"add","a":2,"b":2}}}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5,"totalTokenCount":15}}`

	httpResp, err := MaximGeminiMiddleware(req, vertexNext(toolResp, http.StatusOK))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if httpResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", httpResp.StatusCode)
	}
	logger.Flush()
}

// --- Attachments ---

func TestMaximVertexTransport_Attachment_InlineImageBase64(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	// Minimal 1x1 PNG as base64
	const tinyPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

	req := vertexReq(t, "gemini-2.0-flash", "generateContent", map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]interface{}{
					{
						"inline_data": map[string]interface{}{
							"mime_type": "image/png",
							"data":      tinyPNG,
						},
					},
					{"text": "What is in this image?"},
				},
			},
		},
	}, logger)

	resp, err := MaximGeminiMiddleware(req, vertexNext(`{"candidates":[{"content":{"parts":[{"text":"A tiny dot."}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":4,"totalTokenCount":9}}`, http.StatusOK))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	logger.Flush()
}

func TestMaximVertexTransport_Attachment_FileDataURI(t *testing.T) {
	// file_data with a GCS URI (common in Vertex AI workflows)
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	req := vertexReq(t, "gemini-2.0-flash", "generateContent", map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]interface{}{
					{
						"file_data": map[string]interface{}{
							"mime_type": "image/jpeg",
							"file_uri":  "gs://my-bucket/images/photo.jpg",
						},
					},
					{"text": "Describe this image."},
				},
			},
		},
	}, logger)

	resp, err := MaximGeminiMiddleware(req, vertexNext(`{"candidates":[{"content":{"parts":[{"text":"A photo."}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":4,"totalTokenCount":14}}`, http.StatusOK))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	logger.Flush()
}

func TestMaximVertexTransport_Attachment_MultipleImages(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	imgData := base64.StdEncoding.EncodeToString([]byte("fake-image-bytes"))

	req := vertexReq(t, "gemini-2.0-flash", "generateContent", map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]interface{}{
					{"inline_data": map[string]interface{}{"mime_type": "image/jpeg", "data": imgData}},
					{"inline_data": map[string]interface{}{"mime_type": "image/png", "data": imgData}},
					{"text": "Compare these two images."},
				},
			},
		},
	}, logger)

	resp, err := MaximGeminiMiddleware(req, vertexNext(`{"candidates":[{"content":{"parts":[{"text":"They look similar."}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":8,"candidatesTokenCount":5,"totalTokenCount":13}}`, http.StatusOK))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	logger.Flush()
}

func TestMaximVertexTransport_Attachment_CamelCaseFields(t *testing.T) {
	// Vertex clients may send inlineData/mimeType (camelCase) instead of inline_data/mime_type.
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	imgData := base64.StdEncoding.EncodeToString([]byte("fake-image"))

	req := vertexReq(t, "gemini-2.0-flash", "generateContent", map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]interface{}{
					{"inlineData": map[string]interface{}{"mimeType": "image/png", "data": imgData}},
					{"text": "Describe."},
				},
			},
		},
	}, logger)

	resp, err := MaximGeminiMiddleware(req, vertexNext(`{"candidates":[{"content":{"parts":[{"text":"ok"}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":1,"totalTokenCount":4}}`, http.StatusOK))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	logger.Flush()
}

func TestMaximVertexTransport_Attachment_ImageWithToolCall(t *testing.T) {
	// Multimodal + tool call in the same request.
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	imgData := base64.StdEncoding.EncodeToString([]byte("fake-image"))

	req := vertexReq(t, "gemini-2.0-flash", "generateContent", map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]interface{}{
					{"inline_data": map[string]interface{}{"mime_type": "image/jpeg", "data": imgData}},
					{"text": "Identify the object and look up its price."},
				},
			},
		},
		"tools": []map[string]interface{}{
			{"functionDeclarations": []map[string]interface{}{
				{"name": "lookup_price", "description": "Look up price for an item"},
			}},
		},
	}, logger)

	toolResp := `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"lookup_price","args":{"item":"chair"}}}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":20,"candidatesTokenCount":8,"totalTokenCount":28}}`

	httpResp, err := MaximGeminiMiddleware(req, vertexNext(toolResp, http.StatusOK))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if httpResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", httpResp.StatusCode)
	}
	logger.Flush()
}

// --- System instruction ---

func TestMaximVertexTransport_WithSystemInstruction(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	req := vertexReq(t, "gemini-2.5-flash", "generateContent", map[string]interface{}{
		"systemInstruction": map[string]interface{}{
			"parts": []map[string]interface{}{{"text": "You are a helpful assistant specializing in Go programming."}},
		},
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "How do I use goroutines?"}}},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     0.5,
			"maxOutputTokens": 512,
			"topP":            0.9,
		},
	}, logger)

	resp, err := MaximGeminiMiddleware(req, vertexNext(`{"candidates":[{"content":{"parts":[{"text":"Use the go keyword."}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":25,"candidatesTokenCount":8,"totalTokenCount":33}}`, http.StatusOK))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	logger.Flush()
}

// --- Multi-turn conversation ---

func TestMaximVertexTransport_MultiTurn(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	req := vertexReq(t, "gemini-2.0-flash", "generateContent", map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "My name is Alice."}}},
			{"role": "model", "parts": []map[string]interface{}{{"text": "Hello Alice!"}}},
			{"role": "user", "parts": []map[string]interface{}{{"text": "What is my name?"}}},
		},
	}, logger)

	resp, err := MaximGeminiMiddleware(req, vertexNext(`{"candidates":[{"content":{"parts":[{"text":"Your name is Alice."}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":20,"candidatesTokenCount":6,"totalTokenCount":26}}`, http.StatusOK))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	logger.Flush()
}

// --- Trace & session context ---

func TestMaximVertexTransport_WithCustomTraceID(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	customTraceID := uuid.NewString()

	bodyBytes, _ := json.Marshal(map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hello"}}},
		},
	})
	url := vertexBaseTestURL + "/gemini-2.0-flash:generateContent"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(req.Context(), ContextKeyLogger, logger)
	ctx = context.WithValue(ctx, ContextKeyProvider, logging.ProviderVertex)
	ctx = context.WithValue(ctx, ContextKeyTraceId, customTraceID)
	ctx = context.WithValue(ctx, ContextKeyTraceName, "vertex-custom-trace")
	req = req.WithContext(ctx)

	resp, err := MaximGeminiMiddleware(req, vertexNext(`{"candidates":[{"content":{"parts":[{"text":"hi"}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1,"totalTokenCount":2}}`, http.StatusOK))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	logger.Flush()
}

func TestMaximVertexTransport_WithTraceTags(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	bodyBytes, _ := json.Marshal(map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hello"}}},
		},
	})
	url := vertexBaseTestURL + "/gemini-2.0-flash:generateContent"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(req.Context(), ContextKeyLogger, logger)
	ctx = context.WithValue(ctx, ContextKeyProvider, logging.ProviderVertex)
	ctx = context.WithValue(ctx, ContextKeyTraceTags, map[string]string{
		"env":     "test",
		"feature": "vertex-middleware",
	})
	ctx = context.WithValue(ctx, ContextKeyGenerationTags, map[string]string{
		"model_variant": "flash",
	})
	req = req.WithContext(ctx)

	resp, err := MaximGeminiMiddleware(req, vertexNext(`{"candidates":[{"content":{"parts":[{"text":"hi"}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1,"totalTokenCount":2}}`, http.StatusOK))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	logger.Flush()
}

func TestMaximVertexTransport_WithTraceAndGenerationMetrics(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	bodyBytes, _ := json.Marshal(map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hello"}}},
		},
	})
	url := vertexBaseTestURL + "/gemini-2.0-flash:generateContent"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(req.Context(), ContextKeyLogger, logger)
	ctx = context.WithValue(ctx, ContextKeyProvider, logging.ProviderVertex)
	ctx = context.WithValue(ctx, ContextKeyTraceMetrics, map[string]float64{"priority": 1.0})
	ctx = context.WithValue(ctx, ContextKeyGenerationMetrics, map[string]float64{"retry_count": 0.0})
	req = req.WithContext(ctx)

	resp, err := MaximGeminiMiddleware(req, vertexNext(`{"candidates":[{"content":{"parts":[{"text":"hi"}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1,"totalTokenCount":2}}`, http.StatusOK))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	logger.Flush()
}

func TestMaximVertexTransport_WithSession(t *testing.T) {
	// The middleware uses the logger's session support via a pre-created trace in session.
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	sessionID := uuid.NewString()
	traceID := uuid.NewString()

	// Create a session and attach a trace before the request.
	session := logger.Session(&logging.SessionConfig{Id: sessionID})
	_ = logger.AddTraceToSession(sessionID, &logging.TraceConfig{Id: traceID})

	bodyBytes, _ := json.Marshal(map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hello from session"}}},
		},
	})
	url := vertexBaseTestURL + "/gemini-2.0-flash:generateContent"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	// Attach the trace ID so the middleware logs into the existing trace.
	ctx := context.WithValue(req.Context(), ContextKeyLogger, logger)
	ctx = context.WithValue(ctx, ContextKeyProvider, logging.ProviderVertex)
	ctx = context.WithValue(ctx, ContextKeyTraceId, traceID)
	req = req.WithContext(ctx)

	resp, err := MaximGeminiMiddleware(req, vertexNext(`{"candidates":[{"content":{"parts":[{"text":"hi session"}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":2,"totalTokenCount":5}}`, http.StatusOK))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	session.End()
	logger.Flush()
}

func TestMaximVertexTransport_WithCustomGenerationName(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	bodyBytes, _ := json.Marshal(map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "summarise this"}}},
		},
	})
	url := vertexBaseTestURL + "/gemini-2.5-flash:generateContent"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(req.Context(), ContextKeyLogger, logger)
	ctx = context.WithValue(ctx, ContextKeyProvider, logging.ProviderVertex)
	ctx = context.WithValue(ctx, ContextKeyGenerationName, "summarisation-step")
	req = req.WithContext(ctx)

	resp, err := MaximGeminiMiddleware(req, vertexNext(`{"candidates":[{"content":{"parts":[{"text":"summary"}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":4,"candidatesTokenCount":2,"totalTokenCount":6}}`, http.StatusOK))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	logger.Flush()
}

// --- Error handling ---

func TestMaximVertexTransport_APIError_Unauthenticated(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	req := vertexReq(t, "gemini-2.0-flash", "generateContent", map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hello"}}},
		},
	}, logger)

	errBody := `{"error":{"code":401,"message":"Request had invalid authentication credentials.","status":"UNAUTHENTICATED"}}`

	resp, err := MaximGeminiMiddleware(req, vertexNext(errBody, http.StatusUnauthorized))
	if err != nil {
		t.Fatalf("middleware must not return error for API-level errors: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var got map[string]interface{}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("response body not JSON: %v", err)
	}
	if got["error"] == nil {
		t.Error("expected error field in response body")
	}
	logger.Flush()
}

func TestMaximVertexTransport_APIError_QuotaExceeded(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	req := vertexReq(t, "gemini-2.0-flash", "generateContent", map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hello"}}},
		},
	}, logger)

	errBody := `{"error":{"code":429,"message":"Quota exceeded for quota metric 'generate_content_free_tier_requests'","status":"RESOURCE_EXHAUSTED"}}`

	resp, err := MaximGeminiMiddleware(req, vertexNext(errBody, http.StatusTooManyRequests))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", resp.StatusCode)
	}
	logger.Flush()
}

func TestMaximVertexTransport_APIError_ModelNotFound(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	req := vertexReq(t, "gemini-nonexistent", "generateContent", map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hello"}}},
		},
	}, logger)

	errBody := `{"error":{"code":404,"message":"Model not found.","status":"NOT_FOUND"}}`

	resp, err := MaximGeminiMiddleware(req, vertexNext(errBody, http.StatusNotFound))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
	logger.Flush()
}

func TestMaximVertexTransport_NetworkError_PropagatedToCalller(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	req := vertexReq(t, "gemini-2.0-flash", "generateContent", map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hello"}}},
		},
	}, logger)

	networkErr := errors.New("dial tcp: connection refused")
	next := func(r *http.Request) (*http.Response, error) { return nil, networkErr }

	resp, err := MaximGeminiMiddleware(req, next)
	if resp != nil {
		t.Error("expected nil response on network error")
	}
	if !errors.Is(err, networkErr) {
		t.Errorf("expected networkErr, got %v", err)
	}
	logger.Flush()
}

// --- Edge cases ---

func TestMaximVertexTransport_EmptyRequestBody_NoPanic(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	url := vertexBaseTestURL + "/gemini-2.0-flash:generateContent"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader([]byte("{}")))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(req.Context(), ContextKeyLogger, logger)
	ctx = context.WithValue(ctx, ContextKeyProvider, logging.ProviderVertex)
	req = req.WithContext(ctx)

	resp, err := MaximGeminiMiddleware(req, vertexNext(`{"candidates":[]}`, http.StatusOK))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	logger.Flush()
}

func TestMaximVertexTransport_NilRequestBody_NoPanic(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	url := vertexBaseTestURL + "/gemini-2.0-flash:generateContent"
	req, err := http.NewRequest(http.MethodPost, url, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(req.Context(), ContextKeyLogger, logger)
	ctx = context.WithValue(ctx, ContextKeyProvider, logging.ProviderVertex)
	req = req.WithContext(ctx)

	resp, err := MaximGeminiMiddleware(req, vertexNext(`{"candidates":[]}`, http.StatusOK))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	logger.Flush()
}

func TestMaximVertexTransport_DifferentLocations(t *testing.T) {
	// Model extraction must work regardless of GCP region in the URL.
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	locations := []string{"us-central1", "europe-west4", "asia-northeast1"}
	for _, loc := range locations {
		t.Run(loc, func(t *testing.T) {
			bodyBytes, _ := json.Marshal(map[string]interface{}{
				"contents": []map[string]interface{}{
					{"role": "user", "parts": []map[string]interface{}{{"text": "hi"}}},
				},
			})
			url := "https://" + loc + "-aiplatform.googleapis.com/v1/projects/my-project/locations/" + loc + "/publishers/google/models/gemini-2.0-flash:generateContent"
			req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyBytes))
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.WithValue(req.Context(), ContextKeyLogger, logger)
			ctx = context.WithValue(ctx, ContextKeyProvider, logging.ProviderVertex)
			req = req.WithContext(ctx)

			resp, err := MaximGeminiMiddleware(req, vertexNext(`{"candidates":[{"content":{"parts":[{"text":"hi"}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1,"totalTokenCount":2}}`, http.StatusOK))
			if err != nil {
				t.Fatalf("unexpected error for location %s: %v", loc, err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Errorf("location %s: expected 200, got %d", loc, resp.StatusCode)
			}
		})
	}
	logger.Flush()
}
