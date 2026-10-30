# gateway

Простой HTTP-сервис для учёта транзакций и бюджетов (DTO-слой `internal/api`
поверх доменной библиотеки `ledger`).

## Запуск

```bash
cd second_task/gateway
go run .
```

Сервис слушает порт **8083**. Остановка: `Ctrl+C`.

## Эндпоинты

| Метод | Путь                  | Описание                                   |
|-------|-----------------------|---------------------------------------------|
| GET   | `/ping`                | health-check, `200 OK`, тело `pong`         |
| POST  | `/api/transactions`    | создать транзакцию                          |
| GET   | `/api/transactions`    | список всех транзакций                      |
| POST  | `/api/budgets`         | создать (или обновить) бюджет по категории  |
| GET   | `/api/budgets`         | список всех бюджетов                        |

Все ответы `/api/...` отдаются с заголовком
`Content-Type: application/json; charset=utf-8`.

Ошибки возвращаются в формате `{"error": "описание ошибки"}` со статусом:

- `400 Bad Request` — некорректный JSON или ошибка валидации (например,
  `amount <= 0` или пустая `category`);
- `409 Conflict` — превышение лимита бюджета по категории;
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
{"error":"Amount must be greater than 0!"}
```

## Тесты

Интеграционные тесты HTTP-обработчиков лежат в `internal/api/handlers_test.go`.
Они собирают роутер через `RegisterRoutes` и отправляют запросы с помощью
`httptest.NewRequest` + `httptest.NewRecorder` (настоящий порт не нужен).
Проверяются коды ответов (201, 200, 400, 409), JSON-тело и заголовок `Content-Type`.
Перед каждым сценарием вызывается `ledger.Reset()`.

```bash
cd second_task/gateway
go mod tidy          # один раз, подтянуть testify
go test -v ./...     # подробный вывод, видны подтесты ok / exceeded / bad_json
```

### Покрытие

```bash
go test ./... -cover -coverprofile=cover.out
go tool cover -html=cover.out -o cover.html   # открыть cover.html в браузере
```

**Покрытие тестами пакета `internal/api` ~66%.**

Общий отчёт по обоим модулям можно получить из `second_task`
(там `go.work`, поэтому `./...` из этой папки не работает — пути указываются явно):

```bash
cd second_task
go test ./ledger/... ./gateway/... -coverprofile=cover.out
go tool cover -html=cover.out -o cover.html
```
