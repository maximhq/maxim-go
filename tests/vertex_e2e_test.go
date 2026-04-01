package tests

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	maxim "github.com/maximhq/maxim-go"
	"github.com/maximhq/maxim-go/logging"
	"github.com/maximhq/maxim-go/middlewares"
	"github.com/maximhq/maxim-go/schemas"
	"golang.org/x/oauth2/google"
)

// vertexBaseURL returns the Vertex AI base URL for the given project and location.
func vertexBaseURL(project, location string) string {
	return "https://" + location + "-aiplatform.googleapis.com/v1/projects/" + project + "/locations/" + location + "/publishers/google/models"
}

// getVertexCredentials resolves Vertex AI credentials from the environment and returns
// the project ID, location, and a valid access token. It tries the following sources in order:
//
//  1. VERTEX_ACCESS_TOKEN — a pre-obtained Bearer token (e.g. from gcloud auth print-access-token)
//  2. GOOGLE_CREDENTIALS_JSON — raw service account JSON exported as an env var
//  3. GOOGLE_APPLICATION_CREDENTIALS — path to a service account JSON key file
//
// VERTEX_PROJECT_ID is required unless the credentials JSON contains a project_id field.
// VERTEX_LOCATION defaults to us-central1 if not set.
func getVertexCredentials(t *testing.T) (project, location, accessToken string) {
	t.Helper()

	ctx := context.Background()
	const scope = "https://www.googleapis.com/auth/cloud-platform"

	// Resolve access token
	if tok := os.Getenv("VERTEX_ACCESS_TOKEN"); tok != "" {
		accessToken = tok
	} else {
		// Try raw JSON from env var first, then fall back to file path
		var credJSON []byte
		if raw := os.Getenv("GOOGLE_CREDENTIALS_JSON"); raw != "" {
			credJSON = []byte(raw)
		} else if path := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); path != "" {
			var err error
			credJSON, err = os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read GOOGLE_APPLICATION_CREDENTIALS file %q: %v", path, err)
			}
		}

		if credJSON == nil {
			t.Skip("no Vertex AI credentials found — set VERTEX_ACCESS_TOKEN, GOOGLE_CREDENTIALS_JSON, or GOOGLE_APPLICATION_CREDENTIALS")
		}

		creds, err := google.CredentialsFromJSONWithTypeAndParams(ctx, credJSON, google.ServiceAccount, google.CredentialsParams{
			Scopes: []string{scope},
		})
		if err != nil {
			t.Fatalf("failed to parse credentials JSON: %v", err)
		}

		// Extract project from credentials JSON if VERTEX_PROJECT_ID is not set
		if os.Getenv("VERTEX_PROJECT_ID") == "" && creds.ProjectID != "" {
			project = creds.ProjectID
		}

		tok, err := creds.TokenSource.Token()
		if err != nil {
			t.Fatalf("failed to obtain access token from credentials: %v", err)
		}
		accessToken = tok.AccessToken
	}

	// Resolve project ID
	if project == "" {
		project = os.Getenv("VERTEX_PROJECT_ID")
	}
	if project == "" {
		t.Skip("VERTEX_PROJECT_ID not set and could not be inferred from credentials")
	}

	// Resolve location
	location = os.Getenv("VERTEX_LOCATION")
	if location == "" {
		location = "us-central1"
	}

	return project, location, accessToken
}

// getVertexLogger returns a Maxim logger configured from environment variables.
func getVertexLogger(t *testing.T) *logging.Logger {
	t.Helper()
	apiKey := os.Getenv("MAXIM_API_KEY")
	logRepoID := os.Getenv("MAXIM_LOG_REPO_ID")
	if apiKey == "" || logRepoID == "" {
		t.Skip("MAXIM_API_KEY or MAXIM_LOG_REPO_ID not set, skipping Vertex AI integration test")
	}
	logger, err := maxim.Init(&maxim.MaximSDKConfig{
		ApiKey: apiKey,
	}).GetLogger(&logging.LoggerConfig{
		Id: logRepoID,
	})
	if err != nil {
		t.Fatalf("failed to create Maxim logger: %v", err)
	}
	return logger
}

// vertexChat makes a non-streaming Vertex AI request and returns the parsed response.
func vertexChat(t *testing.T, transport *middlewares.MaximVertexTransport, project, location, accessToken, prompt, model string) map[string]interface{} {
	t.Helper()
	return vertexChatWithContext(t, context.Background(), transport, project, location, accessToken, prompt, model)
}

// vertexChatWithContext is like vertexChat but accepts a context (e.g. with traceId).
func vertexChatWithContext(t *testing.T, ctx context.Context, transport *middlewares.MaximVertexTransport, project, location, accessToken, prompt, model string) map[string]interface{} {
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
	return vertexRequestWithContext(t, ctx, transport, project, location, accessToken, reqBody, model)
}

// vertexRequest makes a Vertex AI request with a custom request body.
func vertexRequest(t *testing.T, transport *middlewares.MaximVertexTransport, project, location, accessToken string, reqBody map[string]interface{}, model string) map[string]interface{} {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return vertexRequestWithContext(t, ctx, transport, project, location, accessToken, reqBody, model)
}

