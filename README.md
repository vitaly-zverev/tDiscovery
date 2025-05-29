# talos-discovery


Быстрый старт для разработки:
```
 git clone https://github.com/siderolabs/discovery-api 
 git clone https://github.com/vitaly-zverev/talos-discovery && cd talos-discovery
 export PATH="$PATH:$(go env GOPATH)/bin"
```
Собрать:
```
make
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
 _out/talos-discovery
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

seq 1 10 | xargs -I{} grpcurl -proto v1alpha1/server/cluster.proto -import-path ./discovery-api/api -plaintext -d "{\"clusterId\": \"xyz\",\"affiliateId\":\"def-{}\",\"affiliateData\":\"MTIzCg==\",\"affiliateEndpoints\":\"MTIzCg==\"}" -H 'X-Real-IP: 1.2.3.4' localhost:3001 sidero.discovery.server.Cluster/AffiliateUpdate
```
