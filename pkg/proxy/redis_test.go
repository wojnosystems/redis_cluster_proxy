package proxy

import (
	"bytes"
	"redis_cluster_proxy/pkg/ip_map"
	"redis_cluster_proxy/pkg/redis"
	"testing"
)

const BufferSizeBytes = 512

const queryCommandSlots = "*2\r\n$7\r\nCLUSTER\r\n$5\r\nslots\r\n"

var clusterRespInput = []redis.ClusterSlotResp{
	redis.NewClusterSlotResp(0, 5460, []redis.ClusterServerResp{
		redis.NewClusterServerResp("172.22.0.2", 7000, "901e06d850fe7a21253fbb200b5bdd55d3286848"),
		redis.NewClusterServerResp("172.22.0.2", 7005, "d39334b20e1f05b1cadcf6858a907360cbae58d9"),
	}),
	redis.NewClusterSlotResp(5461, 10922, []redis.ClusterServerResp{
		redis.NewClusterServerResp("172.22.0.2", 7001, "543675033db89351b9e054ce0eef39294e282c4f"),
		redis.NewClusterServerResp("172.22.0.2", 7003, "42beed9a1b3daaf13929b60371806d01cf51256b"),
	}),
	redis.NewClusterSlotResp(10923, 16383, []redis.ClusterServerResp{
		redis.NewClusterServerResp("172.22.0.2", 7002, "42066efba15b215c63c1ca3d6bd4738f53656fd1"),
		redis.NewClusterServerResp("172.22.0.2", 7004, "9d9b0002ef800a31c1f4284a3a84eeb3e214f66c"),
	}),
}
var clusterRespExpected = []redis.ClusterSlotResp{
	redis.NewClusterSlotResp(0, 5460, []redis.ClusterServerResp{
		redis.NewClusterServerResp("test", 8000, "901e06d850fe7a21253fbb200b5bdd55d3286848"),
		redis.NewClusterServerResp("test", 8005, "d39334b20e1f05b1cadcf6858a907360cbae58d9"),
	}),
	redis.NewClusterSlotResp(5461, 10922, []redis.ClusterServerResp{
		redis.NewClusterServerResp("test", 8001, "543675033db89351b9e054ce0eef39294e282c4f"),
		redis.NewClusterServerResp("test", 8003, "42beed9a1b3daaf13929b60371806d01cf51256b"),
	}),
	redis.NewClusterSlotResp(10923, 16383, []redis.ClusterServerResp{
		redis.NewClusterServerResp("test", 8002, "42066efba15b215c63c1ca3d6bd4738f53656fd1"),
		redis.NewClusterServerResp("test", 8004, "9d9b0002ef800a31c1f4284a3a84eeb3e214f66c"),
	}),
}

func TestMutateClusterSlotResp(t *testing.T) {
	lookup := ip_map.NewConcurrent()
	lookup.Create(ip_map.HostWithPort{Host: "172.22.0.2", Port: 7000}, 8000)
	lookup.Create(ip_map.HostWithPort{Host: "172.22.0.2", Port: 7001}, 8001)
	lookup.Create(ip_map.HostWithPort{Host: "172.22.0.2", Port: 7002}, 8002)
	lookup.Create(ip_map.HostWithPort{Host: "172.22.0.2", Port: 7003}, 8003)
	lookup.Create(ip_map.HostWithPort{Host: "172.22.0.2", Port: 7004}, 8004)
	lookup.Create(ip_map.HostWithPort{Host: "172.22.0.2", Port: 7005}, 8005)
	command, err := stringToComponents(queryCommandSlots)
	if err != nil {
		t.Fatal(err)
	}
	componentOut := mutateClusterSlotsCommand(command, clusterRespInput, clusterRespExpected[0].Servers()[0].Ip(), lookup)

	actualOutput := bytes.Buffer{}
	_, err = redis.ComponentToStream(&actualOutput, componentOut)
	if err != nil {
		t.Fatal(err)
	}
}

func stringToComponents(command string) (component redis.Componenter, err error) {
	buffer := bytes.NewBufferString(command)
	component, _, err = redis.ComponentFromReader(buffer, make([]byte, BufferSizeBytes))
	return
}
