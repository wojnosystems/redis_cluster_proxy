package proxy

import (
	"fmt"
	"github.com/wojnosystems/retry"
	"io"
	"log"
	"net"
	"redis_cluster_proxy/pkg/ip_map"
	"redis_cluster_proxy/pkg/port_pool"
	redisPkg "redis_cluster_proxy/pkg/redis"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	MaxConnections           = 1000
	MaxConcurrentConnections = 100
	ReadBufferSizeBytes      = 1024
)

type Redis struct {
	listenAddr        ip_map.HostWithPort
	clusterAddr       ip_map.HostWithPort
	bufferPool        *bufferPool
	connPool          *connPool
	ipMap             *ip_map.Concurrent
	portCounter       port_pool.Counter
	listenerWaitGroup *sync.WaitGroup

	listenIPs              []net.IP
	clusterIPs             []net.IP
	facadeClusterSlotsResp []redisPkg.ClusterSlotResp
}

func NewRedis(listenAddr, clusterAddr ip_map.HostWithPort, portKeeper port_pool.Counter) (redis *Redis) {
	ret := &Redis{
		listenAddr:        listenAddr,
		clusterAddr:       clusterAddr,
		bufferPool:        newBufferPool(MaxConnections, ReadBufferSizeBytes),
		connPool:          newConnPool(MaxConcurrentConnections),
		ipMap:             ip_map.NewConcurrent(),
		portCounter:       portKeeper,
		listenerWaitGroup: &sync.WaitGroup{},
	}

	return ret
}

var MaxConnectRetries = retry.MaxAttempts{
	Times:   30,
	WaitFor: 2 * time.Second,
}

const DiscoverStatement = "*2\r\n$7\r\nCLUSTER\r\n$5\r\nslots\r\n"

// DiscoverAndListen contacts the cluster and gets the list of nodes, then establishes proxies for each node
// When contacting redis, we get back a list of server addresses WITHIN the cluster These are private addresses)
// Each of these needs to be recorded in the map. A new port needs to be created locally and that port will map to the remote address
// Suppose we're listening on 127.0.0.1:8000 - 8005. And the Redis Cluster listens on 172.20.0.2:7000 - 7005.
// When a request comes into 127.0.0.1, it needs to be forwarded to 172.20.0.2. The ports don't really matter so long as they are consistent.
// When a client requests the mapping, we should respond with our own, internal mapping, based on the "listenAddr".
func (r *Redis) DiscoverAndListen() (err error) {
	log.Printf("Resolving clusterAddr: %s\n", r.clusterAddr.Host)
	err = r.resolveClusterAddrIP()
	if err != nil {
		return
	}

	var cluster net.Conn

	err = retry.How(MaxConnectRetries.New()).This(func(controller retry.ServiceController) error {
		cluster, err = net.Dial("tcp", r.clusterAddr.String())
		return err
	})
	if err != nil {
		return
	}

	// close the connection to Redis
	defer func() { _ = cluster.Close() }()

	_, err = cluster.Write([]byte(DiscoverStatement))
	if err != nil && io.EOF != err {
		log.Println("io error while writing to cluster socket: " + err.Error())
		return
	}

	buffer := r.bufferPool.Get()
	r.facadeClusterSlotsResp, _, err = redisPkg.DeserializeClusterSlotServerResp(cluster, buffer)
	if err != nil && io.EOF != err {
		log.Println("io error while reading cluster socket: " + err.Error())
		return
	}

	serverAddrs := serverAddrAndPortFromSlotResp(r.facadeClusterSlotsResp)
	for _, serverAddr := range serverAddrs {
		if _, exists := r.ipMap.RemoteToLocal(serverAddr); exists {
			// already exists, do nothing
		} else {
			// No mapping exists, open a socket to service it
			var nodeListener net.Listener
			newListenerAddr := ip_map.HostWithPort{
				Host: "", // we don't want to bind to any hostname, bind to all available interfaces
				Port: 0,
			}
			newListenerAddr.Port, err = r.portCounter.Next()
			if err != nil {
				return
			}
			nodeListener, err = net.Listen("tcp", newListenerAddr.String())
			if err != nil {
				return
			}
			// create the mapping entry for the new socket
			r.ipMap.Create(serverAddr, ip_map.HostWithPort{Host: r.listenAddr.Host, Port: newListenerAddr.Port})

			r.listenerWaitGroup.Add(1)
			go func(nodeListener net.Listener, newListenerAddr ip_map.HostWithPort) {
				_ = listenLoop(nodeListener, r, newListenerAddr)
				r.listenerWaitGroup.Done()
			}(nodeListener, newListenerAddr)
		}
	}

	for slotIndex, slot := range r.facadeClusterSlotsResp {
		for serverIndex, server := range slot.Servers() {
			facadeHostWithPort := ip_map.HostWithPort{
				Host: server.Ip(),
				Port: server.Port(),
			}
			var localAddr ip_map.HostWithPort
			localAddr, err = remoteToLocalHostAndPort(r.ipMap, facadeHostWithPort)
			if err != nil {
				return
			}
			r.facadeClusterSlotsResp[slotIndex].Servers()[serverIndex].SetIp(localAddr.Host)
		}
	}
	return nil
}

