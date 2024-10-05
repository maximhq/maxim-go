package logging

import (
	"time"
)

type baseConfig struct {
	Id     string             `json:"id"`
	SpanId *string            `json:"spanId,omitempty"`
	Name   *string            `json:"name,omitempty"`
	Tags   *map[string]string `json:"tags,omitempty"`
}

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

func (b *base) commit(action string, data interface{}) {
	b.writer.commit(newCommitLog(b.entity, b.id, action, data))
}

func (b *base) Id() string {
	return b.id
}

func (b *base) AddTag(key, value string) {
	if b.tags == nil {
		b.tags = &map[string]string{}
	}
	(*b.tags)[key] = value
	b.commit("update", map[string]interface{}{
		"tags": *b.tags,
	})
}

func (b *base) End() {
	b.endTimestamp = utcNowPtr()
	b.commit("end", map[string]interface{}{
		"endTimestamp": b.endTimestamp,
	})
}

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

func addTag(w *writer, entity Entity, id, key, value string) {
	w.commit(newCommitLog(entity, id, "update", map[string]interface{}{
		"tags": map[string]string{
			key: value,
		},
	}))
}

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

func addFeedback(w *writer, entity Entity, id string, feedback *Feedback) {
	w.commit(newCommitLog(entity, id, "add-feedback", feedback))
}

func end(w *writer, entity Entity, id string) {
	w.commit(newCommitLog(entity, id, "end", map[string]interface{}{
		"endTimestamp": utcNow(),
	}))
}
