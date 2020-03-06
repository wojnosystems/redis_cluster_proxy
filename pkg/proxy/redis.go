package proxy

import (
	"bytes"
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
	"time"
)

type Redis struct {
	listenAddr             ip_map.HostWithPort
	clusterAddr            ip_map.HostWithPort
	publicHostname         string
	ipMap                  *ip_map.Concurrent
	portCounter            port_pool.Counter
	listenIPs              []net.IP
	clusterIPs             []net.IP
	facadeClusterSlotsResp []redisPkg.ClusterSlotResp
	facadeClusterNodesResp []redisPkg.ClusterNodeResp
	listeners              []net.Listener
	buffers                *bufferPool
	readBufferByteSize     int
	debugOutputEnabled     bool
}

func NewRedis(listenAddr, clusterAddr ip_map.HostWithPort, publicHostname string, portKeeper port_pool.Counter, numberOfBuffers int, maxConcurrentConnections int, readBufferByteSize int) (redis *Redis) {
	ret := &Redis{
		listenAddr:         listenAddr,
		clusterAddr:        clusterAddr,
		publicHostname:     publicHostname,
		ipMap:              ip_map.NewConcurrent(),
		portCounter:        portKeeper,
		readBufferByteSize: readBufferByteSize,
		listeners:          make([]net.Listener, 0, 6),
		buffers:            newBufferPool(numberOfBuffers, readBufferByteSize),
	}

	return ret
}

var MaxConnectRetries = retry.MaxAttempts{
	Times:   30,
	WaitFor: 2 * time.Second,
}

const ClusterSlotsDiscoverStatement = "*2\r\n$7\r\nCLUSTER\r\n$5\r\nslots\r\n"
const ClusterNodeDiscoverStatement = "*2\r\n$7\r\nCLUSTER\r\n$5\r\nNODES\r\n"

// DiscoverAndListen contacts the cluster and gets the list of nodes, then establishes proxies for each node
// When contacting redis, we get back a list of server addresses WITHIN the cluster These are private addresses)
// Each of these needs to be recorded in the map. A new port needs to be created locally and that port will map to the remote address
// Suppose we're listening on 127.0.0.1:8000 - 8005. And the Redis Cluster listens on 172.20.0.2:7000 - 7005.
// When a request comes into 127.0.0.1, it needs to be forwarded to 172.20.0.2. The ports don't really matter so long as they are consistent.
// When a client requests the mapping, we should respond with our own, internal mapping, based on the "listenAddr".
func (r *Redis) DiscoverAndListen() (err error) {
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

	{
		_, err = cluster.Write([]byte(ClusterSlotsDiscoverStatement))
		if err != nil && io.EOF != err {
			log.Println("io error while writing to cluster socket: " + err.Error())
			return err
		}

		buffer := r.buffers.Get()
		if buffer == nil {
			err = fmt.Errorf("ran out of buffers")
			return err
		}
		defer r.buffers.Put(buffer)
		responseComponent, _, err := redisPkg.ComponentFromReader(cluster, buffer)
		if err != nil && io.EOF != err {
			log.Println("io error while reading cluster socket: " + err.Error())
			return err
		}
		r.facadeClusterSlotsResp, err = redisPkg.NewSlotArrayFromComponent(responseComponent)
		if err != nil {
			log.Println("Unable to read the cluster response: " + err.Error())
			return err
		}
	}

	{
		_, err = cluster.Write([]byte(ClusterNodeDiscoverStatement))
		if err != nil && io.EOF != err {
			log.Println("io error while writing to cluster socket: " + err.Error())
			return err
		}

		buffer := r.buffers.Get()
		if buffer == nil {
			err = fmt.Errorf("ran out of buffers")
			return err
		}
		defer r.buffers.Put(buffer)
		responseComponent, _, err := redisPkg.ComponentFromReader(cluster, buffer)
		if err != nil && io.EOF != err {
			log.Println("io error while reading cluster socket: " + err.Error())
			return err
		}
		r.facadeClusterNodesResp, err = redisPkg.NewClusterNodesRespFromComponent(responseComponent)
		if err != nil {
			log.Println("Unable to read the cluster response: " + err.Error())
			return err
		}
	}

	serverAddrs := serverAddrAndPortFromSlotResp(r.facadeClusterSlotsResp)
	for _, serverAddr := range serverAddrs {
		if _, exists := r.ipMap.RemoteToLocal(serverAddr); exists {
			// already exists, do nothing
		} else {
			// No mapping exists, open a socket to service it
			var nodeListener net.Listener
			newListenerAddr := ip_map.HostWithPort{
				Host: r.listenAddr.Host, // we don't want to bind to any hostname, bind to all available interfaces
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
			r.listeners = append(r.listeners, nodeListener)
			// create the mapping entry for the new socket
			hostAndPort, _ := ip_map.NewHostWithPortFromString(nodeListener.Addr().String())
			r.ipMap.Create(serverAddr, hostAndPort.Port)

			go func(nodeListener net.Listener, newListenerAddr ip_map.HostWithPort) {
				_ = listenLoop(nodeListener, r, newListenerAddr)
			}(nodeListener, newListenerAddr)
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
	keys := make([]uint16, 0, len(localToRemotes))
	for local := range localToRemotes {
		keys = append(keys, local)
	}
	// sort the keys for easier reading
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] <= keys[j]
	})
	for _, local := range keys {
		// This is being reported so that people connecting to the cluster, in case they need to debug
		_, err = fmt.Fprintf(writer, "Listening on: %d proxy to: %s\n", local, localToRemotes[local].String())
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

func (r *Redis) Close() (err error) {
	for _, listener := range r.listeners {
		closeErr := listener.Close()
		if err == nil {
			err = closeErr
		}
	}
	return
}

func (r *Redis) SetDebug(enabled bool) {
	r.debugOutputEnabled = enabled
}

func listenLoop(listener net.Listener, r *Redis, localAddr ip_map.HostWithPort) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		go func(conn net.Conn) {
			err = proxyConnection(conn, r, localAddr)
			if err != nil {
				log.Println(err)
			}
		}(conn)
	}
}

