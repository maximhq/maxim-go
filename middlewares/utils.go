package middlewares

import (
	"context"

	"github.com/google/uuid"
)

// contextKey is a private type for context keys to avoid collisions with other packages.
type contextKey string

// Context keys for tags and metrics (used by E2E tests and middleware).
const (
	ContextKeyLogger            contextKey = "maxim.logger"
	ContextKeyProvider          contextKey = "maxim.provider"
	ContextKeyTraceId           contextKey = "maxim.traceId"
	ContextKeyTraceName         contextKey = "maxim.traceName"
	ContextKeyGenerationName    contextKey = "maxim.generationName"
	ContextKeyTraceMetrics      contextKey = "maxim.traceMetrics"
	ContextKeyGenerationMetrics contextKey = "maxim.generationMetrics"
	ContextKeyTraceTags         contextKey = "maxim.tags"
	ContextKeyGenerationTags    contextKey = "maxim.generationTags"
)

type ContextValues struct {
	TraceId           string
	TraceName         string
	GenerationName    string
	TraceTags         map[string]string
	GenerationTags    map[string]string
	TraceMetrics      map[string]float64
	GenerationMetrics map[string]float64
}

// userProvidedTraceContext returns true if the user explicitly set maxim.traceId in context.
// When true, the user owns the trace lifecycle and the middleware should not call trace.End().
func userProvidedTraceContext(ctx context.Context) bool {
	if val, ok := ctx.Value(ContextKeyTraceId).(string); ok && val != "" {
		return true
	}
	return false
}

func parseContextValues(ctx context.Context) *ContextValues {
	var traceId, traceName, generationName string

	if val, ok := ctx.Value(ContextKeyTraceId).(string); ok {
		traceId = val
	}
	if traceId == "" {
		traceId = uuid.NewString()
	}

	if val, ok := ctx.Value(ContextKeyTraceName).(string); ok {
		traceName = val
	}
	if traceName == "" {
		traceName = "Trace"
	}

	if val, ok := ctx.Value(ContextKeyGenerationName).(string); ok {
		generationName = val
	}
	if generationName == "" {
		generationName = "LLM Call"
	}

	tags, ok := ctx.Value(ContextKeyTraceTags).(map[string]string)
	if !ok {
		tags = make(map[string]string)
	}

	generationTags, ok := ctx.Value(ContextKeyGenerationTags).(map[string]string)
	if !ok {
		generationTags = make(map[string]string)
	}

	traceMetrics, ok := ctx.Value(ContextKeyTraceMetrics).(map[string]float64)
	if !ok {
		traceMetrics = make(map[string]float64)
	}

	generationMetrics, ok := ctx.Value(ContextKeyGenerationMetrics).(map[string]float64)
	if !ok {
		generationMetrics = make(map[string]float64)
	}

	return &ContextValues{
		TraceId:           traceId,
		TraceName:         traceName,
		GenerationName:    generationName,
		TraceTags:         tags,
		GenerationTags:    generationTags,
		TraceMetrics:      traceMetrics,
		GenerationMetrics: generationMetrics,
	}
}
