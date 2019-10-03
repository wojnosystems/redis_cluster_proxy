package port_pool

import (
	"fmt"
	"sync"
)

type Keeper struct {
	mu      *sync.Mutex
	counter uint16
}

func NewKeeper(startAt uint16) *Keeper {
	return &Keeper{
		mu:      &sync.Mutex{},
		counter: startAt,
	}
}

func (k *Keeper) Next() (int, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if 0 == k.counter {
		return 0, fmt.Errorf("out of ports")
	}
	ret := k.counter
	k.counter++
	return int(ret), nil
}
