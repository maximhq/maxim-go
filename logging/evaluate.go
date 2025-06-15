package logging

type evaluateContainer struct {
	entity Entity
	id     string
	writer *writer
}

func newEvaluateContainer(entity Entity, id string, writer *writer) *evaluateContainer {
	return &evaluateContainer{
		entity: entity,
		id:     id,
		writer: writer,
	}
}

func (ec *evaluateContainer) WithVariablesAndEvaluators(variables map[string]string, evaluators []string) error {
	if len(evaluators) == 0 {
		return ErrNoEvaluators
	}
	// Make sure there are no duplicates in the evaluators
	evaluators = removeDuplicateStrings(evaluators)
	ec.writer.commit(newCommitLog(ec.entity, ec.id, "evaluate", map[string]interface{}{
		"with":       "variables",
		"variables":  variables,
		"evaluators": evaluators,
		"timestamp":  utcNow(),
	}))
	return nil
}

func (ec *evaluateContainer) WithEvaluators(evaluators []string) error {
	if len(evaluators) == 0 {
		return ErrNoEvaluators
	}
	evaluators = removeDuplicateStrings(evaluators)
	ec.writer.commit(newCommitLog(ec.entity, ec.id, "evaluate", map[string]interface{}{
		"with":       "evaluators",
		"evaluators": evaluators,
		"timestamp":  utcNow(),
	}))
	return nil
}