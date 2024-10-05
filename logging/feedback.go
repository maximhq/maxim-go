package logging

type Feedback struct {
	Score   int8    `json:"score"`
	Comment *string `json:"comment,omitempty"`
}
