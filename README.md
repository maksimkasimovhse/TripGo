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
| `make generate` | генерирует OpenAPI-код и все моки через `go:generate` |
| `make migrate` | применяет миграции (`goose up`) |
| `make migrate-down` | откатывает последнюю миграцию |
| `make migrate-status` | показывает состояние миграций |
| `make run` | запускает сервис |
| `make test` | тесты с `-race`, покрытие по функциям и итоговый процент |
| `make lint` | обязательные линтеры, форматтеры и проверка границ слоёв |
| `make fmt` | применяет `gofumpt` и `gci` |
| `make test-integration` | тесты с настоящим PostgreSQL, включая файлы с тегом `integration` |
| `make docker-build` | собирает образ `trip-service:local` |
| `make docker-run` | запускает контейнер в docker-сети `kind`, порт 8080 |

## Переменные окружения

| Переменная | Обязательная | По умолчанию | Назначение |
|---|---|---|---|
| `DATABASE_URL` | да | — | строка подключения к PostgreSQL |
| `HTTP_ADDR` | нет | `:8080` | адрес HTTP-сервера |
| `LOG_LEVEL` | нет | `info` | `debug`, `info`, `warn`, `error` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | нет | `http://localhost:4317` для gRPC, `http://localhost:4318` для HTTP | адрес Collector; для lab 02 берите из `.env` |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | нет | `grpc` | `grpc` или `http/protobuf` |
| `OTEL_TRACES_SAMPLER` | нет | `parentbased_always_on` | стратегия семплирования SDK |
| `OTEL_TRACES_SAMPLER_ARG` | нет | — | доля для `traceidratio` / `parentbased_traceidratio`, от 0 до 1 |
| `OTEL_METRIC_EXPORT_INTERVAL` | нет | `60000` | период отправки метрик в миллисекундах; в `.env.example` — `10000` |
| `OTEL_EXPORTER_OTLP_TIMEOUT` | нет | `10000` | сетевой таймаут OTLP в миллисекундах; в `.env.example` — `2000` |
| `OTEL_BSP_SCHEDULE_DELAY` | нет | `5000` | период отправки пачек спанов в миллисекундах; в `.env.example` — `1000` |
| `OTEL_BSP_EXPORT_TIMEOUT` | нет | `30000` | бюджет экспорта пачки спанов в миллисекундах; в `.env.example` — `2000` |
| `OTEL_METRIC_EXPORT_TIMEOUT` | нет | `30000` | бюджет сбора и отправки метрик в миллисекундах; в `.env.example` — `2000` |
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
кластера и NodePort (`tripgo-local-control-plane:30203` для lab 02). Порт, который
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

## Архитектура

Зависимости направлены от транспорта к бизнес-логике. Usecase описывает нужные
операции через интерфейсы и не импортирует конкретный репозиторий:

```text
HTTP handler → usecase → domain
                 ↓
        интерфейс репозитория
                 ↑ реализует
           repository → PostgreSQL
```

`main` создаёт корневой контекст и подписывается на сигналы остановки,
`app.Run(ctx)` управляет запуском и завершением приложения. `container`
собирает конкретные реализации: SDK телеметрии, пул БД, репозитории,
менеджер транзакций, usecase и HTTP-обработчики. `infra` содержит настройку
PostgreSQL, транзакций и OpenTelemetry. Домен не зависит от инфраструктуры;
HTTP-пакеты не импортируют репозитории и `infra`. Эти ограничения проверяет depguard.

Интерфейсы зависимостей объявлены у потребителей в `contract.go`:
HTTP-обработчик задаёт интерфейс своего usecase, usecase — интерфейсы
репозитория и менеджера транзакций. Они содержат только используемые методы.
Так потребитель определяет, что ему нужно, а реализацию можно заменить
без изменения бизнес-логики — например, на сгенерированный gomock в тестах.
Общий HTTP Handler также зависит от интерфейсов отдельных обработчиков,
поэтому между родительским пакетом и пакетами операций нет цикла импортов.

### Распределение файлов

