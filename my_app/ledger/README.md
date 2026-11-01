# ledger

Учёт транзакций с бюджетами по категориям — доменная библиотека и консольная программа.

**Хранение данных.** Ledger хранит бюджеты и траты в **PostgreSQL** (таблицы `budgets` и `expenses`),
поэтому данные переживают перезапуск. Бизнес-правило прежнее: транзакция, которая превышает лимит
бюджета категории, отклоняется с ошибкой `budget exceeded` и в базу не попадает.

**Кеш отчётов.** Отчёт `GetReportSummary(ctx, from, to)` (суммы трат по категориям за период)
кешируется в **Redis** под ключом `report:summary:<from>:<to>` с TTL **30 секунд**:
первый запрос считается в БД (`cache miss` → `cache set`), повторные в течение 30 секунд
берутся из Redis (`cache hit`). Если Redis недоступен, отчёт считается из БД — кеш не обязателен.

Подключение к БД и Redis выполняется в `main.go`, а если ledger используется как библиотека
(например, из gateway) — автоматически при первом обращении к данным.

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
(параллельные, `t.Parallel()`) и сценарий превышения бюджета. Перед каждым тестом,
который меняет данные, вызывается `Reset()`, чтобы тесты не влияли друг на друга.
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
