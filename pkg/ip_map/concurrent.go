package ip_map

import "sync"

type Concurrent struct {
	mu            *sync.RWMutex
	localToRemote map[uint16]HostWithPort
	remoteToLocal map[HostWithPort]uint16
}

func NewConcurrent() *Concurrent {
	return &Concurrent{
		mu:            &sync.RWMutex{},
		localToRemote: make(map[uint16]HostWithPort),
		remoteToLocal: make(map[HostWithPort]uint16),
	}
}

func (c *Concurrent) Create(remote HostWithPort, local uint16) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.localToRemote[local] = remote
	c.remoteToLocal[remote] = local
}

func (c *Concurrent) LocalToRemote(localAddr uint16) (remote HostWithPort, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	remote, ok = c.localToRemote[localAddr]
	return
}

func (c *Concurrent) RemoteToLocal(remoteAddr HostWithPort) (local uint16, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	local, ok = c.remoteToLocal[remoteAddr]
	return
}

func (c *Concurrent) SnapshotLocalsToRemotes() (ret map[uint16]HostWithPort) {
	ret = make(map[uint16]HostWithPort)
	c.mu.RLock()
	defer c.mu.RUnlock()
	for key, value := range c.localToRemote {
		ret[key] = value
	}
	return
}
