package logging

type ErrorConfig struct {
	Id      string  `json:"id"`
	Message string  `json:"message"`
	Code    *string `json:"code,omitempty"`
	Type    *string `json:"type,omitempty"`
	Meta    *string `json:"meta,omitempty"`
}

type Error struct {
	*base
}

func newError(c *ErrorConfig, w *writer) *Error {
	return &Error{base: newBase(EntityError, c.Id, &baseConfig{Id: c.Id}, w)}
}

