package proxy

import (
	"net"
	"time"
)

type pooledConnection struct {
	realConnection net.Conn
	pool           ConnectionPooler
	isClosed       bool
}

func newPooledConnection(pool ConnectionPooler, conn net.Conn) *pooledConnection {
	return &pooledConnection{
		realConnection: conn,
		pool:           pool,
		isClosed:       false,
	}
}

func (p *pooledConnection) Read(b []byte) (n int, err error) {
	return p.realConnection.Read(b)
}

func (p *pooledConnection) Write(b []byte) (n int, err error) {
	return p.realConnection.Write(b)
}

func (p *pooledConnection) Close() error {
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
	return p.realConnection.SetDeadline(t)
}

func (p *pooledConnection) SetReadDeadline(t time.Time) error {
	return p.realConnection.SetReadDeadline(t)
}

func (p *pooledConnection) SetWriteDeadline(t time.Time) error {
	return p.realConnection.SetWriteDeadline(t)
}
