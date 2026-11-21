# gateway

Простой HTTP-сервис для учёта транзакций и бюджетов (DTO-слой `internal/api`
поверх сервиса `ledger`).

HTTP-контракты, DTO и формат ошибок не изменились. Gateway не знает про базу данных:
он получает готовый сервис через фабрику ledger и вызывает его методы.
Данные хранятся в PostgreSQL, отчёты кешируются в Redis на 30 секунд.

## Инициализация сервиса

Gateway зависит только от интерфейса `ledger.Service` и фабрики `ledger.NewService` —
он не импортирует `database/sql`, `pgx`, `ledger/internal/...` и репозитории.

Структура:

```
gateway/
├── main.go                  # инициализация сервиса, маршруты, запуск HTTP-сервера
└── internal/api/
    ├── handler.go           # тип Handler{svc ledger.Service}, NewHandler, HTTP-обработчики
    ├── routes.go            # RegisterRoutes(mux, h)
    ├── dto.go               # DTO запросов/ответов и преобразование в доменные модели
    └── handlers_test.go     # тесты обработчиков через httptest
```

В обработчиках вызываются методы сервиса (`SetBudget`, `ListBudgets`, `AddTransaction`,
`ListTransactions`, `GetReportSummary`) с контекстом запроса `r.Context()`.

## Подготовка

1. Запустить PostgreSQL и Redis, применить миграции — см. `../ledger/README.md` (разделы 1–3).
2. Заполнить `../ledger/.env` (`DATABASE_URL`, `REDIS_ADDR`, `REDIS_DB`, `REDIS_PASSWORD`).

## Запуск

```bash
cd my_app/gateway
set -a; source ../ledger/.env; set +a   # переменные подключения к PostgreSQL и Redis
go run .
```

Сервис слушает порт **8083**. Остановка: `Ctrl+C`. При старте в логе появится
`Connected to database`. Если БД недоступна, gateway пишет ошибку и сразу завершается.

## Эндпоинты

| Метод | Путь                  | Описание                                   |
|-------|-----------------------|---------------------------------------------|
| GET   | `/ping`                | health-check, `200 OK`, тело `pong`         |
| POST  | `/api/transactions`    | создать транзакцию                          |
| GET   | `/api/transactions`    | список всех транзакций                      |
| POST  | `/api/budgets`         | создать (или обновить) бюджет по категории  |
| GET   | `/api/budgets`         | список всех бюджетов                        |
| GET   | `/api/reports/summary` | суммы трат по категориям за период `?from=&to=` (кеш в Redis) |

Все ответы `/api/...` отдаются с заголовком
`Content-Type: application/json; charset=utf-8`.

Ошибки возвращаются в формате `{"error": "описание ошибки"}` со статусом:

- `400 Bad Request` — некорректный JSON или ошибка валидации (например,
  `amount <= 0` или пустая `category`);
- `409 Conflict` — превышение лимита бюджета по категории;
- `400 Bad Request` для отчёта — нет `from`/`to`, даты не в формате `YYYY-MM-DD` или `from` позже `to`;
- `500 Internal Server Error` — прочие непредвиденные ошибки.

## Примеры cURL

### Health-check

```bash
curl -i http://localhost:8083/ping
```

Ожидается `HTTP/1.1 200 OK` и тело `pong`.

### Создать транзакцию

```bash
curl -X POST http://localhost:8083/api/transactions \
  -H "Content-Type: application/json" \
  -d '{"amount":450,"category":"еда","description":"ланч","date":"2025-09-10"}'
```

Успешный ответ — `201 Created`:

```json
{"id":1,"amount":450,"category":"еда","description":"ланч","date":"2025-09-10"}
```

### Получить список транзакций

```bash
curl -i http://localhost:8083/api/transactions
```

`200 OK`, тело — массив `TransactionResponse` (пустой список выглядит как `[]`).

### Создать (или обновить) бюджет

```bash
curl -X POST http://localhost:8083/api/budgets \
  -H "Content-Type: application/json" \
  -d '{"category":"еда","limit":5000}'
```

