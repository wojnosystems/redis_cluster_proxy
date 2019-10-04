package ip_map

import (
	"fmt"
	"net"
	"strconv"
)

type HostWithPort struct {
	Host string
	Port int
}

func NewHostWithPortFromString(hostColonPort string) (hostWithPort HostWithPort, err error) {
	var portString string
	hostWithPort.Host, portString, err = net.SplitHostPort(hostColonPort)
	if err != nil {
		return
	}
	hostWithPort.Port, err = strconv.Atoi(portString)
	return
}

func (h HostWithPort) String() string {
	return fmt.Sprintf("%s:%d", h.Host, h.Port)
}