func (r *Redis) resolveClusterAddrIP() (err error) {
	if len(r.clusterAddr.Host) == 0 {
		return
	}
	clusterIp := net.ParseIP(r.clusterAddr.Host)
	if clusterIp != nil {
		r.clusterIPs = []net.IP{clusterIp}
	} else {
		err = retry.How(MaxConnectRetries.New()).This(func(controller retry.ServiceController) error {
			r.clusterIPs, err = net.LookupIP(r.clusterAddr.Host)
			return err
		})
		if err != nil {
			return
		}
	}
	return
}

func (r Redis) PrintConnectionStatuses(writer io.Writer) (err error) {
	localToRemotes := r.ipMap.SnapshotLocalsToRemotes()
	keys := make([]ip_map.HostWithPort, 0, len(localToRemotes))
	for local := range localToRemotes {
		keys = append(keys, local)
	}
	// sort the keys for easier reading
	sort.Slice(keys, func(i, j int) bool {
		return strings.Compare(keys[i].String(), keys[j].String()) <= 0
	})
	for _, local := range keys {
		// This is being reported so that people connecting to the cluster, in case they need to debug
		_, err = fmt.Fprintf(writer, "Listening on: %s proxy to: %s\n", local.String(), localToRemotes[local].String())
		if err != nil {
			return
		}
	}
	return
}

func serverAddrAndPortFromSlotResp(clusterSlotResponses []redisPkg.ClusterSlotResp) (hostAndPort []ip_map.HostWithPort) {
	hostAndPort = make([]ip_map.HostWithPort, 0, 10)
	for _, slot := range clusterSlotResponses {
		for _, server := range slot.Servers() {
			hostAndPort = append(hostAndPort, ip_map.HostWithPort{Host: server.Ip(), Port: server.Port()})
		}
	}
	return hostAndPort
}

// ListenAndServe waits for a connection requests
func (r *Redis) WaitUntilAddConnectionsComplete() (err error) {
	r.listenerWaitGroup.Wait()
	return nil
}

func listenLoop(listener net.Listener, r *Redis, localAddr ip_map.HostWithPort) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		go func(conn net.Conn) {
			err = proxyConnection(conn, r, localAddr)
			_ = conn.Close()
			if err != nil {
				log.Fatal(err)
			}
		}(conn)
	}
}

func proxyConnection(conn net.Conn, r *Redis, localAddr ip_map.HostWithPort) (err error) {
	buffer := r.bufferPool.Get()
	if nil == buffer {
		log.Println("ran out of buffers, so we cannot handle new connections, consider increasing MaxConnections")
		return
	}
	defer r.bufferPool.Put(buffer)

	var parsedInboundCommand []string
	parsedInboundCommand, err = readRedisToStatement(conn, buffer)
	if err != nil {
		return
	}

	// If it's a cluster query, intercept it and respond back with our own internal mapping
	if isClusterSlotsQuery(parsedInboundCommand) {
		// reply back with the modified request
		// This is where we "lie" to the client.
		_, err = redisPkg.ClusterSlotsRespToRedisStream(conn, r.facadeClusterSlotsResp)
		if err != nil {
			return
		}
	}

	// some other request, proxy it
	clusterAddr, err := localToRemoteHostAndPort(r.ipMap, localAddr)
	if err != nil {
		return
	}
	var clusterConn net.Conn
	clusterConn, err = r.connPool.Dial(clusterAddr.String())
	if err != nil {
		return
	}

	var buffer1, buffer2 []byte
	buffer1, buffer2, err = r.getTwoBuffers()
	if err != nil {
		return
	}
	defer r.bufferPool.Put(buffer1)
	defer r.bufferPool.Put(buffer2)
	Bidirectional(conn, clusterConn, buffer1, buffer2)

	return nil
}

func (r *Redis) getTwoBuffers() (buffer1, buffer2 []byte, err error) {
	buffer1 = r.bufferPool.Get()
	buffer2 = r.bufferPool.Get()
	if buffer1 == nil || buffer2 == nil {
		if buffer1 != nil {
			r.bufferPool.Put(buffer1)
		}
		if buffer2 != nil {
			r.bufferPool.Put(buffer2)
		}
		err = fmt.Errorf("bufferPool exhausted, consider increasing NumberOfBuffers")
		return
	}
	return
}

func localToRemoteHostAndPort(concurrent *ip_map.Concurrent, localAddr ip_map.HostWithPort) (remoteAddr ip_map.HostWithPort, err error) {
	var ok bool
	remoteAddr, ok = concurrent.LocalToRemote(localAddr)
	if !ok {
		err = fmt.Errorf("unable to find local to cluster mapping for localAddr: %s", localAddr.String())
		return
	}
	return
}

func remoteToLocalHostAndPort(concurrent *ip_map.Concurrent, remoteAddr ip_map.HostWithPort) (localAddr ip_map.HostWithPort, err error) {
	var ok bool
	localAddr, ok = concurrent.RemoteToLocal(remoteAddr)
	if !ok {
		err = fmt.Errorf("unable to find local to cluster mapping for remoteAddr: %s", remoteAddr.String())
		return
	}
	return
}

func readRedisToStatement(conn net.Conn, buffer []byte) (statements []string, err error) {
	var readCount int
	readCount, err = conn.Read(buffer)
	if err != nil && io.EOF != err {
		log.Println("io error while reading local socket: " + err.Error())
		return
	}

	buffer = buffer[0:readCount]
	return strings.Split(string(buffer), "\r\n"), err
}

func isClusterSlotsQuery(statements []string) bool {
	return len(statements) > 5 && statements[2] == "CLUSTER" && statements[4] == "slots"
}
