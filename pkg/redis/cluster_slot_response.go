package redis

import (
	"fmt"
)

type ClusterSlotResp struct {
	rangeStart int
	rangeEnd   int
	servers    []ClusterServerResp
}

func NewClusterSlotResp(rangeStart, rangeEnd int, servers []ClusterServerResp) ClusterSlotResp {
	return ClusterSlotResp{
		rangeStart: rangeStart,
		rangeEnd:   rangeEnd,
		servers:    servers,
	}
}

func NewClusterSlotRespFromClusterSlotRespArray(in []ClusterSlotResp) (ret []ClusterSlotResp) {
	ret = make([]ClusterSlotResp, len(in))
	for slotIndex, inSlot := range in {
		ret[slotIndex] = NewClusterSlotRespFromClusterSlotResp(inSlot)
	}
	return
}

func NewClusterSlotRespFromClusterSlotResp(r ClusterSlotResp) ClusterSlotResp {
	ret := ClusterSlotResp{
		rangeStart: r.rangeStart,
		rangeEnd:   r.rangeEnd,
		servers:    make([]ClusterServerResp, len(r.servers)),
	}
	for serverIndex, rServer := range r.servers {
		ret.servers[serverIndex] = NewClusterServerRespFromClusterServerResp(rServer)
	}
	return ret
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

func ClusterSlotArrayRespToComponent(c []ClusterSlotResp) (componenter Componenter) {
	slotArray := make(Array, len(c))
	for slotIndex, slot := range c {
		slotArray[slotIndex] = ClusterSlotRespToComponent(slot)
	}
	return &slotArray
}

func ClusterSlotRespToComponent(c ClusterSlotResp) Componenter {
	slotComponent := make(Array, 2+len(c.servers))
	slotComponent[0] = NewIntFromInt(c.rangeStart)
	slotComponent[1] = NewIntFromInt(c.rangeEnd)
	for serverIndex := 2; serverIndex < len(slotComponent); serverIndex++ {
		slotComponent[serverIndex] = ClusterServerRespToComponent(c.servers[serverIndex-2])
	}
	return &slotComponent
}

// NewSlotArrayFromComponent converts a stream into a redis response structure to CLUSTER slots request
// https://redis.io/topics/protocol
func NewSlotArrayFromComponent(component Componenter) (servers []ClusterSlotResp, err error) {
	if componentArray, ok := component.(*Array); !ok {
		return nil, fmt.Errorf("expected an array, but got %v", component)
	} else {
		servers = make([]ClusterSlotResp, len(*componentArray))
		for serverIndex, component := range *componentArray {
			servers[serverIndex], err = NewSlotFromComponent(component)
		}
		return servers, nil
	}
}

func NewSlotFromComponent(serverComponent Componenter) (slot ClusterSlotResp, err error) {
	if componentArray, ok := serverComponent.(*Array); !ok {
		return slot, fmt.Errorf("expected an array, but got %v", serverComponent)
	} else {
		if len(*componentArray) < 3 {
			return slot, fmt.Errorf("expected slot object to contain at least 2 fields, but got: %d", len(*componentArray))
		}
		if rangeStart, ok := (*componentArray)[0].(*Int); !ok {
			return slot, fmt.Errorf("expected slot object's component [0] to be type Int, but got: %v", (*componentArray)[0])
		} else {
			slot.rangeStart = rangeStart.Int()
		}
		if rangeEnd, ok := (*componentArray)[1].(*Int); !ok {
			return slot, fmt.Errorf("expected slot object's component [1] to be type Int, but got: %v", (*componentArray)[1])
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
