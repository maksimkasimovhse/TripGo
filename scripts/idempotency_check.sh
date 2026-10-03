#!/usr/bin/env bash
# idempotency_check.sh — ручные проверки идемпотентности createTrip
BASE="${BASE:-http://localhost:8080/api/v1}"
command -v uuidgen >/dev/null 2>&1 || uuidgen() { cat /proc/sys/kernel/random/uuid; }

USER_ID=$(uuidgen)
DRIVER_ID=$(uuidgen)

body() { # $1=user_id $2=driver_id $3=price
  cat <<EOF
{"user_id":"$1","driver_id":"$2",
 "start_point":{"latitude":55.75,"longitude":37.61},
 "end_point":{"latitude":55.80,"longitude":37.70},
 "price":$3}
EOF
}

post() { # $1=idempotency key (пусто = без заголовка)  $2=body
  if [ -n "$1" ]; then
    curl -s -i -X POST "$BASE/trips" -H "Content-Type: application/json" \
      -H "Idempotency-Key: $1" -d "$2"
  else
    curl -s -i -X POST "$BASE/trips" -H "Content-Type: application/json" -d "$2"
  fi
  echo; echo
}

section() { echo "=================== $1"; }

KEY=$(uuidgen)
B1=$(body "$USER_ID" "$DRIVER_ID" 1450)

section "1. Первый запрос: ждём 201, Trip в теле, заголовок Location"
post "$KEY" "$B1"

section "2. Повтор с тем же ключом и телом: ждём 201, тот же id и Location"
post "$KEY" "$B1"

section "3. Тот же ключ, другое тело (price другой): ждём 409 idempotency_key_conflict"
post "$KEY" "$(body "$USER_ID" "$DRIVER_ID" 9999)"

section "4. Невалидное тело (нет price): ждём 400"
KEY4=$(uuidgen)
BAD='{"user_id":"'"$(uuidgen)"'","driver_id":"'"$(uuidgen)"'","start_point":{"latitude":55.75,"longitude":37.61},"end_point":{"latitude":55.80,"longitude":37.70}}'
post "$KEY4" "$BAD"

section "4b. Тот же ключ с исправленным телом: ждём 201 (ключ освободился после 400)"
post "$KEY4" "$(body "$(uuidgen)" "$(uuidgen)" 500)"

section "5. Занятый водитель: новый ключ, тот же driver_id (у него активный trip из п.1): ждём 409 driver_busy"
KEY5=$(uuidgen)
post "$KEY5" "$(body "$(uuidgen)" "$DRIVER_ID" 700)"

section "5b. Повтор того же запроса: ждём снова 409 driver_busy, а НЕ request_in_progress"
post "$KEY5" "$(body "$(uuidgen)" "$DRIVER_ID" 700)"

section "6. Без заголовка Idempotency-Key: ждём 400"
post "" "$B1"

section "7. Гонка: два одинаковых запроса параллельно (новые ключ и водитель)"
KEY7=$(uuidgen)
B7=$(body "$(uuidgen)" "$(uuidgen)" 300)
post "$KEY7" "$B7" > /tmp/race1.txt &
post "$KEY7" "$B7" > /tmp/race2.txt &
wait
echo "--- ответ 1:"; cat /tmp/race1.txt
echo "--- ответ 2:"; cat /tmp/race2.txt
echo "Ожидается: 201 + 201 с одним id, либо 201 + 409 request_in_progress."
echo "Проверьте в базе: SELECT count(*) FROM trips WHERE driver_id = <driver из п.7>;  -- должно быть 1"

section "8. finish + повтор create с тем же ключом"
KEY8=$(uuidgen); DRIVER8=$(uuidgen); USER8=$(uuidgen)
B8=$(body "$USER8" "$DRIVER8" 1000)

TRIP_ID=$(curl -s -X POST "$BASE/trips" -H "Content-Type: application/json" \
  -H "Idempotency-Key: $KEY8" -d "$B8" | sed -E 's/.*"id":"([^"]+)".*/\1/')
echo "создан trip: $TRIP_ID"

echo "--- finish: ждём 200"
curl -s -i -X POST "$BASE/trips/$TRIP_ID/finish"; echo; echo

echo "--- повтор create с тем же ключом: ждём 201, тот же id, status = completed (известное ограничение)"
post "$KEY8" "$B8"

echo "--- водитель освободился: новый ключ, тот же driver_id: ждём 201"
post "$(uuidgen)" "$(body "$(uuidgen)" "$DRIVER8" 1100)"