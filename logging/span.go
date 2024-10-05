package logging

type SpanConfig struct {
	Id     string             `json:"id"`
	SpanId *string            `json:"spanId,omitempty"`
	Name   *string            `json:"name,omitempty"`
	Tags   *map[string]string `json:"tags,omitempty"`
}

type Span struct {
	*eventEmitter
}

func newSpan(c *SpanConfig, w *writer) *Span {
	return &Span{
		eventEmitter: &eventEmitter{
			base: newBase(EntitySpan, c.Id, &baseConfig{
				Id:     c.Id,
				SpanId: c.SpanId,
				Name:   c.Name,
				Tags:   c.Tags,
			}, w),
		},
	}
}

func (s *Span) AddGeneration(c *GenerationConfig) *Generation {
	g := newGeneration(c, s.writer)
	genData := g.data()
	genData["id"] = c.Id
	s.commit("add-generation", genData)
	return g
}

func (s *Span) AddSubSpan(c *SpanConfig) *Span {
	subSpan := newSpan(c, s.writer)
	spanData := subSpan.data()
	spanData["id"] = c.Id
	s.commit("add-span", spanData)
	return subSpan
}

func (s *Span) AddRetrieval(c *RetrievalConfig) *Retrieval {
	r := newRetrieval(c, s.writer)
	rData := r.data()
	rData["id"] = c.Id
	s.commit("add-retrieval", rData)
	return r
}
