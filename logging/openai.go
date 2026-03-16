package logging

import (
	"encoding/json"
	"fmt"

	"github.com/maximhq/maxim-go/schemas"
)

func ParseOpenAIResult(jsonData []byte) (*schemas.MaximLLMResult, error) {
	resp := schemas.MaximLLMResult{}
	if err := json.Unmarshal(jsonData, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal OpenAI completion: %w", err)
	}
	// Set the fields
	return &resp, nil
}