// vertexRequestWithContext makes a Vertex AI request with a custom body and context.
func vertexRequestWithContext(t *testing.T, ctx context.Context, transport *middlewares.MaximVertexTransport, project, location, accessToken string, reqBody map[string]interface{}, model string) map[string]interface{} {
	t.Helper()

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	url := vertexBaseURL(project, location) + "/" + model + ":generateContent"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := transport.Do(req)
	if err != nil {
		t.Fatalf("Vertex AI request failed: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if errObj, ok := result["error"]; ok {
		t.Fatalf("Vertex AI returned error: %v", errObj)
	}
	return result
}

// vertexRequestStream makes a streaming Vertex AI request and returns accumulated text.
func vertexRequestStream(t *testing.T, transport *middlewares.MaximVertexTransport, project, location, accessToken string, reqBody map[string]interface{}, model string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return vertexRequestStreamWithContext(t, ctx, transport, project, location, accessToken, reqBody, model)
}

// vertexRequestStreamWithContext is like vertexRequestStream but accepts a context.
func vertexRequestStreamWithContext(t *testing.T, ctx context.Context, transport *middlewares.MaximVertexTransport, project, location, accessToken string, reqBody map[string]interface{}, model string) string {
	t.Helper()

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	url := vertexBaseURL(project, location) + "/" + model + ":streamGenerateContent?alt=sse"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := transport.Do(req)
	if err != nil {
		t.Fatalf("Vertex AI streaming request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Vertex AI stream returned status %d", resp.StatusCode)
	}

	var content strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		data, ok := strings.CutPrefix(line, "data: ")
		if !ok || data == "" || data == "[DONE]" {
			continue
		}
		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		candidates, _ := chunk["candidates"].([]interface{})
		for _, c := range candidates {
			cand, _ := c.(map[string]interface{})
			cont, _ := cand["content"].(map[string]interface{})
			parts, _ := cont["parts"].([]interface{})
			for _, p := range parts {
				if part, ok := p.(map[string]interface{}); ok {
					if text, ok := part["text"].(string); ok {
						content.WriteString(text)
					}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("reading Vertex AI stream: %v", err)
	}
	return content.String()
}

// parseAndValidateVertexResult runs ParseResult for the vertex provider and fails
// the test if parsing returns an error, nil result, or missing choices.
func parseAndValidateVertexResult(t *testing.T, result map[string]interface{}, model string) *schemas.MaximLLMResult {
	t.Helper()
	parsed, err := logging.ParseResult(logging.ProviderVertex, model, result)
	if err != nil {
		t.Fatalf("ParseResult failed: %v", err)
	}
	if parsed == nil {
		t.Fatal("ParseResult returned nil")
	}
	if len(parsed.Choices) == 0 {
		t.Fatal("ParseResult: expected at least one choice")
	}
	t.Logf("ParseResult OK — model=%s tokens=%d", parsed.Model, parsed.Usage.TotalTokens)
	return parsed
}

const vertexModel = "gemini-2.5-flash"

// ---------------------------------------------------------------------------
// Basic completion
// ---------------------------------------------------------------------------

func TestVertex_E2E_SimpleCompletion(t *testing.T) {
	project, location, accessToken := getVertexCredentials(t)
	logger := getVertexLogger(t)
	defer logger.Flush()

	transport := middlewares.NewMaximVertexTransport(logger)

	result := vertexChat(t, transport, project, location, accessToken, "Say hello in one word.", vertexModel)

	t.Logf("raw usageMetadata: %v", result["usageMetadata"])

	parsed := parseAndValidateVertexResult(t, result, vertexModel)
	if parsed.Choices[0].Message.Content == "" {
		t.Error("expected non-empty response content")
	}
	if parsed.Usage.TotalTokens == 0 {
		t.Error("expected non-zero total token count")
	}

	logger.Flush()
	t.Logf("response: %q", parsed.Choices[0].Message.Content)
}

// ---------------------------------------------------------------------------
// Streaming
// ---------------------------------------------------------------------------

func TestVertex_E2E_SimpleCompletionStream(t *testing.T) {
	project, location, accessToken := getVertexCredentials(t)
	logger := getVertexLogger(t)
	defer logger.Flush()

	transport := middlewares.NewMaximVertexTransport(logger)

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "Say hello in one word."}}},
		},
	}
	content := vertexRequestStream(t, transport, project, location, accessToken, reqBody, vertexModel)
	if content == "" {
		t.Error("expected non-empty streamed response")
	}
	logger.Flush()
	t.Logf("stream response: %q", content)
}

// TestVertex_E2E_StreamingWithUsageMetadata verifies that the last SSE chunk carries
// usageMetadata and that the middleware captures token counts for streaming requests.
func TestVertex_E2E_StreamingWithUsageMetadata(t *testing.T) {
	project, location, accessToken := getVertexCredentials(t)
	logger := getVertexLogger(t)
	defer logger.Flush()

	transport := middlewares.NewMaximVertexTransport(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	bodyBytes, err := json.Marshal(map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "Count from 1 to 5."}}},
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	url := vertexBaseURL(project, location) + "/" + vertexModel + ":streamGenerateContent?alt=sse"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := transport.Do(req)
	if err != nil {
		t.Fatalf("stream request failed: %v", err)
	}
	defer resp.Body.Close()

	var lastUsageMetadata map[string]interface{}
	var accumulated strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		data, ok := strings.CutPrefix(line, "data: ")
		if !ok || data == "" || data == "[DONE]" {
			continue
		}
		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if u, ok := chunk["usageMetadata"].(map[string]interface{}); ok {
			lastUsageMetadata = u
		}
		candidates, _ := chunk["candidates"].([]interface{})
		for _, c := range candidates {
			cand, _ := c.(map[string]interface{})
			cont, _ := cand["content"].(map[string]interface{})
			parts, _ := cont["parts"].([]interface{})
			for _, p := range parts {
				if part, ok := p.(map[string]interface{}); ok {
					if text, ok := part["text"].(string); ok {
						accumulated.WriteString(text)
					}
				}
			}
		}
	}

	t.Logf("last usageMetadata chunk: %v", lastUsageMetadata)
	if accumulated.Len() == 0 {
		t.Error("expected non-empty streamed content")
	}
	if lastUsageMetadata == nil {
		t.Error("usageMetadata absent from all streaming chunks")
	}

	logger.Flush()
}

