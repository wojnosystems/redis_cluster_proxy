package redis

import (
	"fmt"
	"io"
)

type ClusterServerResp struct {
	ip   string
	port uint16
	id   string
}

func (c ClusterServerResp) Ip() string {
	return c.ip
}
func (c ClusterServerResp) Port() uint16 {
	return c.port
}
func (c *ClusterServerResp) SetIp(v string) {
	c.ip = v
}
func (c *ClusterServerResp) SetPort(v uint16) {
	c.port = v
}
func (c ClusterServerResp) Id() string {
	return c.ip
}

func ClusterServerRespToRedisStream(writer io.Writer, c ClusterServerResp) (bytesWrittenTotal int, err error) {
	var bytesWritten int
	bytesWritten, err = fmt.Fprintf(writer, "*3\r\n$%d\r\n%s\r\n:%d\r\n$%d\r\n%s\r\n", len(c.ip), c.ip, c.port, len(c.id), c.id)
	if err != nil {
		return bytesWritten, err
	}
	bytesWrittenTotal += bytesWritten
	return
}

func deserializeClusterServer(component Componenter) (server ClusterServerResp, err error) {
	// Skipping the *3, all server records have 3 items: IP, port, and id
	if componentArray, ok := component.(*redisArray); !ok {
		return server, fmt.Errorf("expected server object to contain a redisArray, but got: %v", component)
	} else {
		if len(*componentArray) != 3 {
			return server, fmt.Errorf("expected server object to contain a redisArray of length exactly 3, but got: %d", len(*componentArray))
		}

		if serverIp, ok := (*componentArray)[0].(*redisString); !ok {
			return server, fmt.Errorf("expected server IP field to be a string, but got %v", (*componentArray)[0])
		} else {
			server.ip = serverIp.String()
		}

		if serverPort, ok := (*componentArray)[1].(*redisInt); !ok {
			return server, fmt.Errorf("expected server port field to be an int, but got %v", (*componentArray)[1])
		} else {
			server.port = uint16(serverPort.Int())
		}

		if serverId, ok := (*componentArray)[2].(*redisString); !ok {
			return server, fmt.Errorf("expected server ID field to be a string, but got %v", (*componentArray)[2])
		} else {
			server.id = serverId.String()
		}
	}
	return
}
