package logging

type RetrievalConfig struct {
	Id     string             `json:"id"`
	SpanId *string            `json:"spanId,omitempty"`
	Name   *string            `json:"name,omitempty"`
	Tags   *map[string]string `json:"tags,omitempty"`
}

type Retrieval struct {
	*base
}

func newRetrieval(c *RetrievalConfig, w *writer) *Retrieval {
	return &Retrieval{
		base: newBase(EntityRetrieval, c.Id, &baseConfig{
			Id:     c.Id,
			SpanId: c.SpanId,
			Name:   c.Name,
			Tags:   c.Tags,
		}, w),
	}
}

func (r *Retrieval) SetInput(query string) {
	r.commit("update", map[string]interface{}{
		"input": query,
	})
}

func (r *Retrieval) SetOutput(docs []string) {
	r.commit("end", map[string]interface{}{
		"docs":         docs,
		"endTimestamp": utcNow(),
	})
}