// ---------------------------------------------------------------------------
// Trace context
// ---------------------------------------------------------------------------

func TestVertex_E2E_WithTraceContext(t *testing.T) {
	project, location, accessToken := getVertexCredentials(t)
	logger := getVertexLogger(t)
	defer logger.Flush()

	transport := middlewares.NewMaximVertexTransport(logger)

	ctx := context.Background()
	ctx = context.WithValue(ctx, middlewares.ContextKeyTraceName, "vertex-trace-context-test")
	ctx = context.WithValue(ctx, middlewares.ContextKeyGenerationName, "vertex-gen-context-test")

	result := vertexChatWithContext(t, ctx, transport, project, location, accessToken, "What is 2+2? Reply with just the number.", vertexModel)
	parseAndValidateVertexResult(t, result, vertexModel)

	logger.Flush()
	t.Log("completed Vertex e2e with trace context")
}

func TestVertex_E2E_WithTraceContextStream(t *testing.T) {
	project, location, accessToken := getVertexCredentials(t)
	logger := getVertexLogger(t)
	defer logger.Flush()

	transport := middlewares.NewMaximVertexTransport(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, middlewares.ContextKeyTraceName, "vertex-trace-context-test")
	ctx = context.WithValue(ctx, middlewares.ContextKeyGenerationName, "vertex-gen-context-test")

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "What is 2+2?"}}},
		},
	}
	content := vertexRequestStreamWithContext(t, ctx, transport, project, location, accessToken, reqBody, vertexModel)
	if content == "" {
		t.Error("expected non-empty streamed response")
	}
	logger.Flush()
	t.Logf("stream response: %q", content)
}

// ---------------------------------------------------------------------------
// Tags and metrics
// ---------------------------------------------------------------------------

func TestVertex_E2E_WithTagsAndMetrics(t *testing.T) {
	project, location, accessToken := getVertexCredentials(t)
	logger := getVertexLogger(t)
	defer logger.Flush()

	transport := middlewares.NewMaximVertexTransport(logger)

	ctx := context.Background()
	ctx = context.WithValue(ctx, middlewares.ContextKeyTraceTags, map[string]string{"e2e_test": "vertex_tags", "env": "test"})
	ctx = context.WithValue(ctx, middlewares.ContextKeyGenerationTags, map[string]string{"model_type": "chat", "source": "e2e"})
	ctx = context.WithValue(ctx, middlewares.ContextKeyTraceMetrics, map[string]float64{"latency_ms": 0.69})
	ctx = context.WithValue(ctx, middlewares.ContextKeyGenerationMetrics, map[string]float64{"tokens_in": 100})

	result := vertexChatWithContext(t, ctx, transport, project, location, accessToken, "Say hello in one word.", vertexModel)
	parsed := parseAndValidateVertexResult(t, result, vertexModel)

	logger.Flush()
	t.Logf("response: %q", parsed.Choices[0].Message.Content)
}

func TestVertex_E2E_WithTagsAndMetricsStream(t *testing.T) {
	project, location, accessToken := getVertexCredentials(t)
	logger := getVertexLogger(t)
	defer logger.Flush()

	transport := middlewares.NewMaximVertexTransport(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, middlewares.ContextKeyTraceTags, map[string]string{"e2e_test": "vertex_tags", "env": "test"})
	ctx = context.WithValue(ctx, middlewares.ContextKeyGenerationTags, map[string]string{"model_type": "chat"})
	ctx = context.WithValue(ctx, middlewares.ContextKeyTraceMetrics, map[string]float64{"latency_ms": 0.42})
	ctx = context.WithValue(ctx, middlewares.ContextKeyGenerationMetrics, map[string]float64{"tokens_in": 50})

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "Say hello in one word."}}},
		},
	}
	content := vertexRequestStreamWithContext(t, ctx, transport, project, location, accessToken, reqBody, vertexModel)
	if content == "" {
		t.Error("expected non-empty streamed response")
	}
	logger.Flush()
	t.Logf("stream response: %q", content)
}

// ---------------------------------------------------------------------------
// System instruction
// ---------------------------------------------------------------------------

func TestVertex_E2E_WithSystemInstruction(t *testing.T) {
	project, location, accessToken := getVertexCredentials(t)
	logger := getVertexLogger(t)
	defer logger.Flush()

	transport := middlewares.NewMaximVertexTransport(logger)

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "What color is the sky?"}}},
		},
		"systemInstruction": map[string]interface{}{
			"parts": []map[string]interface{}{{"text": "You are a helpful assistant. Keep answers brief."}},
		},
	}
	result := vertexRequest(t, transport, project, location, accessToken, reqBody, vertexModel)
	parseAndValidateVertexResult(t, result, vertexModel)

	logger.Flush()
	t.Log("completed Vertex e2e with system instruction")
}

func TestVertex_E2E_WithSystemInstructionStream(t *testing.T) {
	project, location, accessToken := getVertexCredentials(t)
	logger := getVertexLogger(t)
	defer logger.Flush()

	transport := middlewares.NewMaximVertexTransport(logger)

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "What color is the sky?"}}},
		},
		"systemInstruction": map[string]interface{}{
			"parts": []map[string]interface{}{{"text": "You are a helpful assistant. Keep answers brief."}},
		},
	}
	content := vertexRequestStream(t, transport, project, location, accessToken, reqBody, vertexModel)
	if content == "" {
		t.Error("expected non-empty streamed response")
	}
	logger.Flush()
	t.Logf("stream response: %q", content)
}

