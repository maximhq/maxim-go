package logging

import (
	"encoding/json"
	"fmt"
)

func ParseOpenAIResult( jsonData []byte) (*MaximLLMResult, error) {
	resp := MaximLLMResult{}
	if err := json.Unmarshal(jsonData, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal OpenAI completion: %w", err)
	}
	// Set the fields
	return &resp, nil
}