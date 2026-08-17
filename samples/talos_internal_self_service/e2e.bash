sudo --preserve-env=HOME talosctl cluster create dev \
     --provisioner qemu --name demo --with-debug     \
     --registry-mirror registry.k8s.io='https://registry-k8s-io.mirrors.sjtug.sjtu.edu.cn' \
     --cpus 1 --cpus-workers 1 \
     --memory 2700m  --memory-workers 1400m  \
     --config-patch '
machine:
  time:
    servers:
      - "0.rhel.pool.ntp.org"
      - "1.rhel.pool.ntp.org"
cluster:
  discovery:
    registries:
      service:
        endpoint: "http://10.109.183.11:3001"
'

kubectl apply -f ~/tDiscovery/samples/talos_internal_self_service/discovery-app.yaml
kubectl apply -f ~/tDiscovery/samples/talos_internal_self_service/discovery-service_explicit-clusterIP.yaml
kubectl get all
 
