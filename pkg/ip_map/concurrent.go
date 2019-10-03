package ip_map

import "sync"

type Concurrent struct {
	mu            *sync.RWMutex
	localToRemote map[string]string
	remoteToLocal map[string]string
}

func NewConcurrent() *Concurrent {
	return &Concurrent{
		mu:            &sync.RWMutex{},
		localToRemote: make(map[string]string),
		remoteToLocal: make(map[string]string),
	}
}

func (c *Concurrent) Create(remote, local string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.localToRemote[local] = remote
	c.remoteToLocal[remote] = local
}

func (c *Concurrent) LocalToRemote(local string) (remote string, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	remote, ok = c.localToRemote[local]
	return
}

func (c *Concurrent) RemoteToLocal(remote string) (local string, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	local, ok = c.remoteToLocal[remote]
	return
}