// ---------------------------------------------------------------------------
// Tool calls
// ---------------------------------------------------------------------------

var vertexWeatherTools = []map[string]interface{}{
	{
		"functionDeclarations": []map[string]interface{}{
			{
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
	},
}

func TestVertex_E2E_WithToolCalls(t *testing.T) {
	project, location, accessToken := getVertexCredentials(t)
	logger := getVertexLogger(t)
	defer logger.Flush()

	transport := middlewares.NewMaximVertexTransport(logger)

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "What's the weather in Paris? Use the get_weather function."}}},
		},
		"tools": vertexWeatherTools,
	}
	result := vertexRequest(t, transport, project, location, accessToken, reqBody, vertexModel)
	parsed := parseAndValidateVertexResult(t, result, vertexModel)

	if len(parsed.Choices[0].Message.ToolCalls) > 0 {
		t.Logf("tool call: %s(%s)", parsed.Choices[0].Message.ToolCalls[0].Function.Name, parsed.Choices[0].Message.ToolCalls[0].Function.Arguments)
	} else {
		t.Logf("text response: %q", parsed.Choices[0].Message.Content)
	}
	logger.Flush()
}

func TestVertex_E2E_WithToolCallsStream(t *testing.T) {
	project, location, accessToken := getVertexCredentials(t)
	logger := getVertexLogger(t)
	defer logger.Flush()

	transport := middlewares.NewMaximVertexTransport(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	bodyBytes, err := json.Marshal(map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "What's the weather in Paris? Use the get_weather function."}}},
		},
		"tools": vertexWeatherTools,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	url := vertexBaseURL(project, location) + "/" + vertexModel + ":streamGenerateContent?alt=sse"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	streamResp, err := transport.Do(req)
	if err != nil {
		t.Fatalf("stream request failed: %v", err)
	}
	defer streamResp.Body.Close()

	var functionCallCount int
	scanner := bufio.NewScanner(streamResp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		data, ok := strings.CutPrefix(line, "data: ")
		if !ok || data == "" || data == "[DONE]" {
			continue
		}
		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		candidates, _ := chunk["candidates"].([]interface{})
		for _, c := range candidates {
			cand, _ := c.(map[string]interface{})
			cont, _ := cand["content"].(map[string]interface{})
			parts, _ := cont["parts"].([]interface{})
			for _, p := range parts {
				if part, ok := p.(map[string]interface{}); ok {
					if _, hasFunctionCall := part["functionCall"]; hasFunctionCall {
						functionCallCount++
					}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("reading stream: %v", err)
	}
	if functionCallCount == 0 {
		t.Error("expected at least one functionCall part in stream, got none")
	}
	logger.Flush()
	t.Logf("function call parts in stream: %d", functionCallCount)
}

// TestVertex_E2E_WithToolCallsFullFlow tests the full tool-calling round-trip:
// request 1 → model returns function call → tool call is logged → we simulate
// execution → request 2 → final text. Both requests share the same traceId so
// all three steps (LLM call 1, tool execution, LLM call 2) appear in one trace.
func TestVertex_E2E_WithToolCallsFullFlow(t *testing.T) {
	project, location, accessToken := getVertexCredentials(t)
	logger := getVertexLogger(t)
	defer logger.Flush()

	transport := middlewares.NewMaximVertexTransport(logger)

	traceId := uuid.NewString()
	// Pre-create the trace so we can add the tool call to it between requests.
	trace := logger.Trace(&logging.TraceConfig{Id: traceId, Name: strPtr("vertex-tool-call-full-flow")})

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, middlewares.ContextKeyTraceId, traceId)
	ctx = context.WithValue(ctx, middlewares.ContextKeyLogger, logger)

	reqBody1 := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "What's the weather in Paris? Use the get_weather function."}}},
		},
		"tools": vertexWeatherTools,
	}

	result1 := vertexRequestWithContext(t, ctx, transport, project, location, accessToken, reqBody1, vertexModel)
	parsed1 := parseAndValidateVertexResult(t, result1, vertexModel)

	if len(parsed1.Choices[0].Message.ToolCalls) == 0 {
		t.Fatal("expected function call in first response")
	}
	funcName := parsed1.Choices[0].Message.ToolCalls[0].Function.Name
	funcArgs := parsed1.Choices[0].Message.ToolCalls[0].Function.Arguments
	toolResultMap := map[string]interface{}{"result": "Sunny, 22°C"}

	// Log the tool execution step so it appears in the trace between the two LLM calls.
	toolResultJSON, _ := json.Marshal(toolResultMap)
	tc := trace.AddToolCall(&logging.ToolCallConfig{
		Id:   uuid.NewString(),
		Name: &funcName,
		Args: &funcArgs,
	})
	tc.SetResult(string(toolResultJSON))

	candidates, _ := result1["candidates"].([]interface{})
	if len(candidates) == 0 {
		t.Fatal("expected candidates in response")
	}
	modelContent := candidates[0].(map[string]interface{})["content"]

	contents2 := []interface{}{
		reqBody1["contents"].([]map[string]interface{})[0],
		modelContent,
		map[string]interface{}{
			"role": "user",
			"parts": []map[string]interface{}{
				{"functionResponse": map[string]interface{}{"name": funcName, "response": toolResultMap}},
			},
		},
	}
	reqBody2 := map[string]interface{}{
		"contents": contents2,
		"tools":    vertexWeatherTools,
	}

	result2 := vertexRequestWithContext(t, ctx, transport, project, location, accessToken, reqBody2, vertexModel)
	parsed2 := parseAndValidateVertexResult(t, result2, vertexModel)

	if parsed2.Choices[0].Message.Content == "" {
		t.Error("expected non-empty final text response after function result")
	}

	trace.End()
	logger.Flush()
	t.Logf("full-flow tool call complete — final: %q", parsed2.Choices[0].Message.Content)
}

