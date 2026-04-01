package logging

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

const (
	ProviderOpenAI    = "openai"
	ProviderAzure     = "azure"
	ProviderAnthropic = "anthropic"
	ProviderBedrock   = "aws"
	ProviderGemini    = "google"
	ProviderVertex    = "vertex"
)

// baseConfig is the configuration for a base entity.
type baseConfig struct {
	Id       string                 `json:"id"`
	SpanId   *string                `json:"spanId,omitempty"`
	Name     *string                `json:"name,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Tags     *map[string]string     `json:"tags,omitempty"`
}

// base is the base entity for all entities.
type base struct {
	entity         Entity
	id             string
	name           *string
	spanId         *string
	tags           *map[string]string
	startTimestamp time.Time
	endTimestamp   *time.Time
	writer         *writer
}

// sanitizeMetadata sanitizes the metadata for a base entity.
func sanitizeMetadata(metadata map[string]interface{}) map[string]string {
	sanitizedMetadata := make(map[string]string)
	for key, value := range metadata {
		switch v := value.(type) {
		case string, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, bool:
			sanitizedMetadata[key] = fmt.Sprintf("%v", v)
		default:
			jsonValue, err := json.Marshal(v)
			if err == nil {
				sanitizedMetadata[key] = string(jsonValue)
			}
		}
	}
	return sanitizedMetadata
}

// newBase creates a new base entity.
func newBase(e Entity, id string, c *baseConfig, w *writer) *base {
	return &base{
		entity:         e,
		id:             id,
		name:           c.Name,
		spanId:         c.SpanId,
		tags:           c.Tags,
		startTimestamp: utcNow(),
		writer:         w,
	}
}

// commit commits a new commit log to the writer.
func (b *base) commit(action string, data interface{}) {
	b.writer.commit(newCommitLog(b.entity, b.id, action, data))
}

// Id returns the id of the base entity.
func (b *base) Id() string {
	return b.id
}

// AddTag adds a tag to the base entity.
func (b *base) AddTag(key, value string) {
	if b.tags == nil {
		b.tags = &map[string]string{}
	}
	(*b.tags)[key] = value
	b.commit("update", map[string]interface{}{
		"tags": *b.tags,
	})
}

// AddMetadata adds metadata to the base entity.
func (b *base) AddMetadata(metadata map[string]interface{}) {
	sanitizedMetadata := sanitizeMetadata(metadata)
	b.commit("update", map[string]interface{}{
		"metadata": sanitizedMetadata,
	})
}

// AddMetric adds a numeric metric to the base entity.
// Uses the same flow as Python SDK: commit("update", {"metrics": {name: value}}).
func (b *base) AddMetric(key string, value float64) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return
	}
	b.commit("update", map[string]interface{}{
		"metrics": map[string]float64{key: value},
	})
}

// End ends the base entity.
func (b *base) End() {
	b.endTimestamp = utcNowPtr()
	b.commit("end", map[string]interface{}{
		"endTimestamp": b.endTimestamp,
	})
}

// data returns the data of the base entity.
func (b *base) data() map[string]interface{} {
	data := map[string]interface{}{
		"startTimestamp": b.startTimestamp,
	}
	if b.name != nil {
		data["name"] = *b.name
	}
	if b.spanId != nil {
		data["spanId"] = *b.spanId
	}
	if b.tags != nil {
		data["tags"] = *b.tags
	}
	if b.endTimestamp != nil {
		data["endTimestamp"] = *b.endTimestamp
	}
	return data
}

// Static methods

// addTag adds a tag to the base entity.
func addTag(w *writer, entity Entity, id, key, value string) {
	w.commit(newCommitLog(entity, id, "update", map[string]interface{}{
		"tags": map[string]string{
			key: value,
		},
	}))
}

// addMetric adds a metric to an entity by ID.
func addMetric(w *writer, entity Entity, id, key string, value float64) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return
	}
	w.commit(newCommitLog(entity, id, "update", map[string]interface{}{
		"metrics": map[string]float64{key: value},
	}))
}

// addEvent adds an event to the base entity.
func addEvent(w *writer, entity Entity, entityId, eId, event string, tags *map[string]string) {
	eventData := map[string]interface{}{
		"id":        eId,
		"name":      event,
		"timestamp": utcNow(),
	}
	if tags != nil {
		eventData["tags"] = tags
	}
	w.commit(newCommitLog(entity, entityId, "add-event", eventData))
}

// addFeedback adds feedback to the base entity.
func addFeedback(w *writer, entity Entity, id string, feedback *Feedback) {
	w.commit(newCommitLog(entity, id, "add-feedback", feedback))
}

// end ends the base entity.
func end(w *writer, entity Entity, id string) {
	w.commit(newCommitLog(entity, id, "end", map[string]interface{}{
		"endTimestamp": utcNow(),
	}))
}