func proxyConnection(conn net.Conn, r *Redis, localAddr ip_map.HostWithPort) (err error) {
	defer func() { _ = conn.Close() }()
	clusterAddr, err := localToRemoteHostAndPort(r.ipMap, localAddr.Port)
	if err != nil {
		return
	}

	var clusterConn net.Conn
	clusterConn, err = net.Dial("tcp", clusterAddr.String())
	if err != nil {
		return
	}
	defer func() { _ = clusterConn.Close() }()

	buffer1, buffer2, err := allocateBufferPair(r.buffers)
	if err != nil {
		return
	}
	defer func() {
		r.buffers.Put(buffer1)
		r.buffers.Put(buffer2)
	}()

	doneChan := make(chan error, 2)

	Bidirectional(conn, clusterConn, func(componenterIn redisPkg.Componenter) (componenterOut redisPkg.Componenter) {
		if componenterOut = mutateClusterSlotsCommand(componenterIn, r.facadeClusterSlotsResp, r.publicHostname, r.ipMap); nil != componenterOut {
			return
		}
		if componenterOut = mutateClusterNodesCommand(componenterIn, r.facadeClusterNodesResp, r.publicHostname, r.ipMap); nil != componenterOut {
			return
		}
		// nil means no interception, pass the query through
		return nil
	}, func(componenterIn redisPkg.Componenter) (componenterOut redisPkg.Componenter) {
		if componenterOut = mutateMovedCommand(r, componenterIn); nil != componenterOut {
			return
		}
		// no changes
		return componenterIn
	}, buffer1, buffer2, doneChan, r.debugOutputEnabled)

	return <-doneChan
}

func allocateBufferPair(bufferPool *bufferPool) (buffer1, buffer2 []byte, err error) {
	buffer1 = bufferPool.Get()
	if buffer1 == nil {
		return nil, nil, fmt.Errorf("ran out of buffers")
	}
	buffer2 = bufferPool.Get()
	if buffer2 == nil {
		bufferPool.Put(buffer1)
		return nil, nil, fmt.Errorf("ran out of buffers")
	}
	return
}

func mutateMovedCommand(r *Redis, componenterIn redisPkg.Componenter) (componenterOut redisPkg.Componenter) {
	if parts, moved := isMoved(componenterIn); moved {
		remoteFromCluster, _ := ip_map.NewHostWithPortFromString(parts[2])
		newLocal, ok := r.ipMap.RemoteToLocal(remoteFromCluster)
		if !ok {
			log.Println("no mapping from cluster address: " + remoteFromCluster.String())
			return nil
		}
		translatedAddr := ip_map.HostWithPort{
			Host: r.publicHostname,
			Port: newLocal,
		}
		re := redisPkg.ErrorComp(fmt.Sprintf("MOVED %s %s", parts[1], translatedAddr.String()))
		return &re
	}
	return nil
}