// TestVertex_E2E_WithToolCallsFullFlowStream is like TestVertex_E2E_WithToolCallsFullFlow
// but the second request uses streamGenerateContent.
func TestVertex_E2E_WithToolCallsFullFlowStream(t *testing.T) {
	project, location, accessToken := getVertexCredentials(t)
	logger := getVertexLogger(t)
	defer logger.Flush()

	transport := middlewares.NewMaximVertexTransport(logger)

	traceId := uuid.NewString()
	// Pre-create the trace so we can add the tool call to it between requests.
	trace := logger.Trace(&logging.TraceConfig{Id: traceId, Name: strPtr("vertex-tool-call-full-flow-stream")})

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, middlewares.ContextKeyTraceId, traceId)
	ctx = context.WithValue(ctx, middlewares.ContextKeyLogger, logger)

	reqBody1 := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "What's the weather in Paris? Use the get_weather function."}}},
		},
		"tools": vertexWeatherTools,
	}

	result1 := vertexRequestWithContext(t, ctx, transport, project, location, accessToken, reqBody1, vertexModel)
	parsed1 := parseAndValidateVertexResult(t, result1, vertexModel)
	if len(parsed1.Choices[0].Message.ToolCalls) == 0 {
		t.Fatal("expected function call in first response")
	}
	funcName := parsed1.Choices[0].Message.ToolCalls[0].Function.Name
	funcArgs := parsed1.Choices[0].Message.ToolCalls[0].Function.Arguments
	toolResultMap := map[string]interface{}{"result": "Sunny, 22°C"}

	// Log the tool execution step between the two LLM calls.
	toolResultJSON, _ := json.Marshal(toolResultMap)
	tc := trace.AddToolCall(&logging.ToolCallConfig{
		Id:   uuid.NewString(),
		Name: &funcName,
		Args: &funcArgs,
	})
	tc.SetResult(string(toolResultJSON))

	candidates, _ := result1["candidates"].([]interface{})
	if len(candidates) == 0 {
		t.Fatal("expected candidates in response")
	}
	modelContent := candidates[0].(map[string]interface{})["content"]

	contents2 := []interface{}{
		reqBody1["contents"].([]map[string]interface{})[0],
		modelContent,
		map[string]interface{}{
			"role":  "user",
			"parts": []map[string]interface{}{{"functionResponse": map[string]interface{}{"name": funcName, "response": toolResultMap}}},
		},
	}
	reqBody2 := map[string]interface{}{"contents": contents2, "tools": vertexWeatherTools}
	content := vertexRequestStreamWithContext(t, ctx, transport, project, location, accessToken, reqBody2, vertexModel)
	if content == "" {
		t.Error("expected non-empty final text response after function result (stream)")
	}

	trace.End()
	logger.Flush()
	t.Logf("full-flow tool call (stream) complete — final: %q", content)
}

// ---------------------------------------------------------------------------
// Multi-turn conversation
// ---------------------------------------------------------------------------

func TestVertex_E2E_MultiTurn(t *testing.T) {
	project, location, accessToken := getVertexCredentials(t)
	logger := getVertexLogger(t)
	defer logger.Flush()

	transport := middlewares.NewMaximVertexTransport(logger)

	traceId := uuid.NewString()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, middlewares.ContextKeyTraceId, traceId)

	// Turn 1
	reqBody1 := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "My name is Alex. Remember it."}}},
		},
	}
	result1 := vertexRequestWithContext(t, ctx, transport, project, location, accessToken, reqBody1, vertexModel)
	candidates1, _ := result1["candidates"].([]interface{})
	if len(candidates1) == 0 {
		t.Fatal("expected candidates in turn 1 response")
	}
	modelTurn1 := candidates1[0].(map[string]interface{})["content"]

	// Turn 2 — ask the model to recall the name
	reqBody2 := map[string]interface{}{
		"contents": []interface{}{
			reqBody1["contents"].([]map[string]interface{})[0],
			modelTurn1,
			map[string]interface{}{
				"role":  "user",
				"parts": []map[string]interface{}{{"text": "What is my name?"}},
			},
		},
	}
	result2 := vertexRequestWithContext(t, ctx, transport, project, location, accessToken, reqBody2, vertexModel)
	parsed2 := parseAndValidateVertexResult(t, result2, vertexModel)

	if !strings.Contains(strings.ToLower(parsed2.Choices[0].Message.Content), "alex") {
		t.Errorf("expected model to recall name 'Alex', got: %q", parsed2.Choices[0].Message.Content)
	}

	logger.EndTrace(traceId)
	logger.Flush()
	t.Logf("multi-turn complete — turn 2 response: %q", parsed2.Choices[0].Message.Content)
}

// ---------------------------------------------------------------------------
// Session
// ---------------------------------------------------------------------------

