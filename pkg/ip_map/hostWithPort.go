package ip_map

import (
	"fmt"
	"net"
	"strconv"
)

type HostWithPort struct {
	Host string
	Port uint16
}

func NewHostWithPortFromString(hostColonPort string) (hostWithPort HostWithPort, err error) {
	var portString string
	hostWithPort.Host, portString, err = net.SplitHostPort(hostColonPort)
	if err != nil {
		return
	}
	portUInt64, err := strconv.ParseUint(portString, 10, 16)
	if err != nil {
		return
	}
	hostWithPort.Port = uint16(portUInt64)
	return
}

func (h HostWithPort) String() string {
	return fmt.Sprintf("%s:%d", h.Host, h.Port)
}
