package logging

import "errors"

var ErrNoEvaluators = errors.New("at least one evaluator is required")
var ErrNoVariables = errors.New("at least one variable is required")
var ErrInvalidAttachmentType = errors.New("invalid attachment type")
