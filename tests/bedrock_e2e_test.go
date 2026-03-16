package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/google/uuid"
	"github.com/maximhq/maxim-go/logging"
	"github.com/maximhq/maxim-go/middlewares"
	"github.com/maximhq/maxim-go/schemas"
)

const bedrockModelID = "global.anthropic.claude-sonnet-4-5-20250929-v1:0"

// getBedrockConfig loads AWS config with Maxim transport and skips if credentials are not available.
func getBedrockConfig(t *testing.T, logger *logging.Logger) aws.Config {
	t.Helper()
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1"
	}
	httpClient := &http.Client{
		Transport: middlewares.NewMaximBedrockTransport(logger),
		Timeout:   60 * time.Second,
	}
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
		config.WithHTTPClient(httpClient),
	)
	if err != nil {
		t.Skipf("failed to load AWS config: %v (set AWS_REGION, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)", err)
	}
	return cfg
}

// bedrockE2EClient creates a Bedrock client with Maxim transport.
func bedrockE2EClient(t *testing.T, logger *logging.Logger) *bedrockruntime.Client {
	t.Helper()
	cfg := getBedrockConfig(t, logger)
	return bedrockruntime.NewFromConfig(cfg)
}

// bedrockE2EConverse makes a Converse API call and returns the response as a map for ParseResult.
func bedrockE2EConverse(t *testing.T, client *bedrockruntime.Client, ctx context.Context, messages []types.Message, modelID string) map[string]interface{} {
	return bedrockE2EConverseWithTools(t, client, ctx, messages, modelID, nil)
}

// bedrockE2EConverseStream makes a ConverseStream API call and returns accumulated text content.
func bedrockE2EConverseStream(t *testing.T, client *bedrockruntime.Client, ctx context.Context, messages []types.Message, modelID string, toolConfig *types.ToolConfiguration) string {
	t.Helper()
	input := &bedrockruntime.ConverseStreamInput{
		ModelId:    aws.String(modelID),
		Messages:   messages,
		ToolConfig: toolConfig,
	}
	output, err := client.ConverseStream(ctx, input)
	if err != nil {
		t.Fatalf("Bedrock ConverseStream failed: %v", err)
	}
	var content strings.Builder
	stream := output.GetStream()
	defer stream.Close()
	for event := range stream.Events() {
		if delta, ok := event.(*types.ConverseStreamOutputMemberContentBlockDelta); ok {
			if text, ok := delta.Value.Delta.(*types.ContentBlockDeltaMemberText); ok {
				content.WriteString(text.Value)
			}
		}
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("Bedrock stream error: %v", err)
	}
	return content.String()
}

// bedrockE2EConverseWithTools makes a Converse API call with optional tool config.
func bedrockE2EConverseWithTools(t *testing.T, client *bedrockruntime.Client, ctx context.Context, messages []types.Message, modelID string, toolConfig *types.ToolConfiguration) map[string]interface{} {
	t.Helper()
	input := &bedrockruntime.ConverseInput{
		ModelId:     aws.String(modelID),
		Messages:    messages,
		ToolConfig:  toolConfig,
	}
	output, err := client.Converse(ctx, input)
	if err != nil {
		t.Fatalf("Bedrock Converse failed: %v", err)
	}
	// Convert to map for ParseResult (BedrockConverseResp format)
	// The SDK returns *ConverseOutput; we marshal to JSON and unmarshal to map.
	outputBytes, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("failed to marshal ConverseOutput: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(outputBytes, &result); err != nil {
		t.Fatalf("failed to unmarshal to map: %v", err)
	}
	return result
}

