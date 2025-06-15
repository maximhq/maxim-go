package logging

import (
	"encoding/json"
	"fmt"
)

// Entity represents the type of entity in the commit log
type Entity string

const (
	EntitySession    Entity = "session"
	EntityTrace      Entity = "trace"
	EntitySpan       Entity = "span"
	EntityGeneration Entity = "generation"
	EntityFeedback   Entity = "feedback"
	EntityRetrieval  Entity = "retrieval"
	EntityToolCall   Entity = "tool_call"
	EntityError      Entity = "error"
)

// CommitLog represents a log entry
type CommitLog struct {
	entity   Entity
	entityID string
	action   string
	data     interface{}
}

// newCommitLog creates a new CommitLog instance (internal use)
func newCommitLog(entity Entity, entityID, action string, data interface{}) *CommitLog {
	return &CommitLog{
		entity:   entity,
		entityID: entityID,
		action:   action,
		data:     data,
	}
}

// Serialize converts the CommitLog to a string representation
func (cl *CommitLog) Serialize() string {
	var dataJSON []byte
	var err error

	if cl.data == nil {
		dataJSON = []byte("{}")
	} else {
		dataJSON, err = json.Marshal(cl.data)
		if err != nil {
			dataJSON = []byte("{}")
		}
	}
	return fmt.Sprintf("%s{id=%s,action=%s,data=%s}", cl.entity, cl.entityID, cl.action, string(dataJSON))
}

// GetEntity returns the entity type of the commit log
func (cl *CommitLog) GetEntity() Entity {
	return cl.entity
}

// GetEntityID returns the entity ID of the commit log
func (cl *CommitLog) GetEntityID() string {
	return cl.entityID
}

// GetAction returns the action of the commit log
func (cl *CommitLog) GetAction() string {
	return cl.action
}

// GetData returns the data of the commit log
func (cl *CommitLog) GetData() interface{} {
	return cl.data
}
