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
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
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
	t.Helper()
	input := &bedrockruntime.ConverseInput{
		ModelId:  aws.String(modelID),
		Messages: messages,
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