// parseAndValidateBedrockE2EResult runs ParseResult for the bedrock provider.
func parseAndValidateBedrockE2EResult(t *testing.T, result map[string]interface{}, model string) *schemas.MaximLLMResult {
	t.Helper()
	parsed, err := logging.ParseResult(logging.ProviderBedrock, model, result)
	if err != nil {
		t.Fatalf("ParseResult failed: %v", err)
	}
	if parsed == nil {
		t.Fatal("ParseResult returned nil")
	}
	expectedModel := strings.TrimPrefix(model, "global.")
	if parsed.Model != expectedModel {
		t.Fatalf("ParseResult: expected Model=%q, got %q", expectedModel, parsed.Model)
	}
	if len(parsed.Choices) == 0 {
		t.Fatal("ParseResult: expected at least one choice")
	}
	t.Logf("ParseResult OK — model=%s tokens=%d", parsed.Model, parsed.Usage.TotalTokens)
	return parsed
}

// TestBedrock_E2E_SimpleCompletion makes a real Bedrock Converse API call through MaximBedrockTransport.
func TestBedrock_E2E_SimpleCompletion(t *testing.T) {
	requireMaximCredentials(t)
	cfg := getBedrockConfig(t, nil)
	if _, err := cfg.Credentials.Retrieve(context.Background()); err != nil {
		t.Skipf("AWS credentials not available: %v", err)
	}

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	client := bedrockE2EClient(t, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, middlewares.ContextKeyLogger, logger)

	messages := []types.Message{
		{
			Role: types.ConversationRoleUser,
			Content: []types.ContentBlock{
				&types.ContentBlockMemberText{Value: "Say hello in one word."},
			},
		},
	}
	result := bedrockE2EConverse(t, client, ctx, messages, bedrockModelID)
	parsed := parseAndValidateBedrockE2EResult(t, result, bedrockModelID)

	logger.Flush()
	t.Logf("Successfully completed Bedrock e2e — response: %q", parsed.Choices[0].Message.Content)
}

// TestBedrock_E2E_SimpleCompletionStream is like TestBedrock_E2E_SimpleCompletion but with ConverseStream.
func TestBedrock_E2E_SimpleCompletionStream(t *testing.T) {
	requireMaximCredentials(t)
	cfg := getBedrockConfig(t, nil)
	if _, err := cfg.Credentials.Retrieve(context.Background()); err != nil {
		t.Skipf("AWS credentials not available: %v", err)
	}
	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{Id: logRepoId, AutoFlush: ptr(false)})
	defer logger.Flush()
	client := bedrockE2EClient(t, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, middlewares.ContextKeyLogger, logger)
	messages := []types.Message{
		{Role: types.ConversationRoleUser, Content: []types.ContentBlock{&types.ContentBlockMemberText{Value: "Say hello in one word."}}},
	}
	content := bedrockE2EConverseStream(t, client, ctx, messages, bedrockModelID, nil)
	if content == "" {
		t.Error("expected non-empty streamed response")
	}
	logger.Flush()
	t.Logf("Successfully completed Bedrock e2e (stream) — response: %q", content)
}

// TestBedrock_E2E_WithTraceContext makes a Bedrock API call with custom trace context.
func TestBedrock_E2E_WithTraceContext(t *testing.T) {
	requireMaximCredentials(t)
	cfg := getBedrockConfig(t, nil)
	if _, err := cfg.Credentials.Retrieve(context.Background()); err != nil {
		t.Skipf("AWS credentials not available: %v", err)
	}

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	client := bedrockE2EClient(t, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, middlewares.ContextKeyLogger, logger)
	ctx = context.WithValue(ctx, middlewares.ContextKeyTraceName, "bedrock-trace-context-test")
	ctx = context.WithValue(ctx, middlewares.ContextKeyGenerationName, "bedrock-gen-context-test")

	messages := []types.Message{
		{
			Role: types.ConversationRoleUser,
			Content: []types.ContentBlock{
				&types.ContentBlockMemberText{Value: "What is 2+2? Reply with just the number."},
			},
		},
	}
	result := bedrockE2EConverse(t, client, ctx, messages, bedrockModelID)
	parseAndValidateBedrockE2EResult(t, result, bedrockModelID)

	logger.Flush()
	t.Log("Successfully completed Bedrock e2e with trace context")
}