// TestVertex_E2E_WithSession creates a Maxim session and runs two Vertex AI requests
// under the same session, each with its own trace, to verify session-level grouping.
func TestVertex_E2E_WithSession(t *testing.T) {
	project, location, accessToken := getVertexCredentials(t)
	logger := getVertexLogger(t)
	defer logger.Flush()

	transport := middlewares.NewMaximVertexTransport(logger)

	sessionName := "vertex-e2e-session"
	session := logger.Session(&logging.SessionConfig{
		Id:   uuid.NewString(),
		Name: &sessionName,
		Tags: &map[string]string{"e2e_test": "vertex_session"},
	})

	// Request 1 — attached to the session via AddTrace
	traceId1 := uuid.NewString()
	_ = logger.AddTraceToSession(session.Id(), &logging.TraceConfig{Id: traceId1, Name: strPtr("turn-1")})

	ctx1, cancel1 := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel1()
	ctx1 = context.WithValue(ctx1, middlewares.ContextKeyTraceId, traceId1)

	result1 := vertexChatWithContext(t, ctx1, transport, project, location, accessToken, "Say hello in one word.", vertexModel)
	parseAndValidateVertexResult(t, result1, vertexModel)
	logger.EndTrace(traceId1)

	// Request 2 — a follow-up under the same session
	traceId2 := uuid.NewString()
	_ = logger.AddTraceToSession(session.Id(), &logging.TraceConfig{Id: traceId2, Name: strPtr("turn-2")})

	ctx2, cancel2 := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel2()
	ctx2 = context.WithValue(ctx2, middlewares.ContextKeyTraceId, traceId2)

	result2 := vertexChatWithContext(t, ctx2, transport, project, location, accessToken, "What is the capital of France?", vertexModel)
	parsed2 := parseAndValidateVertexResult(t, result2, vertexModel)
	logger.EndTrace(traceId2)

	session.End()
	logger.Flush()
	t.Logf("session complete — turn 2 response: %q", parsed2.Choices[0].Message.Content)
}

// TestVertex_E2E_WithSessionStream is like TestVertex_E2E_WithSession but uses streaming.
func TestVertex_E2E_WithSessionStream(t *testing.T) {
	project, location, accessToken := getVertexCredentials(t)
	logger := getVertexLogger(t)
	defer logger.Flush()

	transport := middlewares.NewMaximVertexTransport(logger)

	sessionName := "vertex-e2e-session-stream"
	session := logger.Session(&logging.SessionConfig{
		Id:   uuid.NewString(),
		Name: &sessionName,
	})

	traceId := uuid.NewString()
	_ = logger.AddTraceToSession(session.Id(), &logging.TraceConfig{Id: traceId, Name: strPtr("stream-turn")})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, middlewares.ContextKeyTraceId, traceId)

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "Name three colors."}}},
		},
	}
	content := vertexRequestStreamWithContext(t, ctx, transport, project, location, accessToken, reqBody, vertexModel)
	if content == "" {
		t.Error("expected non-empty streamed response in session")
	}

	logger.EndTrace(traceId)
	session.End()
	logger.Flush()
	t.Logf("session stream complete — response: %q", content)
}

// ---------------------------------------------------------------------------
// TestVertexChat / TestVertexStreamingChat (original smoke tests, kept for compatibility)
// ---------------------------------------------------------------------------

func TestVertexChat(t *testing.T) {
	project, location, accessToken := getVertexCredentials(t)
	logger := getVertexLogger(t)
	defer logger.Flush()

	transport := middlewares.NewMaximVertexTransport(logger)
	model := "gemini-2.5-flash"

	result := vertexChat(t, transport, project, location, accessToken, "Say hello in one word.", model)

	// Log the raw usageMetadata so we can see what the Vertex API actually returned.
	t.Logf("raw usageMetadata from Vertex response: %v", result["usageMetadata"])

	parsed, err := logging.ParseResult(logging.ProviderVertex, model, result)
	if err != nil {
		t.Fatalf("ParseResult failed: %v", err)
	}
	if len(parsed.Choices) == 0 {
		t.Fatal("expected at least one choice in response")
	}
	if parsed.Choices[0].Message.Content == "" {
		t.Error("expected non-empty content in response")
	}
	t.Logf("parsed usage — prompt: %d, completion: %d, total: %d",
		parsed.Usage.PromptTokens, parsed.Usage.CompletionTokens, parsed.Usage.TotalTokens)
	if parsed.Usage.TotalTokens == 0 {
		t.Error("expected non-zero total token count from Vertex AI response")
	}

	logger.Flush()
}

func TestVertexStreamingChat(t *testing.T) {
	project, location, accessToken := getVertexCredentials(t)
	logger := getVertexLogger(t)
	defer logger.Flush()

	transport := middlewares.NewMaximVertexTransport(logger)
	model := "gemini-2.5-flash"

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role":  "user",
				"parts": []map[string]interface{}{{"text": "Count from 1 to 3."}},
			},
		},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	// alt=sse requests server-sent events format
	url := vertexBaseURL(project, location) + "/" + model + ":streamGenerateContent?alt=sse"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := transport.Do(req)
	if err != nil {
		t.Fatalf("Vertex AI streaming request failed: %v", err)
	}
	defer resp.Body.Close()

	// The middleware consumes and restores the SSE body. Read it here to inspect
	// raw chunks and verify usageMetadata is present and non-zero.
	var accumulated strings.Builder
	var lastUsageMetadata map[string]interface{}
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		data, ok := strings.CutPrefix(line, "data: ")
		if !ok || data == "" || data == "[DONE]" {
			continue
		}
		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if u, ok := chunk["usageMetadata"].(map[string]interface{}); ok {
			lastUsageMetadata = u
		}
		if candidates, ok := chunk["candidates"].([]interface{}); ok {
			for _, c := range candidates {
				if cMap, ok := c.(map[string]interface{}); ok {
					if content, ok := cMap["content"].(map[string]interface{}); ok {
						if parts, ok := content["parts"].([]interface{}); ok {
							for _, p := range parts {
								if pMap, ok := p.(map[string]interface{}); ok {
									if text, ok := pMap["text"].(string); ok {
										accumulated.WriteString(text)
									}
								}
							}
						}
					}
				}
			}
		}
	}

	t.Logf("last usageMetadata chunk from Vertex streaming response: %v", lastUsageMetadata)

	if accumulated.Len() == 0 {
		t.Error("expected non-empty streamed response")
	}
	if lastUsageMetadata == nil {
		t.Error("usageMetadata was absent from all streaming chunks — Vertex may require alt=sse or a different request format")
	}

	logger.Flush()
}

