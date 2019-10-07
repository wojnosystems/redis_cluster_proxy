package ip_map

import "sync"

type Concurrent struct {
	mu            *sync.RWMutex
	localToRemote map[HostWithPort]HostWithPort
	remoteToLocal map[HostWithPort]HostWithPort
}

func NewConcurrent() *Concurrent {
	return &Concurrent{
		mu:            &sync.RWMutex{},
		localToRemote: make(map[HostWithPort]HostWithPort),
		remoteToLocal: make(map[HostWithPort]HostWithPort),
	}
}

func (c *Concurrent) Create(remote, local HostWithPort) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.localToRemote[local] = remote
	c.remoteToLocal[remote] = local
}

func (c *Concurrent) LocalToRemote(localAddr HostWithPort) (remote HostWithPort, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	remote, ok = c.localToRemote[localAddr]
	return
}

func (c *Concurrent) RemoteToLocal(remoteAddr HostWithPort) (local HostWithPort, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	local, ok = c.remoteToLocal[remoteAddr]
	return
}

func (c *Concurrent) SnapshotLocalsToRemotes() (ret map[HostWithPort]HostWithPort) {
	ret = make(map[HostWithPort]HostWithPort)
	c.mu.RLock()
	defer c.mu.RUnlock()
	for key, value := range c.localToRemote {
		ret[key] = value
	}
	return
}
