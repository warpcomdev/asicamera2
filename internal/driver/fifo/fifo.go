package buffer

type Fifo[T any] struct {
	items []T
	size  int // size of the ring
	head  int // position of next item to write
	tail  int // position of next item to read. tail < 0 =>  empty FIFO
}

// New Fifo of the given size
func New[T any](size int) *Fifo[T] {
	return &Fifo[T]{
		items: make([]T, size),
		size:  size,
		head:  0,
		tail:  -1,
	}
}

// Push item in the queue, return evicted one if any
func (f *Fifo[T]) Push(item T) (old T, evicted bool) {
	old, f.items[f.head] = f.items[f.head], item
	switch {
	case f.tail < 0:
		f.tail = f.head // no longer empty
	case f.tail == f.head:
		evicted = true // ring was full
	}
	f.head = (f.head + 1) % f.size
	if evicted {
		f.tail = f.head // drag tail if ring was full
	}
	return
}

// Pop item from the queue
func (f *Fifo[T]) Pop() (item T, ok bool) {
	if f.tail < 0 { // empty ring
		return item, false
	}
	tail := f.items[f.tail]
	f.tail = (f.tail + 1) % f.size
	if f.tail == f.head { // the ring has become empty
		f.tail = -1
	}
	return tail, true
}

// Len of the queue
func (f *Fifo[T]) Len() int {
	if f.tail < 0 {
		return 0
	}
	if f.head > f.tail {
		return f.head - f.tail
	}
	return f.head + f.size - f.tail
}
