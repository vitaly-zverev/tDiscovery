# tDiscovery

Для чего все это здесь:
```
talosctl get machineconfig --cluster demo --nodes 10.5.0.2 -o json | jq .spec | yq . | yq .cluster.discovery.registries.service
endpoint: http://192.168.0.119:3001 # External service endpoint.  <--- Все ради вот этого 
```

Не стоит переоценивать этот R&D, он не является заменой промышленному https://discovery.talos.dev/, и, если Вы можете приобрести 
полноценный сервисный пакет для https://github.com/siderolabs/discovery-service, то так и стоит поступить.
Этот R&D скетч появился как альтернатива https://discovery.talos.dev/ в airgap проектах, где нет возможности приобрести сервисный пакет Omni, 
доступ к discovery.talos.dev не возможен, а динамика в обнаружении узлов кластера требуется (в частности, это требование для KubeSpan)

Быстрый старт для разработки:

Пререквизиты:
1) protoc
https://protobuf.dev/installation/ 

2) protoc-gen-go && protoc-gen-go-grpc
```
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

```
 git clone https://github.com/siderolabs/discovery-api 
 git clone https://github.com/vitaly-zverev/tDiscovery && cd tDiscovery
 export PATH="$PATH:$(go env GOPATH)/bin"
```
Собрать:
```
make
protoc -I ../discovery-api/api --proto_path=../discovery-api  --go_out=../discovery-api  --go-grpc_out=../discovery-api --go_opt=paths=source_relative  --go-grpc_opt=paths=source_relative  api/v1alpha1/server/cluster.proto
GOOS=linux GOARCH=amd64 go build -o _out/tdiscovery main.go
go: downloading github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10
go: downloading golang.org/x/net v0.35.0
go: downloading google.golang.org/genproto/googleapis/rpc v0.0.0-20250218202821-56aae31c358a
go: downloading golang.org/x/sys v0.30.0
go: downloading golang.org/x/text v0.22.0

```
Очистить:
```
make clean
```
Кросс-компиляция (например, для Linux x86_64):
```
make OS=linux ARCH=amd64
```
Примеры запуска:
```
 _out/tdiscovery
2025/05/29 23:23:40 gRPC server listening on :3001 (GC interval: 15s)
2025/05/29 23:23:55 garbage collection run  {"removed_clusters": 0, "removed_affiliates": 0, "current_clusters": 0, "current_affiliates": 0, "current_endpoints": 0,"current_subscriptions": 0}

go run main.go --gc-interval=30s --port=7000
2025/05/29 23:12:50 gRPC server listening on :7000 (GC interval: 30s)

go run main.go
2025/05/29 23:12:41 gRPC server listening on :3001 (GC interval: 15s)

Примеры запросов к сервису:

grpcurl -proto v1alpha1/server/cluster.proto -import-path ../discovery-api/api -plaintext -d '{"clusterId": "xyz"}' -H 'X-Real-IP: 1.2.3.4' localhost:3001 sidero.discovery.server.Cluster/Hello | jq .clientIp -r | base64 --decode | od -h --endian big

grpcurl -proto v1alpha1/server/cluster.proto -import-path ../discovery-api/api -plaintext -d '{"clusterId": "xyz","affiliateId":"def","ttl":"15s"}' -H 'X-Real-IP: 1.2.3.4' localhost:3001 sidero.discovery.server.Cluster/AffiliateUpdate

grpcurl -proto v1alpha1/server/cluster.proto -import-path ../discovery-api/api -plaintext -d '{"clusterId": "xyz"}' localhost:3001 sidero.discovery.server.Cluster/List | jq .

grpcurl -proto v1alpha1/server/cluster.proto -import-path ../discovery-api/api -plaintext -d '{"clusterId": "xyz"}' -H 'X-Real-IP: 1.2.3.4' localhost:3001 sidero.discovery.server.Cluster/Watch

