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
	ListenAddrName               = "listenAddr"
	ClusterAddrName              = "clusterAddr"
	PublicHostnameName           = "publicHostname"
	NumberOfBuffersName          = "numberOfBuffers"
	MaxConcurrentConnectionsName = "maxConcurrentConnections"
	ReadBufferByteSizeName       = "readBufferByteSize"
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
					Name:     ListenAddrName,
					EnvVar:   "LISTEN_ADDR",
					Required: true,
				},
				cli.StringFlag{
					Name:     ClusterAddrName,
					EnvVar:   "CLUSTER_ADDR",
					Required: true,
				},
				cli.StringFlag{
					Name:     PublicHostnameName,
					EnvVar:   "PUBLIC_HOSTNAME",
					Required: true,
				},
				cli.IntFlag{
					Name:     NumberOfBuffersName,
					EnvVar:   "NUM_BUFFERS",
					Required: false,
					Value:    1000,
				},
				cli.IntFlag{
					Name:     MaxConcurrentConnectionsName,
					EnvVar:   "MAX_CONNECTIONS",
					Required: false,
					Value:    100,
				},
				cli.IntFlag{
					Name:     ReadBufferByteSizeName,
					EnvVar:   "BUF_SIZE_BYTES",
					Required: false,
					Value:    4096,
				},
			},
			Action: func(c *cli.Context) (err error) {
				var redisProxy *proxy.Redis
				redisProxy, err = newProxy(
					c.String(ListenAddrName),
					c.String(ClusterAddrName),
					c.String(PublicHostnameName),
					c.Int(NumberOfBuffersName),
					c.Int(MaxConcurrentConnectionsName),
					c.Int(ReadBufferByteSizeName),
				)

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
				signal.Notify(exitChan, syscall.SIGINT, syscall.SIGKILL)
				<-exitChan
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
