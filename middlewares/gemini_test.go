package middlewares

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/maximhq/maxim-go/logging"
)

func TestExtractGeminiModel(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "standard generateContent path",
			path:     "/v1beta/models/gemini-2.5-pro:generateContent",
			expected: "gemini-2.5-pro",
		},
		{
			name:     "model without colon suffix",
			path:     "/v1beta/models/gemini-2.5-flash",
			expected: "gemini-2.5-flash",
		},
		{
			name:     "Vertex AI style path",
			path:     "/v1/projects/my-project/locations/us-central1/publishers/google/models/gemini-pro:generateContent",
			expected: "gemini-pro",
		},
		{
			name:     "path with models/ in middle",
			path:     "/api/v1/models/gemini-2.0:generateContent",
			expected: "gemini-2.0",
		},
		{
			name:     "empty path",
			path:     "",
			expected: "",
		},
		{
			name:     "path without models/",
			path:     "/v1/chat/completions",
			expected: "",
		},
		{
			name:     "models/ at end",
			path:     "/v1/models/",
			expected: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractGeminiModel(tt.path)
			if got != tt.expected {
				t.Errorf("extractGeminiModel(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

// newTestLogger creates a logger that pushes to an httptest server (no real API calls).
func newTestLogger(t *testing.T) (*logging.Logger, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/sdk/v3/log" || r.URL.Path == "/api/sdk/v3/log-repositories" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{}"))
		}
	}))
	logger := logging.NewLogger(srv.URL, "test-key", &logging.LoggerConfig{
		Id:        "test-repo",
		AutoFlush: ptr(false),
	})
	return logger, srv
}

// newTestLoggerWithCapture creates a logger that captures push logs body for verification.
func newTestLoggerWithCapture(t *testing.T) (*logging.Logger, *httptest.Server, *[]string) {
	t.Helper()
	var captured []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/sdk/v3/log" || r.URL.Path == "/api/sdk/v3/log-repositories" {
			if r.Body != nil {
				body, _ := io.ReadAll(r.Body)
				captured = append(captured, string(body))
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("{}"))
		}
	}))
	logger := logging.NewLogger(srv.URL, "test-key", &logging.LoggerConfig{
		Id:        "test-repo",
		AutoFlush: ptr(false),
	})
	return logger, srv, &captured
}

func ptr(b bool) *bool {
	return &b
}

// testImageURL is a public image URL used across tests (same as tests package).
const testImageURL = "https://www.hollywoodreporter.com/wp-content/uploads/2014/06/optimuskneesstill.jpg?w=2000&h=1126&crop=1"

