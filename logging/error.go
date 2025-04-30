package logging

type ErrorConfig struct {
	Id        string
	SpanId    *string
	Name      *string
	Tags      *map[string]string
	message   string
	code      string
	errorType string
	metadata map[string]interface{}
}

type Error struct {
	*base
}

func newError(c *ErrorConfig, w *writer) *Error {
	return &Error{
		base: newBase(EntityError, c.Id, &baseConfig{
			Id:     c.Id,
			SpanId: c.SpanId,
			Name:   c.Name,
			Tags:   c.Tags,
			Metadata: c.metadata,
		}, w),
	}
}





