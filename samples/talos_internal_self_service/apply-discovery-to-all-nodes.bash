#!/bin/bash
# Применение патча ко всем Talos узлам из etcd 
# необходимо в случае использования неявного clusterIp (discovery-service_implicit-clusterIP.yaml)
# По умолчанию, в этом примере именно такой сервивис и будет создаваться, если его еще нет в кластере на момент запуска
# Пререквизиты:
# kubectl, talosctl, yq, jq

set -e

echo "🚀 Начало реконфигурирования discovery-сервиса..."

# 1. Создаем или обновляем Service
echo "📝 Проверка/создание discovery-service..."
if ! kubectl get svc -n default discovery-service &>/dev/null; then
  echo "  ↳ Создание discovery-service..."
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  name: discovery-service
  namespace: default
spec:
  selector:
    app: discovery-app
  ports:
  - port: 3001
    targetPort: 3001
  type: ClusterIP
EOF
else
  echo "  ↳ discovery-service уже существует"
fi

# 2. Получаем ClusterIP сервиса
CLUSTER_IP=$(kubectl get svc -n default discovery-service -o jsonpath='{.spec.clusterIP}')
if [ -z "$CLUSTER_IP" ]; then
  echo "❌ Ошибка: не удалось получить ClusterIP"
  exit 1
fi
echo "📡 ClusterIP discovery-service: $CLUSTER_IP"

# 3. Формируем URL для указания в cluster.discovery.registries.service.endpoint
DISCOVERY_URL="http://${CLUSTER_IP}:3001"
echo "🔗 Discovery endpoint: $DISCOVERY_URL"

# 4. Получаем список узлов с их IP-адресами (да, я знаю, что можно указывать имена узлов, но e2e сценарии c qemu провиженером более стабильно работают при использовании ip адресов вместо имен узлов)
echo "📡 Получение списка узлов..."
NODES_JSON=$(kubectl get nodes -o json)
NODE_NAMES=$(echo "$NODES_JSON" | jq -r '.items[].metadata.name')
NODE_IPS=$(echo "$NODES_JSON" | jq -r '.items[].status.addresses[] | select(.type=="InternalIP") | .address')

# Создаем массив узлов с их IP
declare -A NODE_MAP
NODE_ARRAY=()
while IFS= read -r name && IFS= read -r ip <&3; do
  NODE_MAP["$name"]="$ip"
  NODE_ARRAY+=("$name")
done < <(echo "$NODE_NAMES") 3< <(echo "$NODE_IPS")

echo "📡 Найдены узлы:"
for name in "${NODE_ARRAY[@]}"; do
  echo "  - $name (${NODE_MAP[$name]})"
done

# 5. Применяем конфигурацию к каждому узлу
echo ""
echo "⚙️ Применение конфигурации к узлам..."

for NODE_NAME in "${NODE_ARRAY[@]}"; do
  NODE_IP="${NODE_MAP[$NODE_NAME]}"

  echo ""
  echo "📝 Обработка узла: $NODE_NAME (IP: $NODE_IP)"

  # Получаем текущую конфигурацию
  echo "  ↳ Получение текущей конфигурации..."
  if ! sudo --preserve-env=HOME talosctl get mc v1alpha1 \
    -n $NODE_IP -c demo -o yaml 2>/dev/null | yq .spec > /tmp/mc-${NODE_NAME}.yaml; then
    echo "  ❌ Ошибка: не удалось получить конфигурацию для $NODE_NAME"
    continue
  fi

  # Применяем патч
  echo "  ↳ Применение патча с endpoint: $DISCOVERY_URL"
  sudo --preserve-env=HOME talosctl apply-config \
    --config-patch "
cluster:
  discovery:
    registries:
      service:
        endpoint: \"$DISCOVERY_URL\"
" \
    -f /tmp/mc-${NODE_NAME}.yaml \
    -n $NODE_IP -c demo

  if [ $? -eq 0 ]; then
    echo "  ✅ $NODE_NAME успешно обновлен"
  else
    echo "  ❌ Ошибка при обновлении $NODE_NAME"
  fi

  # Очистка
  rm -f /tmp/mc-${NODE_NAME}.yaml

  # Небольшая задержка между узлами
  sleep 2
done

# 6. Проверка применения
echo ""
echo "🔍 Проверка конфигурации на всех узлах..."

for NODE_NAME in "${NODE_ARRAY[@]}"; do
  NODE_IP="${NODE_MAP[$NODE_NAME]}"
  echo ""
  echo "=== $NODE_NAME ($NODE_IP) ==="

  CONFIG=$(sudo --preserve-env=HOME talosctl get mc v1alpha1 \
    -n $NODE_IP -c demo -o yaml 2>/dev/null | yq . | yq .spec.cluster.discovery.registries.service )

  if [ -n "$CONFIG" ]; then
    echo "$CONFIG" 
  else
    echo "  ❌ service секция не найдена"
  fi
done