func TestMaximGeminiMiddleware_NoLogger_PassesThrough(t *testing.T) {
	// Ensure no auto-generated logger (LOG_REPO_ID not set in test env)
	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hello"}}},
		},
	}
	bodyBytes, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, "https://api.gemini.com/v1beta/models/gemini-pro:generateContent", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	nextCalled := false
	var receivedReq *http.Request
	next := func(r *http.Request) (*http.Response, error) {
		nextCalled = true
		receivedReq = r
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"candidates":[{"content":{"parts":[{"text":"hi"}],"role":"model"}],"modelVersion":"gemini-pro"}`)),
		}, nil
	}

	resp, err := MaximGeminiMiddleware(req, next)
	if err != nil {
		t.Fatalf("MaximGeminiMiddleware returned error: %v", err)
	}
	if !nextCalled {
		t.Error("next was not called")
	}
	if receivedReq != req {
		t.Error("next received different request")
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestMaximGeminiMiddleware_WithLogger_PassesThroughAndLogs(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hello"}}},
		},
	}
	bodyBytes, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, "https://api.gemini.com/v1beta/models/gemini-pro:generateContent", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), ContextKeyLogger, logger)
	req = req.WithContext(ctx)

	geminiResp := map[string]interface{}{
		"candidates": []map[string]interface{}{
			{
				"content": map[string]interface{}{
					"parts": []map[string]interface{}{{"text": "hello back"}},
					"role":  "model",
				},
				"finishReason": "STOP",
			},
		},
		"modelVersion": "gemini-pro",
		"usageMetadata": map[string]interface{}{
			"promptTokenCount":     10,
			"candidatesTokenCount": 5,
			"totalTokenCount":      15,
		},
	}
	respBytes, _ := json.Marshal(geminiResp)

	next := func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(respBytes)),
		}, nil
	}

	resp, err := MaximGeminiMiddleware(req, next)
	if err != nil {
		t.Fatalf("MaximGeminiMiddleware returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	// Verify response body is still readable
	gotBody, _ := io.ReadAll(resp.Body)
	var got map[string]interface{}
	if err := json.Unmarshal(gotBody, &got); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}
	if got["modelVersion"] != "gemini-pro" {
		t.Errorf("expected modelVersion gemini-pro, got %v", got["modelVersion"])
	}

	logger.Flush()
}

func TestMaximGeminiMiddleware_NextReturnsError_SetsGenerationError(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hello"}}},
		},
	}
	bodyBytes, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, "https://api.gemini.com/v1beta/models/gemini-pro:generateContent", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), ContextKeyLogger, logger)
	req = req.WithContext(ctx)

	nextErr := context.DeadlineExceeded
	next := func(r *http.Request) (*http.Response, error) {
		return nil, nextErr
	}

	resp, err := MaximGeminiMiddleware(req, next)
	if resp != nil {
		t.Error("expected nil response when next returns error")
	}
	if err != nextErr {
		t.Errorf("expected error %v, got %v", nextErr, err)
	}

	logger.Flush()
}

func TestMaximGeminiMiddleware_APIErrorInResponse_SetsGenerationError(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hello"}}},
		},
	}
	bodyBytes, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, "https://api.gemini.com/v1beta/models/unknown-model:generateContent", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), ContextKeyLogger, logger)
	req = req.WithContext(ctx)

	// Simulate Gemini API returning "unknown model" error (HTTP 404 with error in body)
	apiErrorResp := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    404,
			"message": "models/unknown-model is not found",
			"status":  "NOT_FOUND",
		},
	}
	apiErrorBytes, _ := json.Marshal(apiErrorResp)

	next := func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(bytes.NewBuffer(apiErrorBytes)),
		}, nil
	}

	resp, err := MaximGeminiMiddleware(req, next)
	if err != nil {
		t.Fatalf("middleware should not return error for API-level errors: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}
	// Verify response body is still readable
	gotBody, _ := io.ReadAll(resp.Body)
	var got map[string]interface{}
	if err := json.Unmarshal(gotBody, &got); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if got["error"] == nil {
		t.Error("expected error in response body")
	}

	logger.Flush()
	t.Log("API error correctly captured and logged to Maxim")
}

func TestMaximGeminiMiddleware_WithFileDataUrl_AddsUrlAttachment(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	// file_data with file_uri (e.g. from Gemini Files API upload)
	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]interface{}{
					{
						"file_data": map[string]interface{}{
							"mime_type": "image/jpeg",
							"file_uri":  testImageURL,
						},
					},
					{"text": "Describe this image."},
				},
			},
		},
	}
	bodyBytes, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, "https://api.gemini.com/v1beta/models/gemini-pro:generateContent", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), ContextKeyLogger, logger)
	req = req.WithContext(ctx)

	next := func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"candidates":[],"modelVersion":"gemini-pro"}`)),
		}, nil
	}

	resp, err := MaximGeminiMiddleware(req, next)
	if err != nil {
		t.Fatalf("MaximGeminiMiddleware returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	logger.Flush()
	t.Log("file_data with file_uri correctly processed (UrlAttachment added)")
}

func TestMaximGeminiMiddleware_ProviderOverrideFromContext(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hello"}}},
		},
	}
	bodyBytes, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, "https://api.gemini.com/v1beta/models/gemini-pro:generateContent", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), ContextKeyLogger, logger)
	ctx = context.WithValue(ctx, ContextKeyProvider, "vertex")
	req = req.WithContext(ctx)

	next := func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"candidates":[],"modelVersion":"gemini-pro"}`)),
		}, nil
	}

	resp, err := MaximGeminiMiddleware(req, next)
	if err != nil {
		t.Fatalf("MaximGeminiMiddleware returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	logger.Flush()
}

func TestMaximGeminiMiddleware_WithSystemInstruction(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hello"}}},
		},
		"systemInstruction": map[string]interface{}{
			"parts": []map[string]interface{}{{"text": "You are helpful."}},
		},
		"generationConfig": map[string]interface{}{
			"temperature": 0.7,
		},
	}
	bodyBytes, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, "https://api.gemini.com/v1beta/models/gemini-1.5-pro:generateContent", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), ContextKeyLogger, logger)
	req = req.WithContext(ctx)

	next := func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"candidates":[],"modelVersion":"gemini-1.5-pro"}`)),
		}, nil
	}

	resp, err := MaximGeminiMiddleware(req, next)
	if err != nil {
		t.Fatalf("MaximGeminiMiddleware returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	logger.Flush()
}

func TestMaximGeminiMiddleware_EmptyBody_NoPanic(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	req, err := http.NewRequest(http.MethodPost, "https://api.gemini.com/v1beta/models/gemini-pro:generateContent", bytes.NewReader([]byte("{}")))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), ContextKeyLogger, logger)
	req = req.WithContext(ctx)

	next := func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"candidates":[]}`)),
		}, nil
	}

	resp, err := MaximGeminiMiddleware(req, next)
	if err != nil {
		t.Fatalf("MaximGeminiMiddleware returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	logger.Flush()
}

func TestMaximGeminiMiddleware_RequestBodyRestored(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hello"}}},
		},
	}
	bodyBytes, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, "https://api.gemini.com/v1beta/models/gemini-pro:generateContent", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), ContextKeyLogger, logger)
	req = req.WithContext(ctx)

	next := func(r *http.Request) (*http.Response, error) {
		// Verify request body is still readable (middleware reads and restores it)
		readBody, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read request body in next: %v", err)
		}
		var m map[string]interface{}
		if err := json.Unmarshal(readBody, &m); err != nil {
			t.Errorf("failed to parse request body in next: %v", err)
		}
		if m["contents"] == nil {
			t.Error("request body contents was lost")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"candidates":[]}`)),
		}, nil
	}

	_, err = MaximGeminiMiddleware(req, next)
	if err != nil {
		t.Fatalf("MaximGeminiMiddleware returned error: %v", err)
	}

	logger.Flush()
}

