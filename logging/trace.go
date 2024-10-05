package logging

type TraceConfig struct {
	Id        string             `json:"id"`
	SpanId    *string            `json:"spanId,omitempty"`
	Name      *string            `json:"name,omitempty"`
	Tags      *map[string]string `json:"tags,omitempty"`
	SessionId *string
}

type Trace struct {
	*eventEmitter
	SessionId *string
}

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

func (t *Trace) AddGeneration(c *GenerationConfig) *Generation {
	g := newGeneration(c, t.writer)
	gData := g.data()
	gData["id"] = c.Id
	t.commit("add-generation", gData)
	return g
}

func (t *Trace) SetFeedback(f *Feedback) {
	t.commit("add-feedback", f)
}

func (t *Trace) AddSpan(c *SpanConfig) *Span {
	s := newSpan(c, t.writer)
	sData := s.data()
	sData["id"] = c.Id
	t.commit("add-span", sData)
	return s
}

func (t *Trace) AddRetrieval(c *RetrievalConfig) *Retrieval {
	r := newRetrieval(c, t.writer)
	rData := r.data()
	rData["id"] = c.Id
	t.commit("add-retrieval", rData)
	return r
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

func (t *Trace) data() map[string]interface{} {
	bData := t.base.data()
	if t.SessionId != nil {
		bData["sessionId"] = t.SessionId
	}
	return bData
}
