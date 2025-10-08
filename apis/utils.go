package apis

type MaximError struct {
	Message string `json:"message"`
}

func newMaximError(err error) *MaximError {
	return &MaximError{Message: err.Error()}
}
