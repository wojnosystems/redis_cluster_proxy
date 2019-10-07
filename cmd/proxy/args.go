package main

import (
	"github.com/urfave/cli"
	"log"
	"os"
	"redis_cluster_proxy/pkg/ip_map"
	"redis_cluster_proxy/pkg/port_pool"
	"redis_cluster_proxy/pkg/proxy"
)

const (
	UIntMax = 65535
)

const (
	ListenAddrName  = "listenAddr"
	ClusterAddrName = "clusterAddr"
	PortStartName   = "portStart"
)

func buildArguments() *cli.App {
	app := cli.NewApp()
	app.Commands = []cli.Command{
		{
			Name:        "server",
			Usage:       "proxy local redis cluster requests to a private cluster",
			UsageText:   "server -listenAddr=LOCAL_ADDR:LOCAL_PORT -clusterAddr=CLUSTER_ADDR:CLUSTER_PORT",
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
			},
			Action: func(c *cli.Context) (err error) {
				listenHostWithPort, err := ip_map.NewHostWithPortFromString(c.String(ListenAddrName))
				if err != nil {
					return
				}
				clusterHostWithPort, err := ip_map.NewHostWithPortFromString(c.String(ClusterAddrName))
				if err != nil {
					return
				}

				portKeeper := port_pool.NewCounter(listenHostWithPort.Port)

				redisProxy := proxy.NewRedis(listenHostWithPort, clusterHostWithPort, portKeeper)

				// Discovers the cluster ips and ports
				err = redisProxy.DiscoverAndListen(clusterHostWithPort)
				if err != nil {
					log.Fatal(err)
				}

				// print the status to the stdout so that people can see what's going on
				err = redisProxy.PrintConnectionStatuses(os.Stdout)
				if err != nil {
					log.Fatal(err)
				}

				log.Fatal(redisProxy.WaitUntilAddConnectionsComplete())
				return nil
			},
		},
	}
	return app
}
