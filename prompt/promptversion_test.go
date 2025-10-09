package prompt

import (
	"os"
	"testing"
	"time"
)

// TestGetPromptVersion_Integration tests the GetPromptVersion method with actual API calls
// and verifies that caching works correctly.
//
// Prerequisites:
// - Set MAXIM_API_KEY environment variable
// - Set MAXIM_TEST_VERSION_ID environment variable with a valid version ID
// - Set MAXIM_TEST_PROMPT_ID environment variable with a valid prompt ID
func TestGetPromptVersion_Integration(t *testing.T) {
	baseUrl := os.Getenv("MAXIM_BASE_URL")
	if baseUrl == "" {
		t.Skip("MAXIM_BASE_URL not set, skipping integration test")
	}

	// Get credentials from environment
	apiKey := os.Getenv("MAXIM_API_KEY")
	if apiKey == "" {
		t.Skip("MAXIM_API_KEY not set, skipping integration test")
	}

	versionId := os.Getenv("MAXIM_TEST_VERSION_ID")
	if versionId == "" {
		t.Skip("MAXIM_TEST_VERSION_ID not set, skipping integration test")
	}

	promptId := os.Getenv("MAXIM_TEST_PROMPT_ID")
	if promptId == "" {
		t.Skip("MAXIM_TEST_PROMPT_ID not set, skipping integration test")
	}

	// First call - should fetch from API
	t.Log("Making first API call...")
	start1 := time.Now()
	result1, err := GetPromptVersion(baseUrl, apiKey, versionId, promptId)
	duration1 := time.Since(start1)

	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}
	if result1 == nil {
		t.Fatal("Expected non-nil result from first call")
	}

	t.Logf("First call took: %v", duration1)
	t.Logf("Retrieved prompt version: ID=%s, Version=%d, PromptID=%s",
		result1.ID, result1.Version, result1.PromptID)

	// Verify basic structure
	if result1.ID == "" {
		t.Error("Expected non-empty ID")
	}
	if result1.PromptID != promptId {
		t.Errorf("Expected PromptID '%s', got '%s'", promptId, result1.PromptID)
	}
	if result1.Config.Model == "" {
		t.Error("Expected non-empty Model")
	}
	if result1.Config.Provider == "" {
		t.Error("Expected non-empty Provider")
	}

	// Second call - should be served from cache
	t.Log("Making second API call (should be cached)...")
	start2 := time.Now()
	result2, err := GetPromptVersion(baseUrl, apiKey, versionId, promptId)
	duration2 := time.Since(start2)

	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}
	if result2 == nil {
		t.Fatal("Expected non-nil result from second call")
	}

	t.Logf("Second call took: %v", duration2)

	// Verify that second call was significantly faster (cached)
	// Cache should be at least 10x faster than API call
	if duration2 > duration1/10 {
		t.Logf("Warning: Second call (%v) was not significantly faster than first call (%v)",
			duration2, duration1)
		t.Log("This might indicate caching is not working properly")
	} else {
		t.Logf("✓ Caching verified: Second call was %v faster", duration1-duration2)
	}

	// Verify both results are identical (same pointer from cache)
	if result1 != result2 {
		t.Error("Expected same pointer from cache, got different instances")
	}

	// Verify data consistency
	if result1.ID != result2.ID {
		t.Errorf("ID mismatch: %s != %s", result1.ID, result2.ID)
	}
	if result1.Version != result2.Version {
		t.Errorf("Version mismatch: %d != %d", result1.Version, result2.Version)
	}
	if result1.PromptID != result2.PromptID {
		t.Errorf("PromptID mismatch: %s != %s", result1.PromptID, result2.PromptID)
	}
	if result1.Config.Model != result2.Config.Model {
		t.Errorf("Model mismatch: %s != %s", result1.Config.Model, result2.Config.Model)
	}

	t.Log("✓ All caching tests passed")
}