```
cmd/service/                         main: корневой контекст, сигналы, app.Run(ctx), вывод ошибки
internal/app/                        конфиг, вызов контейнера, HTTP-сервер и shutdown
internal/config/                     чтение и проверка env
internal/container/                  создание и связывание зависимостей, закрытие ресурсов
internal/domain/trip/                поездка, валидация, переход Finish, доменные ошибки
internal/domain/idempotency/         ошибки идемпотентности
internal/model/                      статус ключа идемпотентности
internal/usecase/trip/create/         создание поездки, contract.go
internal/usecase/trip/get/            чтение поездки, contract.go
internal/usecase/trip/finish/         завершение поездки, contract.go
internal/usecase/idempotency/         операции с ключами, contract.go
internal/http/trip/                  общий Handler, health/readiness, общие функции поездок
internal/http/trip/create/           ручка создания, contract.go, преобразование и проверка тела
internal/http/trip/get/              ручка чтения, contract.go
internal/http/trip/finish/           ручка завершения, contract.go
internal/http/domainerror/           единый маппинг доменных ошибок в HTTP
internal/http/respond/               запись JSON и Problem
internal/http/middleware/            декоратор идемпотентности и ответ на повтор
internal/repository/trip/            SQL поездок, истории статусов и ключей
internal/infra/postgres/             создание, настройка и стартовый Ping пула pgx
internal/infra/postgres/transaction/ менеджер транзакций
internal/generated/                  сгенерированный код OpenAPI (не редактируется)
scripts/                             ручные проверки (curl)
migrations/                          миграции goose
contracts/                           контракт курса (не редактируется)
Dockerfile                           многостадийная сборка образа сервиса
.dockerignore                        белый список файлов для контекста сборки
```

### Отступления от целевой структуры

- Конфиг и управление запуском находятся в `app.Run(ctx)`, сборка зависимостей —
  в `container`. `main` создаёт корневой контекст, подписывается на сигналы,
  вызывает `app.Run(ctx)` и выводит ошибку. Такое разделение сохраняет короткую
  точку входа, а бизнес-код получает контекст извне.
- `pgxpool.NewWithConfig` вызывается внутри `infra/postgres.NewPool`, а контейнер
  вызывает этот конструктор. Настройка драйвера, проверка соединения и очистка при
  ошибке инкапсулированы в инфраструктуре; контейнер выбирает реализации и
  владеет успешно созданным пулом.
- Между слоями передаётся `domain/trip.Trip`. Отдельная идентичная `model.Trip`
  не заведена, чтобы не дублировать поля и преобразования. `model` содержит
  только статус ключа идемпотентности.
- Файлы репозитория разделены по данным: `trip.go`, `status_history.go`,
  `idempotency_keys.go`. Общий код SQL поездок пока помещается в одном файле;
  отдельные `save.go`, `fetch.go` и `convert.go` не требуются. Интерфейсы
  репозиториев объявлены у потребителей в usecase.
- Общий `http/trip.Handler` реализует сгенерированный `api.ServerInterface`
  и делегирует операции отдельным ручкам через интерфейсы. Он не импортирует
  пакеты ручек; конкретные обработчики связывает контейнер. Пакет `respond`
  хранит общую запись HTTP-ответов и не содержит бизнес-операций.
- Доменные sentinel-ошибки поездки пока объявлены в `trip.go`, а тип
  `ValidationError` — в `errors.go`. Оба файла принадлежат `domain/trip`,
  поэтому ошибки остаются рядом с сущностью без отдельного пакета ошибок.

### Интеграционные сценарии

Добавлены интеграционные тесты создания, чтения и завершения
поездки, отсутствующих записей, занятого и освободившегося водителя,
истории статусов и ключей идемпотентности. Отдельные сценарии проверяют
коммит создания и завершения через реальные usecase, откат вставки поездки
и откат её завершения при ошибке записи истории.

### Линтеры и форматирование

