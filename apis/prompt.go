package apis

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// PromptVersionResponse is the wrapper for the API response
type PromptVersionResponse struct {
	Data PromptVersion `json:"data"`
}

// PromptVersion represents a version of a prompt with its configuration
type PromptVersion struct {
	ID          string       `json:"id"`
	Version     int          `json:"version"`
	Description string       `json:"description"`
	PromptID    string       `json:"promptId"`
	Config      PromptConfig `json:"config"`
	CreatedAt   string       `json:"createdAt"`
	UpdatedAt   string       `json:"updatedAt"`
	DeletedAt   string       `json:"deletedAt,omitempty"`
}

// PromptConfig contains the configuration settings for a prompt version
type PromptConfig struct {
	Tags            map[string]interface{} `json:"tags"`
	Model           string                 `json:"model"`
	Author          Author                 `json:"author"`
	ModelID         string                 `json:"modelId"`
	Messages        []Message              `json:"messages"`
	Provider        string                 `json:"provider"`
	ModelParameters ModelParameters        `json:"modelParameters"`
}

// Author represents the author information
type Author struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Image string `json:"image"`
}

// Message represents a message in the prompt
type Message struct {
	ID           string         `json:"id"`
	Index        int            `json:"index"`
	Payload      MessagePayload `json:"payload"`
	CurrentType  string         `json:"currentType"`
	OriginalType string         `json:"originalType"`
}

// MessagePayload contains the role and content of a message
type MessagePayload struct {
	Role    string                `json:"role"`
	Content MessagePayloadContent `json:"content"`
}

type MessagePayloadContent struct {
	MessagePayloadContentStr   *string
	MessagePayloadContentArray []MessagePayloadContentBlock
}

type MessagePayloadContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (m *MessagePayloadContent) UnmarshalJSON(data []byte) error {
	var messageStr string
	if err := json.Unmarshal(data, &messageStr); err == nil {
		m.MessagePayloadContentStr = &messageStr
		return nil
	}
	var messageArray []MessagePayloadContentBlock
	if err := json.Unmarshal(data, &messageArray); err == nil {
		m.MessagePayloadContentArray = messageArray
		return nil
	}
	return fmt.Errorf("failed to unmarshal MessagePayloadContent")
}

// ModelParameters contains the model configuration parameters
type ModelParameters struct {
	N                int      `json:"n"`
	TopP             float64  `json:"top_p"`
	Logprobs         bool     `json:"logprobs"`
	MaxTokens        int      `json:"max_tokens"`
	PromptTools      []string `json:"promptTools"`
	Temperature      float64  `json:"temperature"`
	PresencePenalty  float64  `json:"presence_penalty"`
	FrequencyPenalty float64  `json:"frequency_penalty"`
}

// GetPromptVersion fetches a specific version of a prompt.
//
// Parameters:
//   - baseUrl: The base URL of the API endpoint.
//   - apiKey: The API key for authentication.
//   - versionId: The version ID you want to query.
//   - promptId: The prompt ID whose versions you want to query.
//
// Returns:
//   - *PromptVersion: The prompt version if successful, or nil with an error.
//   - *MaximError: An error if the request fails.
func GetPromptVersion(baseUrl, apiKey, versionId, promptId string) (*PromptVersion, *MaximError) {
	url := fmt.Sprintf("%s/api/public/v1/prompts/versions?id=%s&promptId=%s", baseUrl, versionId, promptId)
	client := &http.Client{
		Timeout: 15 * time.Second,
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, newMaximError(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-maxim-api-key", apiKey)
	resp, err := client.Do(req)
	if err != nil {
		return nil, newMaximError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, newMaximError(fmt.Errorf("unexpected status code: %d", resp.StatusCode))
	}

	var response PromptVersionResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, newMaximError(err)
	}

	return &response.Data, nil
}