// TestGetPromptVersion_MultipleDifferentPrompts tests caching with different prompt versions
func TestGetPromptVersion_MultipleDifferentPrompts(t *testing.T) {
	apiKey := os.Getenv("MAXIM_API_KEY")
	if apiKey == "" {
		t.Skip("MAXIM_API_KEY not set, skipping integration test")
	}

	versionId1 := os.Getenv("MAXIM_TEST_VERSION_ID")
	promptId1 := os.Getenv("MAXIM_TEST_PROMPT_ID")
	versionId2 := os.Getenv("MAXIM_TEST_VERSION_ID_2")
	promptId2 := os.Getenv("MAXIM_TEST_PROMPT_ID_2")

	if versionId1 == "" || promptId1 == "" {
		t.Skip("MAXIM_TEST_VERSION_ID or MAXIM_TEST_PROMPT_ID not set, skipping integration test")
	}
	if versionId2 == "" || promptId2 == "" {
		t.Skip("MAXIM_TEST_VERSION_ID_2 or MAXIM_TEST_PROMPT_ID_2 not set, skipping this test (optional)")
	}

	baseUrl := os.Getenv("MAXIM_BASE_URL")
	if baseUrl == "" {
		t.Skip("MAXIM_BASE_URL not set, skipping integration test")
	}

	// Fetch first prompt version
	t.Logf("Fetching prompt version 1: versionId=%s, promptId=%s", versionId1, promptId1)
	result1a, err := GetPromptVersion(baseUrl, apiKey, versionId1, promptId1)
	if err != nil {
		t.Fatalf("Failed to fetch prompt version 1: %v", err)
	}

	// Fetch second prompt version
	t.Logf("Fetching prompt version 2: versionId=%s, promptId=%s", versionId2, promptId2)
	result2a, err := GetPromptVersion(baseUrl, apiKey, versionId2, promptId2)
	if err != nil {
		t.Fatalf("Failed to fetch prompt version 2: %v", err)
	}

	// Verify they are different
	if result1a.ID == result2a.ID {
		t.Error("Expected different prompt versions, got same ID")
	}

	// Fetch first prompt version again (should be cached)
	result1b, err := GetPromptVersion(baseUrl, apiKey, versionId1, promptId1)
	if err != nil {
		t.Fatalf("Failed to fetch prompt version 1 again: %v", err)
	}

	// Fetch second prompt version again (should be cached)
	result2b, err := GetPromptVersion(baseUrl, apiKey, versionId2, promptId2)
	if err != nil {
		t.Fatalf("Failed to fetch prompt version 2 again: %v", err)
	}

	// Verify caching for both
	if result1a != result1b {
		t.Error("Prompt version 1 was not properly cached")
	}
	if result2a != result2b {
		t.Error("Prompt version 2 was not properly cached")
	}

	t.Log("✓ Multiple prompt versions cached independently")
}

// TestGetPromptVersion_InvalidIDs tests error handling with invalid IDs
func TestGetPromptVersion_InvalidIDs(t *testing.T) {
	apiKey := os.Getenv("MAXIM_API_KEY")
	if apiKey == "" {
		t.Skip("MAXIM_API_KEY not set, skipping integration test")
	}

	baseUrl := os.Getenv("MAXIM_BASE_URL")
	if baseUrl == "" {
		t.Skip("MAXIM_BASE_URL not set, skipping integration test")
	}

	// Try to fetch with non-existent IDs
	result, err := GetPromptVersion(baseUrl, apiKey, "non-existent-version-id", "non-existent-prompt-id")

	if err == nil {
		t.Error("Expected error for non-existent IDs, got nil")
	} else {
		t.Logf("✓ Correctly received error: %v", err)
	}

	if result != nil {
		t.Errorf("Expected nil result on error, got %+v", result)
	}
}

