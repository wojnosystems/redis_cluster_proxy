package proxy

import "net"

type connEntry struct {
	idle        []net.Conn
	activeCount int
}

func newConnEntry(maxIdleConnections int) *connEntry {
	return &connEntry{
		idle:        make([]net.Conn, 0, maxIdleConnections),
		activeCount: 0,
	}
}

func (c *connEntry) Get() net.Conn {
	if len(c.idle) > 0 {
		c.activeCount++
		idleConnection := c.idle[len(c.idle)-1]
		c.idle = c.idle[0 : len(c.idle)-1]
		return idleConnection
	}
	return nil
}

// AddIdle
// Append the connection to the idle connections pool unless it already exists
// @return true if added, false if it already exists and was not added
func (c *connEntry) AddIdle(conn net.Conn) bool {
	if !c.doesConnExist(conn) {
		c.idle = append(c.idle, conn)
		return true
	}
	return false
}

func (c *connEntry) Put(conn net.Conn) {
	if c.AddIdle(conn) {
		c.activeCount--
	}
}

func (c connEntry) doesConnExist(conn net.Conn) bool {
	for _, value := range c.idle {
		if conn == value {
			return true
		}
	}
	return false
}

func (c connEntry) ActiveConnectionCount() int {
	return c.activeCount
}

func (c connEntry) IdleConnectionCount() int {
	return len(c.idle)
}

func (c connEntry) TotalOpenConnections() int {
	return c.ActiveConnectionCount() + c.IdleConnectionCount()
}