seq 1 10 | xargs -I{} grpcurl -proto v1alpha1/server/cluster.proto -import-path ../discovery-api/api -plaintext -d "{\"clusterId\": \"xyz\",\"affiliateId\":\"def-{}\",\"affiliateData\":\"MTIzCg==\",\"affiliateEndpoints\":\"MTIzCg==\"}" -H 'X-Real-IP: 1.2.3.4' localhost:3001 sidero.discovery.server.Cluster/AffiliateUpdate
```

Пример запуска с Talos кластером (192.168.0.119 - адрес узла с tDicovery сервисом):

```
sudo --preserve-env=HOME talosctl cluster create --provisioner qemu --name demo --with-debug --config-patch '[{"op": "replace", "path": "/cluster/discovery/registries/service/endpoint", "value": "http://192.168.0.119:3001"}]'
validating CIDR and reserving IPs
generating PKI and tokens
creating state directory in "/home/vzverev/.talos/clusters/demo"
creating network demo
creating load balancer
creating controlplane nodes
creating dhcpd
creating worker nodes
waiting for API
bootstrapping cluster
waiting for etcd to be healthy: OK
waiting for etcd members to be consistent across nodes: OK
waiting for etcd members to be control plane nodes: OK
waiting for apid to be ready: OK
waiting for all nodes memory sizes: OK
waiting for all nodes disk sizes: OK
waiting for no diagnostics: OK
waiting for kubelet to be healthy: OK
waiting for all nodes to finish boot sequence: OK
waiting for all k8s nodes to report: OK
waiting for all control plane static pods to be running: OK
waiting for all control plane components to be ready: OK
waiting for all k8s nodes to report ready: OK
waiting for kube-proxy to report ready: OK
waiting for coredns to report ready: OK
waiting for all k8s nodes to report schedulable: OK

merging kubeconfig into "/home/vzverev/.kube/config"
renamed cluster "demo" -> "demo-1"
renamed auth info "admin@demo" -> "admin@demo-1"
renamed context "admin@demo" -> "admin@demo-1"
PROVISIONER           qemu
NAME                  demo
NETWORK NAME          demo
NETWORK CIDR          10.5.0.0/24
NETWORK GATEWAY       10.5.0.1
NETWORK MTU           1500
KUBERNETES ENDPOINT   https://10.5.0.1:6443

NODES:

NAME                  TYPE           IP         CPU    RAM      DISK
demo-controlplane-1   controlplane   10.5.0.2   2.00   2.1 GB   6.4 GB
demo-worker-1         worker         10.5.0.3   2.00   2.1 GB   6.4 GB

```
Журнал с консоли tdiscovery:

```
 _out/tdiscovery -gc-interval 5s
2025/05/30 18:12:42 gRPC server listening on :3001 (GC interval: 5s, Watch buffer size: :32 )
2025/05/30 18:12:44 Hello called: cluster_id="EvFJSSTLF-y-tSjUK8NUE20TRC2L8gShfVyXg5_MaDo=", client_version="v1.11.0-alpha.0-68-g5d0224093"
2025/05/30 18:12:44 Client IP from peer: 10.5.0.3
2025/05/30 18:12:44 Hello called: cluster_id="EvFJSSTLF-y-tSjUK8NUE20TRC2L8gShfVyXg5_MaDo=", client_version="v1.11.0-alpha.0-68-g5d0224093"
2025/05/30 18:12:44 Client IP from peer: 10.5.0.2
2025/05/30 18:12:47 garbage collection run  {"removed_clusters": 0, "removed_affiliates": 0, "current_clusters": 1, "current_affiliates": 2, "current_endpoints": 0, "current_subscriptions": 2}
2025/05/30 18:12:52 garbage collection run  {"removed_clusters": 0, "removed_affiliates": 0, "current_clusters": 1, "current_affiliates": 2, "current_endpoints": 0, "current_subscriptions": 2}
2025/05/30 18:12:57 garbage collection run  {"removed_clusters": 0, "removed_affiliates": 0, "current_clusters": 1, "current_affiliates": 2, "current_endpoints": 0, "current_subscriptions": 2}
2025/05/30 18:13:02 garbage collection run  {"removed_clusters": 0, "removed_affiliates": 0, "current_clusters": 1, "current_affiliates": 2, "current_endpoints": 0, "current_subscriptions": 2}
2025/05/30 18:13:07 garbage collection run  {"removed_clusters": 0, "removed_affiliates": 0, "current_clusters": 1, "current_affiliates": 2, "current_endpoints": 0, "current_subscriptions": 2}
```