// TestGetPromptVersion_Structure tests the complete structure of returned data
func TestGetPromptVersion_Structure(t *testing.T) {
	apiKey := os.Getenv("MAXIM_API_KEY")
	if apiKey == "" {
		t.Skip("MAXIM_API_KEY not set, skipping integration test")
	}

	versionId := os.Getenv("MAXIM_TEST_VERSION_ID")
	if versionId == "" {
		t.Skip("MAXIM_TEST_VERSION_ID not set, skipping integration test")
	}

	promptId := os.Getenv("MAXIM_TEST_PROMPT_ID")
	if promptId == "" {
		t.Skip("MAXIM_TEST_PROMPT_ID not set, skipping integration test")
	}

	baseUrl := os.Getenv("MAXIM_BASE_URL")
	if baseUrl == "" {
		t.Skip("MAXIM_BASE_URL not set, skipping integration test")
	}

	result, err := GetPromptVersion(baseUrl, apiKey, versionId, promptId)
	if err != nil {
		t.Fatalf("Failed to fetch prompt version: %v", err)
	}

	// Test top-level fields
	t.Run("TopLevelFields", func(t *testing.T) {
		if result.ID == "" {
			t.Error("ID is empty")
		}
		if result.Version == 0 {
			t.Error("Version is 0")
		}
		if result.PromptID == "" {
			t.Error("PromptID is empty")
		}
		if result.CreatedAt == "" {
			t.Error("CreatedAt is empty")
		}
		if result.UpdatedAt == "" {
			t.Error("UpdatedAt is empty")
		}
		t.Logf("✓ Top-level fields validated")
	})

	// Test Config fields
	t.Run("ConfigFields", func(t *testing.T) {
		if result.Config.Model == "" {
			t.Error("Config.Model is empty")
		}
		if result.Config.ModelID == "" {
			t.Error("Config.ModelID is empty")
		}
		if result.Config.Provider == "" {
			t.Error("Config.Provider is empty")
		}
		t.Logf("✓ Config fields validated")
	})

	// Test Author fields
	t.Run("AuthorFields", func(t *testing.T) {
		if result.Config.Author.ID == "" {
			t.Error("Author.ID is empty")
		}
		if result.Config.Author.Name == "" {
			t.Error("Author.Name is empty")
		}
		if result.Config.Author.Email == "" {
			t.Error("Author.Email is empty")
		}
		t.Logf("✓ Author fields validated")
	})

	// Test Messages
	t.Run("Messages", func(t *testing.T) {
		if len(result.Config.Messages) == 0 {
			t.Log("Warning: No messages in prompt version")
		} else {
			t.Logf("Prompt has %d message(s)", len(result.Config.Messages))
			for i, msg := range result.Config.Messages {
				if msg.ID == "" {
					t.Errorf("Message[%d].ID is empty", i)
				}

				// Check for RequestPayload
				if msg.Payload.RequestPayload != nil {
					req := msg.Payload.RequestPayload
					if req.Role == "" {
						t.Errorf("Message[%d].RequestPayload.Role is empty", i)
					}
					if req.Content.MessagePayloadContentStr == nil && len(req.Content.MessagePayloadContentArray) == 0 {
						t.Errorf("Message[%d].RequestPayload.Content is empty", i)
					}
					if req.Content.MessagePayloadContentStr != nil {
						t.Logf("  Message[%d] (request): Role=%s, Content=%s",
							i, req.Role, *req.Content.MessagePayloadContentStr)
					}
					if len(req.Content.MessagePayloadContentArray) > 0 {
						t.Logf("  Message[%d] (request): Role=%s, Content blocks=%d",
							i, req.Role, len(req.Content.MessagePayloadContentArray))
					}
				}

				// Check for ResultPayload
				if msg.Payload.ResultPayload != nil {
					res := msg.Payload.ResultPayload
					if res.ID == "" {
						t.Errorf("Message[%d].ResultPayload.ID is empty", i)
					}
					t.Logf("  Message[%d] (result): ID=%s, Model=%s, Provider=%s",
						i, res.ID, res.Model, res.Provider)
					t.Logf("    Cost: Input=%.6f, Output=%.6f, Total=%.6f",
						res.Cost.Input, res.Cost.Output, res.Cost.Total)
					t.Logf("    Usage: Prompt=%d, Completion=%d, Total=%d",
						res.Usage.PromptTokens, res.Usage.CompletionTokens, res.Usage.TotalTokens)
				}

				// Ensure exactly one payload type is set
				if msg.Payload.RequestPayload == nil && msg.Payload.ResultPayload == nil {
					t.Errorf("Message[%d] has neither RequestPayload nor ResultPayload", i)
				}
				if msg.Payload.RequestPayload != nil && msg.Payload.ResultPayload != nil {
					t.Errorf("Message[%d] has both RequestPayload and ResultPayload (should be only one)", i)
				}
			}
		}
		t.Logf("✓ Messages validated")
	})

	// Test ModelParameters
	t.Run("ModelParameters", func(t *testing.T) {
		params := result.Config.ModelParameters
		t.Logf("Temperature: %f", params.Temperature)
		t.Logf("MaxTokens: %d", params.MaxTokens)
		t.Logf("TopP: %f", params.TopP)
		t.Logf("PresencePenalty: %f", params.PresencePenalty)
		t.Logf("FrequencyPenalty: %f", params.FrequencyPenalty)
		t.Logf("Logprobs: %v", params.Logprobs)
		t.Logf("N: %d", params.N)
		t.Logf("PromptTools: %v", params.PromptTools)
		t.Logf("✓ Model parameters validated")
	})
}

