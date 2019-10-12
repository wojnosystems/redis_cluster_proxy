package proxy

import (
	"io"
	"log"
	"net"
	"redis_cluster_proxy/pkg/redis"
)

type RewriteFunc func(componenterIn redis.Componenter) (componenterOut redis.Componenter)

// Bidirectional creates a two-way proxy, buffering data. BLocks until one or both sides are closed
func Bidirectional(client, cluster net.Conn, intercept RewriteFunc, reWrite RewriteFunc) {
	go halfDuplex(client, cluster, intercept, reWrite, "cli["+client.LocalAddr().String()+"] -> cluster["+cluster.RemoteAddr().String()+"]")
	go halfDuplex(cluster, client, intercept, reWrite, "cluster["+cluster.RemoteAddr().String()+"] -> cli["+client.LocalAddr().String()+"]")
}

func halfDuplex(read, write net.Conn, intercept RewriteFunc, reWrite RewriteFunc, label string) {
	log.Println("new " + label)
	buffer := make([]byte, 32000)
	var interceptedComponent redis.Componenter
	var componenter redis.Componenter
	var readErr error
	var writeErr error
	for {
		log.Println(label + ": about to read")
		componenter, _, readErr = redis.ComponentFromReader(read, buffer)
		log.Println(label + ": done read")
		if readErr != nil {
			if readErr != io.EOF {
				log.Println(label + ": error reading " + readErr.Error())
			} else {
				log.Println(label + ": EOF")
			}
			_ = write.Close()
			return
		}
		interceptedComponent = intercept(componenter)
		if interceptedComponent != nil {
			_, writeErr = redis.ComponentToStream(read, interceptedComponent)
			if writeErr != nil {
				log.Println(label + ": error intercepting " + writeErr.Error())
				_ = write.Close()
				_ = read.Close()
				return
			}
			continue
		}
		componenter = reWrite(componenter)
		debugClientIn(label, componenter)
		log.Println(label + ": about to write")
		_, writeErr = redis.ComponentToStream(write, componenter)
		log.Println(label + ": done write")
		if writeErr != nil {
			if writeErr != io.EOF {
				log.Println(label + ": error writing " + writeErr.Error())
			} else {
				log.Println(label + ": EOF")
				_ = write.Close()
			}
			return
		}
	}
}
