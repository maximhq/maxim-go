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
)

// CommitLog represents a log entry
type CommitLog struct {
	entity   Entity
	entityID string
	action   string
	data     interface{}
}

// NewCommitLog creates a new CommitLog instance
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
