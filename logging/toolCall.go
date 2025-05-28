package logging

type ToolCallConfig struct {
	Id          string             `json:"id"`
	SpanId      *string            `json:"spanId"`
	Name        *string            `json:"name"`
	Description *string            `json:"description"`
	Args        *string            `json:"args"`
	Tags        *map[string]string `json:"tags"`
}

type ToolCall struct {
	*base
	Description *string
	Args        *string
}

func newToolCall(c *ToolCallConfig, w *writer) *ToolCall {
	tc := &ToolCall{
		base: newBase(EntityToolCall, c.Id, &baseConfig{
			Id:     c.Id,
			SpanId: c.SpanId,
			Name:   c.Name,

			Tags: c.Tags,
		}, w),
		Description: c.Description,
		Args:        c.Args,
	}
	tcData := tc.data()
	tcData["description"] = tc.Description
	tcData["args"] = tc.Args
	tc.commit("create", tcData)
	return tc
}

func (tc *ToolCall) SetResult(result string) {
	tc.commit("result", map[string]interface{}{
		"result": result,
	})
	tc.End()
}

func (tc *ToolCall) SetError(err error) {
	tc.commit("error", map[string]interface{}{
		"error": err.Error(),
	})
	tc.End()
}

func (tc *ToolCall) End() {
	tc.base.End()
}

func (tc *ToolCall) data() map[string]interface{} {
	baseData := tc.base.data()
	baseData["description"] = tc.Description
	baseData["args"] = tc.Args
	return baseData
}
