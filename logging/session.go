package logging

type SessionConfig struct {
	Id   string             `json:"id"`
	Name *string            `json:"name,omitempty"`
	Tags *map[string]string `json:"tags,omitempty"`
}

type Session struct {
	*base
}

func newSession(c *SessionConfig, w *writer) *Session {
	s := &Session{
		base: newBase(EntitySession, c.Id, &baseConfig{
			Id:   c.Id,
			Name: c.Name,
			Tags: c.Tags,
		}, w),
	}
	sData := s.data()
	sData["id"] = c.Id
	s.commit("create", sData)
	return s
}

func (s *Session) Evaluate() *evaluateContainer {
	return newEvaluateContainer(EntitySession, s.Id(), s.writer)
}

func (s *Session) SetFeedback(f *Feedback) {
	s.commit("add-feedback", f)
}

func (s *Session) AddTrace(c *TraceConfig) *Trace {
	c.SessionId = &s.id
	return newTrace(c, s.writer)
}
