package port_pool

import (
	"fmt"
	"sync"
)

type Counter interface {
	Next() (uint16, error)
}

type counter struct {
	mu      *sync.Mutex
	counter uint16
}

func NewCounter(startAt uint16) Counter {
	return &counter{
		mu:      &sync.Mutex{},
		counter: startAt,
	}
}

func (k *counter) Next() (uint16, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if 0 == k.counter {
		return 0, fmt.Errorf("out of ports")
	}
	ret := k.counter
	k.counter++
	return ret, nil
}