// TestBedrock_E2E_WithTraceContextStream is like TestBedrock_E2E_WithTraceContext but with ConverseStream.
func TestBedrock_E2E_WithTraceContextStream(t *testing.T) {
	requireMaximCredentials(t)
	cfg := getBedrockConfig(t, nil)
	if _, err := cfg.Credentials.Retrieve(context.Background()); err != nil {
		t.Skipf("AWS credentials not available: %v", err)
	}
	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{Id: logRepoId, AutoFlush: ptr(false)})
	defer logger.Flush()
	client := bedrockE2EClient(t, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, middlewares.ContextKeyLogger, logger)
	ctx = context.WithValue(ctx, middlewares.ContextKeyTraceName, "bedrock-trace-context-test")
	ctx = context.WithValue(ctx, middlewares.ContextKeyGenerationName, "bedrock-gen-context-test")
	messages := []types.Message{
		{Role: types.ConversationRoleUser, Content: []types.ContentBlock{&types.ContentBlockMemberText{Value: "What is 2+2? Reply with just the number."}}},
	}
	content := bedrockE2EConverseStream(t, client, ctx, messages, bedrockModelID, nil)
	if content == "" {
		t.Error("expected non-empty streamed response")
	}
	logger.Flush()
	t.Logf("Successfully completed Bedrock e2e with trace context (stream) — response: %q", content)
}

// TestBedrock_E2E_WithSystemMessage tests Bedrock with system message.
func TestBedrock_E2E_WithSystemMessage(t *testing.T) {
	requireMaximCredentials(t)
	cfg := getBedrockConfig(t, nil)
	if _, err := cfg.Credentials.Retrieve(context.Background()); err != nil {
		t.Skipf("AWS credentials not available: %v", err)
	}

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	client := bedrockE2EClient(t, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, middlewares.ContextKeyLogger, logger)

	system := []types.SystemContentBlock{
		&types.SystemContentBlockMemberText{Value: "You are a helpful assistant. Keep answers brief."},
	}
	messages := []types.Message{
		{
			Role: types.ConversationRoleUser,
			Content: []types.ContentBlock{
				&types.ContentBlockMemberText{Value: "What color is the sky?"},
			},
		},
	}
	input := &bedrockruntime.ConverseInput{
		ModelId:  aws.String(bedrockModelID),
		Messages: messages,
		System:   system,
	}
	output, err := client.Converse(ctx, input)
	if err != nil {
		t.Fatalf("Bedrock Converse failed: %v", err)
	}
	outputBytes, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("failed to marshal ConverseOutput: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(outputBytes, &result); err != nil {
		t.Fatalf("failed to unmarshal ConverseOutput: %v", err)
	}
	parseAndValidateBedrockE2EResult(t, result, bedrockModelID)

	logger.Flush()
	t.Log("Successfully completed Bedrock e2e with system message")
}

