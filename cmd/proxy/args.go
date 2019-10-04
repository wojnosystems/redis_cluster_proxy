package main

import (
	"github.com/urfave/cli"
	"log"
	"redis_cluster_proxy/pkg/ip_map"
	"redis_cluster_proxy/pkg/proxy"
)

func buildArguments() *cli.App {
	app := cli.NewApp()
	app.Commands = []cli.Command{
		{
			Name:        "server",
			Usage:       "proxy local redis cluster requests to a private cluster",
			UsageText:   "server -master REDIS_CLUSTER_PRIVATE_IP:REDIS_PORT",
			Description: "launches a proxy server that translates local ip addresses to the cluster-private IP addresses for a redis cluster",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "listenAddr",
					EnvVar:   "LISTEN_ADDR",
					Required: true,
				},
				cli.StringFlag{
					Name:     "clusterAddr",
					EnvVar:   "CLUSTER_ADDR",
					Required: true,
				},
				cli.UintFlag{
					Name:   "portStart",
					Usage:  "[-portStart 7001]",
					EnvVar: "PORT_START",
				},
			},
			Action: func(c *cli.Context) (err error) {
				listenHostWithPort, err := ip_map.NewHostWithPortFromString(c.String("listenAddr"))
				if err != nil {
					return
				}
				clusterHostWithPort, err := ip_map.NewHostWithPortFromString(c.String("clusterAddr"))
				if err != nil {
					return
				}

				log.Fatal(proxy.NewRedis(listenHostWithPort, clusterHostWithPort).ListenAndServe())
				return nil
			},
		},
	}
	return app
}
