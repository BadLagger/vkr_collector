package models

import (
	"sync"
)

type ValueType string

const (
	TypeNumber ValueType  = "number"
	TypeString ValueType  = "string"
	TypeBashNum ValueType = "bashnum"
	TypeNull   ValueType  = "null"
)

type MetricValue struct {
	Type ValueType `json:"type"`
	Number float64 `json:"number,omitempty"`
	String string  `json:"string,omitempty"`
	Null   any     `json:"-"`
}

func (mv MetricValue) ToInterface() interface{} {
	switch mv.Type {
	case TypeNumber, TypeBashNum:
		return mv.Number
	case TypeString:
		return mv.String
	default:
		return mv.Null
	}
}

type DataPoint struct {
	Timestamp int64                  `json:"timestamp"`
	Values    map[string]interface{} `json:"values"`
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
