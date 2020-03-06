package main

import (
	"github.com/urfave/cli"
	"log"
	"os"
	"os/signal"
	"redis_cluster_proxy/pkg/ip_map"
	"redis_cluster_proxy/pkg/port_pool"
	"redis_cluster_proxy/pkg/proxy"
	"syscall"
)

const (
	ListenAddrFlagName               = "listenAddr"
	ClusterAddrFlagName              = "clusterAddr"
	PublicHostFlagName               = "publicHost"
	NumberOfBuffersFlagName          = "numberOfBuffers"
	MaxConcurrentConnectionsFlagName = "maxConcurrentConnections"
	ReadBufferByteSizeFlagName       = "readBufferByteSize"
	EnableDebuggingFlagName          = "debug"
)

func buildArguments() *cli.App {
	app := cli.NewApp()
	app.Commands = []cli.Command{
		{
			Name:        "server",
			Usage:       "proxy local redis cluster requests to a private cluster",
			UsageText:   "server -listenAddr LOCAL_ADDR:LOCAL_PORT -clusterAddr CLUSTER_ADDR:CLUSTER_PORT",
			Description: "launches a proxy server that translates local ip addresses to the cluster-private IP addresses for a redis cluster",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     ListenAddrFlagName,
					EnvVar:   "LISTEN_ADDR",
					Required: true,
					Usage:    "HOST_OR_IP:PORT this is the first hostname/ipv4 and port to start listening for incoming connections from redis clients",
				},
				cli.StringFlag{
					Name:     ClusterAddrFlagName,
					EnvVar:   "CLUSTER_ADDR",
					Required: true,
					Usage:    "HOST_OR_IP:PORT this is how the proxy knows how to contact the cluster. This should be the address of any node in the cluster, the other nodes will be discovered by the proxy",
				},
				cli.StringFlag{
					Name:     PublicHostFlagName,
					EnvVar:   "PUBLIC_HOST",
					Required: true,
					Usage:    "HOST_OR_IP of the public address of the proxy. Clients connecting to the proxy will use this address to route traffic to the proxy",
				},
				cli.IntFlag{
					Name:     NumberOfBuffersFlagName,
					EnvVar:   "NUM_BUFFERS",
					Required: false,
					Value:    100,
					Usage:    "[100] The number of buffers to allocate to the proxy for reading requests from clients and responses from servers",
				},
				cli.IntFlag{
					Name:     MaxConcurrentConnectionsFlagName,
					EnvVar:   "MAX_CONNECTIONS",
					Required: false,
					Value:    100,
					Usage:    "Not activate at this time",
				},
				cli.IntFlag{
					Name:     ReadBufferByteSizeFlagName,
					EnvVar:   "BUF_SIZE_BYTES",
					Required: false,
					Value:    16384, // 16KB
					Usage:    "[16384] the number of bytes that the read buffers are created with. Set this to the maximum size of any bulk string you need to send",
				},
				cli.BoolFlag{
					Name:     EnableDebuggingFlagName,
					Usage:    "specify this flag to enable verbose output so you can see messages that the proxy intercepts and sends back out",
					EnvVar:   "DEBUG",
					Required: false,
				},
			},
			Action: func(c *cli.Context) (err error) {
				var redisProxy *proxy.Redis
				redisProxy, err = newProxy(
					c.String(ListenAddrFlagName),
					c.String(ClusterAddrFlagName),
					c.String(PublicHostFlagName),
					c.Int(NumberOfBuffersFlagName),
					c.Int(MaxConcurrentConnectionsFlagName),
					c.Int(ReadBufferByteSizeFlagName),
				)

				if err != nil {
					log.Print(err)
				}

				redisProxy.SetDebug(c.Bool(EnableDebuggingFlagName))

				// Discovers the cluster ips and ports
				err = redisProxy.DiscoverAndListen()
				if err != nil {
					log.Fatal(err)
				}

				// print the status to the stdout so that people can see what's going on
				err = redisProxy.PrintConnectionStatuses(os.Stdout)
				if err != nil {
					log.Fatal(err)
				}

				exitChan := make(chan os.Signal, 1)
				signal.Notify(exitChan, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM)
				<-exitChan
				err = redisProxy.Close()
				if err != nil {
					log.Fatal(err)
				}
				return nil
			},
		},
	}
	return app
}

func newProxy(listenAddr, clusterAddr, publicHostname string, numberOfBuffers int, maxConcurrentConnections int, readBufferByteSize int) (redisProxy *proxy.Redis, err error) {

	listenHostWithPort, err := ip_map.NewHostWithPortFromString(listenAddr)
	if err != nil {
		return
	}
	clusterHostWithPort, err := ip_map.NewHostWithPortFromString(clusterAddr)
	if err != nil {
		return
	}

	portKeeper := port_pool.NewCounter(listenHostWithPort.Port)

	return proxy.NewRedis(listenHostWithPort, clusterHostWithPort, publicHostname, portKeeper, numberOfBuffers, maxConcurrentConnections, readBufferByteSize), nil
}