`.golangci.yml` основан на [конфиге курса](https://github.com/course-go-autumn-2026/course/blob/main/homework/config/golangci.yml).
Сохранены все обязательные линтеры и форматтеры. Устаревшая настройка gofumpt
`extra-rules` заменена на эквивалентные `extra.group-params` и `extra.clothe-returns`.
Добавлены правила depguard, запрещающие HTTP-пакетам импортировать репозитории
и инфраструктуру, а домену и usecase — сгенерированный OpenAPI-контракт.
Линтер проверяет и интеграционные тесты, но не запускает их: PostgreSQL не нужен.

```bash
make fmt
make lint
make test
```

При первом запуске `make lint` или `make fmt` скачивается и собирается
`golangci-lint v2.15.0` в `bin/tools/golangci-lint/v2.15.0/`. Версия закреплена
в Makefile, установленный глобально линтер не используется. Первый запуск
требует доступа к сети, следующие используют локальный бинарник.
Профиль покрытия сохраняется в `bin/coverage.out`; посмотреть его в браузере:

```bash
go tool cover -html=bin/coverage.out
```

## Логирование

`infra/telemetry.NewLogger` создаёт `slog.Logger` с JSON-выводом в stdout
и полем `service=trip-service`. `LOG_LEVEL` задаёт минимальный уровень:
`debug`, `info`, `warn`, `error`. Логгер передаётся через контейнер
обработчикам, middleware и usecase создания и завершения поездки.
Глобальные `log.Printf` и `slog.Default()` не используются.

Middleware `RequestLogging` записывает одну запись `request handled` после
обработки запроса: `method`, `route`, `status`, `duration_ms`.
В `route` используется шаблон chi, например `/api/v1/trips/{tripId}`;
для неизвестных маршрутов — постоянное значение `unmatched`.
Тела запросов, ответов и query string не записываются.
Ответы 2xx–4xx логируются на уровне Info, 5xx — Error.

Usecase отдельно записывают `trip created` и `trip completed` с `trip_id`
только после успешного возврата из менеджера транзакций. При ошибке коммита
событие успеха не записывается. Внутренние ошибки логируются как отдельная
диагностика, а наружу возвращается безопасный Problem.

`Recover` логирует панику со стеком и возвращает `500 internal_error`, если
ответ ещё не отправлен. Заголовки уже отправленного ответа изменить нельзя.
Порядок подключённых middleware: Recover → Tracing → Logging → Metrics → роутер.

Обработчик логов добавляет `trace_id` и `span_id` из контекста спана.
Для этого код использует контекстные методы `InfoContext`, `ErrorContext`
и `Log`. Без валидного спана идентификаторы не выдумываются.
HTTP-спаны и экспорт в коллектор подключены; идентификатор из лога можно
вставить в поиск Jaeger.

## Трейсы и Jaeger

Путь данных: сервис → OTLP → OpenTelemetry Collector → Jaeger.
`infra/telemetry.NewTracing` создаёт SDK и фоновый пакетный экспортёр.
Контейнер передаёт один `TracerProvider` в HTTP middleware и пул pgx.
Домен и usecase не знают о Jaeger.

`otelhttp` создаёт серверный спан на запрос, `otelpgx` — дочерние спаны
запросов PostgreSQL, включая запросы внутри транзакций. Имя HTTP-спана
содержит шаблон маршрута: `GET /api/v1/trips/{tripId}`. Неизвестный маршрут
называется `GET unmatched`. Заголовок W3C `traceparent` продолжает входящий
трейс; без него SDK создаёт новый. Контекст передаётся до репозитория.

SQL-спаны содержат `db.operation.name` и `db.collection.name` для таблиц
`trips`, `trip_status_history`, `idempotency_keys`. Таблицы определяются
по простым SQL-командам текущих репозиториев; при добавлении сложных запросов
или новых таблиц нужно расширить `infra/postgres/queryCollection`.
У `BEGIN` / `COMMIT` / `ROLLBACK` таблицы нет. Текст SQL и параметры
в спаны не записываются. Ошибки SQL получают статус Error и `error.type`
(SQLSTATE для ошибок PostgreSQL), HTTP 5xx и паники — статус Error;
ожидаемые HTTP 4xx не считаются ошибками серверного спана.

Ресурс сервиса: `service.name=trip-service`, `service.namespace=tripgo`,
`deployment.environment=local`, `service.version=dev`. Версию сборки можно
задать через `make run VERSION=...` или `make docker-build VERSION=...`.
Обычный `go run ./cmd/service` использует `dev`.

Запуск с настройками окружения:

```bash
tripgoctl environment start
tripgoctl connect
make migrate
make run VERSION=lab02
```

`tripgoctl connect` показывает адреса сервисов. Для lab 02 в `.env` обычно
указаны `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:22318` и
`OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf`. Альтернатива — gRPC:
`http://localhost:22317` и `grpc`. Переменные `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`
и `OTEL_EXPORTER_OTLP_TRACES_PROTOCOL` имеют приоритет над общими настройками;
для HTTP специфичный endpoint должен включать `/v1/traces`.
В Docker используйте `make docker-run`: Collector доступен по NodePort
`tripgo-local-control-plane:30206`; адрес меняется через `DOCKER_OTLP_ENDPOINT`.

В другом терминале сделайте запросы создания, получения и завершения поездки,
затем откройте адрес Jaeger из `tripgoctl connect` (обычно
<http://localhost:22686>). Выберите сервис `trip-service`, нажмите Find Traces
и откройте запрос. В нём видны HTTP-спан и вложенные SQL-спаны. Можно также
скопировать `trace_id` из JSON-лога в поле поиска по Trace ID.
Для `POST /api/v1/trips` без ключа идемпотентности ожидаются `BEGIN`,
`INSERT trips`, `INSERT trip_status_history`, `COMMIT`.

Для локальной разработки `parentbased_always_on` сохраняет все новые трейсы.
`parentbased_traceidratio` с `OTEL_TRACES_SAMPLER_ARG=0.1` сохраняет примерно
10% новых трейсов и следует решению семплирования родительского спана.
`always_off` отключает запись спанов. Отсутствие трейса при семплировании
не означает, что запрос не выполнялся.

Если Collector недоступен, HTTP-сервис продолжает работать: экспорт выполняется
в фоне, очередь ограничена и не блокирует запросы. Ошибки экспорта попадают
в JSON-лог. При длительном отказе спаны могут теряться; очередь не заменяет
постоянное хранилище. При остановке сначала завершаются HTTP-запросы, затем
SDK метрик делает последний сбор и отправку, закрывается пул, SDK трейсов
отправляет накопленные спаны. На все этапы действует один `SHUTDOWN_TIMEOUT`.
Контекст остановки наследует значения контекста приложения, но не его отмену.
Если бюджет исчерпан, приложение пишет ошибку и возвращает управление в `main`,
который завершает процесс с ненулевым кодом, даже если закрытие ресурса зависло.

Таймауты и период отправки берутся стандартными средствами SDK из env:
`OTEL_EXPORTER_OTLP_TIMEOUT`, `OTEL_BSP_SCHEDULE_DELAY`,
`OTEL_BSP_EXPORT_TIMEOUT`, `OTEL_METRIC_EXPORT_TIMEOUT`.
Специфичные `OTEL_EXPORTER_OTLP_TRACES_TIMEOUT` и
`OTEL_EXPORTER_OTLP_METRICS_TIMEOUT` имеют приоритет над общим сетевым таймаутом.

Тесты проверяют контекст родителя и дочерних спанов, имена и атрибуты,
статусы ошибок и паник, связь с логами, реальный OTLP gRPC и HTTP/protobuf экспорт,
семплирование и инициализацию при недоступном Collector для обоих протоколов.

## Метрики

Имена и атрибуты соответствуют [каталогу лабы](https://github.com/course-go-autumn-2026/course/blob/main/homework/docs/observability.md).
Сервис отправляет метрики по OTLP в Collector; Prometheus забирает их из
Collector, Grafana строит графики по данным Prometheus. Отдельная ручка
`/metrics` в сервисе не нужна.

| Метрика OTel | Тип | Где собирается |
|---|---|---|
| `http.server.request.duration` | histogram, секунды | middleware: метод, шаблон маршрута, HTTP-статус |
| `http.server.active_requests` | up-down counter | middleware: метод и шаблон маршрута |
| `db.client.operation.duration` | histogram, секунды | инструментирование пула: операция, таблица, тип ошибки при её наличии |
| `tripgo.trips.created` | counter | usecase создания, после успешного коммита |
| `tripgo.trips.completed` | counter | usecase завершения, после успешного коммита |
| `tripgo.trips.active` | observable gauge | `SELECT count(*) FROM trips WHERE status = 'active'` при сборе метрик |

HTTP middleware определяет шаблон маршрута до запуска обработчика, чтобы
активные запросы уже во время выполнения имели правильный `http.route`.
Неизвестные маршруты агрегируются под `unmatched`. Паника до отправки ответа
учитывается как 500, число активных запросов уменьшается даже при панике.
SQL-метрики работают независимо от семплирования трейсов. Автоматические
метрики `otelhttp` и `otelpgx` отключены, чтобы измерения не дублировались;
адаптер пула добавляет требуемые каталогом атрибуты SQL.

Повтор идемпотентного запроса не увеличивает счётчик созданных поездок,
поскольку usecase создания повторно не вызывается. Ошибка коммита также
не увеличивает бизнес-счётчики. Gauge активных поездок берёт фактическое
состояние БД и корректно работает после перезапуска; при ошибке чтения значение
пропускается, а в лог пишется ошибка. При нескольких экземплярах сервиса,
читающих одну БД, для этого gauge используйте `max`, а не `sum`.

Для метрик используются общие `OTEL_EXPORTER_OTLP_ENDPOINT` и
`OTEL_EXPORTER_OTLP_PROTOCOL`. Специфичные переменные
`OTEL_EXPORTER_OTLP_METRICS_ENDPOINT` и `OTEL_EXPORTER_OTLP_METRICS_PROTOCOL`
имеют приоритет; HTTP endpoint для метрик должен заканчиваться на `/v1/metrics`.
`OTEL_METRIC_EXPORT_INTERVAL=10000` включает отправку раз в 10 секунд.
По умолчанию SDK отправляет раз в 60 секунд; добавьте настройку в `.env`
или запустите `OTEL_METRIC_EXPORT_INTERVAL=10000 make run`.

После создания и завершения поездки подождите период экспорта и опроса
Prometheus. Откройте Grafana → Explore → Prometheus (адреса — в
`tripgoctl connect`). В Prometheus точки в именах заменяются подчёркиваниями,
счётчики обычно получают суффикс `_total`, секунды — `_seconds`.
Сначала найдите реальные имена запросами:

```promql
{__name__=~"http_server_request_duration.*|http_server_active_requests.*|db_client_operation_duration.*|tripgo_trips_.*"}
```

При стандартном преобразовании имён Collector примеры запросов такие:

```promql
# RPS по маршрутам
sum by (http_route) (rate(http_server_request_duration_seconds_count[5m]))

# p95 времени ответа по маршрутам
histogram_quantile(0.95, sum by (le, http_route) (rate(http_server_request_duration_seconds_bucket[5m])))

# Доля HTTP 5xx
sum(rate(http_server_request_duration_seconds_count{http_response_status_code=~"5.."}[5m]))
/
sum(rate(http_server_request_duration_seconds_count[5m]))

# Созданные поездки в секунду
sum(rate(tripgo_trips_created_total[5m]))

# Активные поездки в общей БД
max(tripgo_trips_active)
```

SDK хранит агрегаты в памяти и отправляет в фоне. При отказе Collector
запросы продолжают выполняться, ошибки экспорта логируются; гарантированной
доставки и хранения метрик на диске в сервисе нет. Тесты проверяют значения
и типы метрик через ManualReader, учёт ошибок и коммитов, а также настоящий
экспорт gRPC и HTTP/protobuf на локальные тестовые приёмники.

## Тесты

Unit-тесты не требуют PostgreSQL:

```bash
make generate-mocks
make test
```

Интеграционные тесты используют PostgreSQL окружения `tripgoctl` и отделены
тегом `//go:build integration`:

```bash
tripgoctl environment start
make migrate
make test-integration
```

Строка подключения берётся из `DATABASE_URL`, а если переменная не задана —
из корневого `.env`. Используйте локальную базу для разработки или тестов.
Тесты создают записи с уникальными UUID и удаляют только собственные данные
через `t.Cleanup`; общая очистка таблиц не выполняется. Обычный `make test`
интеграционные тесты не запускает.

Интеграционные сценарии находятся в `internal/repository/trip/`:

- `trip_integration_test.go`: поездки, уникальность активной поездки водителя,
  завершение и отсутствие записей;
- `status_history_integration_test.go`: сохранение истории с пустым и
  непустым предыдущим статусом, ограничение внешнего ключа;
- `idempotency_keys_integration_test.go`: вставка и чтение ключа, дубликаты,
  завершение, удаление и отсутствующие записи;
- `transaction_integration_test.go`: реальные usecase и менеджер транзакций,
  успешное сохранение истории и откат при нарушении `CHECK` во второй записи.

Тесты отката не меняют схему и не отключают ограничения. Они передают
недопустимый конечный статус в репозиторий истории внутри транзакции,
проверяют ошибку PostgreSQL `23514`, а затем читают состояние вне транзакции.
Отказ подключения к БД считается ошибкой теста, а не пропуском.

Первый сценарий можно запустить отдельно и повторить дважды:

```bash
go test -race -tags=integration ./internal/repository/trip -run TestTripCreateAndGetIntegration -count=2 -v
```

## Что сделано

- `POST /api/v1/trips`, `GET /api/v1/trips/{tripId}`,
  `POST /api/v1/trips/{tripId}/finish`, `GET /health`, `GET /ready` по контракту.
- Ошибки в `application/problem+json` с полем `code`: `invalid_request`,
  `trip_not_found`, `trip_completed`, `driver_busy`, `idempotency_conflict`,
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

`internal/infra/postgres/transaction`. Бизнес-код видит только интерфейс
из своего `contract.go`:

```go
type txManager interface {
    Do(ctx context.Context, fn func(ctx context.Context) error) error
}
```

- `Do` открывает транзакцию (`BeginTx` с `ReadCommitted`), кладёт `pgx.Tx` в
  контекст под приватным ключом и вызывает `fn` с этим контекстом.
- `fn` вернула `nil` → `COMMIT`. Ошибка или паника → `ROLLBACK` в `defer`;
  ошибка возвращается наружу без изменений (чтобы работал `errors.Is`), паника
  летит дальше. Откат выполняется с `context.WithoutCancel` и собственным
  таймаутом, поэтому срабатывает и тогда, когда контекст запроса уже отменён.
- Репозиторий получает исполнителя через `transaction.GetExecutor(ctx, pool)`:
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

В одной транзакции usecase сначала загружает поездку через обычный `SELECT`
без `FOR UPDATE`. Отсутствующая поездка даёт `404 trip_not_found`. Затем
`Trip.Finish(at)` проверяет активный статус и меняет локальный объект в памяти;
для уже завершённой поездки возвращается `409 trip_completed`.

Изменение сохраняется условным запросом:

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

Таким образом, домен определяет допустимость перехода, а SQL защищает от
изменения состояния между чтением и записью. После успешного `UPDATE` usecase
добавляет историю статусов и коммитит транзакцию. При ошибке записи истории
или коммита наружу возвращается ошибка без изменённой поездки. Локальный
объект не разделяется между запросами и не публикуется до успешного коммита.
Успешное завершение теперь требует дополнительного `SELECT`.

### Время

`id`, `status` и `started_at` / `finished_at` задаёт сервис. Время хранится в
`TIMESTAMPTZ` и отдаётся в UTC, обрезанным до микросекунд — с той точностью,
с которой его хранит PostgreSQL, чтобы ответы на `POST` и `GET` совпадали.

### Идемпотентность создания поездки

`POST /api/v1/trips` принимает необязательный заголовок `Idempotency-Key` (UUID).
Без него выполняется обычное создание поездки без проверки идемпотентности.
Повтор запроса с тем же ключом не создаёт вторую поездку.

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
| тело другое | `409 idempotency_conflict` |
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
  заголовка со свободным водителем → `201`, гонка двух запросов → один `201` и один `409`;
- 20 параллельных запросов с одним ключом → один `201`, девятнадцать `409`;
- образ: `/health` и `/ready` из контейнера отвечают `200`, процесс запущен от
  `65532`, в образе нет исходников и Go toolchain.
