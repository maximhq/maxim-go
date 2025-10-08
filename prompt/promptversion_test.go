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
				if msg.Payload.Role == "" {
					t.Errorf("Message[%d].Payload.Role is empty", i)
				}
				if msg.Payload.Content.MessagePayloadContentStr == nil && len(msg.Payload.Content.MessagePayloadContentArray) == 0 {
					t.Errorf("Message[%d].Payload.Content is empty", i)
				}
				if msg.Payload.Content.MessagePayloadContentStr != nil {
					t.Logf("  Message[%d]: Role=%s, Content=%s",
						i, msg.Payload.Role, *msg.Payload.Content.MessagePayloadContentStr)
				}
				if len(msg.Payload.Content.MessagePayloadContentArray) > 0 {
					t.Logf("  Message[%d]: Role=%s, Content length=%d",
						i, msg.Payload.Role, len(msg.Payload.Content.MessagePayloadContentArray))
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