func TestMaximGeminiHTTPClient_Do_WithLogger(t *testing.T) {
	logger, logSrv := newTestLogger(t)
	defer logSrv.Close()
	defer logger.Flush()

	// Backend server that returns a valid Gemini response
	geminiResp := map[string]interface{}{
		"candidates": []map[string]interface{}{
			{
				"content": map[string]interface{}{
					"parts": []map[string]interface{}{{"text": "hello from backend"}},
					"role":  "model",
				},
				"finishReason": "STOP",
			},
		},
		"modelVersion": "gemini-pro",
		"usageMetadata": map[string]interface{}{
			"promptTokenCount":     5,
			"candidatesTokenCount": 10,
			"totalTokenCount":      15,
		},
	}
	geminiRespBytes, _ := json.Marshal(geminiResp)
	backendSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(geminiRespBytes)
	}))
	defer backendSrv.Close()

	client := NewMaximGeminiTransport(logger)

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hello"}}},
		},
	}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequest(http.MethodPost, backendSrv.URL+"/v1beta/models/gemini-pro:generateContent", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	gotBody, _ := io.ReadAll(resp.Body)
	var got map[string]interface{}
	if err := json.Unmarshal(gotBody, &got); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if got["modelVersion"] != "gemini-pro" {
		t.Errorf("expected modelVersion gemini-pro, got %v", got["modelVersion"])
	}

	logger.Flush()
}

