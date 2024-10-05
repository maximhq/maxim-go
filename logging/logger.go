package logging

type LoggerConfig struct {
	Id                   string
	AutoFlush            *bool
	FlushIntervalSeconds *int
	IsDebug              bool
}

type Logger struct {
	config LoggerConfig
	writer *writer
}

func NewLogger(baseUrl string, apiKey string, c *LoggerConfig) *Logger {
	autoFlush := true
	if c.AutoFlush != nil {
		autoFlush = *c.AutoFlush
	}
	flushIntervalSeconds := 10
	if c.FlushIntervalSeconds != nil {
		flushIntervalSeconds = *c.FlushIntervalSeconds
	}
	return &Logger{
		config: *c,
		writer: newWriter(&writerConfig{
			BaseUrl:              baseUrl,
			ApiKey:               apiKey,
			RepoId:               c.Id,
			AutoFlush:            autoFlush,
			FlushIntervalSeconds: flushIntervalSeconds,
			IsDebug:              c.IsDebug,
		}),
	}
}

func (l *Logger) Id() string {
	return l.config.Id
}

// Session methods

func (l *Logger) Session(c *SessionConfig) *Session {
	return newSession(c, l.writer)
}

func (l *Logger) AddTagToSession(sessionId, key, value string) {
	addTag(l.writer, EntitySession, sessionId, key, value)
}

func (l *Logger) SessionEnd(sessionId string) {
	end(l.writer, EntitySession, sessionId)
}

func (l *Logger) SessionAddTrace(sessionId string, c *TraceConfig) *Trace {
	c.SessionId = &sessionId
	return newTrace(c, l.writer)
}

// Trace methods

func (l *Logger) Trace(c *TraceConfig) *Trace {
	return newTrace(c, l.writer)
}

func (l *Logger) AddGenerationToTrace(traceId string, c *GenerationConfig) *Generation {
	g := newGeneration(c, l.writer)
	gData := g.data()
	gData["id"] = c.Id
	l.writer.commit(newCommitLog(EntityTrace, traceId, "add-generation", gData))
	return g
}

func (l *Logger) AddRetrievalToTrace(traceId string, c *RetrievalConfig) *Retrieval {
	r := newRetrieval(c, l.writer)
	rData := r.data()
	rData["id"] = c.Id
	l.writer.commit(newCommitLog(EntityTrace, traceId, "add-retrieval", rData))
	return r
}

func (l *Logger) SetTraceInput(traceId, input string) {
	l.writer.commit(newCommitLog(EntityTrace, traceId, "update", map[string]interface{}{
		"input": input,
	}))
}

func (l *Logger) SetTraceOutput(traceId, output string) {
	l.writer.commit(newCommitLog(EntityTrace, traceId, "update", map[string]interface{}{
		"output": output,
	}))
}

func (l *Logger) AddSpanToTrace(traceId string, c *SpanConfig) *Span {
	s := newSpan(c, l.writer)
	sData := s.data()
	sData["id"] = c.Id
	l.writer.commit(newCommitLog(EntityTrace, traceId, "add-span", sData))
	return s
}

func (l *Logger) AddFeedbackToTrace(traceId string, f *Feedback) {
	addFeedback(l.writer, EntityTrace, traceId, f)
}

func (l *Logger) AddTagToTrace(traceId, key, value string) {
	addTag(l.writer, EntityTrace, traceId, key, value)
}

func (l *Logger) AddEventToTrace(traceId, eventId, event string, tags *map[string]string) {
	addEvent(l.writer, EntityTrace, traceId, eventId, event, tags)
}

func (l *Logger) EndTrace(traceId string) {
	end(l.writer, EntityTrace, traceId)
}

// Generation methods

func (l *Logger) SetModelToGeneration(gId, model string) {
	l.writer.commit(newCommitLog(EntityGeneration, gId, "update", map[string]interface{}{
		"model": model,
	}))
}

func (l *Logger) AddMessageToGeneration(gId string, message CompletionRequest) {
	l.writer.commit(newCommitLog(EntityGeneration, gId, "update", map[string]interface{}{
		"messages": []CompletionRequest{message},
	}))
}

func (l *Logger) SetModelParametersForGeneration(gId string, params map[string]interface{}) {
	l.writer.commit(newCommitLog(EntityGeneration, gId, "update", map[string]interface{}{
		"modelParameters": params,
	}))
}

func (l *Logger) AddTagToGeneration(gId, key, value string) {
	addTag(l.writer, EntityGeneration, gId, key, value)
}

func (l *Logger) AddEventToGeneration(gId, eventId, event string, tags *map[string]string) {
	addEvent(l.writer, EntityGeneration, gId, eventId, event, tags)
}

func (l *Logger) AddResultToGeneration(gId string, result interface{}) {
	l.writer.commit(newCommitLog(EntityGeneration, gId, "result", map[string]interface{}{
		"result": result,
	}))
	end(l.writer, EntityGeneration, gId)
}

func (l *Logger) SetGenerationError(gId string, error *GenerationError) {
	l.writer.commit(newCommitLog(EntityGeneration, gId, "error", map[string]interface{}{
		"error": error,
	}))
}

func (l *Logger) EndGeneration(gId string) {
	end(l.writer, EntityGeneration, gId)
}

// Span methods

func (l *Logger) AddGenerationToSpan(sId string, c *GenerationConfig) *Generation {
	g := newGeneration(c, l.writer)
	gData := g.data()
	gData["id"] = c.Id
	l.writer.commit(newCommitLog(EntitySpan, sId, "add-generation", gData))
	return g
}

func (l *Logger) AddRetrievalToSpan(sId string, c *RetrievalConfig) *Retrieval {
	r := newRetrieval(c, l.writer)
	rData := r.data()
	rData["id"] = c.Id
	l.writer.commit(newCommitLog(EntitySpan, sId, "add-retrieval", rData))
	return r
}

func (l *Logger) AddSubSpanToSpan(sId string, c *SpanConfig) *Span {
	s := newSpan(c, l.writer)
	sData := s.data()
	sData["id"] = c.Id
	l.writer.commit(newCommitLog(EntitySpan, sId, "add-span", sData))
	return s
}

func (l *Logger) AddTagToSpan(spanId, key, value string) {
	addTag(l.writer, EntitySpan, spanId, key, value)
}

func (l *Logger) AddEventToSpan(spanId, eventId, event string, tags *map[string]string) {
	addEvent(l.writer, EntitySpan, spanId, eventId, event, tags)
}

func (l *Logger) EndSpan(spanId string) {
	end(l.writer, EntitySpan, spanId)
}

// Retrieval methods

func (l *Logger) EndRetrieval(rId string) {
	end(l.writer, EntityRetrieval, rId)
}

func (l *Logger) SetRetrievalInput(rId, input string) {
	l.writer.commit(newCommitLog(EntityRetrieval, rId, "update", map[string]interface{}{
		"input": input,
	}))
}

func (l *Logger) SetRetrievalOutput(rId string, output []string) {
	l.writer.commit(newCommitLog(EntityRetrieval, rId, "end", map[string]interface{}{
		"docs":         output,
		"endTimestamp": utcNow(),
	}))
}

func (l *Logger) AddTagToRetrieval(rId, key, value string) {
	addTag(l.writer, EntityRetrieval, rId, key, value)
}

func (l *Logger) Cleanup() {
	l.writer.cleanup()
}
