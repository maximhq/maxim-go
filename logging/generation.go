package logging

import (
	"encoding/json"
	"log"
	"time"
)

type GenerationError struct {
	Message string  `json:"message"`
	Code    *string `json:"code,omitempty"`
	Type    *string `json:"type,omitempty"`
}

type MaximLLMResult struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Created int64  `json:"created"`
	Choices []struct {
		Message struct {
			Role      string                   `json:"role"`
			Content   string                   `json:"content"`
			ToolCalls []ChatCompletionToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type BedrockToolUse struct {
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
	ID    string                 `json:"toolUseId"`
}

type BedrockConverseResp struct {
	Metrics struct {
		LatencyMs int `json:"latencyMs"`
	} `json:"metrics"`
	Output struct {
		Value *struct {
			Content []struct {
				Value interface{} `json:"value"`
			} `json:"content"`
			Role string `json:"role"`
		} `json:"value,omitempty"`
		Message *struct {
			Content []struct {
				Text    string          `json:"text"`
				ToolUse *BedrockToolUse `json:"toolUse"`
			} `json:"content"`
			Role string `json:"role"`
		} `json:"message,omitempty"`
	} `json:"output"`
	StopReason string `json:"stopReason"`
	Usage      struct {
		InputTokens  int `json:"inputTokens"`
		OutputTokens int `json:"outputTokens"`
		TotalTokens  int `json:"totalTokens"`
	} `json:"usage"`
}

type ChatCompletionResult struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   Usage                  `json:"usage"`
	Error   *GenerationError       `json:"error,omitempty"`
}

type TextCompletionResult struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []TextCompletionChoice `json:"choices"`
	Usage   Usage                  `json:"usage"`
	Error   *GenerationError       `json:"error,omitempty"`
}

type ToolCallFunction struct {
	Arguments string `json:"arguments"`
	Name      string `json:"name"`
}

type ChatCompletionToolCall struct {
	ID       string           `json:"id"`
	Function ToolCallFunction `json:"function"`
	Type     string           `json:"type"`
}

type ChatCompletionMessage struct {
	Role         string                   `json:"role"`
	Content      *string                  `json:"content"`
	FunctionCall *ToolCallFunction        `json:"function_call,omitempty"`
	ToolCalls    []ChatCompletionToolCall `json:"tool_calls,omitempty"`
}

type ChatCompletionChoice struct {
	Index        int                   `json:"index"`
	Message      ChatCompletionMessage `json:"message"`
	LogProbs     interface{}           `json:"logprobs"`
	FinishReason string                `json:"finish_reason"`
}

type TextCompletionChoice struct {
	Index        int         `json:"index"`
	Text         string      `json:"text"`
	LogProbs     interface{} `json:"logprobs"`
	FinishReason string      `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type CompletionRequestTextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type CompletionRequestImageUrlContent struct {
	Type     string `json:"type"`
	ImageURL struct {
		URL    string  `json:"url"`
		Detail *string `json:"detail,omitempty"`
	} `json:"image_url"`
}

type CompletionRequestContent interface{}

type CompletionRequest struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type GenerationConfig struct {
	Id              string                 `json:"id"`
	SpanId          *string                `json:"spanId,omitempty"`
	Name            *string                `json:"name,omitempty"`
	Tags            *map[string]string     `json:"tags,omitempty"`
	Provider        string                 `json:"provider"`
	Model           string                 `json:"model"`
	MaximPromptID   *string                `json:"maximPromptId,omitempty"`
	Messages        []CompletionRequest    `json:"messages"`
	ModelParameters map[string]interface{} `json:"modelParameters"`
}

type Generation struct {
	*base
	maximPromptID   *string
	model           string
	provider        string
	messages        []CompletionRequest
	modelParameters map[string]interface{}
	error           *GenerationError
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

func (g *Generation) AddMessage(msg CompletionRequest) {
	g.commit("update", map[string]interface{}{
		"messages": []CompletionRequest{msg},
	})
}

func (g *Generation) AddMessages(m []CompletionRequest) {
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

func (g *Generation) handleBedrockConverseResult(jsonData []byte) (*MaximLLMResult, error) {
	return ParseBedrockResult(g.model, jsonData)
}

// handleOpenAIResult extracts and logs data from an OpenAI completion
func (g *Generation) handleOpenAIResult(jsonData []byte) (*MaximLLMResult, error) {
	return ParseOpenAIResult(jsonData)
}

// handleAnthropicResult extracts and logs data from an Anthropic completion
func (g *Generation) handleAnthropicResult(jsonData []byte) (*MaximLLMResult, error) {
	return ParseAnthropicResult(jsonData)
}

// handleAzure extracts and logs data from an Azure OpenAI completion
func (g *Generation) handleAzure(jsonData []byte, _ time.Duration) (*MaximLLMResult, error) {
	// Azure OpenAI has the same response format as OpenAI
	return g.handleOpenAIResult(jsonData)
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

func (g *Generation) SetResult(r interface{}) {
	var finalResult *MaximLLMResult
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
	})`)
		return
	}
	g.commit("result", map[string]interface{}{
		"result": finalResult,
	})
}

func (g *Generation) SetError(err *GenerationError) {
	g.error = err
	g.commit("update", map[string]interface{}{
		"error": g.error,
	})
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
