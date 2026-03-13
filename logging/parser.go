package logging

import (
	"encoding/json"
	"log"

	"github.com/maximhq/maxim-go/schemas"
)

func ParseResult(provider, model string, r interface{}) (*schemas.MaximLLMResult, error) {
	jsonData, err := json.Marshal(r)
	if err != nil {
		log.Printf("Failed to marshal result: %v", err)
		return nil, err
	}
	switch provider {
	case ProviderOpenAI:
		return ParseOpenAIResult(jsonData)
	case ProviderAzure:
		return ParseOpenAIResult(jsonData)
	case ProviderBedrock:
		return ParseBedrockResult(model, jsonData)
	case ProviderAnthropic:
		return ParseAnthropicResult(jsonData)
	}
	return nil, nil
}