// ---------------------------------------------------------------------------
// Attachments (vision / multimodal)
// ---------------------------------------------------------------------------

// TestVertex_E2E_WithInlineImage reads a local image, base64-encodes it as
// inline_data, and sends it to Vertex AI. Skips if the test image is not found.
func TestVertex_E2E_WithInlineImage(t *testing.T) {
	project, location, accessToken := getVertexCredentials(t)
	logger := getVertexLogger(t)
	defer logger.Flush()

	imageData, err := os.ReadFile(testImagePath)
	if err != nil {
		t.Skipf("test image not found at %s: %v", testImagePath, err)
	}

	transport := middlewares.NewMaximVertexTransport(logger)

	b64 := base64.StdEncoding.EncodeToString(imageData)
	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]interface{}{
					{"inline_data": map[string]interface{}{"mime_type": "image/jpeg", "data": b64}},
					{"text": "Describe this image in one sentence."},
				},
			},
		},
	}
	result := vertexRequest(t, transport, project, location, accessToken, reqBody, vertexModel)
	parsed := parseAndValidateVertexResult(t, result, vertexModel)

	logger.Flush()
	t.Logf("inline image response: %q", parsed.Choices[0].Message.Content)
}

// TestVertex_E2E_WithInlineImageStream is like TestVertex_E2E_WithInlineImage but with streamGenerateContent.
func TestVertex_E2E_WithInlineImageStream(t *testing.T) {
	project, location, accessToken := getVertexCredentials(t)
	logger := getVertexLogger(t)
	defer logger.Flush()

	imageData, err := os.ReadFile(testImagePath)
	if err != nil {
		t.Skipf("test image not found at %s: %v", testImagePath, err)
	}

	transport := middlewares.NewMaximVertexTransport(logger)

	b64 := base64.StdEncoding.EncodeToString(imageData)
	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]interface{}{
					{"inline_data": map[string]interface{}{"mime_type": "image/jpeg", "data": b64}},
					{"text": "Describe this image in one sentence."},
				},
			},
		},
	}
	content := vertexRequestStream(t, transport, project, location, accessToken, reqBody, vertexModel)
	if content == "" {
		t.Error("expected non-empty streamed response")
	}
	logger.Flush()
	t.Logf("inline image stream response: %q", content)
}

// TestVertex_E2E_WithImageUrl fetches an image from a public URL, base64-encodes
// it, and sends it as inline_data to Vertex AI.
func TestVertex_E2E_WithImageUrl(t *testing.T) {
	project, location, accessToken := getVertexCredentials(t)
	logger := getVertexLogger(t)
	defer logger.Flush()

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Get(testImageURL)
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

	transport := middlewares.NewMaximVertexTransport(logger)

	b64 := base64.StdEncoding.EncodeToString(imageData)
	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]interface{}{
					{"inline_data": map[string]interface{}{"mime_type": "image/jpeg", "data": b64}},
					{"text": "Describe this image in one sentence."},
				},
			},
		},
	}
	result := vertexRequest(t, transport, project, location, accessToken, reqBody, vertexModel)
	parsed := parseAndValidateVertexResult(t, result, vertexModel)

	logger.Flush()
	t.Logf("image URL response: %q", parsed.Choices[0].Message.Content)
}

// TestVertex_E2E_WithImageUrlStream is like TestVertex_E2E_WithImageUrl but with streamGenerateContent.
func TestVertex_E2E_WithImageUrlStream(t *testing.T) {
	project, location, accessToken := getVertexCredentials(t)
	logger := getVertexLogger(t)
	defer logger.Flush()

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Get(testImageURL)
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

	transport := middlewares.NewMaximVertexTransport(logger)

	b64 := base64.StdEncoding.EncodeToString(imageData)
	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]interface{}{
					{"inline_data": map[string]interface{}{"mime_type": "image/jpeg", "data": b64}},
					{"text": "Describe this image in one sentence."},
				},
			},
		},
	}
	content := vertexRequestStream(t, transport, project, location, accessToken, reqBody, vertexModel)
	if content == "" {
		t.Error("expected non-empty streamed response")
	}
	logger.Flush()
	t.Logf("image URL stream response: %q", content)
}

// TestVertex_E2E_WithGCSFileUri sends an image stored in Google Cloud Storage
// using the Vertex-specific file_data.file_uri field. Skips if VERTEX_GCS_IMAGE_URI
// is not set (e.g. gs://my-bucket/image.jpg).
func TestVertex_E2E_WithGCSFileUri(t *testing.T) {
	project, location, accessToken := getVertexCredentials(t)
	logger := getVertexLogger(t)
	defer logger.Flush()

	gcsURI := os.Getenv("VERTEX_GCS_IMAGE_URI")
	if gcsURI == "" {
		t.Skip("VERTEX_GCS_IMAGE_URI not set, skipping GCS file_data test")
	}

	transport := middlewares.NewMaximVertexTransport(logger)

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]interface{}{
					{
						"file_data": map[string]interface{}{
							"file_uri":  gcsURI,
							"mime_type": "image/jpeg",
						},
					},
					{"text": "Describe this image in one sentence."},
				},
			},
		},
	}
	result := vertexRequest(t, transport, project, location, accessToken, reqBody, vertexModel)
	parsed := parseAndValidateVertexResult(t, result, vertexModel)

	logger.Flush()
	t.Logf("GCS file_data response: %q", parsed.Choices[0].Message.Content)
}

