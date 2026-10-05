# Trip Service

Сервис поездок TripGo. Лабораторная работа 1: HTTP API по контракту OpenAPI,
хранение в PostgreSQL, собственный менеджер транзакций.

## Требования

- Go 1.27+
- Docker и [`tripgoctl`](https://github.com/course-go-autumn-2026/course-infra)
- `make`

Инструменты кодогенерации и миграций (`oapi-codegen`, `goose`) подключены в
`go.mod` как `tool` и запускаются через `go tool`, отдельно их ставить не нужно.

## Запуск

```bash
tripgoctl cluster start        # один раз на машине
tripgoctl environment start    # поднимает PostgreSQL и создаёт .env
make migrate                   # накатывает миграции
make run                       # запускает сервис
```

`.env` создаёт `tripgoctl`. Если каких-то переменных из таблицы ниже в нём
нет, сервис возьмёт значения по умолчанию; переопределить их можно, дописав
строки из `.env.example` в `.env`.

Проверка:

```bash
curl -i localhost:8080/health
curl -i localhost:8080/ready
```

Запуск в контейнере описан в разделе «Docker-образ».

## Команды Makefile

| Команда | Что делает |
|---|---|
| `make generate` | генерирует `internal/generated/api.gen.go` из OpenAPI-контракта |
| `make migrate` | применяет миграции (`goose up`) |
| `make migrate-down` | откатывает последнюю миграцию |
| `make migrate-status` | показывает состояние миграций |
| `make run` | запускает сервис |
| `make test` | `go test -race ./...` |
| `make docker-build` | собирает образ `trip-service:local` |
| `make docker-run` | запускает контейнер в docker-сети `kind`, порт 8080 |

## Переменные окружения

| Переменная | Обязательная | По умолчанию | Назначение |
|---|---|---|---|
| `DATABASE_URL` | да | — | строка подключения к PostgreSQL |
| `HTTP_ADDR` | нет | `:8080` | адрес HTTP-сервера |
| `LOG_LEVEL` | нет | `info` | `debug`, `info`, `warn`, `error` |
| `SHUTDOWN_TIMEOUT` | нет | `5s` | бюджет на graceful shutdown |
| `HTTP_READ_HEADER_TIMEOUT` | нет | `2s` | время на чтение заголовков запроса |
| `HTTP_READ_TIMEOUT` | нет | `10s` | время на чтение всего запроса |
| `HTTP_WRITE_TIMEOUT` | нет | `10s` | время на обработку и запись ответа |
| `HTTP_IDLE_TIMEOUT` | нет | `3m` | простой keep-alive соединения |
| `DATABASE_MAX_CONNS` | нет | `10` | максимум соединений в пуле |
| `DATABASE_MIN_CONNS` | нет | `2` | минимум соединений в пуле |
| `DATABASE_MAX_CONN_LIFETIME` | нет | `30m` | время жизни соединения |
| `DATABASE_CONNECT_TIMEOUT` | нет | `5s` | таймаут подключения к БД (и стартового `Ping`) |
| `DATABASE_QUERY_TIMEOUT` | нет | `3s` | таймаут одного запроса к БД |

Сервис не стартует, если не задан `DATABASE_URL`, если какое-то значение
задано с ошибкой (например, `SHUTDOWN_TIMEOUT=10 секунд`) или если база
недоступна на старте. Все проблемы конфигурации выводятся одной ошибкой.

## Docker-образ

Сборка и запуск:

```bash
tripgoctl environment start    # PostgreSQL окружения должен работать
make migrate
make docker-build
make docker-run
```

В другом терминале:

```bash
curl -i localhost:8080/health
curl -i localhost:8080/ready
```

Ожидаемый ответ на обе команды: `200 OK` и `{"status":"ok"}`.

Контейнер подключается к PostgreSQL через docker-сеть `kind`, по имени узла
кластера и NodePort (`tripgo-local-control-plane:30103`). Порт, который
`tripgoctl` пробрасывает на хост (`localhost:2xxxx`), слушает только
`127.0.0.1`, поэтому из контейнера он недоступен. Сеть и адрес можно
переопределить: `make docker-run DOCKER_NETWORK=... DOCKER_DATABASE_URL=...`.

**Размер итогового образа:** 22.8 MB на диске (`docker images`, колонка
`DISK USAGE`), около 5.2 MB в сжатом виде (`CONTENT SIZE`).

Что внутри:

- сборка в два этапа: на `golang:1.27.1` собирается статический бинарь
  (`CGO_ENABLED=0`, `-trimpath`, `-ldflags="-s -w"`), в итоговый образ
  копируется только он;
- итоговая база `gcr.io/distroless/static-debian12:nonroot`: исходников,
  Go toolchain и кеша сборки в образе нет;
- процесс запускается от `65532:65532`, не от root;
- `.dockerignore` устроен как белый список: в контекст сборки попадают только
  `go.mod`, `go.sum`, `cmd/` и `internal/` (без `*_test.go` и `testdata/`).

Как проверить:

```bash
docker inspect -f '{{.Config.User}}' trip-service:local     # 65532:65532
docker create --name tmp-check trip-service:local
docker export tmp-check | tar -t                            # только trip-service и системные файлы
docker rm tmp-check
```

## Структура

```
cmd/trip-service/        main: запускает app.Run(), единственный log.Fatalf
internal/app/            Run(): конфиг, пул, сборка зависимостей, HTTP-сервер, graceful shutdown
internal/config/         чтение и проверка переменных окружения
internal/domain/         модель поездки и доменные ошибки
internal/generated/      код, сгенерированный из OpenAPI (не редактируется)
internal/handler/        HTTP-ручки, валидация, ответы в JSON / problem+json
internal/txmanager/      менеджер транзакций
internal/repository/     SQL через squirrel: trips, trip_status_history, idempotency_keys
scripts/                 ручные проверки (curl), idempotency_check.sh
migrations/              миграции goose
contracts/               контракт курса (не редактируется)
Dockerfile               многостадийная сборка образа сервиса
.dockerignore            белый список файлов для контекста сборки
```

## Что сделано

- `POST /api/v1/trips`, `GET /api/v1/trips/{tripId}`,
  `POST /api/v1/trips/{tripId}/finish`, `GET /health`, `GET /ready` по контракту.
- Ошибки в `application/problem+json` с полем `code`: `invalid_request`,
  `trip_not_found`, `trip_completed`, `driver_busy`, `idempotency_key_conflict`,
  `request_in_progress`, `internal_error`.
  Текст ошибок БД в ответ не попадает, только в лог.
- Таблицы `trips` и `trip_status_history`, журнал статусов пишется при создании
  и завершении поездки в той же транзакции.
- Graceful shutdown по `SIGINT` / `SIGTERM` в пределах `SHUTDOWN_TIMEOUT`.
- Идемпотентность `POST /api/v1/trips` по заголовку `Idempotency-Key`
  (см. раздел «Решения»).
- Dockerfile с многостадийной сборкой, образ на distroless без root
  (см. раздел «Docker-образ»).

## Решения

### Уровень изоляции: Read Committed

Транзакции открываются на уровне `READ COMMITTED` (он же уровень PostgreSQL по
умолчанию). Обе гонки из задания закрыты не уровнем изоляции, а схемой БД и
формой запроса:

- две активные поездки водителя запрещает частичный уникальный индекс;
- двойное завершение запрещает условие `status = 'active'` внутри `UPDATE`.

Эти механизмы работают на любом уровне изоляции. `REPEATABLE READ` и
`SERIALIZABLE` добавили бы ошибки сериализации (`40001`), которые пришлось бы
ловить и повторять, не давая здесь дополнительных гарантий.

### Менеджер транзакций

`internal/txmanager`. Бизнес-код видит только интерфейс:

```go
type TxManager interface {
    Do(ctx context.Context, fn func(ctx context.Context) error) error
}
```

- `Do` открывает транзакцию (`BeginTx` с `ReadCommitted`), кладёт `pgx.Tx` в
  контекст под приватным ключом и вызывает `fn` с этим контекстом.
- `fn` вернула `nil` → `COMMIT`. Ошибка или паника → `ROLLBACK` в `defer`;
  ошибка возвращается наружу без изменений (чтобы работал `errors.Is`), паника
  летит дальше. Откат выполняется с `context.WithoutCancel` и собственным
  таймаутом, поэтому срабатывает и тогда, когда контекст запроса уже отменён.
- Репозиторий получает исполнителя через `txmanager.GetExecutor(ctx, pool)`:
  если в контексте есть транзакция — запрос идёт через неё, если нет — через
  пул. Транзакция аргументом в репозиторий не передаётся.
- Вложенный `Do` видит транзакцию в контексте и просто вызывает `fn` в ней, не
  открывая вторую. Коммит и откат делает только внешний `Do`.
- Вызов репозитория вне `Do` (например, `GET /trips/{id}`) выполняется через
  пул, одним запросом в autocommit.

Создание поездки: `INSERT` в `trips` и `INSERT` в `trip_status_history` внутри
одного `Do`. Если вторая вставка падает, первая откатывается.

### Запрет двух активных поездок водителя

Миграция `00001` (создаёт таблицу `trips` вместе с индексом):

```sql
CREATE UNIQUE INDEX trips_one_active_per_driver_idx ON trips (driver_id)
WHERE status = 'active';
```

Уникальность проверяется только среди активных поездок. При двух одновременных
вставках вторая ждёт коммита первой и получает `23505`; репозиторий по коду
ошибки и имени индекса превращает её в `domain.ErrDriverBusy`, хендлер отдаёт
`409 driver_busy`. Проверка «SELECT, потом INSERT» здесь не подходит: оба
запроса прошли бы проверку до вставки.

### Защита завершения от гонки

```sql
UPDATE trips SET status = 'completed', finished_at = $1, updated_at = $1
WHERE id = $2 AND status = 'active'
RETURNING ...
```

Проверка статуса и запись — одна команда. Второй параллельный `finish` ждёт
блокировку строки, после коммита первого перепроверяет условие, видит
`completed` и не обновляет ничего. Если строк не вернулось, дополнительное
чтение по `id` различает случаи: поездки нет → `404 trip_not_found`, есть →
`409 trip_completed`. `finished_at` не перезаписывается.

### Время

`id`, `status` и `started_at` / `finished_at` задаёт сервис. Время хранится в
`TIMESTAMPTZ` и отдаётся в UTC, обрезанным до микросекунд — с той точностью,
с которой его хранит PostgreSQL, чтобы ответы на `POST` и `GET` совпадали.

### Идемпотентность создания поездки

`POST /api/v1/trips` требует заголовок `Idempotency-Key` (UUID); без него
`400 invalid_request`. В контракте заголовок необязательный, обязательность
проверяет сервис. Повтор запроса с тем же ключом не создаёт вторую поездку.

Ключ хранится в таблице `idempotency_keys`: `id` (сам ключ), `request_hash`
(SHA-256 тела запроса), `trip_id`, `status` (`processing` / `completed`),
`created_at`.

Порядок обработки:

1. Хэшируется тело, затем ключ резервируется вставкой
   `INSERT ... ON CONFLICT (id) DO NOTHING` вместе с заранее выданным `trip_id`.
   Выдать `id` заранее нужно, чтобы повтор мог вернуть ту же поездку; хендлер
   создания берёт его из контекста запроса.
2. Если вставка удалась, запрос выполняется как обычно, а по его итогам:
   ответ `2xx` → ключ получает статус `completed`; ответ `4xx` / `5xx` или
   паника → ключ удаляется, и клиент может повторить запрос с тем же ключом.
3. Если ключ уже занят:

| Ситуация | Ответ |
|---|---|
| тело другое | `409 idempotency_key_conflict` |
| исходный запрос ещё выполняется (`processing`) | `409 request_in_progress` |
| исходный запрос завершён (`completed`) | `201` и тот же trip, как в первом ответе, включая `Location` |

Резервирование через `ON CONFLICT DO NOTHING` не даёт двум параллельным запросам
создать две поездки: ключ достаётся ровно одному. Если ключ успели удалить между
вставкой и чтением (первый запрос закончился ошибкой), запрос один раз повторяет
резервирование.

Известные ограничения:

- повтор возвращает текущее состояние поездки, а не снимок первого ответа:
  после `finish` повтор вернёт `status: completed`;
- пометка `completed` выполняется отдельно от транзакции создания поездки; если
  процесс аварийно завершится между коммитом и пометкой, ключ останется в
  `processing` (повторы будут получать `409 request_in_progress`);
- записи в `idempotency_keys` не удаляются по сроку.

Сценарии ручной проверки идемпотентности: `scripts/idempotency_check.sh`.

## Проверено

- `up` / `down` / `up` всех миграций;
- 20 параллельных созданий на одного водителя → один `201`, девятнадцать `409`;
- 20 параллельных `finish` одной поездки → один `200`, девятнадцать `409`,
  в журнале ровно две записи;
- падение вставки в журнал → `500`, в `trips` новой строки нет;
- `/ready` отдаёт `503` при остановленной БД и `200` после её запуска,
  `/health` в БД не ходит;
- `SIGTERM` во время запроса: запрос завершается с `200`, затем процесс выходит;
- без доступной БД сервис не стартует;
- `scripts/idempotency_check.sh`: повтор с тем же ключом возвращает ту же
  поездку, другое тело → `409`, ошибочный запрос освобождает ключ, запрос без
  заголовка → `400`, гонка двух запросов → один `201` и один `409`;
- 20 параллельных запросов с одним ключом → один `201`, девятнадцать `409`;
- образ: `/health` и `/ready` из контейнера отвечают `200`, процесс запущен от
  `65532`, в образе нет исходников и Go toolchain.