// TestBedrock_E2E_WithSystemMessageStream is like TestBedrock_E2E_WithSystemMessage but with ConverseStream.
func TestBedrock_E2E_WithSystemMessageStream(t *testing.T) {
	requireMaximCredentials(t)
	cfg := getBedrockConfig(t, nil)
	if _, err := cfg.Credentials.Retrieve(context.Background()); err != nil {
		t.Skipf("AWS credentials not available: %v", err)
	}
	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{Id: logRepoId, AutoFlush: ptr(false)})
	defer logger.Flush()
	client := bedrockE2EClient(t, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, middlewares.ContextKeyLogger, logger)
	system := []types.SystemContentBlock{&types.SystemContentBlockMemberText{Value: "You are a helpful assistant. Keep answers brief."}}
	messages := []types.Message{
		{Role: types.ConversationRoleUser, Content: []types.ContentBlock{&types.ContentBlockMemberText{Value: "What color is the sky?"}}},
	}
	input := &bedrockruntime.ConverseStreamInput{
		ModelId:  aws.String(bedrockModelID),
		Messages: messages,
		System:   system,
	}
	output, err := client.ConverseStream(ctx, input)
	if err != nil {
		t.Fatalf("Bedrock ConverseStream failed: %v", err)
	}
	var content strings.Builder
	stream := output.GetStream()
	defer stream.Close()
	for event := range stream.Events() {
		if delta, ok := event.(*types.ConverseStreamOutputMemberContentBlockDelta); ok {
			if text, ok := delta.Value.Delta.(*types.ContentBlockDeltaMemberText); ok {
				content.WriteString(text.Value)
			}
		}
	}
	if stream.Err() != nil {
		t.Fatalf("Bedrock stream error: %v", stream.Err())
	}
	if content.String() == "" {
		t.Error("expected non-empty streamed response")
	}
	logger.Flush()
	t.Logf("Successfully completed Bedrock e2e with system message (stream) — response: %q", content.String())
}

// TestBedrock_E2E_WithInlineImage reads a local image, base64-encodes it, and sends to Bedrock vision.
func TestBedrock_E2E_WithInlineImage(t *testing.T) {
	requireMaximCredentials(t)
	cfg := getBedrockConfig(t, nil)
	if _, err := cfg.Credentials.Retrieve(context.Background()); err != nil {
		t.Skipf("AWS credentials not available: %v", err)
	}

	imageData, err := os.ReadFile(testImagePath)
	if err != nil {
		t.Skipf("test image not found at %s: %v", testImagePath, err)
	}

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	client := bedrockE2EClient(t, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, middlewares.ContextKeyLogger, logger)

	messages := []types.Message{
		{
			Role: types.ConversationRoleUser,
			Content: []types.ContentBlock{
				&types.ContentBlockMemberImage{
					Value: types.ImageBlock{
						Format: types.ImageFormatJpeg,
						Source: &types.ImageSourceMemberBytes{Value: imageData},
					},
				},
				&types.ContentBlockMemberText{Value: "Describe this image in one sentence."},
			},
		},
	}
	result := bedrockE2EConverse(t, client, ctx, messages, bedrockModelID)
	parsed := parseAndValidateBedrockE2EResult(t, result, bedrockModelID)

	logger.Flush()
	t.Logf("Successfully completed Bedrock vision e2e (inline) — response: %q", parsed.Choices[0].Message.Content)
}

// TestBedrock_E2E_WithInlineImageStream is like TestBedrock_E2E_WithInlineImage but with ConverseStream.
func TestBedrock_E2E_WithInlineImageStream(t *testing.T) {
	requireMaximCredentials(t)
	cfg := getBedrockConfig(t, nil)
	if _, err := cfg.Credentials.Retrieve(context.Background()); err != nil {
		t.Skipf("AWS credentials not available: %v", err)
	}
	imageData, err := os.ReadFile(testImagePath)
	if err != nil {
		t.Skipf("test image not found at %s: %v", testImagePath, err)
	}
	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{Id: logRepoId, AutoFlush: ptr(false)})
	defer logger.Flush()
	client := bedrockE2EClient(t, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, middlewares.ContextKeyLogger, logger)
	messages := []types.Message{
		{
			Role: types.ConversationRoleUser,
			Content: []types.ContentBlock{
				&types.ContentBlockMemberImage{Value: types.ImageBlock{Format: types.ImageFormatJpeg, Source: &types.ImageSourceMemberBytes{Value: imageData}}},
				&types.ContentBlockMemberText{Value: "Describe this image in one sentence."},
			},
		},
	}
	content := bedrockE2EConverseStream(t, client, ctx, messages, bedrockModelID, nil)
	if content == "" {
		t.Error("expected non-empty streamed response")
	}
	logger.Flush()
	t.Logf("Successfully completed Bedrock vision e2e (inline, stream) — response: %q", content)
}

