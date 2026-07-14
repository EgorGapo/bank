# Мини-банк «Ledger» — техническое задание

Учебно-боевой проект уровня middle+: сервис счетов и переводов, корректный под высокой
конкурентной нагрузкой. Цель — не продукт, а инженерная глубина: транзакции, идемпотентность,
transactional outbox, observability.

---

## 1. Функциональные требования

### Пользовательские сценарии
1. Создать счёт (валюта одна — минорные единицы, «копейки», `bigint`; никаких float).
2. Пополнить счёт (deposit) / снять со счёта (withdraw).
3. Перевести деньги со счёта на счёт (transfer) — ядро проекта.
4. Посмотреть баланс счёта.
5. Посмотреть историю операций по счёту (с курсорной пагинацией, не offset!).
6. Получать события об операциях во внешнем мире (Kafka) — на их основе работает
   второй сервис — **notifier** (пишет «уведомления» в лог/таблицу).

### Инварианты (главное в проекте — они не нарушаются НИКОГДА)
- I1: Баланс счёта не может стать отрицательным.
- I2: Деньги не появляются и не исчезают: сумма всех проводок системы = 0
  (double-entry: каждая операция — минимум две записи в ledger, дебет + кредит).
- I3: Повторный запрос с тем же `Idempotency-Key` не создаёт вторую операцию,
  а возвращает результат первой (включая повтор во время выполнения первой — тут думать).
- I4: Каждая завершённая операция порождает ровно одно событие в Kafka
  (at-least-once доставка + идемпотентный consumer = effectively-once).

---

## 2. Архитектура

```
клиент → nginx (reverse proxy, rate limit) → api (Go)
                                               ├── PostgreSQL (данные + outbox)
                                               └── (пишет в outbox в той же транзакции)
outbox-relay (горутина внутри api или отдельный процесс) → Kafka
Kafka → notifier (Go, отдельный сервис) → таблица notifications / лог
Prometheus ← /metrics обоих сервисов;  Grafana;  трейсы → OTel collector → Jaeger/Tempo
```

Два Go-сервиса в одном репозитории (монорепо, `cmd/api`, `cmd/notifier`).
Всё поднимается одним `docker compose up`.

### Слои api-сервиса
- `transport/http` — хендлеры, валидация, коды ошибок. Никакой бизнес-логики.
- `service` — бизнес-логика, управление транзакциями (Unit of Work).
- `repository` — SQL. Никаких `SELECT *`.
- `pkg/...` — логгер, middleware и прочее переиспользуемое.

---

## 3. Модель данных (Postgres)

Схема словами — DDL пишешь сам через миграции (goose или golang-migrate).

- **accounts**: id (uuid), status (active/frozen/closed), balance (bigint, минорные единицы),
  created_at, updated_at. Баланс хранится денормализованно для скорости чтения,
  но истина — в ledger; их сходимость проверяется тестом/джобой.
- **transfers** (операции: deposit/withdraw/transfer): id, idempotency_key (uniq),
  type, from_account_id (nullable для deposit), to_account_id (nullable для withdraw),
  amount, status (pending/completed/failed), error_code, created_at, completed_at.
- **ledger_entries** (double-entry, append-only, никогда не UPDATE/DELETE):
  id (bigserial), transfer_id, account_id, amount (со знаком: дебет минус, кредит плюс),
  balance_after, created_at.
- **outbox**: id, topic, key, payload (jsonb), created_at, sent_at (nullable).
  Индекс по (sent_at) WHERE sent_at IS NULL — частичный, чтобы relay читал дёшево.
