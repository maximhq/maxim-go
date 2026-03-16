package schemas

// BedrockToolUse represents a tool use from AWS Bedrock Converse API.
type BedrockToolUse struct {
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
	ID    string                 `json:"toolUseId"`
}

// BedrockConverseResp represents the response from AWS Bedrock Converse API.
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