// TestBedrock_E2E_WithToolCalls tests Bedrock Converse API with tool/function calling.
func TestBedrock_E2E_WithToolCalls(t *testing.T) {
	requireMaximCredentials(t)
	cfg := getBedrockConfig(t, nil)
	if _, err := cfg.Credentials.Retrieve(context.Background()); err != nil {
		t.Skipf("AWS credentials not available: %v", err)
	}

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	client := bedrockE2EClient(t, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, middlewares.ContextKeyLogger, logger)

	toolConfig := &types.ToolConfiguration{
		Tools: []types.Tool{
			&types.ToolMemberToolSpec{
				Value: types.ToolSpecification{
					Name:        aws.String("get_weather"),
					Description: aws.String("Get the current weather for a location."),
					InputSchema: &types.ToolInputSchemaMemberJson{
						Value: document.NewLazyDocument(map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"location": map[string]interface{}{
									"type":        "string",
									"description": "City name, e.g. Paris or Tokyo",
								},
							},
							"required": []string{"location"},
						}),
					},
				},
			},
		},
	}

	messages := []types.Message{
		{
			Role: types.ConversationRoleUser,
			Content: []types.ContentBlock{
				&types.ContentBlockMemberText{Value: "What's the weather in Paris?"},
			},
		},
	}
	result := bedrockE2EConverseWithTools(t, client, ctx, messages, bedrockModelID, toolConfig)
	parsed := parseAndValidateBedrockE2EResult(t, result, bedrockModelID)

	// Model may return tool call or text; either is valid
	if len(parsed.Choices[0].Message.ToolCalls) > 0 {
		t.Logf("Tool call: %s(%s)", parsed.Choices[0].Message.ToolCalls[0].Function.Name, parsed.Choices[0].Message.ToolCalls[0].Function.Arguments)
	} else {
		t.Logf("Text response: %q", parsed.Choices[0].Message.Content)
	}

	logger.Flush()
	t.Log("Successfully completed Bedrock e2e with tool calls")
}

// TestBedrock_E2E_WithToolCallsStream is like TestBedrock_E2E_WithToolCalls but with ConverseStream.
func TestBedrock_E2E_WithToolCallsStream(t *testing.T) {
	requireMaximCredentials(t)
	cfg := getBedrockConfig(t, nil)
	if _, err := cfg.Credentials.Retrieve(context.Background()); err != nil {
		t.Skipf("AWS credentials not available: %v", err)
	}
	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{Id: logRepoId, AutoFlush: ptr(false)})
	defer logger.Flush()
	client := bedrockE2EClient(t, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, middlewares.ContextKeyLogger, logger)
	toolConfig := &types.ToolConfiguration{
		Tools: []types.Tool{
			&types.ToolMemberToolSpec{
				Value: types.ToolSpecification{
					Name:        aws.String("get_weather"),
					Description: aws.String("Get the current weather for a location."),
					InputSchema: &types.ToolInputSchemaMemberJson{
						Value: document.NewLazyDocument(map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{"location": map[string]interface{}{"type": "string", "description": "City name"}},
							"required": []string{"location"},
						}),
					},
				},
			},
		},
	}
	messages := []types.Message{
		{Role: types.ConversationRoleUser, Content: []types.ContentBlock{&types.ContentBlockMemberText{Value: "What's the weather in Paris?"}}},
	}
	input := &bedrockruntime.ConverseStreamInput{
		ModelId:    aws.String(bedrockModelID),
		Messages:   messages,
		ToolConfig: toolConfig,
	}
	output, err := client.ConverseStream(ctx, input)
	if err != nil {
		t.Fatalf("Bedrock ConverseStream failed: %v", err)
	}
	stream := output.GetStream()
	defer stream.Close()
	var toolUseCount int
	for event := range stream.Events() {
		if start, ok := event.(*types.ConverseStreamOutputMemberContentBlockStart); ok {
			if _, isToolUse := start.Value.Start.(*types.ContentBlockStartMemberToolUse); isToolUse {
				toolUseCount++
			}
		}
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("Bedrock stream error: %v", err)
	}
	if toolUseCount == 0 {
		t.Error("expected at least one tool-use block in stream, got none")
	}
	logger.Flush()
	t.Logf("Successfully completed Bedrock e2e with tool calls (stream) — tool use blocks=%d", toolUseCount)
}

