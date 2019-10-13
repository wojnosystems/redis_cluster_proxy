FROM golang:1.13.1-alpine3.10 AS DEPENDENCIES
WORKDIR /tmp/build
COPY go.mod .
COPY go.sum .
RUN go mod download

FROM DEPENDENCIES AS BUILDER
COPY cmd cmd
COPY pkg pkg
RUN mkdir -p target
RUN cd cmd/proxy && \
    CGO_ENABLED=0 go test -run="" /tmp/build/pkg/* && \
    go build -i -o /tmp/build/target/redisClusterProxyServer .

FROM alpine:3.10.2
WORKDIR /svc/redis-cluster-proxy
ENV LISTEN_ADDR=":8000"
ENV CLUSTER_ADDR="cluster:7000"
ENV START_DELAY="0s"
ENV PUBLIC_HOST="127.0.0.1"
COPY docker/docker-entry-point.sh docker-entry-point.sh
COPY --from=BUILDER /tmp/build/target/redisClusterProxyServer .
CMD ["./docker-entry-point.sh"]