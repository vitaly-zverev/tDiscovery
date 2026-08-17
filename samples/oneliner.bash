# Получить ClusterIP и применить ко всем узлам одной командой
CLUSTER_IP=$(kubectl get svc -n default discovery-service -o jsonpath='{.spec.clusterIP}'); \
for node in $(kubectl get nodes -o json | jq -r '.items[].metadata.name'); do \
  ip=$(kubectl get node $node -o json | jq -r '.status.addresses[] | select(.type=="InternalIP") | .address'); \
  echo "Обновление $node ($ip)..."; \
  sudo --preserve-env=HOME talosctl get mc v1alpha1 -n $ip -c demo -o yaml | yq .spec > /tmp/mc-$node.yaml; \
  sudo --preserve-env=HOME talosctl apply-config --config-patch "cluster: discovery: registries: service: endpoint: \"http://$CLUSTER_IP:3001\"" -f /tmp/mc-$node.yaml -n $ip -c demo; \
  rm -f /tmp/mc-$node.yaml; \
  sleep 2; \
done; \
echo "✅ Готово!"