// TestBedrock_E2E_WithToolCallsFullFlow tests the full tool-calling flow: request 1 returns tool calls,
// we simulate tool execution, request 2 with tool result returns final text. Uses shared traceId.
func TestBedrock_E2E_WithToolCallsFullFlow(t *testing.T) {
	requireMaximCredentials(t)
	cfg := getBedrockConfig(t, nil)
	if _, err := cfg.Credentials.Retrieve(context.Background()); err != nil {
		t.Skipf("AWS credentials not available: %v", err)
	}

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	client := bedrockE2EClient(t, logger)

	traceId := uuid.NewString()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, middlewares.ContextKeyTraceId, traceId)
	ctx = context.WithValue(ctx, middlewares.ContextKeyLogger, logger)

	toolConfig := &types.ToolConfiguration{
		Tools: []types.Tool{
			&types.ToolMemberToolSpec{
				Value: types.ToolSpecification{
					Name:        aws.String("get_weather"),
					Description: aws.String("Get the current weather for a location."),
					InputSchema: &types.ToolInputSchemaMemberJson{
						Value: document.NewLazyDocument(map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"location": map[string]interface{}{
									"type":        "string",
									"description": "City name, e.g. Paris or Tokyo",
								},
							},
							"required": []string{"location"},
						}),
					},
				},
			},
		},
	}

	messages1 := []types.Message{
		{
			Role: types.ConversationRoleUser,
			Content: []types.ContentBlock{
				&types.ContentBlockMemberText{Value: "What's the weather in Paris?"},
			},
		},
	}

	result1 := bedrockE2EConverseWithTools(t, client, ctx, messages1, bedrockModelID, toolConfig)
	parsed1 := parseAndValidateBedrockE2EResult(t, result1, bedrockModelID)

	if len(parsed1.Choices[0].Message.ToolCalls) == 0 {
		t.Fatal("expected tool use in first response")
	}
	toolUseId := parsed1.Choices[0].Message.ToolCalls[0].ID
	toolResult := "Sunny, 22°C"

	var inputMap map[string]interface{}
	if err := json.Unmarshal([]byte(parsed1.Choices[0].Message.ToolCalls[0].Function.Arguments), &inputMap); err != nil {
		inputMap = map[string]interface{}{}
	}
	inputDoc := document.NewLazyDocument(inputMap)
	funcName := parsed1.Choices[0].Message.ToolCalls[0].Function.Name

	assistantContent := []types.ContentBlock{
		&types.ContentBlockMemberToolUse{
			Value: types.ToolUseBlock{
				ToolUseId: aws.String(toolUseId),
				Name:      aws.String(funcName),
				Input:     inputDoc,
			},
		},
	}

	toolResultContent := []types.ToolResultContentBlock{
		&types.ToolResultContentBlockMemberText{Value: toolResult},
	}

	messages2 := []types.Message{
		messages1[0],
		{
			Role:    types.ConversationRoleAssistant,
			Content: assistantContent,
		},
		{
			Role: types.ConversationRoleUser,
			Content: []types.ContentBlock{
				&types.ContentBlockMemberToolResult{
					Value: types.ToolResultBlock{
						ToolUseId: aws.String(toolUseId),
						Content:   toolResultContent,
						Status:    types.ToolResultStatusSuccess,
					},
				},
			},
		},
	}

	result2 := bedrockE2EConverseWithTools(t, client, ctx, messages2, bedrockModelID, toolConfig)
	parsed2 := parseAndValidateBedrockE2EResult(t, result2, bedrockModelID)

	if parsed2.Choices[0].Message.Content == "" {
		t.Error("expected non-empty final text response after tool result")
	}

	logger.EndTrace(traceId)
	logger.Flush()
	t.Logf("Successfully completed Bedrock e2e full-flow tool calls — final: %q", parsed2.Choices[0].Message.Content)
}

