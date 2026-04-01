package logging

import (
	"encoding/json"
	"log"
	"time"

	"github.com/maximhq/maxim-go/schemas"
)

type GenerationConfig struct {
	Id              string                      `json:"id"`
	SpanId          *string                     `json:"spanId,omitempty"`
	Name            *string                     `json:"name,omitempty"`
	Tags            *map[string]string          `json:"tags,omitempty"`
	Provider        string                      `json:"provider"`
	Model           string                      `json:"model"`
	MaximPromptID   *string                     `json:"maximPromptId,omitempty"`
	Messages        []schemas.CompletionRequest `json:"messages"`
	ModelParameters map[string]interface{}      `json:"modelParameters"`
}

type Generation struct {
	*base
	maximPromptID   *string
	model           string
	provider        string
	messages        []schemas.CompletionRequest
	modelParameters map[string]interface{}
	error           *schemas.GenerationError
}

func newGeneration(c *GenerationConfig, w *writer) *Generation {
	return &Generation{
		base: newBase(EntityGeneration, c.Id, &baseConfig{
			SpanId: c.SpanId,
			Name:   c.Name,
			Tags:   c.Tags,
			Id:     c.Id,
		}, w),
		model:           c.Model,
		provider:        c.Provider,
		messages:        c.Messages,
		modelParameters: c.ModelParameters,
	}
}

func (g *Generation) SetModel(m string) {
	g.model = m
	g.commit("update", map[string]interface{}{
		"model": g.model,
	})
}

func (g *Generation) AddMessage(msg schemas.CompletionRequest) {
	g.commit("update", map[string]interface{}{
		"messages": []schemas.CompletionRequest{msg},
	})
}

func (g *Generation) AddMessages(m []schemas.CompletionRequest) {
	g.commit("update", map[string]interface{}{
		"messages": m,
	})
}

func (g *Generation) SetModelParameters(mp map[string]interface{}) {
	g.modelParameters = mp
	g.commit("update", map[string]interface{}{
		"modelParameters": g.modelParameters,
	})
}

func (g *Generation) handleBedrockConverseResult(jsonData []byte) (*schemas.MaximLLMResult, error) {
	return ParseBedrockResult(g.model, jsonData)
}

// handleOpenAIResult extracts and logs data from an OpenAI completion
func (g *Generation) handleOpenAIResult(jsonData []byte) (*schemas.MaximLLMResult, error) {
	return ParseOpenAIResult(jsonData)
}

// handleAnthropicResult extracts and logs data from an Anthropic completion
func (g *Generation) handleAnthropicResult(jsonData []byte) (*schemas.MaximLLMResult, error) {
	return ParseAnthropicResult(jsonData)
}

// handleAzure extracts and logs data from an Azure OpenAI completion
func (g *Generation) handleAzure(jsonData []byte, _ time.Duration) (*schemas.MaximLLMResult, error) {
	// Azure OpenAI has the same response format as OpenAI
	return g.handleOpenAIResult(jsonData)
}

func (g *Generation) handleGeminiResult(jsonData []byte) (*schemas.MaximLLMResult, error) {
	return ParseGeminiResult(jsonData)
}

func (g *Generation) SetMaximPromptID(pId string) {
	g.maximPromptID = &pId
	g.commit("update", map[string]interface{}{
		"maximPromptId": g.maximPromptID,
	})
}

func (g *Generation) Evaluate() *evaluateContainer {
	return newEvaluateContainer(EntityGeneration, g.Id(), g.writer)
}

// AddAttachment adds an attachment to this generation.
// The attachment can be *FileAttachment, *FileDataAttachment, *UrlAttachment, or map[string]interface{}.
func (g *Generation) AddAttachment(attachment interface{}) {
	g.commit("upload-attachment", attachment)
}

func (g *Generation) SetResult(r interface{}) {
	var finalResult *schemas.MaximLLMResult
	var err error
	var jsonData []byte
	jsonData, err = json.Marshal(r)
	if err != nil {
		log.Printf("Failed to marshal result: %v", err)
		return
	}
	// Parsing the result
	switch g.provider {
	case ProviderOpenAI:
		finalResult, err = g.handleOpenAIResult(jsonData)
	case ProviderAzure:
		finalResult, err = g.handleAzure(jsonData, time.Duration(0))
	case ProviderBedrock:
		finalResult, err = g.handleBedrockConverseResult(jsonData)
	case ProviderAnthropic:
		finalResult, err = g.handleAnthropicResult(jsonData)
	case ProviderGemini, ProviderVertex:
		finalResult, err = g.handleGeminiResult(jsonData)
	}
	if err != nil {
		log.Println("[MaximSDK] Failed to parse result", err)
	}
	if finalResult == nil {
		log.Println("[MaximSDK] No result to set. Here is the valid format for the result: ")
		log.Println(`generation.SetResult(map[string]interface{}{
		"id": uuid.New().String(),
		"model": "gpt-4o",
		"created": time.Now().Unix(),
		"choices": []map[string]interface{}{
			{
				"message": map[string]interface{}{
					"role": "assistant",
					"content": "Hello, world!",
				},
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens": 10,
			"completion_tokens": 10,
			"total_tokens": 20,
		},
		"cost": map[string]interface{}{ // optional if you want to override the cost
			"input": 0.0,
			"output": 0.0,
			"total": 0.0,
		},
	})`)
		return
	}
	g.commit("result", map[string]interface{}{
		"result": finalResult,
	})
}

func (g *Generation) SetError(err *schemas.GenerationError) {
	g.error = err
	g.commit("result", map[string]interface{}{
		"result": map[string]interface{}{
			"error": g.error,
		},
	})
	g.End()
}

func (g *Generation) data() map[string]interface{} {
	base := g.base.data()
	base["provider"] = g.provider
	base["model"] = g.model
	if g.maximPromptID != nil {
		base["maximPromptId"] = *g.maximPromptID
	}
	if len(g.messages) > 0 {
		base["messages"] = g.messages
	}
	if len(g.modelParameters) > 0 {
		base["modelParameters"] = g.modelParameters
	}
	return base
}