func TestMaximGeminiHTTPClient_Do_WithNilLogger_PassesThrough(t *testing.T) {
	// Client with nil logger - when autoGeneratedLogger is also nil, it should still work
	client := NewMaximGeminiTransport(nil)

	geminiResp := map[string]interface{}{
		"candidates": []map[string]interface{}{
			{
				"content": map[string]interface{}{
					"parts": []map[string]interface{}{{"text": "hi"}},
					"role":  "model",
				},
				"finishReason": "STOP",
			},
		},
		"modelVersion": "gemini-pro",
		"usageMetadata": map[string]interface{}{
			"promptTokenCount":     1,
			"candidatesTokenCount": 1,
			"totalTokenCount":      2,
		},
	}
	geminiRespBytes, _ := json.Marshal(geminiResp)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(geminiRespBytes)
	}))
	defer srv.Close()

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hello"}}},
		},
	}
	bodyBytes, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/v1beta/models/gemini-pro:generateContent", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	gotBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(gotBody, &got); err != nil {
		t.Fatalf("failed to parse response %q: %v", string(gotBody), err)
	}
	if got["modelVersion"] != "gemini-pro" {
		t.Errorf("expected modelVersion gemini-pro, got %v", got["modelVersion"])
	}
}

func TestMaximGeminiMiddleware_WithToolsInBody(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hello"}}},
		},
		"tools": []map[string]interface{}{
			{"functionDeclarations": []map[string]interface{}{{"name": "get_weather"}}},
		},
	}
	bodyBytes, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, "https://api.gemini.com/v1beta/models/gemini-pro:generateContent", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), ContextKeyLogger, logger)
	req = req.WithContext(ctx)

	next := func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"candidates":[],"modelVersion":"gemini-pro"}`)),
		}, nil
	}

	resp, err := MaximGeminiMiddleware(req, next)
	if err != nil {
		t.Fatalf("MaximGeminiMiddleware returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	logger.Flush()
}

func TestMaximGeminiMiddleware_WithTags_AddsTagsToTraceAndGeneration(t *testing.T) {
	logger, srv, captured := newTestLoggerWithCapture(t)
	defer srv.Close()
	defer logger.Flush()

	traceTags := map[string]string{"env": "test", "team": "ai"}
	generationTags := map[string]string{"model_type": "chat", "source": "api"}

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hello"}}},
		},
	}
	bodyBytes, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, "https://api.gemini.com/v1beta/models/gemini-pro:generateContent", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), ContextKeyLogger, logger)
	ctx = context.WithValue(ctx, ContextKeyTraceTags, traceTags)
	ctx = context.WithValue(ctx, ContextKeyGenerationTags, generationTags)
	req = req.WithContext(ctx)

	next := func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"candidates":[{"content":{"parts":[{"text":"hi"}],"role":"model"}],"modelVersion":"gemini-pro"}`)),
		}, nil
	}

	resp, err := MaximGeminiMiddleware(req, next)
	if err != nil {
		t.Fatalf("MaximGeminiMiddleware returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	logger.Flush()
	if len(*captured) == 0 {
		t.Fatal("expected push logs to be captured")
	}
	bodyStr := ""
	for _, b := range *captured {
		bodyStr += b
	}
	// Tags are in create/add-generation data
	if !strings.Contains(bodyStr, `"env":"test"`) || !strings.Contains(bodyStr, `"team":"ai"`) {
		t.Errorf("expected trace tags in push logs, got: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, `"model_type":"chat"`) || !strings.Contains(bodyStr, `"source":"api"`) {
		t.Errorf("expected generation tags in push logs, got: %s", bodyStr)
	}
}

func TestMaximGeminiMiddleware_WithMetrics_AddsMetricsToTraceAndGeneration(t *testing.T) {
	logger, srv, captured := newTestLoggerWithCapture(t)
	defer srv.Close()
	defer logger.Flush()

	traceMetrics := map[string]float64{"latency_ms": 150.5, "tool_calls_count": 3}
	generationMetrics := map[string]float64{"tokens_in": 100, "tokens_out": 50}

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hello"}}},
		},
	}
	bodyBytes, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, "https://api.gemini.com/v1beta/models/gemini-pro:generateContent", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), ContextKeyLogger, logger)
	ctx = context.WithValue(ctx, ContextKeyTraceMetrics, traceMetrics)
	ctx = context.WithValue(ctx, ContextKeyGenerationMetrics, generationMetrics)
	req = req.WithContext(ctx)

	next := func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"candidates":[{"content":{"parts":[{"text":"hi"}],"role":"model"}],"modelVersion":"gemini-pro"}`)),
		}, nil
	}

	resp, err := MaximGeminiMiddleware(req, next)
	if err != nil {
		t.Fatalf("MaximGeminiMiddleware returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	logger.Flush()
	if len(*captured) == 0 {
		t.Fatal("expected push logs to be captured")
	}
	bodyStr := ""
	for _, b := range *captured {
		bodyStr += b
	}
	if !strings.Contains(bodyStr, `"metrics"`) {
		t.Errorf("expected metrics in push logs, got: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "latency_ms") || !strings.Contains(bodyStr, "150.5") {
		t.Errorf("expected trace metric latency_ms in push logs, got: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "tokens_in") || !strings.Contains(bodyStr, "100") {
		t.Errorf("expected generation metric tokens_in in push logs, got: %s", bodyStr)
	}
}

func TestMaximGeminiMiddleware_PathWithoutModel_NoGeneration(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hello"}}},
		},
	}
	bodyBytes, _ := json.Marshal(body)
	// Path without models/ - extractGeminiModel returns ""
	req, err := http.NewRequest(http.MethodPost, "https://api.gemini.com/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), ContextKeyLogger, logger)
	req = req.WithContext(ctx)

	next := func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
		}, nil
	}

	resp, err := MaximGeminiMiddleware(req, next)
	if err != nil {
		t.Fatalf("MaximGeminiMiddleware returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	logger.Flush()
}

func TestMaximGeminiMiddleware_InvalidResponseJSON_StillPassesThrough(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hello"}}},
		},
	}
	bodyBytes, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, "https://api.gemini.com/v1beta/models/gemini-pro:generateContent", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), ContextKeyLogger, logger)
	req = req.WithContext(ctx)

	// Invalid JSON response - middleware should still pass it through
	invalidJSON := `{invalid json}`
	next := func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(invalidJSON)),
		}, nil
	}

	resp, err := MaximGeminiMiddleware(req, next)
	if err != nil {
		t.Fatalf("MaximGeminiMiddleware returned error: %v", err)
	}
	gotBody, _ := io.ReadAll(resp.Body)
	if string(gotBody) != invalidJSON {
		t.Errorf("expected response body to be passed through unchanged, got %q", string(gotBody))
	}

	logger.Flush()
}

func TestMaximGeminiMiddleware_ContextValues(t *testing.T) {
	logger, srv := newTestLogger(t)
	defer srv.Close()
	defer logger.Flush()

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "hello"}}},
		},
	}
	bodyBytes, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, "https://api.gemini.com/v1beta/models/gemini-pro:generateContent", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), ContextKeyLogger, logger)
	ctx = context.WithValue(ctx, ContextKeyTraceId, uuid.NewString())
	ctx = context.WithValue(ctx, ContextKeyTraceName, "CustomTrace")
	ctx = context.WithValue(ctx, ContextKeyGenerationName, "CustomGen")
	req = req.WithContext(ctx)

	next := func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"candidates":[]}`)),
		}, nil
	}

	resp, err := MaximGeminiMiddleware(req, next)
	if err != nil {
		t.Fatalf("MaximGeminiMiddleware returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	logger.Flush()
}
