#!/bin/bash

# Скрипт для тестирования HTTPS соединения

HOST=${1:-localhost}
PORT=${2:-8080}
HTTPS_PORT=${3:-443}

echo "🧪 Тестирование HTTP/HTTPS соединений для $HOST"
echo "================================================="

# Тест HTTP health check
echo "📡 Тестирование HTTP ($HOST:$PORT)..."
HTTP_RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" http://$HOST:$PORT/health 2>/dev/null)
if [ "$HTTP_RESPONSE" = "200" ]; then
    echo "✅ HTTP работает (код: $HTTP_RESPONSE)"
else
    echo "❌ HTTP не работает (код: $HTTP_RESPONSE)"
fi

# Тест HTTPS health check
echo "🔒 Тестирование HTTPS ($HOST:$HTTPS_PORT)..."
HTTPS_RESPONSE=$(curl -s -k -o /dev/null -w "%{http_code}" https://$HOST:$HTTPS_PORT/health 2>/dev/null)
if [ "$HTTPS_RESPONSE" = "200" ]; then
    echo "✅ HTTPS работает (код: $HTTPS_RESPONSE)"
    
    # Проверка сертификата
    echo "🔍 Информация о сертификате:"
    echo | openssl s_client -connect $HOST:$HTTPS_PORT -servername $HOST 2>/dev/null | openssl x509 -noout -subject -issuer -dates 2>/dev/null || echo "❌ Не удалось получить информацию о сертификате"
else
    echo "❌ HTTPS не работает (код: $HTTPS_RESPONSE)"
fi

# Тест webhook endpoint
echo "🎣 Тестирование webhook endpoint..."
WEBHOOK_RESPONSE=$(curl -s -k -X POST -o /dev/null -w "%{http_code}" https://$HOST:$HTTPS_PORT/webhook 2>/dev/null)
if [ "$WEBHOOK_RESPONSE" = "400" ]; then
    echo "✅ Webhook endpoint доступен (ожидаемый код: $WEBHOOK_RESPONSE - нет подписи)"
else
    echo "⚠️  Webhook endpoint ответил с кодом: $WEBHOOK_RESPONSE"
fi

echo "================================================="
echo "🏁 Тестирование завершено" 