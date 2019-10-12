package redis

import (
	"fmt"
)

type ClusterServerResp struct {
	ip   string
	port uint16
	id   string
}

func NewClusterServerResp(ip string, port uint16, id string) ClusterServerResp {
	return ClusterServerResp{
		ip:   ip,
		port: port,
		id:   id,
	}
}

func NewClusterServerRespFromClusterServerResp(resp ClusterServerResp) ClusterServerResp {
	return ClusterServerResp{
		ip:   resp.ip,
		port: resp.port,
		id:   resp.id,
	}
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

func ClusterServerRespToComponent(c ClusterServerResp) Componenter {
	component := make(Array, 3)
	component[0] = NewBulkStringFromString(c.ip)
	component[1] = NewIntFromInt(int(c.port))
	component[2] = NewBulkStringFromString(c.id)
	return &component
}

func deserializeClusterServer(component Componenter) (server ClusterServerResp, err error) {
	// Skipping the *3, all server records have 3 items: IP, port, and id
	if componentArray, ok := component.(*Array); !ok {
		return server, fmt.Errorf("expected server object to contain a Array, but got: %v", component)
	} else {
		if len(*componentArray) != 3 {
			return server, fmt.Errorf("expected server object to contain a Array of length exactly 3, but got: %d", len(*componentArray))
		}

		if serverIp, ok := (*componentArray)[0].(*BulkString); !ok {
			return server, fmt.Errorf("expected server IP field to be a string, but got %v", (*componentArray)[0])
		} else {
			server.ip = serverIp.String()
		}

		if serverPort, ok := (*componentArray)[1].(*Int); !ok {
			return server, fmt.Errorf("expected server port field to be an int, but got %v", (*componentArray)[1])
		} else {
			server.port = uint16(serverPort.Int())
		}

		if serverId, ok := (*componentArray)[2].(*BulkString); !ok {
			return server, fmt.Errorf("expected server ID field to be a string, but got %v", (*componentArray)[2])
		} else {
			server.id = serverId.String()
		}
	}
	return
}