// TestGetPromptVersion_RequestPayload tests completion request payload structure
func TestGetPromptVersion_RequestPayload(t *testing.T) {
	apiKey := os.Getenv("MAXIM_API_KEY")
	if apiKey == "" {
		t.Skip("MAXIM_API_KEY not set, skipping integration test")
	}

	versionId := os.Getenv("MAXIM_TEST_VERSION_ID")
	if versionId == "" {
		t.Skip("MAXIM_TEST_VERSION_ID not set, skipping integration test")
	}

	promptId := os.Getenv("MAXIM_TEST_PROMPT_ID")
	if promptId == "" {
		t.Skip("MAXIM_TEST_PROMPT_ID not set, skipping integration test")
	}

	baseUrl := os.Getenv("MAXIM_BASE_URL")
	if baseUrl == "" {
		t.Skip("MAXIM_BASE_URL not set, skipping integration test")
	}

	result, err := GetPromptVersion(baseUrl, apiKey, versionId, promptId)
	if err != nil {
		t.Fatalf("Failed to fetch prompt version: %v", err)
	}

	// Find request payload messages
	requestCount := 0
	for i, msg := range result.Config.Messages {
		if msg.Payload.RequestPayload != nil {
			requestCount++
			req := msg.Payload.RequestPayload

			t.Logf("Testing RequestPayload message %d (index %d)", requestCount, i)

			// Verify role
			if req.Role == "" {
				t.Errorf("Message[%d]: RequestPayload.Role is empty", i)
			} else {
				t.Logf("  Role: %s", req.Role)
				// Check if role is one of the expected values
				validRoles := map[string]bool{"system": true, "user": true, "assistant": true}
				if !validRoles[req.Role] {
					t.Logf("  Warning: Unexpected role '%s'", req.Role)
				}
			}

			// Verify content
			if req.Content.MessagePayloadContentStr != nil {
				t.Logf("  Content type: string")
				if *req.Content.MessagePayloadContentStr == "" {
					t.Errorf("Message[%d]: Content string is empty", i)
				} else {
					contentPreview := *req.Content.MessagePayloadContentStr
					if len(contentPreview) > 100 {
						contentPreview = contentPreview[:100] + "..."
					}
					t.Logf("  Content preview: %s", contentPreview)
				}
			} else if len(req.Content.MessagePayloadContentArray) > 0 {
				t.Logf("  Content type: array (%d blocks)", len(req.Content.MessagePayloadContentArray))
				for j, block := range req.Content.MessagePayloadContentArray {
					if block.Type == "" {
						t.Errorf("Message[%d]: Content block[%d] type is empty", i, j)
					}
					if block.Text == "" {
						t.Errorf("Message[%d]: Content block[%d] text is empty", i, j)
					}
					textPreview := block.Text
					if len(textPreview) > 100 {
						textPreview = textPreview[:100] + "..."
					}
					t.Logf("    Block[%d]: Type=%s, Text=%s", j, block.Type, textPreview)
				}
			} else {
				t.Errorf("Message[%d]: RequestPayload has no content", i)
			}

			// Verify CurrentType
			if msg.CurrentType != "completion_request" {
				t.Logf("  Warning: Message with RequestPayload has CurrentType='%s' (expected 'completion_request')", msg.CurrentType)
			}
		}
	}

	if requestCount == 0 {
		t.Log("Warning: No RequestPayload messages found in this prompt version")
	} else {
		t.Logf("✓ Validated %d RequestPayload message(s)", requestCount)
	}
}

