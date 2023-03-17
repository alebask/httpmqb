package queue

type Node[T any] struct {
	next  *Node[T]
	value T
}

type Queue[T any] struct {
	head *Node[T]
	tail *Node[T]
}

func New[T any]() *Queue[T] {
	n := &Node[T]{}
	return &Queue[T]{
		head: n, tail: n,
	}
}

func (q *Queue[T]) Push(value T) {
	n := &Node[T]{value: value}
	q.tail.next = n
	q.tail = n
}
func (q *Queue[T]) Pop() (T, bool) {
	var nilValue T
	if q.head.next == nil {
		return nilValue, false
	} else {
		q.head = q.head.next
		value := q.head.value
		q.head.value = nilValue
		return value, true
	}
}
