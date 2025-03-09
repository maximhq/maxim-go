package middlewares

import (
	"context"

	"github.com/google/uuid"
)

type ContextValues struct {
	TraceId        string
	TraceName      string
	GenerationName string
	TraceTags      map[string]string
	GenerationTags map[string]string
}

func parseContextValues(ctx context.Context) *ContextValues {
	var traceId, traceName, generationName string

	if val, ok := ctx.Value("maxim.traceId").(string); ok {
		traceId = val
	}
	if traceId == "" {
		traceId = uuid.NewString()
	}

	if val, ok := ctx.Value("maxim.traceName").(string); ok {
		traceName = val
	}
	if traceName == "" {
		traceName = "Trace"
	}

	if val, ok := ctx.Value("maxim.generationName").(string); ok {
		generationName = val
	}
	if generationName == "" {
		generationName = "LLM Call"
	}

	tags, ok := ctx.Value("maxim.tags").(map[string]string)
	if !ok {
		tags = make(map[string]string)
	}

	generationTags, ok := ctx.Value("maxim.generationTags").(map[string]string)
	if !ok {
		generationTags = make(map[string]string)
	}

	return &ContextValues{
		TraceId:        traceId,
		TraceName:      traceName,
		GenerationName: generationName,
		TraceTags:      tags,
		GenerationTags: generationTags,
	}
}
