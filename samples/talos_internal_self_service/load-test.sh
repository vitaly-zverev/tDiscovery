#!/bin/bash

echo "Устанавливаем Metrics Server с патчем для работы в Talos..."
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

echo "Патчим для установки с зеркал и для работы в ограниченных ресурсах"
kubectl patch deployment metrics-server -n kube-system --type='json' -p='[
  {"op": "replace", "path": "/spec/template/spec/containers/0/image", "value": "k8s.gcr.io/metrics-server/metrics-server:v0.9.0"},
  {"op": "replace", "path": "/spec/template/spec/containers/0/resources/requests/memory", "value": "50Mi"},
  {"op": "replace", "path": "/spec/template/spec/containers/0/resources/requests/cpu", "value": "50m"}
]'


echo "Патчим для работы с самоподписанными сертификатами (нужно для Talos)"
kubectl patch deployment metrics-server -n kube-system --type='json' -p='[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--kubelet-insecure-tls"}]'

echo "Ждем запуска metrics-server"
kubectl wait --for=condition=ready pod -l k8s-app=metrics-server -n kube-system --timeout=60s

echo "Проверяем работоспособность metrics-server..."
kubectl top nodes
kubectl top pods

echo "=== Начинаем нагрузочное тестирование discovery сервиса ==="

# Получаем имя Pod'а
POD=$(kubectl get pods -l app=discovery-app -o jsonpath='{.items[0].metadata.name}')
echo "Target Pod: $POD"
echo ""

# Удаляем старые нагрузочные Pod'ы
kubectl delete deployment load-generator 2>/dev/null
kubectl delete job load-test-discovery 2>/dev/null

# Применяем нагрузочный Deployment
echo "Запуск генерации нагрузки..."
kubectl apply -f load-test-deployment.yaml

# Ждем запуска Pod'ов
sleep 5

# Функция для сбора метрик
collect_metrics() {
  echo "Время,CPU(cores),Memory(Mi)" > metrics.csv
  for i in {1..30}; do
    TIMESTAMP=$(date +%H:%M:%S)
    
    # Получаем метрики для discovery Pod'а
    METRICS=$(kubectl top pod $POD --containers --no-headers 2>/dev/null | grep discovery | awk '{print $2","$3}')
    
    if [ -n "$METRICS" ]; then
      echo "$TIMESTAMP,$METRICS" >> metrics.csv
      echo "[$TIMESTAMP] CPU: $(echo $METRICS | cut -d',' -f1), Memory: $(echo $METRICS | cut -d',' -f2)"
    else
      echo "[$TIMESTAMP] Метрики недоступны (возможно, metrics-server не установлен)"
    fi
    
    # Также показываем нагрузочные Pod'ы
    LOAD_PODS=$(kubectl get pods -l app=load-generator --field-selector=status.phase=Running -o name | wc -l)
    echo "     Активных нагрузочных Pod'ов: $LOAD_PODS"
    
    sleep 5
  done
}

# Запускаем сбор метрик в фоне
collect_metrics &
MONITOR_PID=$!

# Ждем 150 секунд (30 итераций * 5 секунд)
echo ""
echo "Сбор метрик в течение 150 секунд..."
wait $MONITOR_PID

# Останавливаем нагрузку
echo ""
echo "Остановка генерации нагрузки..."
kubectl delete deployment load-generator

echo ""
echo "=== Результаты ==="

if [ -f metrics.csv ] && [ $(wc -l < metrics.csv) -gt 1 ]; then
  echo ""
  echo "--- Сводка по использованию ресурсов discovery сервиса ---"
  
  # Вычисляем средние значения
  AVG_CPU=$(awk -F',' 'NR>1 && $2!="" {sum+=$2; count++} END {if(count>0) printf "%.2f", sum/count}' metrics.csv)
  MAX_CPU=$(awk -F',' 'NR>1 && $2!="" {if($2>max) max=$2} END {printf "%.2f", max}' metrics.csv)
  
  AVG_MEM=$(awk -F',' 'NR>1 && $3!="" {gsub(/Mi/,"",$3); sum+=$3; count++} END {if(count>0) printf "%.2f", sum/count}' metrics.csv)
  MAX_MEM=$(awk -F',' 'NR>1 && $3!="" {gsub(/Mi/,"",$3); if($3>max) max=$3} END {printf "%.2f", max}' metrics.csv)
  
  echo "Средний CPU:  ${AVG_CPU}m"
  echo "Пиковый CPU:  ${MAX_CPU}m"
  echo "Средняя память: ${AVG_MEM}Mi"
  echo "Пиковая память: ${MAX_MEM}Mi"
  
  echo ""
  echo "--- Текущие лимиты ---"
  kubectl describe pod $POD | grep -E "Limits:|Requests:" -A 1 | head -6
  
  echo ""
  echo "--- Рекомендации по настройке ---"
  if (( $(echo "$MAX_CPU > 180" | bc -l) )); then
    echo "⚠️  CPU лимит (200m) близок к пиковому использованию (${MAX_CPU}m). Рекомендуется увеличить до 300-500m"
  else
    echo "✅ CPU лимит (200m) достаточен (пик: ${MAX_CPU}m)"
  fi
  
  if (( $(echo "$MAX_MEM > 18" | bc -l) )); then
    echo "⚠️  Memory лимит (20Mi) близок к пиковому использованию (${MAX_MEM}Mi). Рекомендуется увеличить до 64-128Mi"
  else
    echo "✅ Memory лимит (20Mi) достаточен (пик: ${MAX_MEM}Mi)"
  fi
  
  echo ""
  echo "--- Детальные метрики (последние 10 записей) ---"
  tail -10 metrics.csv | column -t -s','
else
  echo "Метрики не собраны. Возможные причины:"
  echo "  - metrics-server не установлен"
  echo "  - Pod не запущен"
  echo ""
  echo "Проверьте вручную:"
  echo "  kubectl top pod $POD --containers"
fi

echo ""
echo "Состояние discovery Pod'а после нагрузки:"
kubectl describe pod $POD | grep -E "Restart Count|State|OOMKilled" -A 2 | head -15

echo ""
echo "Последние логи discovery сервиса (20 строк):"
kubectl logs $POD --tail=20
