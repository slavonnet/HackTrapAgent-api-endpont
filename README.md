# HackTrapAgent API Endpoint

HTTP API для регистрации событий из access-логов (syslog/journald) с записью в ClickHouse.

## Что делает сервис

- Принимает событие `POST /event`.
- Дополняет событие полями:
  - `registered_at` = текущее время (`now()` в Go).
  - `source` = IP отправителя HTTP-запроса.
- Проверяет `blacklist` / `whitelist`.
- Применяет in-memory rate-limit для минимальной задержки ответа.
- Восстанавливает состояние rate-limit при старте из данных в ClickHouse.

## API

### `POST /event`

`Content-Type: application/json`

Поля запроса:

- `event_datetime` (optional): дата/время события, поддерживаются `RFC3339`, `RFC3339Nano`, `YYYY-MM-DD HH:MM:SS`.
- `mashine_id` (required): уникальный ID машины.
- `container_id` (optional)
- `unit_name` (optional)
- `hostname` (optional)
- `id` (optional): username/keyid/snmp-community.
- `dst_ip` (optional)
- `dst_fqdn` (optional)
- `src_ip` (required)
- `src_port` (optional)
- `dst_port` (optional)
- `protocol` (optional)
- `service_port` (optional)
- `action` (optional, default=`deny`)
- `extra` (optional, any JSON)

Пример:

`curl -X POST http://localhost:8080/event -H "Content-Type: application/json" -d '{"mashine_id":"node-1","src_ip":"10.10.1.5","dst_ip":"8.8.8.8","protocol":"tcp","dst_port":53}'`

Ответы (`{"code":"..."}`):

- `ok`
- `access denided`
- `parse error`
- `error`
- `ratelimit`
- `mashine_id not found`
- `dst_ip not found`

## Настройки (только ENV)

### HTTP

- `HTTP_ADDR` (default `:8080`)
- `REQUEST_BODY_LIMIT_BYTES` (default `1048576`)

### ClickHouse

- `CLICKHOUSE_ADDRS` (default `localhost:9000`, CSV)
- `CLICKHOUSE_DATABASE` (default `default`)
- `CLICKHOUSE_USERNAME` (default `default`)
- `CLICKHOUSE_PASSWORD` (optional)
- `CLICKHOUSE_TABLE` (default `access_events`)
- `CLICKHOUSE_SECURE` (default `false`)
- `CLICKHOUSE_DIAL_TIMEOUT_SECONDS` (default `5`)
- `CLICKHOUSE_MAX_OPEN_CONNS` (default `10`)
- `CLICKHOUSE_MAX_IDLE_CONNS` (default `10`)
- `CLICKHOUSE_CONN_MAX_LIFETIME_SECONDS` (default `300`)

### Rate-limit

- `LIMITS` (optional JSON-массив):

`[{"keys":["source","mashine_id"],"window":60,"limit":100},{"keys":["source"],"window":10,"limit":20}]`

Логика:

- Ключ корзины: `rule + keys`.
- Удаляются все timestamps, вышедшие из окна `window`.
- Если внутри окна количество `>= limit` -> `ratelimit`.
- Иначе текущее время добавляется, запрос пропускается.

При старте приложения счётчики инициализируются из ClickHouse за максимальное окно `window`.

### Whitelist / Blacklist

- `WHITELIST` (optional JSON array)
- `BLACKLIST` (optional JSON array)

Формат правила — map полей события, которые должны одновременно совпасть:

`[{"source":"10.0.0.5"},{"mashine_id":"trusted-node","src_ip":"10.1.1.20"}]`

Порядок применения:

1. `whitelist` (если совпало — запрос сразу разрешается и пишется в БД),
2. `blacklist` (`access denided`),
3. `ratelimit`.

## Локальный запуск

1) Заполнить ENV.
2) Запустить:

`go run ./cmd/server`

## Docker

Сборка:

`docker build -t ghcr.io/slavonnet/hacktrapagent-api-endpont:latest .`

Запуск:

`docker run --rm -p 8080:8080 -e CLICKHOUSE_ADDRS=clickhouse:9000 -e CLICKHOUSE_DATABASE=default -e CLICKHOUSE_USERNAME=default -e CLICKHOUSE_PASSWORD=secret -e CLICKHOUSE_TABLE=access_events ghcr.io/slavonnet/hacktrapagent-api-endpont:latest`

Публикация после релиза:

- при `GitHub Release` workflow публикует образ в GHCR:
  - `ghcr.io/slavonnet/hacktrapagent-api-endpont:<tag>`
  - `ghcr.io/slavonnet/hacktrapagent-api-endpont:latest`
- а также прикладывает архив образа в assets релиза:
  - `hacktrapagent-api-endpont-<tag>-linux-amd64.tar.gz`

Ссылки:

- GHCR package: `https://github.com/slavonnet/HackTrapAgent-api-endpont/pkgs/container/hacktrapagent-api-endpont`
- Release assets: `https://github.com/slavonnet/HackTrapAgent-api-endpont/releases`

Загрузка из release-архива (например для Proxmox CT):

`wget https://github.com/slavonnet/HackTrapAgent-api-endpont/releases/download/<tag>/hacktrapagent-api-endpont-<tag>-linux-amd64.tar.gz`

`gunzip hacktrapagent-api-endpont-<tag>-linux-amd64.tar.gz && docker load -i hacktrapagent-api-endpont-<tag>-linux-amd64.tar`

Особенности Dockerfile:

- multi-stage (`deps` -> `build` -> runtime),
- отдельный слой `go mod download` для кэширования зависимостей,
- минимальный runtime образ `distroless`,
- статически собранный бинарник (`CGO_ENABLED=0`, `-s -w`).