func mutateClusterSlotsCommand(componenterIn redisPkg.Componenter, clusterSlotResp []redisPkg.ClusterSlotResp, publicHostname string, lookup *ip_map.Concurrent) (componenterOut redisPkg.Componenter) {
	if isClusterSlotsQuery(componenterIn) {
		slotResponse := redisPkg.NewClusterSlotRespFromClusterSlotRespArray(clusterSlotResp)
		for slotIndex := range slotResponse {
			for serverIndex := range slotResponse[slotIndex].Servers() {
				clusterAddr := ip_map.HostWithPort{Host: slotResponse[slotIndex].Servers()[serverIndex].Ip(), Port: slotResponse[slotIndex].Servers()[serverIndex].Port()}
				localAddr, ok := lookup.RemoteToLocal(clusterAddr)
				if !ok {
					log.Println("no mapping from cluster address: " + clusterAddr.String())
				}
				// Lie to the client
				slotResponse[slotIndex].Servers()[serverIndex].SetIp(publicHostname)
				slotResponse[slotIndex].Servers()[serverIndex].SetPort(localAddr)
			}
		}
		return redisPkg.ClusterSlotArrayRespToComponent(slotResponse)
	}
	return nil
}

func isClusterSlotsQuery(statements redisPkg.Componenter) bool {
	if array, ok := statements.(*redisPkg.Array); !ok {
		return false
	} else {
		if command, ok := (*array)[0].(*redisPkg.BulkString); !ok {
			return false
		} else {
			if !strings.EqualFold(command.String(), "CLUSTER") {
				return false
			}
		}
		if command, ok := (*array)[1].(*redisPkg.BulkString); !ok {
			return false
		} else {
			if !strings.EqualFold(command.String(), "SLOTS") {
				return false
			}
		}
		return true
	}
}

func mutateClusterNodesCommand(componenterIn redisPkg.Componenter, clusterNodeResp []redisPkg.ClusterNodeResp, publicHostname string, lookup *ip_map.Concurrent) (componenterOut redisPkg.Componenter) {
	if isClusterNodesQuery(componenterIn) {
		nodes := redisPkg.NewClusterNodeRespFromClusterNodeRespArray(clusterNodeResp)
		for nodeIndex := range nodes {
			clusterAddr := ip_map.HostWithPort{Host: nodes[nodeIndex].Ip(), Port: nodes[nodeIndex].Port()}
			localAddr, ok := lookup.RemoteToLocal(clusterAddr)
			if !ok {
				log.Println("no mapping from cluster address: " + clusterAddr.String())
			}
			// Lie to the client
			nodes[nodeIndex].SetIp(publicHostname)
			nodes[nodeIndex].SetPort(localAddr)
		}
		return redisPkg.ClusterNodeArrayToComponent(nodes)
	}
	return nil
}

func isClusterNodesQuery(statements redisPkg.Componenter) bool {
	if array, ok := statements.(*redisPkg.Array); !ok {
		return false
	} else {
		if command, ok := (*array)[0].(*redisPkg.BulkString); !ok {
			return false
		} else {
			if !strings.EqualFold(command.String(), "CLUSTER") {
				return false
			}
		}
		if command, ok := (*array)[1].(*redisPkg.BulkString); !ok {
			return false
		} else {
			if !strings.EqualFold(command.String(), "NODES") {
				return false
			}
		}
		return true
	}
}

func localToRemoteHostAndPort(concurrent *ip_map.Concurrent, localPort uint16) (remoteAddr ip_map.HostWithPort, err error) {
	var ok bool
	remoteAddr, ok = concurrent.LocalToRemote(localPort)
	if !ok {
		err = fmt.Errorf("unable to find remote to cluster mapping for local Port: %d", localPort)
		return
	}
	return
}

func remoteToLocalHostAndPort(concurrent *ip_map.Concurrent, remoteAddr ip_map.HostWithPort) (localPort uint16, err error) {
	var ok bool
	localPort, ok = concurrent.RemoteToLocal(remoteAddr)
	if !ok {
		err = fmt.Errorf("unable to find local to cluster mapping for remoteAddr: %s", remoteAddr.String())
		return
	}
	return
}

func debugClientIn(label string, debugOutputEnabled bool, componenter redisPkg.Componenter) {
	if debugOutputEnabled {
		buffer := &bytes.Buffer{}
		_, _ = redisPkg.ComponentToStream(buffer, componenter)
		log.Println(label + ": \"" + strings.ReplaceAll(buffer.String(), "\r\n", "\\r\\n") + "\"")
	}
}

func isMoved(componenter redisPkg.Componenter) (parts []string, moved bool) {
	if e, ok := componenter.(*redisPkg.ErrorComp); !ok {
		return []string{}, false
	} else {
		parts := strings.Split(e.String(), " ")
		if len(parts) <= 1 {
			return []string{}, false
		}
		if parts[0] == "MOVED" {
			return parts, true
		}
	}
	return []string{}, false
}
