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

// MessagePayload holds either a request or result payload
type MessagePayload struct {
	RequestPayload *ChoiceMessage
	ResultPayload  *CompletionResultPayload
}

// UnmarshalJSON unmarshals the MessagePayload from JSON
func (m *MessagePayload) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as CompletionRequestPayload first
	var reqPayload ChoiceMessage
	if err := json.Unmarshal(data, &reqPayload); err == nil && reqPayload.Role != "" {
		m.RequestPayload = &reqPayload
		return nil
	}

	// Try to unmarshal as CompletionResultPayload
	var resPayload CompletionResultPayload
	if err := json.Unmarshal(data, &resPayload); err == nil {
		m.ResultPayload = &resPayload
		return nil
	}

	return fmt.Errorf("failed to unmarshal MessagePayload")
}

// MarshalJSON marshals the MessagePayload to JSON
func (m *MessagePayload) MarshalJSON() ([]byte, error) {
	if m.RequestPayload != nil {
		return json.Marshal(m.RequestPayload)
	}
	return json.Marshal(m.ResultPayload)
}

// CompletionResultPayload contains the completion result information
type CompletionResultPayload struct {
	ID                      string                 `json:"id"`
	Cost                    Cost                   `json:"cost"`
	Model                   string                 `json:"model"`
	Trace                   Trace                  `json:"trace"`
	Usage                   Usage                  `json:"usage"`
	Choices                 []Choice               `json:"choices"`
	Provider                string                 `json:"provider"`
	ModelParams             map[string]interface{} `json:"modelParams"`
	VariableBoundRetrievals map[string]interface{} `json:"variableBoundRetrievals"`
}

// Cost represents token cost information
type Cost struct {
	Input  float64 `json:"input"`
	Total  float64 `json:"total"`
	Output float64 `json:"output"`
}

// Trace contains input/output trace information
type Trace struct {
	Input  TraceInput  `json:"input"`
	Output TraceOutput `json:"output"`
}

// TraceInput contains the input messages for the trace
type TraceInput struct {
	Messages []ChoiceMessage `json:"messages"`
}

// TraceOutput contains the output from the completion
type TraceOutput struct {
	ID                string   `json:"id"`
	Model             string   `json:"model"`
	Usage             Usage    `json:"usage"`
	Object            string   `json:"object"`
	Choices           []Choice `json:"choices"`
	Created           int64    `json:"created"`
	ServiceTier       string   `json:"service_tier"`
	SystemFingerprint string   `json:"system_fingerprint"`
}

// Usage contains token usage information
type Usage struct {
	Latency                 float64                  `json:"latency,omitempty"`
	TotalTokens             int                      `json:"total_tokens"`
	PromptTokens            int                      `json:"prompt_tokens"`
	CompletionTokens        int                      `json:"completion_tokens"`
	PromptTokensDetails     *PromptTokensDetails     `json:"prompt_tokens_details,omitempty"`
	CompletionTokensDetails *CompletionTokensDetails `json:"completion_tokens_details,omitempty"`
}

// PromptTokensDetails contains details about prompt tokens
type PromptTokensDetails struct {
	AudioTokens  int `json:"audio_tokens"`
	CachedTokens int `json:"cached_tokens"`
}

// CompletionTokensDetails contains details about completion tokens
type CompletionTokensDetails struct {
	AudioTokens              int `json:"audio_tokens"`
	ReasoningTokens          int `json:"reasoning_tokens"`
	AcceptedPredictionTokens int `json:"accepted_prediction_tokens"`
	RejectedPredictionTokens int `json:"rejected_prediction_tokens"`
}

// Choice represents a completion choice
type Choice struct {
	Index        int           `json:"index"`
	Message      ChoiceMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
	Logprobs     interface{}   `json:"logprobs"`
}

// ChoiceMessage represents a message in a choice
type ChoiceMessage struct {
	Role        string                `json:"role"`
	Content     MessagePayloadContent `json:"content"`
	Refusal     *string               `json:"refusal"`
	Annotations []interface{}         `json:"annotations,omitempty"`
}

type MessagePayloadContent struct {
	MessagePayloadContentStr   *string
	MessagePayloadContentArray []MessagePayloadContentBlock
}

type MessagePayloadContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// UnmarshalJSON unmarshals the MessagePayloadContent from JSON
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

// MarshalJSON marshals the MessagePayloadContent to JSON
func (m *MessagePayloadContent) MarshalJSON() ([]byte, error) {
	if m.MessagePayloadContentStr != nil {
		return json.Marshal(m.MessagePayloadContentStr)
	}
	return json.Marshal(m.MessagePayloadContentArray)
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
