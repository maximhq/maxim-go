package logging

// TraceConfig represents the configuration for a trace
type TraceConfig struct {
	Id        string             `json:"id"`
	SpanId    *string            `json:"spanId,omitempty"`
	Name      *string            `json:"name,omitempty"`
	Tags      *map[string]string `json:"tags,omitempty"`
	SessionId *string
}

// Trace represents a trace in the logging system
type Trace struct {
	*eventEmitter
	SessionId *string
}

// newTrace creates a new trace
func newTrace(c *TraceConfig, w *writer) *Trace {
	t := &Trace{
		eventEmitter: &eventEmitter{
			base: newBase(EntityTrace, c.Id, &baseConfig{
				Id:     c.Id,
				SpanId: c.SpanId,
				Name:   c.Name,
				Tags:   c.Tags,
			}, w),
		},
		SessionId: c.SessionId,
	}
	tData := t.data()
	tData["id"] = c.Id
	t.commit("create", tData)
	return t
}

// AddGeneration adds a generation to the trace
func (t *Trace) AddGeneration(c *GenerationConfig) *Generation {
	g := newGeneration(c, t.writer)
	gData := g.data()
	gData["id"] = c.Id
	t.commit("add-generation", gData)
	return g
}

// SetFeedback adds a feedback to the trace
func (t *Trace) SetFeedback(f *Feedback) {
	t.commit("add-feedback", f)
}

// AddSpan adds a span to the trace
func (t *Trace) AddSpan(c *SpanConfig) *Span {
	s := newSpan(c, t.writer)
	sData := s.data()
	sData["id"] = c.Id
	t.commit("add-span", sData)
	return s
}

// AddRetrieval adds a retrieval to the trace
func (t *Trace) AddRetrieval(c *RetrievalConfig) *Retrieval {
	r := newRetrieval(c, t.writer)
	rData := r.data()
	rData["id"] = c.Id
	t.commit("add-retrieval", rData)
	return r
}

func (t *Trace) Evaluate() *evaluateContainer {
	return newEvaluateContainer(EntityTrace, t.Id(), t.writer)
}

func (t *Trace) AddError(c *ErrorConfig) *Error {
	e := newError(c, t.writer)
	eData := e.data()
	eData["id"] = c.Id
	t.commit("add-error", eData)
	return e
}

func (t *Trace) AddToolCall(c *ToolCallConfig) *ToolCall {
	tc := newToolCall(c, t.writer)
	tcData := tc.data()
	tcData["id"] = c.Id
	t.commit("add-tool-call", tcData)
	return tc
}

func (t *Trace) SetInput(i string) *Trace {
	t.commit("update", map[string]interface{}{
		"input": i,
	})
	return t
}

func (t *Trace) SetOutput(o string) *Trace {
	t.commit("update", map[string]interface{}{
		"output": o,
	})
	return t
}

// AddAttachment adds an attachment to this trace.
func (t *Trace) AddAttachment(attachment interface{}) {
	t.commit("upload-attachment", attachment)
}

func (t *Trace) data() map[string]interface{} {
	bData := t.base.data()
	if t.SessionId != nil {
		bData["sessionId"] = t.SessionId
	}
	return bData
}