- **notifications** (в notifier, можно та же БД, отдельная схема): event_id (uniq — вот она,
  идемпотентность consumer'а), account_id, text, created_at.

Ключевые решения, которые надо принять и обосновать (в README):
- Уровень изоляции: READ COMMITTED + явные блокировки строк vs SERIALIZABLE + retry. 
  Рекомендация: READ COMMITTED + `SELECT ... FOR UPDATE`, локи брать **в детерминированном
  порядке** (например, по возрастанию account_id) — иначе дедлок на встречных переводах A→B / B→A.
- Идемпотентность: уникальный индекс на idempotency_key + обработка конфликта. Продумать
  случай «второй запрос пришёл, пока первый ещё выполняется» (статус pending).

---

## 4. HTTP API (v1)

Все мутации принимают заголовок `Idempotency-Key` (uuid, обязателен).

| Метод | Путь | Что делает |
|---|---|---|
| POST | /v1/accounts | создать счёт |
| GET | /v1/accounts/{id} | счёт + баланс |
| POST | /v1/accounts/{id}/deposit | пополнение {amount} |
| POST | /v1/accounts/{id}/withdraw | снятие {amount} |
| POST | /v1/transfers | перевод {from, to, amount} |
| GET | /v1/transfers/{id} | статус операции |
| GET | /v1/accounts/{id}/history?cursor=&limit= | выписка по ledger, курсорная пагинация |
| GET | /healthz, /readyz | liveness / readiness (readyz проверяет БД) |

Формат ошибок единый: `{"error": {"code": "insufficient_funds", "message": "..."}}`.
Коды: 400 валидация, 404 нет счёта, 409 конфликт идемпотентности/статуса,
422 insufficient_funds, 429 от nginx, 5xx только на реальные аварии.

---

## 5. Kafka

- Топик `ledger.operations` (partitions ≥ 3): события `operation.completed` /
  `operation.failed`. **Key = account_id** — гарантирует порядок событий по счёту
  внутри партиции (для transfer — ключ from_account_id, обосновать выбор).
- Схема события: event_id (uuid, = id записи outbox), type, occurred_at, payload.
  Версионировать поле `schema_version` сразу.
- Producer: только через outbox-relay (никогда напрямую из хендлера!). Relay читает
  неотправленные записи батчами, шлёт с acks=all, помечает sent_at. Допускаются дубли
  при падении между send и commit — поэтому consumer идемпотентен (uniq по event_id).
- Consumer (notifier): consumer group, ручной commit оффсетов ПОСЛЕ записи в БД,
  обработка poison message (после N ретраев — в топик `ledger.operations.dlq`).
- Библиотека: segmentio/kafka-go или franz-go (рекомендую franz-go).

---

## 6. Нефункциональные требования

### Нагрузка (критерии приёмки)
- 1000+ RPS переводов на локальной машине через nginx, p99 < 100ms.
- Нагрузочный сценарий «hot account»: 100 горутин долбят переводы с/на один счёт —
  всё сериализуется корректно, дедлоков в логах нет (или есть retry и они невидимы клиенту).
- **Тест-инвариант**: после любого нагрузочного прогона `SUM(amount) по ledger_entries = 0`
  и `accounts.balance = SUM по ledger` для каждого счёта. Расхождение = провал.
- Инструмент: k6 или vegeta.

### Логирование
- `log/slog`, JSON. Обязательные поля: request_id (генерит/принимает middleware,
  прокидывается через context, уходит в ответ заголовком и в Kafka-события),
  method, path, status, duration_ms.
- Уровни осмысленно: Info — факты бизнес-операций, Warn — ретраи/конфликты, Error — аварии.
  Никаких логов в цикле на каждый ledger entry.

### Observability
- **Метрики (Prometheus)**: RED для HTTP (rate, errors, duration histogram),
  бизнес-метрики (transfers_total by status, insufficient_funds_total),
  outbox lag (кол-во неотправленных), kafka consumer lag, пул коннектов pgx.
- **Трейсы (OpenTelemetry)**: span на HTTP-запрос → span на транзакцию БД → span
  на публикацию в Kafka; trace context пробрасывается через Kafka headers в notifier —
  один трейс от HTTP-запроса до записи уведомления.
- **Grafana**: один дашборд с RED + бизнес-метрики + лаги.

### Nginx
- Reverse proxy перед api, `limit_req` (например, 50 rps/ip с burst), таймауты,
  access log в JSON с request_id (проброс `X-Request-Id`).

### Прочее
- Конфигурация через env (без магии, можно caarlos0/env).
- Graceful shutdown обоих сервисов: перестать принимать, дождаться in-flight, закрыть пул/producer.
- Миграции: goose, запускаются отдельной командой/контейнером, не при старте api.
- pgx v5 + pgxpool напрямую (без ORM — цель проекта пощупать SQL и транзакции руками).
- Тесты: unit на service-слой, интеграционные на repository через testcontainers-go,
  e2e-скрипт на docker compose.
- Линт: golangci-lint. CI по желанию (GitHub Actions: lint + test).

---

## 7. Что почитать перед соответствующими этапами
- Transactional outbox — microservices.io/patterns/data/transactional-outbox.html
- Уровни изоляции Postgres — глава 13 доки Postgres
- Идемпотентность API — статья Stripe «Designing robust and predictable APIs with idempotency»
