package logging

import (
	"encoding/json"
	"log"
)

func ParseResult(provider, model string, r interface{}) (*MaximLLMResult, error) {
	var finalResult *MaximLLMResult
	var err error
	var jsonData []byte
	jsonData, err = json.Marshal(r)
	if err != nil {
		log.Printf("Failed to marshal result: %v", err)
		return nil, err
	}
	// Parsing the result
	switch provider {
	case ProviderOpenAI:
		finalResult, err = ParseOpenAIResult(jsonData)
	case ProviderAzure:
		finalResult, err = ParseOpenAIResult(jsonData)
	case ProviderBedrock:
		finalResult, err = ParseBedrockResult(model, jsonData)
	case ProviderAnthropic:
		finalResult, err = ParseAnthropicResult(jsonData)
	}
	return finalResult, err
}