// TestBedrock_E2E_WithToolCallsFullFlowStream is like TestBedrock_E2E_WithToolCallsFullFlow but the second request uses ConverseStream.
func TestBedrock_E2E_WithToolCallsFullFlowStream(t *testing.T) {
	requireMaximCredentials(t)
	cfg := getBedrockConfig(t, nil)
	if _, err := cfg.Credentials.Retrieve(context.Background()); err != nil {
		t.Skipf("AWS credentials not available: %v", err)
	}
	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{Id: logRepoId, AutoFlush: ptr(false)})
	defer logger.Flush()
	client := bedrockE2EClient(t, logger)
	traceId := uuid.NewString()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, middlewares.ContextKeyTraceId, traceId)
	ctx = context.WithValue(ctx, middlewares.ContextKeyLogger, logger)
	toolConfig := &types.ToolConfiguration{
		Tools: []types.Tool{
			&types.ToolMemberToolSpec{
				Value: types.ToolSpecification{
					Name:        aws.String("get_weather"),
					Description: aws.String("Get the current weather for a location."),
					InputSchema: &types.ToolInputSchemaMemberJson{
						Value: document.NewLazyDocument(map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{"location": map[string]interface{}{"type": "string", "description": "City name"}},
							"required": []string{"location"},
						}),
					},
				},
			},
		},
	}
	messages1 := []types.Message{
		{Role: types.ConversationRoleUser, Content: []types.ContentBlock{&types.ContentBlockMemberText{Value: "What's the weather in Paris?"}}},
	}
	result1 := bedrockE2EConverseWithTools(t, client, ctx, messages1, bedrockModelID, toolConfig)
	parsed1 := parseAndValidateBedrockE2EResult(t, result1, bedrockModelID)
	if len(parsed1.Choices[0].Message.ToolCalls) == 0 {
		t.Fatal("expected tool use in first response")
	}
	toolUseId := parsed1.Choices[0].Message.ToolCalls[0].ID
	var inputMap map[string]interface{}
	_ = json.Unmarshal([]byte(parsed1.Choices[0].Message.ToolCalls[0].Function.Arguments), &inputMap)
	if inputMap == nil {
		inputMap = map[string]interface{}{}
	}
	assistantContent := []types.ContentBlock{
		&types.ContentBlockMemberToolUse{
			Value: types.ToolUseBlock{
				ToolUseId: aws.String(toolUseId),
				Name:      aws.String(parsed1.Choices[0].Message.ToolCalls[0].Function.Name),
				Input:     document.NewLazyDocument(inputMap),
			},
		},
	}
	messages2 := []types.Message{
		messages1[0],
		{Role: types.ConversationRoleAssistant, Content: assistantContent},
		{
			Role: types.ConversationRoleUser,
			Content: []types.ContentBlock{
				&types.ContentBlockMemberToolResult{
					Value: types.ToolResultBlock{
						ToolUseId: aws.String(toolUseId),
						Content:   []types.ToolResultContentBlock{&types.ToolResultContentBlockMemberText{Value: "Sunny, 22°C"}},
						Status:    types.ToolResultStatusSuccess,
					},
				},
			},
		},
	}
	content := bedrockE2EConverseStream(t, client, ctx, messages2, bedrockModelID, toolConfig)
	if content == "" {
		t.Error("expected non-empty final text response after tool result (stream)")
	}
	logger.EndTrace(traceId)
	logger.Flush()
	t.Logf("Successfully completed Bedrock e2e full-flow tool calls (stream) — final: %q", content)
}

