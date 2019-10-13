package proxy

import (
	"io"
	"net"
	"redis_cluster_proxy/pkg/redis"
)

type RewriteFunc func(componenterIn redis.Componenter) (componenterOut redis.Componenter)

// Bidirectional creates a two-way proxy, buffering data. BLocks until one or both sides are closed
func Bidirectional(client, cluster net.Conn, intercept RewriteFunc, reWrite RewriteFunc, buffer1, buffer2 []byte, doneChan chan<- error, debugOutputEnabled bool) {
	go halfDuplex(client, cluster, intercept, reWrite, buffer1, doneChan, "cli["+client.LocalAddr().String()+"] -> cluster["+cluster.RemoteAddr().String()+"]", debugOutputEnabled)
	go halfDuplex(cluster, client, intercept, reWrite, buffer2, doneChan, "cluster["+cluster.RemoteAddr().String()+"] -> cli["+client.LocalAddr().String()+"]", debugOutputEnabled)
}

func halfDuplex(read, write net.Conn, intercept RewriteFunc, reWrite RewriteFunc, buffer []byte, doneChan chan<- error, label string, debugOutputEnabled bool) {
	var interceptedComponent redis.Componenter
	var componenter redis.Componenter
	var err error
	for {
		componenter, _, err = redis.ComponentFromReader(read, buffer)
		if err != nil {
			_ = write.Close()
			break
		}
		interceptedComponent = intercept(componenter)
		if interceptedComponent != nil {
			_, err = redis.ComponentToStream(read, interceptedComponent)
			if err != nil {
				_ = write.Close()
				_ = read.Close()
				break
			}
			continue
		}
		componenter = reWrite(componenter)
		debugClientIn(label, debugOutputEnabled, componenter)
		_, err = redis.ComponentToStream(write, componenter)
		if err != nil {
			if err == io.EOF {
				_ = write.Close()
			}
			break
		}
	}
	doneChan <- hideErrors(err)
}

func hideErrors(err error) error {
	if err == io.EOF {
		return nil
	}
	if opErr, ok := err.(*net.OpError); ok {
		if opErr.Err.Error() == "use of closed network connection" {
			return nil
		}
	}
	return err
}
