package redis

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

var clusterSlotResponseInputFixture string

const BufferSizeBytes = 512

func TestSerialize(t *testing.T) {
	fixtureStream := bytes.NewBufferString(clusterSlotResponseInputFixture)
	buffer := make([]byte, BufferSizeBytes)
	decoded, _, err := DeserializeClusterSlotServerResp(fixtureStream, buffer)
	if err != nil {
		t.Fatal(err)
	}
	actualOutput := bytes.Buffer{}
	_, err = ClusterSlotsRespToRedisStream(&actualOutput, decoded)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, clusterSlotResponseInputFixture, actualOutput.String())
}

func init() {
	clusterSlotResponseInputFixture = strings.Join(
		[]string{
			"*3",
			"*4",
			":0",
			":5460",
			"*3",
			"$10",
			"172.22.0.2",
			":7000",
			"$40",
			"901e06d850fe7a21253fbb200b5bdd55d3286848",
			"*3",
			"$10",
			"172.22.0.2",
			":7005",
			"$40",
			"d39334b20e1f05b1cadcf6858a907360cbae58d9",
			"*4",
			":5461",
			":10922",
			"*3",
			"$10",
			"172.22.0.2",
			":7001",
			"$40",
			"543675033db89351b9e054ce0eef39294e282c4f",
			"*3",
			"$10",
			"172.22.0.2",
			":7003",
			"$40",
			"42beed9a1b3daaf13929b60371806d01cf51256b",
			"*4",
			":10923",
			":16383",
			"*3",
			"$10",
			"172.22.0.2",
			":7002",
			"$40",
			"42066efba15b215c63c1ca3d6bd4738f53656fd1",
			"*3",
			"$10",
			"172.22.0.2",
			":7004",
			"$40",
			"9d9b0002ef800a31c1f4284a3a84eeb3e214f66c",
		}, "\r\n",
	) + "\r\n"
}
