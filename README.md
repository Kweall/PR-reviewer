# PR Reviewer Service

Сервис назначения ревьюеров для Pull Request’ов внутри команды.
Микросервис автоматически назначает ревьюеров на PR, управляет командами и участниками. Взаимодействие происходит через HTTP API.

## Описание

Сервис позволяет:

* Создавать команды и управлять пользователями (активность, массовая деактивация).
* Создавать Pull Request и автоматически назначать до двух ревьюверов из команды автора.
* Переназначать ревьюверов на случайного активного участника команды.
* Получать список PR, назначенных конкретному пользователю.
* Получать статистику по PR и ревьюверам.
* Идемпотентно выполнять merge PR, после чего изменение ревьюверов невозможно.

## Основные сущности

* **User**: `user_id`, `username`, `team_name`, `is_active`
* **Team**: `team_name`, `members`
* **Pull Request**: `pull_request_id`, `pull_request_name`, `author_id`, `status` (OPEN|MERGED), `assigned_reviewers` (до 2), `needMoreReviewers`, `createdAt`, `megedAt`

## Логика работы PR

* При создании PR назначаются до двух активных ревьюверов из команды автора (автор исключается).
* Переназначение заменяет одного ревьювера на случайного активного участника команды.
* После MERGED PR нельзя менять состав ревьюверов.
* Если доступных кандидатов меньше двух, назначается доступное количество (0/1).

## API

Сервис использует HTTP API. OpenAPI спецификация доступна в файле [`openapi.yml`](./openapi.yml).

Основные эндпоинты:

| Метод | URL                   | Описание                                 |
| ----- | --------------------- | ---------------------------------------- |
| POST  | /team/add             | Добавить команду с пользователями        |
| GET   | /team/get             | Получить информацию о команде            |
| POST  | /users/setIsActive    | Активировать/деактивировать пользователя |
| POST  | /pullRequest/create   | Создать PR и назначить ревьюверов        |
| POST  | /pullRequest/merge    | Обновить статус PR на MERGED             |
| POST  | /pullRequest/reassign | Переназначить ревьювера                  |
| GET   | /users/getReview      | Получить список PR для пользователя      |
| GET   | /stats                | Получить статистику по PR и ревьюверам   |
| POST  | /team/deactivate      | Массово деактивировать команду           |

## Условия и ограничения

* Объём данных: до 20 команд и 200 пользователей.
* RPS: 5, SLI времени ответа: 300 мс, SLI успешности: 99.9%.
* Пользователи с `isActive = false` не назначаются на ревью, но остаются видимыми в списках.
* Операция merge идемпотентна.
* Сервис и зависимости запускаются командой `docker-compose up`.
* Сервис доступен на порту 8080.

## Дополнительные возможности

* Эндпоинт статистики (`/stats`).
* Нагрузочное тестирование (см. каталог `/loadtest`).
* Массовая деактивация пользователей команды с безопасной переназначаемостью открытых PR.
* Интеграционное/E2E-тестирование (`/e2e`).
* Конфигурация линтера описана в `.golangci.yml`.

## Запуск сервиса

1. Клонируйте репозиторий:

```bash
git clone https://github.com/kweall/PR-reviewer
cd PR-reviewer
```

2. Создайте `.env` (необязательно, по умолчанию используется порт 8080 и БД `prdb`):

```
PORT=8080
DATABASE_DSN=postgres://pruser:prpass@localhost:5432/prdb?sslmode=disable
```

3. Запустите сервис через Docker Compose:

```bash
docker-compose up --build
```

4. Сервис доступен на `http://localhost:8080`.

## Миграции базы данных

Сервис использует PostgreSQL. Миграции выполняются автоматически при запуске через `docker-compose up`.

Схема:

```sql
CREATE TABLE IF NOT EXISTS teams (
    team_name TEXT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS users (
    user_id TEXT PRIMARY KEY,
    username TEXT NOT NULL,
    team_name TEXT NOT NULL REFERENCES teams(team_name) ON DELETE CASCADE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS pull_requests (
    pull_request_id TEXT PRIMARY KEY,
    pull_request_name TEXT NOT NULL,
    author_id TEXT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    merged_at TIMESTAMP NULL
);

CREATE TABLE IF NOT EXISTS pr_reviewers (
    pull_request_id TEXT NOT NULL REFERENCES pull_requests(pull_request_id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    PRIMARY KEY (pull_request_id, user_id)
);
```

## Тестирование

* E2E-тесты находятся в `/e2e/e2e_test.go`.
* Нагрузочное тестирование реализовано с помощью `k6` (см. `/loadtest`).
* Для запуска E2E тестов:

```bash
go test ./e2e -v
```

**Время выполнения:** 0.670s

## Тестируемые сценарии

### 1. Add Team

* Team: `backend`
* Members:
  * `u1` – Alice (active: true)
  * `u2` – Bob (active: true)
  * `u3` – Charlie (active: false)
* **Result:** Team created successfully, members added.

### 2. Create Pull Request

* PR: `Test PR` (`pr-10000`)
* Author: `u1`
* **Result:** PR created and assigned automatically to `u2`.

### 3. Get Team Info

* Endpoint: GET `/team/get?team_name=backend`
* **Result:** Successfully returned all members of the `backend` team.

### 4. Reassign Pull Request

* PR `pr-10000` reassigned to user `u2`
* **Result:** Successfully reassigned.

### 5. Get User Reviews

* Endpoint: GET `/users/getReview`
* **Result:** Successfully retrieved all reviews.

### 6. Deactivate Team

* Endpoint: POST `/team/deactivate`
* **Result:** Team `backend` deactivated successfully, verified in DB.

---

## Summary

* Все API эндпоинты прошли E2E тест.
* Тестовая база данных была создана и почищена.
* Время выполнения: **0.670 секунды**.
* Ошибок нет.

## Линтер

Используется `golangci-lint` с конфигурацией в `.golangci.yml`:

* Включены проверки: `errcheck`, `govet`, `staticcheck`, `gosimple`.
* Запуск:

```bash
golangci-lint run
```


## Результаты нагрузочного тестирования

### Общие параметры тестирования

* Объём данных: до 20 команд, до 200 пользователей
* RPS (запросов в секунду): 5 (целевое)
* SLI времени ответа: ≤ 300 мс
* SLI успешности: ≥ 99.9%

### Настройки тестов

* Скрипты: pr_create.js, pr_reassign.js, team_add.js, team_get.js, user_get_review.js
* Итерации: от 10 до 20 на сценарий

### Результаты выполнения

Сводная таблица по всем сценариям:

| Сценарий        | Итерации | Среднее время отклика | p90     | p95     | SLI Успешности |
| --------------- | -------- | --------------------- | ------- | ------- | ---------- |
| pr_create       | 20       | 4.0 мс                | 10.2 мс | 14.5 мс | 100%       |
| pr_reassign     | 20       | 4.0 мс                | 10.5 мс | 16.2 мс | 100%       |
| team_add        | 10       | 4.1 мс                | 10.0 мс | 13.0 мс | 100%       |
| team_get        | 20       | 4.5 мс                | 11.0 мс | 15.0 мс | 100%       |
| user_get_review | 20       | 4.6 мс                | 11.5 мс | 16.0 мс | 100%       |

* Общее количество итераций: 90
* Максимальная нагрузка: ~650 запросов/сек (RPS)
* Процент ошибок: 0%
* Максимальное p95 время отклика: 16.2 мс

### Вывод

Все сценарии нагрузочного тестирования успешно выполнены. Время отклика и успешность запросов полностью соответствуют заданным SLI. Приложение устойчиво работает при нагрузке, превышающей целевой RPS (5 RPS), с большим запасом по времени отклика и успешности.