Успешный ответ — `201 Created`:

```json
{"category":"еда","limit":5000,"period":"2026/10"}
```

### Получить список бюджетов

```bash
curl -i http://localhost:8083/api/budgets
```

`200 OK`, тело — массив `BudgetResponse` (пустой список выглядит как `[]`).

### Отчёт по категориям и эффект кеша

Выполните запрос **дважды подряд** (в пределах 30 секунд):

```bash
curl -i "http://localhost:8083/api/reports/summary?from=2026-10-01&to=2026-10-31"
curl -i "http://localhost:8083/api/reports/summary?from=2026-10-01&to=2026-10-31"
```

Оба ответа — `200 OK` с одинаковым телом:

```json
{"from":"2026-10-01","to":"2026-10-31","totals":{"electronics":60000,"food":3000,"profit":50,"travel":4000}}
```

В логе gateway видно, откуда взят результат:

```
cache miss: report:summary:2026-10-01:2026-10-31
cache set: report:summary:2026-10-01:2026-10-31 TTL 30s
GET /api/reports/summary 3.1ms
cache hit: report:summary:2026-10-01:2026-10-31
GET /api/reports/summary 450µs
```

Через 30 секунд ключ истекает, и следующий запрос снова будет `cache miss`.
Посмотреть ключ и оставшееся время жизни:

```bash
docker exec -it cashcraft-redis redis-cli TTL report:summary:2026-10-01:2026-10-31
```

### Проверка, что данные лежат в PostgreSQL

```bash
docker exec -it cashapp-db psql -U postgres -d cashapp -c "SELECT * FROM budgets;"
docker exec -it cashapp-db psql -U postgres -d cashapp -c "SELECT * FROM expenses ORDER BY id;"
```

После перезапуска gateway (`Ctrl+C`, `go run .`) `GET /api/budgets` и
`GET /api/transactions` возвращают те же записи.

### Пример ошибки: превышение бюджета

Если сумма транзакций по категории превысит `limit` установленного бюджета,
сервис ответит `409 Conflict`:

```bash
curl -i -X POST http://localhost:8083/api/transactions \
  -H "Content-Type: application/json" \
  -d '{"amount":1000000,"category":"еда","date":"2025-09-10"}'
```

```json
{"error":"budget exceeded"}
```

### Пример ошибки: невалидные данные

```bash
curl -i -X POST http://localhost:8083/api/transactions \
  -H "Content-Type: application/json" \
  -d '{"amount":0,"category":"еда"}'
```

`400 Bad Request`:

```json
{"error":"amount must be greater than 0!"}
```

## Тесты

Интеграционные тесты HTTP-обработчиков лежат в `internal/api/handlers_test.go`.
Они получают сервис через `ledger.NewService`, собирают роутер через
`RegisterRoutes(mux, NewHandler(svc))` и отправляют запросы с помощью
`httptest.NewRequest` + `httptest.NewRecorder` (настоящий порт не нужен).
Проверяются коды ответов (201, 200, 400, 409), JSON-тело и заголовок `Content-Type`.
Сценарии, которые пишут данные, требуют запущенного PostgreSQL и загруженного `../ledger/.env`
(без БД они пропускаются). Проверки формата и порядка дат отчёта также требуют сервиса и БД;
проверка отсутствующих параметров выполняется без них.

```bash
cd my_app/gateway
go mod tidy          # один раз, подтянуть testify
go test -v ./...     # подробный вывод, видны подтесты ok / exceeded / bad_json
```

### Покрытие

```bash
go test ./... -cover -coverprofile=cover.out
go tool cover -html=cover.out -o cover.html   # открыть cover.html в браузере
```

**Покрытие тестами пакета `internal/api` ~66%.**

Общий отчёт по обоим модулям можно получить из `my_app`
(там `go.work`, поэтому `./...` из этой папки не работает — пути указываются явно):

```bash
cd my_app
go test ./ledger/... ./gateway/... -coverprofile=cover.out
go tool cover -html=cover.out -o cover.html
```
