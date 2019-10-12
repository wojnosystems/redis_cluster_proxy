package proxy

import "sync"

type bufferPool struct {
	pool *sync.Pool
}

func newBufferPool(numberOfBuffers, bufferSizeInBytes int) *bufferPool {
	bp := &bufferPool{
		pool: &sync.Pool{
			New: func() interface{} {
				return nil
			},
		},
	}

	for i := 0; i < numberOfBuffers; i++ {
		bp.pool.Put(make([]byte, bufferSizeInBytes))
	}
	return bp
}

func (b *bufferPool) Get() (buffer []byte) {
	ret := b.pool.Get()
	if ret == nil {
		return nil
	}
	return ret.([]byte)
}

func (b *bufferPool) Put(buffer []byte) {
	buffer = resetBuffer(buffer)
	eraseBuffer(buffer)
	b.pool.Put(buffer)
}

func eraseBuffer(buffer []byte) {
	for i := 0; i < len(buffer); i++ {
		buffer[i] = 0x0
	}
}

func resetBuffer(buffer []byte) []byte {
	return buffer[0:cap(buffer)]
}
