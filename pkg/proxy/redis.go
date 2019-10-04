package proxy

import (
	"fmt"
	"io"
	"log"
	"net"
	"redis_cluster_proxy/pkg/ip_map"
	"redis_cluster_proxy/pkg/port_pool"
	"strconv"
	"strings"
	"sync"
)

const (
	MaxConnections      = 100
	ReadBufferSizeBytes = 1024
)

type Redis struct {
	listenAddr  ip_map.HostWithPort
	clusterAddr ip_map.HostWithPort
	bufferPool  *sync.Pool
	ipMap       *ip_map.Concurrent
	portKeeper  *port_pool.Keeper
}

func NewRedis(listenAddr, clusterAddr ip_map.HostWithPort) *Redis {
	ret := &Redis{
		listenAddr:  listenAddr,
		clusterAddr: clusterAddr,
		bufferPool: &sync.Pool{New: func() interface{} {
			return nil
		}},
		ipMap:      ip_map.NewConcurrent(),
		portKeeper: port_pool.NewKeeper(0),
	}
	for i := 0; i < MaxConnections; i++ {
		ret.bufferPool.Put(make([]byte, ReadBufferSizeBytes))
	}

	// add the master node

	ret.ipMap.Create(clusterAddr, listenAddr)

	return ret
}

func (r *Redis) ListenAndServe() error {
	listener, err := net.Listen("tcp", r.listenAddr.String())
	if err != nil {
		return err
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		go func(conn net.Conn) {
			err := proxyConnection(conn, r.bufferPool, r.ipMap, r.portKeeper, r.clusterAddr.String())
			_ = conn.Close()
			if err != nil {
				log.Fatal(err)
			}
		}(conn)
	}
}

func proxyConnection(conn net.Conn, bufferPool *sync.Pool, ipMap *ip_map.Concurrent, portKeeper *port_pool.Keeper, clusterAddr string) (err error) {
	buffer := bufferPool.Get().([]byte)
	if nil == buffer {
		log.Println("ran out of buffers, so we cannot handle new connections, consider increasing MaxConnections")
		return
	}
	defer func() {
		buffer = buffer[0:cap(buffer)]
		for i := 0; i < len(buffer); i++ {
			buffer[i] = 0x0
		}
		bufferPool.Put(buffer)
	}()

	readCount, err := conn.Read(buffer)
	if err != nil && io.EOF != err {
		log.Println("io error while reading local socket: " + err.Error())
		return
	}
	buffer = buffer[0:readCount]
	parts := strings.Split(string(buffer), "\r\n")
	if parts[2] == "CLUSTER" && parts[4] == "slots" {
		// connection is attempting to query the cluster, forward the request
		cluster, err := net.Dial("tcp", clusterAddr)
		if err != nil {
			log.Fatal("unable to connect to the redis cluster at: " + clusterAddr)
		}
		_, _ = cluster.Write(buffer)
		// get the repsonse back from the actual redis system
		buffer = buffer[0:cap(buffer)]
		readCount, err = cluster.Read(buffer)
		buffer = buffer[0:readCount]
		slots, err := deserializeClusterSlotServerResp(buffer)
		_ = slots

		// create a new socket for each slot, map it
		for _, slot := range slots {
			for serverIndex, server := range slot.servers {
				remoteHostWithPort := ip_map.HostWithPort{Host: server.ip, Port: server.port}
				if localSocket, exists := ipMap.RemoteToLocal(remoteHostWithPort); !exists {
					// socket doesn't exist, spawn it, use a random address
					nodeListener, err := net.Listen("tcp", "")
					if err != nil {
						return
					}
					localListenerHostWithPort, err := ip_map.NewHostWithPortFromString(nodeListener.Addr().String())
					if err != nil {
						return
					}
					// create the mapping entry for the new socket
					ipMap.Create(remoteHostWithPort, localListenerHostWithPort)
					go func(listener net.Listener) {
						for {
							listenerConn, err := listener.Accept()
							if err != nil {
								break
							}
							go func(listenerConn net.Conn) {
								// TODO: continue implementing here
								startNodeThread(remoteHostWithPort)
								_ = listenerConn.Close()
							}(listenerConn)
						}
					}(nodeListener)
				} else {
					// socket exists, swap it out
					slot.servers[serverIndex].ip = localSocket.Host
					slot.servers[serverIndex].port = localSocket.Port
				}
			}
		}
	}
}

func deserializeClusterSlotServerResp(buffer []byte) (servers []clusterSlotResp, err error) {
	// https://redis.io/topics/protocol
	statements := strings.Split(string(buffer), "\r\n")
	numServers := 0
	numServers, err = strconv.Atoi(statements[0][1:])
	servers = make([]clusterSlotResp, numServers)
	statements = statements[1:]
	offset := 0
	for serverIndex := 0; serverIndex < numServers; serverIndex++ {
		servers[serverIndex], offset, err = deserializeSlot(statements)
		statements = statements[offset:]
	}

	return
}

func deserializeSlot(statements []string) (slot clusterSlotResp, skipStatements int, err error) {
	records := 0
	records, err = strconv.Atoi(statements[0][1:])
	skipStatements++
	if err != nil {
		return
	}
	slot.rangeStart, err = strconv.Atoi(statements[1][1:])
	skipStatements++
	if err != nil {
		return
	}
	slot.rangeEnd, err = strconv.Atoi(statements[2][1:])
	skipStatements++
	if err != nil {
		return
	}
	serverCount := records - 2
	slot.servers = make([]clusterServerResp, serverCount)
	for serverIndex := 0; serverIndex < len(slot.servers); serverIndex++ {
		skipStatements++ // Skipping the *3, all server records have 3 items: IP, port, and id
		slot.servers[serverIndex].ip = statements[skipStatements+1]
		slot.servers[serverIndex].port, err = strconv.Atoi(statements[skipStatements+2][1:])
		slot.servers[serverIndex].id = statements[skipStatements+4]
		skipStatements += 5
	}
	return
}

type clusterSlotResp struct {
	rangeStart int
	rangeEnd   int
	servers    []clusterServerResp
}

func clusterSlotRespToRedisStream(c clusterSlotResp) []string {
	resp := make([]string, 4)
	resp[0] = "*3"
	resp[1] = fmt.Sprintf(":%d", c.rangeStart)
	resp[2] = fmt.Sprintf(":%d", c.rangeEnd)
	resp[3] = fmt.Sprintf("*%d", len(c.servers))
	return resp
}

type clusterServerResp struct {
	ip   string
	port int
	id   string
}

func clusterServerRespToRedisStream(c clusterServerResp) []string {
	resp := make([]string, 6)
	resp[0] = "*3"
	resp[1] = fmt.Sprintf("$%d", len(c.ip))
	resp[2] = c.ip
	resp[3] = fmt.Sprintf(":%d", c.port)
	resp[4] = fmt.Sprintf("$%d", len(c.id))
	resp[5] = c.id
	return resp
}
