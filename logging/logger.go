package logging

import (
	"os"

	"github.com/maximhq/maxim-go/schemas"
)

// LoggerConfig contains configuration parameters for the Logger
type LoggerConfig struct {
	Id                   string // Log repository id. You can get it from the Maxim dashboard
	AutoFlush            *bool  // Whether to automatically flush logs
	FlushIntervalSeconds *int   // Interval in seconds between automatic flushes
	IsDebug              bool   // Whether to enable debug logging
}

// Logger provides methods for logging various events and entities
type Logger struct {
	config LoggerConfig
	writer *writer
}

// NewLogger creates a new Logger instance with the provided configuration
func NewLogger(baseUrl string, apiKey string, c *LoggerConfig) *Logger {
	autoFlush := true
	if c.AutoFlush != nil {
		autoFlush = *c.AutoFlush
	}
	flushIntervalSeconds := 10
	if c.FlushIntervalSeconds != nil {
		flushIntervalSeconds = *c.FlushIntervalSeconds
	}
	if !c.IsDebug && os.Getenv("MAXIM_LOG_IS_DEBUG") != "" {
		c.IsDebug = os.Getenv("MAXIM_LOG_IS_DEBUG") == "true" || os.Getenv("MAXIM_LOG_IS_DEBUG") == "1"
	}

	if c.Id == "" {
		// We will check if its present in the env
		// First case is for backward compatibility
		if os.Getenv("MAXIM_LOG_ID") != "" {
			c.Id = os.Getenv("MAXIM_LOG_ID")
		} else if os.Getenv("MAXIM_LOG_REPO_ID") != "" {
			c.Id = os.Getenv("MAXIM_LOG_REPO_ID")
		} else {
			panic("MAXIM_LOG_REPO_ID environment variable is not set. Either set log repo id it in the LoggerConfig or set environment variable MAXIM_LOG_REPO_ID")
		}
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

// Id returns the Log repository id
func (l *Logger) Id() string {
	return l.config.Id
}

// Session methods

// Session creates a new session with the provided configuration
// A Session represents a group of related traces. It can be used to track the entire interaction of a
// user with your application. For example, a session might represent a conversation with a chatbot,
// including multiple back-and-forth messages (each represented as a Trace), or a single user journey
// through your application. Sessions allow for organizing traces in a hierarchical structure and can
// help analyze patterns and metrics across related user interactions.
func (l *Logger) Session(c *SessionConfig) *Session {
	return newSession(c, l.writer)
}

// AddTagToSession adds a key-value tag to the specified session
func (l *Logger) AddTagToSession(sessionId, key, value string) {
	addTag(l.writer, EntitySession, sessionId, key, value)
}

// SessionEnd marks the specified session as ended
// Deprecated: please use EndSession instead, this will be removed in a future version
func (l *Logger) SessionEnd(sessionId string) {
	end(l.writer, EntitySession, sessionId)
}

// EndSession marks the specified session as ended
func (l *Logger) EndSession(sessionId string) {
	end(l.writer, EntitySession, sessionId)
}

// AddTraceToSession adds a trace to the specified session and returns the created trace
// A Trace represents a single unit of work in your application, such as processing a specific user request
// or performing a targeted task. Traces can be associated with Sessions and can contain their own
// sub-entities like Generations, Retrievals, and Spans. Traces are useful for detailed analysis of
// specific operations, allowing you to track inputs, outputs, and all related processing steps.
// Use traces to monitor individual API calls, user messages, or other discrete operations that you
// want to analyze for performance, quality, or debugging purposes.
func (l *Logger) AddTraceToSession(sessionId string, c *TraceConfig) *Trace {
	c.SessionId = &sessionId
	return newTrace(c, l.writer)
}

// Trace methods

// Trace creates a new trace with the provided configuration
// A Trace represents a single unit of work in your application. It can be used to
// track and analyze specific operations such as processing a user request,
// executing a function, or any discrete task. Traces can contain sub-entities
// like Generations, Retrievals, and Spans, allowing for detailed performance
// monitoring and debugging.
//
// Use traces to analyze inputs, outputs, latency, and the execution flow of
// individual operations within your application. They provide valuable insights
// for performance optimization, quality assessment, and debugging issues in
// production environments.
func (l *Logger) Trace(c *TraceConfig) *Trace {
	return newTrace(c, l.writer)
}

// AddGenerationToTrace adds a generation entity to the specified trace
// A Generation represents an interaction with a language model (LLM).
// It captures all details of the LLM call including input messages/prompts,
// model parameters (like temperature, top_p), the model used, and the response.
//
// Generations are useful for:
// - Tracking model performance and behavior
// - Analyzing costs associated with model calls
// - Debugging model outputs and fine-tuning prompts
// - Comparing different model configurations
//
// You can attach a Generation to a Trace to see it in context of the broader
// application flow, or analyze it independently to improve your LLM interactions.
func (l *Logger) AddGenerationToTrace(traceId string, c *GenerationConfig) *Generation {
	g := newGeneration(c, l.writer)
	gData := g.data()
	gData["id"] = c.Id
	l.writer.commit(newCommitLog(EntityTrace, traceId, "add-generation", gData))
	return g
}

// AddRetrievalToTrace adds a retrieval entity to the specified trace
// A Retrieval captures information about data retrieval operations from your knowledge
// base or vector database. It tracks details like the query used, documents retrieved,
// time taken, and any associated metadata.
//
// Retrievals help you:
// - Monitor the performance of your retrieval system
// - Identify common queries and their results
// - Analyze the relevance of returned documents
// - Debug issues with retrieval quality or latency
//
// By tracking Retrievals alongside other entities like Generations, you can
// understand how effectively your system is utilizing context information and
// improve the overall quality of retrieval-augmented applications.
func (l *Logger) AddRetrievalToTrace(traceId string, c *RetrievalConfig) *Retrieval {
	r := newRetrieval(c, l.writer)
	rData := r.data()
	rData["id"] = c.Id
	l.writer.commit(newCommitLog(EntityTrace, traceId, "add-retrieval", rData))
	return r
}

// SetTraceInput sets the input value for the specified trace
func (l *Logger) SetTraceInput(traceId, input string) {
	l.writer.commit(newCommitLog(EntityTrace, traceId, "update", map[string]interface{}{
		"input": input,
	}))
}

// SetTraceOutput sets the output value for the specified trace
func (l *Logger) SetTraceOutput(traceId, output string) {
	l.writer.commit(newCommitLog(EntityTrace, traceId, "update", map[string]interface{}{
		"output": output,
	}))
}

// AddSpanToTrace adds a span entity to the specified trace
// A Span represents a period of time during which a specific operation or task is performed.
// It is typically used to track the duration and details of operations within your application,
// making it useful for performance monitoring and debugging.
//
// Spans can:
// - Capture start and end times of operations
// - Track nested operations through parent-child relationships
// - Include custom attributes and events
// - Represent operations like database queries, API calls, or computational tasks
//
// By analyzing Spans, you can identify performance bottlenecks, understand execution flows,
// and debug issues by examining the timing and relationships between different parts of your code.
func (l *Logger) AddSpanToTrace(traceId string, c *SpanConfig) *Span {
	s := newSpan(c, l.writer)
	sData := s.data()
	sData["id"] = c.Id
	l.writer.commit(newCommitLog(EntityTrace, traceId, "add-span", sData))
	return s
}

// AddFeedbackToTrace adds feedback to the specified trace
func (l *Logger) AddFeedbackToTrace(traceId string, f *Feedback) {
	addFeedback(l.writer, EntityTrace, traceId, f)
}

// AddTagToTrace adds a key-value tag to the specified trace
func (l *Logger) AddTagToTrace(traceId, key, value string) {
	addTag(l.writer, EntityTrace, traceId, key, value)
}

// AddMetricToTrace adds a numeric metric to the specified trace
func (l *Logger) AddMetricToTrace(traceId, key string, value float64) {
	addMetric(l.writer, EntityTrace, traceId, key, value)
}

// AddEventToTrace adds an event to the specified trace with optional tags
func (l *Logger) AddEventToTrace(traceId, eventId, event string, tags *map[string]string) {
	addEvent(l.writer, EntityTrace, traceId, eventId, event, tags)
}

// TraceAddAttachment adds an attachment to the specified trace by ID.
// The attachment can be *FileAttachment, *FileDataAttachment, *UrlAttachment, or map[string]interface{}.
func (l *Logger) TraceAddAttachment(traceId string, attachment interface{}) {
	l.writer.commit(newCommitLog(EntityTrace, traceId, "upload-attachment", attachment))
}

// EndTrace marks the specified trace as ended
func (l *Logger) EndTrace(traceId string) {
	end(l.writer, EntityTrace, traceId)
}

// Generation methods

// SetModelToGeneration sets the model name for the specified generation
func (l *Logger) SetModelToGeneration(gId, model string) {
	l.writer.commit(newCommitLog(EntityGeneration, gId, "update", map[string]interface{}{
		"model": model,
	}))
}

// AddMessageToGeneration adds a message to the specified generation
func (l *Logger) AddMessageToGeneration(gId string, message schemas.CompletionRequest) {
	l.writer.commit(newCommitLog(EntityGeneration, gId, "update", map[string]interface{}{
		"messages": []schemas.CompletionRequest{message},
	}))
}

// SetModelParametersForGeneration sets model parameters for the specified generation
func (l *Logger) SetModelParametersForGeneration(gId string, params map[string]interface{}) {
	l.writer.commit(newCommitLog(EntityGeneration, gId, "update", map[string]interface{}{
		"modelParameters": params,
	}))
}

// AddTagToGeneration adds a key-value tag to the specified generation
func (l *Logger) AddTagToGeneration(gId, key, value string) {
	addTag(l.writer, EntityGeneration, gId, key, value)
}

// AddMetricToGeneration adds a numeric metric to the specified generation
func (l *Logger) AddMetricToGeneration(gId, key string, value float64) {
	addMetric(l.writer, EntityGeneration, gId, key, value)
}

// AddEventToGeneration adds an event to the specified generation with optional tags
func (l *Logger) AddEventToGeneration(gId, eventId, event string, tags *map[string]string) {
	addEvent(l.writer, EntityGeneration, gId, eventId, event, tags)
}

// AddResultToGeneration adds a result to the specified generation and marks it as ended.
func (l *Logger) AddResultToGeneration(gId string, result interface{}) {
	var payload interface{} = result
	if img, ok := isImageGenerationResult(result); ok {
		commitImageGenerationAttachments(l, gId, img)
		payload = imageGenerationResultToMaximLLMResult(img)
	}
	l.writer.commit(newCommitLog(EntityGeneration, gId, "result", map[string]interface{}{
		"result": payload,
	}))
	end(l.writer, EntityGeneration, gId)
}

// SetGenerationError sets an error for the specified generation
func (l *Logger) SetGenerationError(gId string, error *schemas.GenerationError) {
	l.writer.commit(newCommitLog(EntityGeneration, gId, "result", map[string]interface{}{
		"result": map[string]interface{}{
			"error": error,
		},
	}))
	end(l.writer, EntityGeneration, gId)
}

// EndGeneration marks the specified generation as ended
func (l *Logger) EndGeneration(gId string) {
	end(l.writer, EntityGeneration, gId)
}

// GenerationAddAttachment adds an attachment to the specified generation by ID.
// The attachment can be *FileAttachment, *FileDataAttachment, *UrlAttachment, or map[string]interface{}.
func (l *Logger) GenerationAddAttachment(generationId string, attachment interface{}) {
	l.writer.commit(newCommitLog(EntityGeneration, generationId, "upload-attachment", attachment))
}

// Span methods

// AddGenerationToSpan adds a generation entity to the specified span
func (l *Logger) AddGenerationToSpan(sId string, c *GenerationConfig) *Generation {
	g := newGeneration(c, l.writer)
	gData := g.data()
	gData["id"] = c.Id
	l.writer.commit(newCommitLog(EntitySpan, sId, "add-generation", gData))
	return g
}

// AddRetrievalToSpan adds a retrieval entity to the specified span
func (l *Logger) AddRetrievalToSpan(sId string, c *RetrievalConfig) *Retrieval {
	r := newRetrieval(c, l.writer)
	rData := r.data()
	rData["id"] = c.Id
	l.writer.commit(newCommitLog(EntitySpan, sId, "add-retrieval", rData))
	return r
}

// AddSubSpanToSpan adds a sub-span to the specified span
func (l *Logger) AddSubSpanToSpan(sId string, c *SpanConfig) *Span {
	s := newSpan(c, l.writer)
	sData := s.data()
	sData["id"] = c.Id
	l.writer.commit(newCommitLog(EntitySpan, sId, "add-span", sData))
	return s
}

// AddTagToSpan adds a key-value tag to the specified span
func (l *Logger) AddTagToSpan(spanId, key, value string) {
	addTag(l.writer, EntitySpan, spanId, key, value)
}

// AddEventToSpan adds an event to the specified span with optional tags
func (l *Logger) AddEventToSpan(spanId, eventId, event string, tags *map[string]string) {
	addEvent(l.writer, EntitySpan, spanId, eventId, event, tags)
}

// SpanAddAttachment adds an attachment to the specified span by ID.
// The attachment can be *FileAttachment, *FileDataAttachment, *UrlAttachment, or map[string]interface{}.
func (l *Logger) SpanAddAttachment(spanId string, attachment interface{}) {
	l.writer.commit(newCommitLog(EntitySpan, spanId, "upload-attachment", attachment))
}

// EndSpan marks the specified span as ended
func (l *Logger) EndSpan(spanId string) {
	end(l.writer, EntitySpan, spanId)
}

// Retrieval methods

// EndRetrieval marks the specified retrieval as ended
func (l *Logger) EndRetrieval(rId string) {
	end(l.writer, EntityRetrieval, rId)
}

// SetRetrievalInput sets the input value for the specified retrieval
func (l *Logger) SetRetrievalInput(rId, input string) {
	l.writer.commit(newCommitLog(EntityRetrieval, rId, "update", map[string]interface{}{
		"input": input,
	}))
}

// SetRetrievalOutput sets the output documents for the specified retrieval and marks it as ended
func (l *Logger) SetRetrievalOutput(rId string, output []string) {
	l.writer.commit(newCommitLog(EntityRetrieval, rId, "end", map[string]interface{}{
		"docs":         output,
		"endTimestamp": utcNow(),
	}))
}

// AddTagToRetrieval adds a key-value tag to the specified retrieval
func (l *Logger) AddTagToRetrieval(rId, key, value string) {
	addTag(l.writer, EntityRetrieval, rId, key, value)
}

// Flush flushes all pending logs to the destination and cleans up resources
func (l *Logger) Flush() {
	l.writer.flush()
	l.writer.cleanup()
}
