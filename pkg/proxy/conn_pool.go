package proxy

import (
	"fmt"
	"net"
	"sync"
)

type ConnectionPooler interface {
	Dial(destinationAddr string) (conn net.Conn, err error)
	ReleaseConnection(connection *pooledConnection) error
}

type connPool struct {
	mu                      *sync.Mutex
	pool                    map[string]*connEntry
	maxConnectionsPerTarget int
}

func newConnPool(maxConnectionsPerTarget int) *connPool {
	return &connPool{
		mu:                      &sync.Mutex{},
		pool:                    make(map[string]*connEntry),
		maxConnectionsPerTarget: maxConnectionsPerTarget,
	}
}

var ErrPoolDepleted = fmt.Errorf("no connections available")

func (c *connPool) Dial(destinationAddr string) (conn net.Conn, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if connEntry, ok := c.pool[destinationAddr]; ok {
		conn = connEntry.Get()
	} else {
		var newConn net.Conn
		newConn, err = net.Dial("tcp", destinationAddr)
		if err != nil {
			return
		}
		connEntry := newConnEntry(c.maxConnectionsPerTarget)
		connEntry.AddIdle(newConn)
		c.pool[destinationAddr] = connEntry
		conn = connEntry.Get()
	}
	if conn == nil {
		return nil, ErrPoolDepleted
	}
	// wrap the connection so that it auto-returns to the pool when Close is called
	conn = newPooledConnection(c, conn)
	return
}

func (c *connPool) ReleaseConnection(connection *pooledConnection) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.pool[connection.RemoteAddr().String()].Put(connection.realConnection)
	return nil
}

type connEntry struct {
	idle        []net.Conn
	activeCount int
}

func newConnEntry(maxConnections int) *connEntry {
	return &connEntry{
		idle:        make([]net.Conn, 0, maxConnections),
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

func (c *connEntry) AddIdle(conn net.Conn) {
	c.idle = append(c.idle, conn)
}

func (c *connEntry) Put(conn net.Conn) {
	c.activeCount--
	c.AddIdle(conn)
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
