package redis

import (
	"fmt"
	"io"
)

type ClusterSlotResp struct {
	rangeStart int
	rangeEnd   int
	servers    []ClusterServerResp
}

func (c ClusterSlotResp) RangeStart() int {
	return c.rangeStart
}
func (c ClusterSlotResp) RangeEnd() int {
	return c.rangeEnd
}
func (c ClusterSlotResp) Servers() []ClusterServerResp {
	return c.servers
}

func ClusterSlotsRespToRedisStream(writer io.Writer, c []ClusterSlotResp) (bytesWrittenTotal int, err error) {
	var bytesWritten int
	bytesWritten, err = fmt.Fprintf(writer, "*%d\r\n", len(c))
	if err != nil {
		return bytesWritten, err
	}
	bytesWrittenTotal += bytesWritten
	for _, slot := range c {
		bytesWritten, err = ClusterSlotRespToRedisStream(writer, slot)
		if err != nil {
			return bytesWritten, err
		}
		bytesWrittenTotal += bytesWritten
	}
	return
}

func ClusterSlotRespToRedisStream(writer io.Writer, c ClusterSlotResp) (bytesWrittenTotal int, err error) {
	var bytesWritten int
	bytesWritten, err = fmt.Fprintf(writer, "*%d\r\n:%d\r\n:%d\r\n", len(c.servers)+2, c.rangeStart, c.rangeEnd)
	if err != nil {
		return bytesWritten, err
	}
	bytesWrittenTotal += bytesWritten

	for _, server := range c.servers {
		bytesWritten, err = ClusterServerRespToRedisStream(writer, server)
		if err != nil {
			return bytesWritten, err
		}
		bytesWrittenTotal += bytesWritten
	}
	return
}

// DeserializeClusterSlotServerResp converts a stream into a redis response structure to CLUSTER slots request
// https://redis.io/topics/protocol
func DeserializeClusterSlotServerResp(reader io.Reader, buffer []byte) (servers []ClusterSlotResp, bytesRead int, err error) {
	var components Componenter
	components, bytesRead, err = ComponentFromReader(reader, buffer)
	if err != nil {
		return
	}
	if componentArray, ok := components.(*redisArray); !ok {
		return nil, bytesRead, fmt.Errorf("expected an array, but got %v", components)
	} else {
		servers = make([]ClusterSlotResp, len(*componentArray))
		for serverIndex, component := range *componentArray {
			servers[serverIndex], err = deserializeSlot(component)
		}
		return servers, bytesRead, nil
	}
}

func deserializeSlot(serverComponent Componenter) (slot ClusterSlotResp, err error) {
	if componentArray, ok := serverComponent.(*redisArray); !ok {
		return slot, fmt.Errorf("expected an array, but got %v", serverComponent)
	} else {
		if len(*componentArray) < 3 {
			return slot, fmt.Errorf("expected slot object to contain at least 2 fields, but got: %d", len(*componentArray))
		}
		if rangeStart, ok := (*componentArray)[0].(*redisInt); !ok {
			return slot, fmt.Errorf("expected slot object's component [0] to be type redisInt, but got: %v", (*componentArray)[0])
		} else {
			slot.rangeStart = rangeStart.Int()
		}
		if rangeEnd, ok := (*componentArray)[1].(*redisInt); !ok {
			return slot, fmt.Errorf("expected slot object's component [1] to be type redisInt, but got: %v", (*componentArray)[1])
		} else {
			slot.rangeEnd = rangeEnd.Int()
		}

		serverCount := len(*componentArray) - 2
		slot.servers = make([]ClusterServerResp, serverCount)
		for serverIndex, component := range (*componentArray)[2:] {
			slot.servers[serverIndex], err = deserializeClusterServer(component)
			if err != nil {
				return
			}
		}
	}

	return
}
