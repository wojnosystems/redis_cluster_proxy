package proxy

import (
	"net"
)

// Bidirectional creates a two-way proxy, buffering data. BLocks until one or both sides are closed
func Bidirectional(a, b net.Conn, aBuffer, bBuffer []byte) {
	go halfDuplex(a, b, aBuffer)
	halfDuplex(b, a, bBuffer)
}

func halfDuplex(read, write net.Conn, buffer []byte) {
	for {
		buffer = buffer[0:cap(buffer)]
		bytesRead, err := read.Read(buffer)
		if err != nil {
			_ = write.Close()
			return
		}
		buffer = buffer[0:bytesRead]
		_, err = write.Write(buffer)
		if err != nil {
			_ = read.Close()
			return
		}
	}
}