// TestVertex_E2E_WithGCSFileUriStream is like TestVertex_E2E_WithGCSFileUri but with streamGenerateContent.
func TestVertex_E2E_WithGCSFileUriStream(t *testing.T) {
	project, location, accessToken := getVertexCredentials(t)
	logger := getVertexLogger(t)
	defer logger.Flush()

	gcsURI := os.Getenv("VERTEX_GCS_IMAGE_URI")
	if gcsURI == "" {
		t.Skip("VERTEX_GCS_IMAGE_URI not set, skipping GCS file_data stream test")
	}

	transport := middlewares.NewMaximVertexTransport(logger)

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]interface{}{
					{
						"file_data": map[string]interface{}{
							"file_uri":  gcsURI,
							"mime_type": "image/jpeg",
						},
					},
					{"text": "Describe this image in one sentence."},
				},
			},
		},
	}
	content := vertexRequestStream(t, transport, project, location, accessToken, reqBody, vertexModel)
	if content == "" {
		t.Error("expected non-empty streamed response")
	}
	logger.Flush()
	t.Logf("GCS file_data stream response: %q", content)
}

// TestVertex_E2E_WithMultipleImages sends two images in the same request and
// asks the model to compare them. Uses the public test image fetched twice.
func TestVertex_E2E_WithMultipleImages(t *testing.T) {
	project, location, accessToken := getVertexCredentials(t)
	logger := getVertexLogger(t)
	defer logger.Flush()

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Get(testImageURL)
	if err != nil {
		t.Skipf("failed to fetch test image: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skipf("test image URL returned status %d", resp.StatusCode)
	}
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Skipf("failed to read image: %v", err)
	}

	transport := middlewares.NewMaximVertexTransport(logger)

	b64 := base64.StdEncoding.EncodeToString(imageData)
	inlinePart := map[string]interface{}{
		"inline_data": map[string]interface{}{"mime_type": "image/jpeg", "data": b64},
	}
	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]interface{}{
					inlinePart,
					inlinePart,
					{"text": "I sent you two images. Are they the same? Answer yes or no."},
				},
			},
		},
	}
	result := vertexRequest(t, transport, project, location, accessToken, reqBody, vertexModel)
	parsed := parseAndValidateVertexResult(t, result, vertexModel)

	logger.Flush()
	t.Logf("multiple images response: %q", parsed.Choices[0].Message.Content)
}

// TestVertex_E2E_WithMultipleImagesStream is like TestVertex_E2E_WithMultipleImages but with streamGenerateContent.
func TestVertex_E2E_WithMultipleImagesStream(t *testing.T) {
	project, location, accessToken := getVertexCredentials(t)
	logger := getVertexLogger(t)
	defer logger.Flush()

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Get(testImageURL)
	if err != nil {
		t.Skipf("failed to fetch test image: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skipf("test image URL returned status %d", resp.StatusCode)
	}
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Skipf("failed to read image: %v", err)
	}

	transport := middlewares.NewMaximVertexTransport(logger)

	b64 := base64.StdEncoding.EncodeToString(imageData)
	inlinePart := map[string]interface{}{
		"inline_data": map[string]interface{}{"mime_type": "image/jpeg", "data": b64},
	}
	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]interface{}{
					inlinePart,
					inlinePart,
					{"text": "I sent you two images. Are they the same? Answer yes or no."},
				},
			},
		},
	}
	content := vertexRequestStream(t, transport, project, location, accessToken, reqBody, vertexModel)
	if content == "" {
		t.Error("expected non-empty streamed response")
	}
	logger.Flush()
	t.Logf("multiple images stream response: %q", content)
}

// TestVertex_E2E_WithImageAndToolCall sends an image alongside a tool declaration
// to verify multimodal + function calling works end-to-end.
func TestVertex_E2E_WithImageAndToolCall(t *testing.T) {
	project, location, accessToken := getVertexCredentials(t)
	logger := getVertexLogger(t)
	defer logger.Flush()

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Get(testImageURL)
	if err != nil {
		t.Skipf("failed to fetch test image: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skipf("test image URL returned status %d", resp.StatusCode)
	}
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Skipf("failed to read image: %v", err)
	}

	transport := middlewares.NewMaximVertexTransport(logger)

	b64 := base64.StdEncoding.EncodeToString(imageData)
	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"role": "user",
				"parts": []map[string]interface{}{
					{"inline_data": map[string]interface{}{"mime_type": "image/jpeg", "data": b64}},
					{"text": "What object is in this image? Use the classify_object function."},
				},
			},
		},
		"tools": []map[string]interface{}{
			{
				"functionDeclarations": []map[string]interface{}{
					{
						"name":        "classify_object",
						"description": "Classify the primary object in an image.",
						"parameters": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"object_name": map[string]interface{}{
									"type":        "string",
									"description": "Name of the primary object",
								},
							},
							"required": []string{"object_name"},
						},
					},
				},
			},
		},
	}
	result := vertexRequest(t, transport, project, location, accessToken, reqBody, vertexModel)
	// Model may return a function call or fall back to text — both are valid.
	parsed := parseAndValidateVertexResult(t, result, vertexModel)
	if len(parsed.Choices[0].Message.ToolCalls) > 0 {
		t.Logf("tool call: %s(%s)", parsed.Choices[0].Message.ToolCalls[0].Function.Name, parsed.Choices[0].Message.ToolCalls[0].Function.Arguments)
	} else {
		t.Logf("text response: %q", parsed.Choices[0].Message.Content)
	}
	logger.Flush()
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func strPtr(s string) *string { return &s }
