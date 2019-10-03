# Overview

A proxy for a Redis-Cluster.

# Purpose

I needed a Redis Cluster with at least 3 master nodes running in a Docker cluster as I was testing the JedisCluster (Java redis cluster SDK client). However, because Redis uses IP addresses when connecting to the cluster from a client and those IP addresses aren't routable outside of the cluster, it is not possible to access a redis cluster directly. However, by using a proxy with the ability to rename IP addresses, it is possible to support external connections.

# Copyright

Chris Wojno 2019. All rights Reserved

# License

![cc logo](https://i.creativecommons.org/l/by-sa/4.0/88x31.png)

This work is licensed under a Creative Commons Attribution-ShareAlike 4.0 International License.

# Disclaimer

This is not production code. Don't use it in production. Use at your own risk. Not responsible for anything. You've been warned.