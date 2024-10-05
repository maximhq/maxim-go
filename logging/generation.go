package logging

type GenerationError struct {
	Message string  `json:"message"`
	Code    *string `json:"code,omitempty"`
	Type    *string `json:"type,omitempty"`
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

type ToolCall struct {
	ID       string           `json:"id"`
	Function ToolCallFunction `json:"function"`
	Type     string           `json:"type"`
}

type ChatCompletionMessage struct {
	Role         string            `json:"role"`
	Content      *string           `json:"content"`
	FunctionCall *ToolCallFunction `json:"function_call,omitempty"`
	ToolCalls    []ToolCall        `json:"tool_calls,omitempty"`
}

type ChatCompletionChoice struct {
	Index        int                     `json:"index"`
	Messages     []ChatCompletionMessage `json:"messages"`
	LogProbs     interface{}             `json:"logprobs"`
	FinishReason string                  `json:"finish_reason"`
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

func (g *Generation) AddMessages(m []CompletionRequest) {
	if g.messages == nil {
		g.messages = make([]CompletionRequest, 0)
	}
	for _, msg := range m {
		g.messages = append(g.messages, msg)
	}
	g.commit("update", map[string]interface{}{
		"messages": g.messages,
	})
}

func (g *Generation) SetModelParameters(mp map[string]interface{}) {
	g.modelParameters = mp
	g.commit("update", map[string]interface{}{
		"modelParameters": g.modelParameters,
	})
}

func (g *Generation) SetMaximPromptID(pId string) {
	g.maximPromptID = &pId
	g.commit("update", map[string]interface{}{
		"maximPromptId": g.maximPromptID,
	})
}

func (g *Generation) SetResult(r interface{}) {
	g.commit("result", map[string]interface{}{
		"result": r,
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