// TestBedrock_E2E_WithTagsAndMetrics makes a Bedrock API call with tags and metrics in context.
func TestBedrock_E2E_WithTagsAndMetrics(t *testing.T) {
	requireMaximCredentials(t)
	cfg := getBedrockConfig(t, nil)
	if _, err := cfg.Credentials.Retrieve(context.Background()); err != nil {
		t.Skipf("AWS credentials not available: %v", err)
	}

	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{
		Id:        logRepoId,
		AutoFlush: ptr(false),
	})
	defer logger.Flush()

	client := bedrockE2EClient(t, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, middlewares.ContextKeyLogger, logger)

	traceTags := map[string]string{"e2e_test": "tags_metrics", "env": "test"}
	generationTags := map[string]string{"model_type": "chat", "source": "e2e"}
	traceMetrics := map[string]float64{"latency_ms": 0.69, "tool_calls_count": 0.12}
	generationMetrics := map[string]float64{"tokens_in": 100, "tokens_out": 50}

	ctx = context.WithValue(ctx, middlewares.ContextKeyTraceTags, traceTags)
	ctx = context.WithValue(ctx, middlewares.ContextKeyGenerationTags, generationTags)
	ctx = context.WithValue(ctx, middlewares.ContextKeyTraceMetrics, traceMetrics)
	ctx = context.WithValue(ctx, middlewares.ContextKeyGenerationMetrics, generationMetrics)

	messages := []types.Message{
		{
			Role: types.ConversationRoleUser,
			Content: []types.ContentBlock{
				&types.ContentBlockMemberText{Value: "Say hello in one word."},
			},
		},
	}
	result := bedrockE2EConverse(t, client, ctx, messages, bedrockModelID)
	parsed := parseAndValidateBedrockE2EResult(t, result, bedrockModelID)

	logger.Flush()
	t.Logf("Successfully completed Bedrock e2e with tags and metrics — response: %q", parsed.Choices[0].Message.Content)
}

// TestBedrock_E2E_WithTagsAndMetricsStream is like TestBedrock_E2E_WithTagsAndMetrics but with ConverseStream.
func TestBedrock_E2E_WithTagsAndMetricsStream(t *testing.T) {
	requireMaximCredentials(t)
	cfg := getBedrockConfig(t, nil)
	if _, err := cfg.Credentials.Retrieve(context.Background()); err != nil {
		t.Skipf("AWS credentials not available: %v", err)
	}
	logger := logging.NewLogger(baseUrl, apiKey, &logging.LoggerConfig{Id: logRepoId, AutoFlush: ptr(false)})
	defer logger.Flush()
	client := bedrockE2EClient(t, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, middlewares.ContextKeyLogger, logger)
	ctx = context.WithValue(ctx, middlewares.ContextKeyTraceTags, map[string]string{"e2e_test": "tags_metrics", "env": "test"})
	ctx = context.WithValue(ctx, middlewares.ContextKeyGenerationTags, map[string]string{"model_type": "chat", "source": "e2e"})
	ctx = context.WithValue(ctx, middlewares.ContextKeyTraceMetrics, map[string]float64{"latency_ms": 0.69})
	ctx = context.WithValue(ctx, middlewares.ContextKeyGenerationMetrics, map[string]float64{"tokens_in": 100})
	messages := []types.Message{
		{Role: types.ConversationRoleUser, Content: []types.ContentBlock{&types.ContentBlockMemberText{Value: "Say hello in one word."}}},
	}
	content := bedrockE2EConverseStream(t, client, ctx, messages, bedrockModelID, nil)
	if content == "" {
		t.Error("expected non-empty streamed response")
	}
	logger.Flush()
	t.Logf("Successfully completed Bedrock e2e with tags and metrics (stream) — response: %q", content)
}
