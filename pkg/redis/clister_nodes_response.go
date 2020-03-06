package redis

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type ClusterNodeResp struct {
	id           string
	ip           string
	port         uint16
	cport        uint16
	flags        string
	master       string
	pingSent     uint64
	pongReceived uint64
	configEpic   uint64
	linkState    string
	slots        []string
}

func (c *ClusterNodeResp) Slots() []string {
	return c.slots
}

func (c *ClusterNodeResp) SetSlots(slots []string) {
	c.slots = slots
}

func (c *ClusterNodeResp) LinkState() string {
	return c.linkState
}

func (c *ClusterNodeResp) SetLinkState(linkState string) {
	c.linkState = linkState
}

func (c *ClusterNodeResp) ConfigEpic() uint64 {
	return c.configEpic
}

func (c *ClusterNodeResp) SetConfigEpic(configEpic uint64) {
	c.configEpic = configEpic
}

func (c *ClusterNodeResp) PongReceived() uint64 {
	return c.pongReceived
}

func (c *ClusterNodeResp) SetPongReceived(pongReceived uint64) {
	c.pongReceived = pongReceived
}

func (c *ClusterNodeResp) PingSent() uint64 {
	return c.pingSent
}

func (c *ClusterNodeResp) SetPingSent(pingSent uint64) {
	c.pingSent = pingSent
}

func (c *ClusterNodeResp) Flags() string {
	return c.flags
}

func (c *ClusterNodeResp) SetFlags(flags string) {
	c.flags = flags
}

func (c *ClusterNodeResp) Cport() uint16 {
	return c.cport
}

func (c *ClusterNodeResp) SetCport(cport uint16) {
	c.cport = cport
}

func (c *ClusterNodeResp) Port() uint16 {
	return c.port
}

func (c *ClusterNodeResp) SetPort(port uint16) {
	c.port = port
}

func (c *ClusterNodeResp) Ip() string {
	return c.ip
}

func (c *ClusterNodeResp) SetIp(ip string) {
	c.ip = ip
}

func (c *ClusterNodeResp) Id() string {
	return c.id
}

func (c *ClusterNodeResp) SetId(id string) {
	c.id = id
}

func (c *ClusterNodeResp) Master() string {
	return c.master
}

func (c *ClusterNodeResp) SetMaster(master string) {
	c.master = master
}

func NewClusterNodeRespFromStringRecord(record string) (node ClusterNodeResp, err error) {
	node.slots = make([]string, 0, 0)
	columns := strings.Split(record, " ")
	if len(columns) < 8 {
		return node, fmt.Errorf("CLUSTER NODE record had insufficient columns; record: '%s'", record)
	}
	node.id = columns[0]
	ipParts := strings.Split(columns[1], ":")
	if len(ipParts) != 2 {
		return node, fmt.Errorf("CLUSTER NODE ip was missing colon for port")
	}
	node.ip = ipParts[0]

	portParts := strings.Split(ipParts[1], "@")
	if len(portParts) != 2 {
		return node, fmt.Errorf("CLUSTER NODE ip was missing at (@) for port and client port")
	}

	var nodePortTmp uint64
	nodePortTmp, err = strconv.ParseUint(portParts[0], 10, 16)
	if err != nil {
		return
	}
	node.port = uint16(nodePortTmp)

	nodePortTmp, err = strconv.ParseUint(portParts[1], 10, 16)
	if err != nil {
		return
	}
	node.cport = uint16(nodePortTmp)
	node.flags = columns[2]
	node.master = columns[3]
	node.pingSent, err = strconv.ParseUint(columns[4], 10, 64)
	if err != nil {
		return
	}
	node.pongReceived, err = strconv.ParseUint(columns[5], 10, 64)
	if err != nil {
		return
	}
	node.configEpic, err = strconv.ParseUint(columns[6], 10, 64)
	if err != nil {
		return
	}
	node.linkState = columns[7]
	for i := 8; i < len(columns); i++ {
		node.slots = append(node.slots, columns[i])
	}
	return
}

func (c ClusterNodeResp) Record() string {
	parts := make([]string, 0, 1+len(c.Slots()))
	parts = append(parts, fmt.Sprint(
		c.Id(), " ",
		c.Ip(), ":", c.Port(), "@", c.Cport(), " ",
		c.PingSent(), " ",
		c.PongReceived(), " ",
		c.ConfigEpic(), " ",
		c.LinkState()))
	for _, slot := range c.Slots() {
		parts = append(parts, slot)
	}
	return strings.Join(parts, " ")
}

func NewClusterNodesRespFromComponent(componenter Componenter) (nodes []ClusterNodeResp, err error) {
	nodes = make([]ClusterNodeResp, 0, 6)
	if nodesPayload, ok := (componenter).(*BulkString); !ok {
		return nil, errors.New("failed to read the CLUSTER NODES response payload")
	} else {
		records := strings.Split(nodesPayload.String(), "\n")
		for _, record := range records {
			if len(record) == 0 {
				break
			}
			var node ClusterNodeResp
			node, err = NewClusterNodeRespFromStringRecord(record)
			if err != nil {
				return
			}
			nodes = append(nodes, node)
		}
	}
	return
}

func NewClusterNodeRespFromClusterNodeRespArray(nodes []ClusterNodeResp) (ret []ClusterNodeResp) {
	ret = make([]ClusterNodeResp, len(nodes))
	for i, node := range nodes {
		ret[i] = node
	}
	return
}

func ClusterNodeArrayToComponent(nodes []ClusterNodeResp) Componenter {
	nodeRecords := make([]string, len(nodes))
	for nodeIndex, node := range nodes {
		nodeRecords[nodeIndex] = node.Record()
	}
	return NewBulkStringFromString(strings.Join(nodeRecords, "\n") + "\n")
}
