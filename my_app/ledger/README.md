# ledger

Учёт транзакций с бюджетами по категориям — доменная библиотека и консольная программа.

**Хранение данных.** Ledger хранит бюджеты и траты в **PostgreSQL** (таблицы `budgets` и `expenses`),
поэтому данные переживают перезапуск. Бизнес-правило прежнее: транзакция, которая превышает лимит
бюджета категории, отклоняется с ошибкой `budget exceeded` и в базу не попадает.

**Кеш отчётов.** Отчёт `GetReportSummary(ctx, from, to)` (суммы трат по категориям за период)
кешируется в **Redis** под ключом `report:summary:<from>:<to>` с TTL **30 секунд**:
первый запрос считается в БД (`cache miss` → `cache set`), повторные в течение 30 секунд
берутся из Redis (`cache hit`). Если Redis недоступен, отчёт считается из БД — кеш не обязателен.

Подключение к БД выполняет фабрика `ledger.NewService(ctx)` (см. раздел «Архитектура»).
Redis подключает фабрика; если он недоступен, сервис работает без кеша.

## Архитектура: слои и каталоги

Ledger разделён на слои по принципам чистой архитектуры. Зависимости направлены внутрь — к домену.

```
ledger/
├── service.go                  # публичный фасад: тип Service и фабрика NewService(ctx)
├── ledger.go                   # алиасы доменных типов, ошибки и вспомогательные функции
├── cmd/ledger/main.go          # консольная демонстрация, собирает сервис через app.New
└── internal/
    ├── domain/                 # сущности Transaction, Budget, их Validate(),
    │                           # интерфейсы BudgetRepository и ExpenseRepository, доменные ошибки
    ├── repository/pg/          # реализация интерфейсов репозиториев на PostgreSQL (SQL-запросы)
    ├── service/                # интерфейс FinanceService и бизнес-логика
    │                           # (валидация, проверка лимита бюджета)
    ├── app/                    # слой сборки: подключение к БД → репозитории → сервис
    ├── db/                     # чтение конфигурации из окружения и подключение к PostgreSQL
    └── cache/                  # реализация кеша отчётов на Redis
```

| Слой | Зависит от | Не зависит от |
|------|-----------|---------------|
| `domain` | только стандартной библиотеки | репозиториев, БД, HTTP |
| `repository/pg` | `domain`, `database/sql` | сервиса, HTTP, gateway |
| `service` | `domain` (интерфейсы репозиториев и кеша) | конкретной реализации хранения |
| `cache` | `domain`, Redis-клиента | сервиса, HTTP, gateway |
| `app` | `db`, `repository/pg`, `cache`, `service` | HTTP, gateway |

**Бизнес-правило** (в `service.AddTransaction`): транзакция валидируется, затем проверяется,
что «сумма трат по категории + новая транзакция ≤ лимит бюджета». Если бюджета для категории нет,
транзакция добавляется без проверки. При превышении возвращается `domain.ErrBudgetExceeded`.

**Фабрика** `ledger.NewService(ctx)` (внутри `app.New`):
1. читает DSN из переменных окружения;
2. открывает подключение к PostgreSQL и делает `Ping` с переданным `ctx`;
3. пробует подключить Redis и создать кеш; при ошибке продолжает без кеша;
4. создаёт репозитории и собирает `service.New(...)`, передавая необязательный кеш;
5. возвращает `(Service, closeFn, error)`; `closeFn` закрывает БД и Redis, если он подключён.

Миграции фабрика не запускает — они применяются отдельно через goose (раздел 2).

## Требования

- Go (версия не ниже указанной в `go.mod`), проверить: `go version`
- Docker (для PostgreSQL и Redis)
- [goose](https://github.com/pressly/goose) для миграций:
  `go install github.com/pressly/goose/v3/cmd/goose@latest` или `brew install goose`

## 1. PostgreSQL: создать БД

```bash
docker run --name cashapp-db -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=cashapp \
  -p 55432:5432 -d postgres:17
```

Если контейнер уже есть, но остановлен: `docker start cashapp-db`.
Проверить, что база создана: `docker exec -it cashapp-db psql -U postgres -l`.

## 2. Применить миграции Goose

Миграции лежат в `migrations/` (таблицы `budgets`, `expenses` и индекс по `(category, date)`).

```bash
cd my_app/ledger
export DATABASE_URL="postgres://postgres:postgres@localhost:55432/cashapp?sslmode=disable"

goose -dir migrations postgres "$DATABASE_URL" up      # применить все миграции
goose -dir migrations postgres "$DATABASE_URL" status  # посмотреть статус
goose -dir migrations postgres "$DATABASE_URL" down    # откатить последнюю
```

Миграции применяются перед первым запуском Ledger и после каждого изменения схемы.

## 3. Redis

```bash
docker run -p 6379:6379 --name cashcraft-redis -d redis:7-alpine
docker exec -it cashcraft-redis redis-cli ping   # → PONG
```

## 4. Переменные окружения

| Переменная       | Назначение                                   | По умолчанию     |
|------------------|----------------------------------------------|------------------|
| `DATABASE_URL`   | строка подключения к PostgreSQL (приоритетная) | —              |
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASS`, `DB_NAME` | используются, если `DATABASE_URL` не задана | `localhost`, `5432`, `postgres`, `postgres`, `cashapp` |
| `REDIS_ADDR`     | адрес Redis                                  | `localhost:6379` |
| `REDIS_DB`       | номер базы Redis                             | `0`              |
| `REDIS_PASSWORD` | пароль Redis                                 | пусто            |


## 5. Запуск

```bash
cd my_app/ledger
set -a; source .env; set +a
go run ./cmd/ledger
```

Программа подключается к PostgreSQL и Redis, загружает бюджеты из `budgets.json`
(читается из **текущей директории**, формат — массив `[{"Category": "food", "Limit": 5000, "Period": "2026/10"}]`), добавляет несколько транзакций, печатает список
и дважды запрашивает отчёт. Ожидаемый вывод (фрагмент):

```
Connected to database
Connected to Redis
...
cache miss: report:summary:2026-10-01:2026-10-31
cache set: report:summary:2026-10-01:2026-10-31 TTL 30s
cache hit: report:summary:2026-10-01:2026-10-31
```

Проверить кеш в Redis:

```bash
docker exec -it cashcraft-redis redis-cli KEYS 'report:*'
docker exec -it cashcraft-redis redis-cli TTL report:summary:2026-10-01:2026-10-31
```

HTTP-проверка (cURL) — через сервис gateway, см. `../gateway/README.md`.

## Тесты

```bash
cd my_app/ledger
go test ./...        # все тесты
go test -v ./...     # подробный вывод
go vet ./...         # статическая проверка
```

Тесты в `ledger_test.go`: табличные проверки `Validate()` для транзакций и бюджетов
(параллельные, `t.Parallel()`) и сценарий превышения бюджета через сервис (`NewService`).
Каждый тест, который пишет в базу, использует уникальную категорию и удаляет её данные после себя.
Сценарии, которые пишут в базу, требуют запущенного PostgreSQL и загруженного `.env`.

Перед первым запуском нужна зависимость testify:

```bash
go get github.com/stretchr/testify
```

### Покрытие

```bash
go test ./... -cover -coverprofile=cover.out
go tool cover -html=cover.out -o cover.html   # открыть cover.html в браузере
```

**Покрытие тестами пакета `ledger` ~54%.**
