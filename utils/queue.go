package utils

type Queue[T any] struct {
	storage []T
}

// NewQueue creates a new Queue.
func NewQueue[T any]() *Queue[T] {
	return &Queue[T]{}
}

// Enqueue adds an element to the end of the queue.
func (q *Queue[T]) Enqueue(ele T) {
	q.storage = append(q.storage, ele)
}

// Dequeue removes and returns the element at the front of the queue.
func (q *Queue[T]) Dequeue() (T, bool) {
	var zero T
	if len(q.storage) == 0 {
		return zero, false
	}
	element := q.storage[0]
	q.storage = q.storage[1:]
	return element, true
}

func (q *Queue[T]) DequeueAll() []T {
	elements := q.storage
	q.storage = nil
	return elements
}
