package logging

type eventEmitter struct {
	*base
}

func (e *eventEmitter) AddEvent(id, name string, tags *map[string]string) {
	event := map[string]interface{}{
		"id":   id,
		"name": name,
	}
	if tags != nil {
		event["tags"] = tags
	}
	e.commit("add-event", event)
}