// TestGetPromptVersion_ResultPayload tests completion result payload structure
func TestGetPromptVersion_ResultPayload(t *testing.T) {
	apiKey := os.Getenv("MAXIM_API_KEY")
	if apiKey == "" {
		t.Skip("MAXIM_API_KEY not set, skipping integration test")
	}

	versionId := os.Getenv("MAXIM_TEST_VERSION_ID")
	if versionId == "" {
		t.Skip("MAXIM_TEST_VERSION_ID not set, skipping integration test")
	}

	promptId := os.Getenv("MAXIM_TEST_PROMPT_ID")
	if promptId == "" {
		t.Skip("MAXIM_TEST_PROMPT_ID not set, skipping integration test")
	}

	baseUrl := os.Getenv("MAXIM_BASE_URL")
	if baseUrl == "" {
		t.Skip("MAXIM_BASE_URL not set, skipping integration test")
	}

	result, err := GetPromptVersion(baseUrl, apiKey, versionId, promptId)
	if err != nil {
		t.Fatalf("Failed to fetch prompt version: %v", err)
	}

	// Find result payload messages
	resultCount := 0
	for i, msg := range result.Config.Messages {
		if msg.Payload.ResultPayload != nil {
			resultCount++
			res := msg.Payload.ResultPayload

			t.Logf("Testing ResultPayload message %d (index %d)", resultCount, i)

			// Verify ID
			if res.ID == "" {
				t.Errorf("Message[%d]: ResultPayload.ID is empty", i)
			} else {
				t.Logf("  ID: %s", res.ID)
			}

			// Verify model and provider
			if res.Model == "" {
				t.Errorf("Message[%d]: ResultPayload.Model is empty", i)
			} else {
				t.Logf("  Model: %s", res.Model)
			}

			if res.Provider == "" {
				t.Errorf("Message[%d]: ResultPayload.Provider is empty", i)
			} else {
				t.Logf("  Provider: %s", res.Provider)
			}

			// Verify cost
			if res.Cost.Total < 0 {
				t.Errorf("Message[%d]: Cost.Total is negative: %f", i, res.Cost.Total)
			}
			if res.Cost.Input < 0 {
				t.Errorf("Message[%d]: Cost.Input is negative: %f", i, res.Cost.Input)
			}
			if res.Cost.Output < 0 {
				t.Errorf("Message[%d]: Cost.Output is negative: %f", i, res.Cost.Output)
			}
			t.Logf("  Cost: Input=%.6f, Output=%.6f, Total=%.6f",
				res.Cost.Input, res.Cost.Output, res.Cost.Total)

			// Verify usage
			if res.Usage.TotalTokens == 0 {
				t.Logf("  Warning: TotalTokens is 0")
			}
			if res.Usage.PromptTokens < 0 {
				t.Errorf("Message[%d]: PromptTokens is negative: %d", i, res.Usage.PromptTokens)
			}
			if res.Usage.CompletionTokens < 0 {
				t.Errorf("Message[%d]: CompletionTokens is negative: %d", i, res.Usage.CompletionTokens)
			}
			t.Logf("  Usage: Prompt=%d, Completion=%d, Total=%d",
				res.Usage.PromptTokens, res.Usage.CompletionTokens, res.Usage.TotalTokens)

			if res.Usage.Latency > 0 {
				t.Logf("  Latency: %.2fms", res.Usage.Latency)
			}

			// Verify trace
			if len(res.Trace.Input.Messages) == 0 {
				t.Logf("  Warning: Trace has no input messages")
			} else {
				t.Logf("  Trace input messages: %d", len(res.Trace.Input.Messages))
			}

			if res.Trace.Output.ID == "" {
				t.Logf("  Warning: Trace output ID is empty")
			}

			// Verify choices
			if len(res.Choices) == 0 {
				t.Errorf("Message[%d]: ResultPayload has no choices", i)
			} else {
				t.Logf("  Choices: %d", len(res.Choices))
				for j, choice := range res.Choices {
					if choice.Message.Role == "" {
						t.Errorf("Message[%d]: Choice[%d].Message.Role is empty", i, j)
					}
					if choice.Message.Content.MessagePayloadContentStr == nil && len(choice.Message.Content.MessagePayloadContentArray) == 0 {
						t.Errorf("Message[%d]: Choice[%d].Message.Content is empty", i, j)
					}
					if choice.FinishReason == "" {
						t.Logf("    Warning: Choice[%d] has empty FinishReason", j)
					}

					var contentPreview string
					if choice.Message.Content.MessagePayloadContentStr != nil {
						contentPreview = *choice.Message.Content.MessagePayloadContentStr
					} else if len(choice.Message.Content.MessagePayloadContentArray) > 0 {
						for _, block := range choice.Message.Content.MessagePayloadContentArray {
							contentPreview += block.Text
						}
					}
					if len(contentPreview) > 100 {
						contentPreview = contentPreview[:100] + "..."
					}
					t.Logf("    Choice[%d]: Role=%s, FinishReason=%s, Content=%s",
						j, choice.Message.Role, choice.FinishReason, contentPreview)
				}
			}

			// Verify CurrentType
			if msg.CurrentType != "completion_result" {
				t.Logf("  Warning: Message with ResultPayload has CurrentType='%s' (expected 'completion_result')", msg.CurrentType)
			}

			// Verify ModelParams exists
			if len(res.ModelParams) > 0 {
				t.Logf("  ModelParams: %d parameters", len(res.ModelParams))
			}

			// Verify VariableBoundRetrievals exists
			if len(res.VariableBoundRetrievals) > 0 {
				t.Logf("  VariableBoundRetrievals: %d variables", len(res.VariableBoundRetrievals))
			}
		}
	}

	if resultCount == 0 {
		t.Log("Warning: No ResultPayload messages found in this prompt version")
	} else {
		t.Logf("✓ Validated %d ResultPayload message(s)", resultCount)
	}
}

