package schemas

// MaximLLMChoiceMessage represents the message within a choice in MaximLLMResult.
type MaximLLMChoiceMessage struct {
	Role      string                   `json:"role"`
	Content   string                   `json:"content"`
	ToolCalls []ChatCompletionToolCall `json:"tool_calls,omitempty"`
}

// MaximLLMChoice represents a single choice in MaximLLMResult.
type MaximLLMChoice struct {
	Message      MaximLLMChoiceMessage `json:"message"`
	FinishReason string                `json:"finish_reason"`
}

// MaximLLMResult is the normalized LLM response format used across providers.
type MaximLLMResult struct {
	ID      string           `json:"id"`
	Model   string           `json:"model"`
	Created int64            `json:"created"`
	Choices []MaximLLMChoice `json:"choices"`
	Usage   Usage            `json:"usage"`
	Cost    *Cost            `json:"cost,omitempty"`
}

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
	Cost    *Cost                  `json:"cost,omitempty"`
	Error   *GenerationError       `json:"error,omitempty"`
}

type TextCompletionResult struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []TextCompletionChoice `json:"choices"`
	Usage   Usage                  `json:"usage"`
	Cost    *Cost                  `json:"cost,omitempty"`
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

type Cost struct {
	Input  float64 `json:"input"`
	Output float64 `json:"output"`
	Total  float64 `json:"total"`
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

type MaximImageGenerationResult struct {
	ID           string                     `json:"id,omitempty"`
	Created      int64                      `json:"created,omitempty"`
	Model        string                     `json:"model,omitempty"`
	Data         []MaximImageGenerationData `json:"data"`
	OutputFormat string                     `json:"output_format,omitempty"`
	Background   string                     `json:"background,omitempty"`
	Quality      string                     `json:"quality,omitempty"`
	Size         string                     `json:"size,omitempty"`
	Usage        *MaximImageGenerationUsage `json:"usage,omitempty"`
}

type MaximImageGenerationData struct {
	URL           string `json:"url,omitempty"`
	B64JSON       string `json:"b64_json,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
	Index         int    `json:"index"`
}

type MaximImageGenerationUsage struct {
	InputTokens  int `json:"prompt_tokens,omitempty"`
	OutputTokens int `json:"completion_tokens,omitempty"`
	TotalTokens  int `json:"total_tokens,omitempty"`
}
