package proxy

import (
	"io"
	"net"
	"sync"
	"time"
)

type pooledConnection struct {
	realConnection net.Conn
	pool           ConnectionPooler
	isClosed       bool
	mu             *sync.RWMutex
}

func newPooledConnection(pool ConnectionPooler, conn net.Conn) *pooledConnection {
	return &pooledConnection{
		realConnection: conn,
		pool:           pool,
		isClosed:       false,
		mu:             &sync.RWMutex{},
	}
}

func (p *pooledConnection) Read(b []byte) (n int, err error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.isClosed {
		return p.realConnection.Read(b)
	}
	return 0, io.ErrClosedPipe
}

func (p *pooledConnection) Write(b []byte) (n int, err error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.isClosed {
		return p.realConnection.Write(b)
	}
	return 0, io.ErrClosedPipe

}

func (p *pooledConnection) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.isClosed {
		p.isClosed = true
		return p.pool.ReleaseConnection(p)
	}
	return nil
}

func (p pooledConnection) LocalAddr() net.Addr {
	return p.realConnection.LocalAddr()
}

func (p pooledConnection) RemoteAddr() net.Addr {
	return p.realConnection.RemoteAddr()
}

func (p *pooledConnection) SetDeadline(t time.Time) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.isClosed {
		return p.realConnection.SetDeadline(t)
	}
	return io.ErrClosedPipe
}

func (p *pooledConnection) SetReadDeadline(t time.Time) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.isClosed {
		return p.realConnection.SetReadDeadline(t)
	}
	return io.ErrClosedPipe
}

func (p *pooledConnection) SetWriteDeadline(t time.Time) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.isClosed {
		return p.realConnection.SetWriteDeadline(t)
	}
	return io.ErrClosedPipe
}