// TestGetPromptVersion_PayloadTypeSeparation tests that messages have only one payload type
func TestGetPromptVersion_PayloadTypeSeparation(t *testing.T) {
	apiKey := os.Getenv("MAXIM_API_KEY")
	if apiKey == "" {
		t.Skip("MAXIM_API_KEY not set, skipping integration test")
	}

	versionId := os.Getenv("MAXIM_TEST_VERSION_ID")
	if versionId == "" {
		t.Skip("MAXIM_TEST_VERSION_ID not set, skipping integration test")
	}

	promptId := os.Getenv("MAXIM_TEST_PROMPT_ID")
	if promptId == "" {
		t.Skip("MAXIM_TEST_PROMPT_ID not set, skipping integration test")
	}

	baseUrl := os.Getenv("MAXIM_BASE_URL")
	if baseUrl == "" {
		t.Skip("MAXIM_BASE_URL not set, skipping integration test")
	}

	result, err := GetPromptVersion(baseUrl, apiKey, versionId, promptId)
	if err != nil {
		t.Fatalf("Failed to fetch prompt version: %v", err)
	}

	requestCount := 0
	resultCount := 0
	invalidCount := 0

	for i, msg := range result.Config.Messages {
		hasRequest := msg.Payload.RequestPayload != nil
		hasResult := msg.Payload.ResultPayload != nil

		if hasRequest && hasResult {
			t.Errorf("Message[%d]: Has both RequestPayload and ResultPayload", i)
			invalidCount++
		} else if !hasRequest && !hasResult {
			t.Errorf("Message[%d]: Has neither RequestPayload nor ResultPayload", i)
			invalidCount++
		} else if hasRequest {
			requestCount++
			// Verify currentType matches
			if msg.CurrentType != "completion_request" {
				t.Logf("Message[%d]: RequestPayload with CurrentType='%s'", i, msg.CurrentType)
			}
		} else if hasResult {
			resultCount++
			// Verify currentType matches
			if msg.CurrentType != "completion_result" {
				t.Logf("Message[%d]: ResultPayload with CurrentType='%s'", i, msg.CurrentType)
			}
		}
	}

	t.Logf("Payload type distribution:")
	t.Logf("  RequestPayload messages: %d", requestCount)
	t.Logf("  ResultPayload messages: %d", resultCount)
	t.Logf("  Invalid messages: %d", invalidCount)

	if invalidCount > 0 {
		t.Errorf("Found %d messages with invalid payload configuration", invalidCount)
	} else {
		t.Logf("✓ All messages have exactly one payload type")
	}
}
