# What it does

A proxy for a [Redis Cluster](https://redis.io/topics/cluster-tutorial) written in Go. A way to by-pass the lack of NAT support in Redis Clusters.

This is, essentially, a Redis Cluster-aware NAT protocol.

# Using it

## Standalone

```bash
./redisClusterProxyServer server -listenAddr :8000 -clusterAddr cluster:7000
```

Where ports 8000 - 8005 and 7000 - 7005 are exposed in your docker-compose.yaml file. cluster is the service name of your [Redis Cluster](https://redis.io/topics/cluster-tutorial). See the docker-compose.yaml file for an example.

## Using the Docker-compose file

There is a docker-compose.yaml in the root of the project. It using [grokzen's Redis Cluster](https://github.com/Grokzen/docker-redis-cluster) [docker image](https://hub.docker.com/r/grokzen/redis-cluster/) to test.

```bash
docker-compose up
```

Target your Redis Cluster client to: `127.0.0.1:8000`. This is actually the proxy. Use the docker-compose.yaml as a template for your projects.

### Commandline Flags / Environment

 * **listenAddr**/**LISTEN_ADDR**: is the HOST_OR_IP:PORT that the proxy should listen to. This needs to be a private address or just leave the host part blank to use all available. The port must be free and for every node in the cluster of size n, the next n - 1 ports need to be available as the proxy will start listening for client connections on PORT, PORT+1, PORT+2... PORT+(n - 1)
 * **clusterAddr**/**CLUSTER_ADDR**: This is the HOST_OR_IP:PORT of any node in the cluster. The other nodes will be auto-discovered
 * **publicHost**/**PUBLIC_HOST**: This is the HOST or IP (without port) of the proxy. Redis clients connecting to the proxy will be given this host so that they can dial back to the proxy
 * **numberOfBuffers**/**NUM_BUFFERS**: how many string buffers to allocate. Each connection to the proxy uses 2 buffers 
 * **readBufferByteSize**/**BUF_SIZE_BYTES**: the size of the buffers. This should be set to the number of bytes of your largest Bulk String AKA your largest value stored in Redis
 * **debug**: set this flag to enable verbose debugging. This will echo all communications through the proxy. This is extremely useful for testing. 

### More on the setup

```
                  PUBLIC_HOST        LISTEN_ADDR       CLUSTER_ADDR
+-----------+     +------------+     +-------+         +------------+
| Redis-cli | <-> | NAT/Docker | <-> | Redis | <--+--> | Redis Node |
+-----------+     | network    |     | Proxy |    |    +------------+
                  +------------+     +-------+    |    +------------+
                                                  +--> | Redis Node |
                                                       +------------+
                                                             ...
```

Suppose you have a [Redis Cluster](https://redis.io/topics/cluster-tutorial) with 3 masters and 3 replicas with the following IP addresses:

* 172.23.0.2:7000 (master 1)
* 172.23.0.2:7004 (replica 1)
* 172.23.0.2:7002 (master 2)
* 172.23.0.2:7003 (replica 2)
* 172.23.0.2:7001 (master 3)
* 172.23.0.2:7005 (replica 3)

If you run the redisClusterProxy in that same address block, say at IP address: 172.23.0.3 and create a routable "public" ip address (if using docker, any local IP will work), you can start the proxy using the following settings:

When you use a [Redis Cluster](https://redis.io/topics/cluster-tutorial) client such as [Jedis](https://github.com/xetorthio/jedis) and all of the RedisCluster nodes have public IPs, then Jedis will connect to the master, download the IPs, ports, and slot ranges for each server and then create direct connections to each node. This fails when your cluster nodes do not have publicly routable IP addresses, however.

When using the redisClusterProxy with a client such as [Jedis](https://github.com/xetorthio/jedis), you configure Jedis to connect to the proxy at an accessible address such as: 127.0.0.1:8000. The client will download the ip addresses and ports of the other nodes in the cluster, including the replicas.

This reason this library exists is because if you want to connect to the cluster through a NAT or firewall where the nodes do not have public IP addresses, when the client connects, the client will get the private IP addresses and not the public addresses. When the client attempts to send a query to a node, this will fail because the IP is not routable as this is a private IP.

The proxy works by creating local sockets to which the client will connect. The proxy intercepts the IP addresses and translates them to the listenAddr network.

If you're running in a docker-compose cluster (or any cluster, such as Kubernetes), you can start the redisClusterProxy and expose the number of ports needed to match your cluster. For example, the above cluster uses ports 7000-7005, so we need to open 6 ports on our proxy as well.

You need to indicate which port to start listening to using the `-listenAddr` flag / `LISTEN_ADD` environment variable. This is the public address of the redisClusterProxy. If you're using docker-compose, this should only include the ":PORT" where PORT is the starting port that you've exposed.

You also need to specify the `-clusterAddr` flag / `CLUSTER_ADDR` environment variable. This is the private dns hostname OR cluster-internal IP address (this is the non-public address/hostname). In the above example, I chose: "172.23.0.2:7000" but any node can serve as the address.

Note, that when starting the proxy, the cluster must already be running and initialized as a cluster. To facilitate this, the docker-entry-point.sh file has a 10s wait time. Change the `START_DELAY` environment variable on the docker image if you need more or want to wait less time for the [Redis Cluster](https://redis.io/topics/cluster-tutorial) to start. If you don't wait long enough, the redisClusterProxy may fail or will not obtain a complete picture of the cluster when it starts, thus, only a few nodes will be accounted for and you need all nodes to be included.

When the redisClusterProxy starts, it contacts the cluster and asks for all of the slots using the CLUSTER:slots command. This triggers the redisClusterProxy to spawn new ports to listen for connections. These will start at the `-listenAddr`/`LISTEN_ADDR` port and will increment by 1 for each server that is in your [Redis Cluster](https://redis.io/topics/cluster-tutorial). This is why if you're listening on: `-listenAddr :8000` you should open ports: 8000 through 8005.

To help debug your configuration, when the proxy starts, it will print the mapping, such as below:

```
Listening on: :8000 proxy to: 172.23.0.2:7002
Listening on: :8001 proxy to: 172.23.0.2:7003
Listening on: :8002 proxy to: 172.23.0.2:7001
Listening on: :8003 proxy to: 172.23.0.2:7005
Listening on: :8004 proxy to: 172.23.0.2:7000
Listening on: :8005 proxy to: 172.23.0.2:7004
```

Whenever a RedisCluster client connects to the proxy, the proxy will lie to it ;). Instead of sending the client the actual node IPs and ports, which are un-routable local addresses, it sends the client the IP and port of the proxy. Because the proxy is lying to the client, everything will magically work.

# Purpose

I needed a [Redis Cluster](https://redis.io/topics/cluster-tutorial) with at least 3 master nodes running in a Docker cluster as I was testing the JedisCluster (Java redis cluster SDK client). However, because Redis uses IP addresses when connecting to the cluster from a client and those IP addresses aren't routable outside of the cluster, it is not possible to access a redis cluster directly. However, by using a proxy with the ability to rename IP addresses, it is possible to support external connections.

Read up on [additional history](https://github.com/antirez/redis/issues/2527) on this problem.

# Caveats

## Lack of testing

I wrote this library in a few days of spare time. Testing is limited. This should support the entire [Redis Protocol](https://redis.io/topics/protocol), but I have NOT tested it.

I am only making a single Redis request to gather the cluster nodes to establish the proxies. The rest of the data is forwarded blindly.

## Lack of timeouts

There are no timeouts. Again, don't use this in production.

## Lack of Online cluster resizing

If you add or remove a shard from your cluster, this proxy doesn't do anything about that. You should probably start a new proxy pointing to the same hosts and then point your application to use the new proxy should you need an online resize. But you're not using this library for production, right?

# Future Work

Break up the bi-directional proxy and make them aware of which side they're proxying traffic for. As of this writing, every request is de-serialized and interpreted by the proxy. We don't really need to do this. We need to listen for CLUSTER slots and MOVED command and error, respectively. Those messages need to be intercepted and re-written. Other messages should be passed without a full de-serialization.

# Copyright

Chris Wojno 2019. All rights Reserved

# License

![cc logo](https://i.creativecommons.org/l/by-sa/4.0/88x31.png)

This work is licensed under a Creative Commons Attribution-ShareAlike 4.0 International License.

# Disclaimer

This is not production code. Don't use it in production. Use at your own risk. Author and contributors are not responsible for anything. You've been warned.

Neither I, nor this project, is affiliated with or endorsed by [Redis](https://redis.io), [Jedis](https://github.com/xetorthio/jedis), or [grokzen](https://github.com/Grokzen).

# Thanks

Thank you to [grokzen](https://github.com/Grokzen) for making a very cool Redis Cluster docker image!