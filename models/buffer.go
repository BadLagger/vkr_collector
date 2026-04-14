package models

import (
	"sync"
)

type DataPoint struct {
	Timestamp int64              `json:"timestamp"`
	Values    map[string]float64 `json:"values"`
}

type CircularBuffer struct {
	data  []DataPoint
	size  int
	head  int
	count int
	mu    sync.RWMutex
}

func NewCircularBuffer(size int) *CircularBuffer {
	return &CircularBuffer{
		data:  make([]DataPoint, size),
		size:  size,
		head:  0,
		count: 0,
	}
}

func (cb *CircularBuffer) Push(point DataPoint) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.data[cb.head] = point
	cb.head = (cb.head + 1) % cb.size
	if cb.count < cb.size {
		cb.count++
	}
}

func (cb *CircularBuffer) GetAll() []DataPoint {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	result := make([]DataPoint, cb.count)
	for i := 0; i < cb.count; i++ {
		idx := (cb.head - cb.count + i) % cb.size
		if idx < 0 {
			idx += cb.size
		}
		result[i] = cb.data[idx]
	}
	return result
}
