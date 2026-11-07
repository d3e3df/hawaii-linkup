# HTTP API LinkUp — контракт ЛР3

## Базовые правила

Приложение по умолчанию слушает `http://127.0.0.1:8080`, локальный Profile API — `http://127.0.0.1:8081`.

Все три бизнес-маршрута требуют заголовок `X-Demo-User`. Его значение является идентификатором текущего пользователя (actor). Отсутствие или неверный формат заголовка даёт `400 invalid_identity`. Это учебная подстановка identity, а не аутентификация: любой клиент может указать чужой идентификатор.

ID пользователя имеет формат `u` + положительное десятичное число без ведущих нулей (регулярное выражение `^u[1-9][0-9]*$`). Path-параметр и demo identity проверяются по этому правилу.

Сервер сам создаёт request ID и возвращает его в `X-Request-ID`. Входящее значение этого заголовка не используется как доверенное.

Для всех бизнес-маршрутов тело запроса запрещено. Пустое тело допустимо; непустое даёт `400 body_not_allowed`. Читается не более одного байта для обнаружения тела. Поскольку JSON-body не принимается, статусы 413 и 415 не применяются. Неизвестные и повторённые query-параметры запрещены. Для Like и Inspect query должна отсутствовать.

Успешные ответы и обработанные ошибки бизнес-маршрутов имеют `Content-Type: application/json`. Общая форма ошибки:

```json
{"code":"target_not_found","message":"Целевой профиль не найден"}
```

Внутренний текст ошибки в ответ не включается. Неизвестные пути и методы обрабатываются стандартным `http.ServeMux` с ответами 404/405 не в JSON-формате.

## 1. Выдача кандидатов

`GET /v1/me/discovery`

Query:

- `interest` — необязательный предметный фильтр по существующему полю `User.Interests`; перед сравнением выполняются `TrimSpace` и перевод в нижний регистр. Пустое значение означает отсутствие фильтра.
- `limit` — необязательный, по умолчанию 20, целое от 1 до 50.
- `offset` — необязательный, по умолчанию 0, неотрицательное целое.

Пример:

```http
GET /v1/me/discovery?interest=travel&limit=2&offset=0
X-Demo-User: u3
```

Успех — `200 OK`:

```json
{
  "items": [
    {"id":"u1","name":"Анна","age":25,"city":"Москва","interests":["music","travel"]},
    {"id":"u6","name":"Егор","age":35,"city":"Москва","interests":["travel"]}
  ],
  "limit": 2,
  "offset": 0
}
```

Сначала применяются прежние правила ленты: исключаются actor, неактивные, заблокированные в любую сторону и уже лайкнутые actor пользователи. Затем применяется `interest`, после этого — `offset` и `limit`. Порядок соответствует стабильному порядку пользователей в memory store. Offset за концом выдачи и отсутствие совпадений возвращают `"items":[]`, а не `null` и не ошибку. Между разными запросами не обещается общий снимок изменяющихся данных.

## 2. Команда Like

`PUT /v1/me/likes/{targetID}`

Actor определяется только через `X-Demo-User`; body отсутствует. Перед локальным изменением приложение запрашивает target в Profile API. Профиль должен существовать и быть активным. Затем применяются прежние `eligibility.Check`, `interaction.Decide` и `reaction.Transition`.

Первый допустимый Like — `200 OK`:

```json
{"outcome":"pending","changed":true,"likes":1,"matched":false,"matched_now":false}
```

Повтор в неизменных условиях — согласованный no-op без новой записи:

```json
{"outcome":"pending","changed":false,"likes":1,"matched":false,"matched_now":false}
```

Встречный Like возвращает `outcome:"match"`, `likes:2`, `matched:true`; `matched_now:true` только у запроса, который создал match. Повтор после match имеет `changed:false` и `matched_now:false`.

## 3. Чтение состояния пары

`GET /v1/me/pairs/{targetID}`

Маршрут только читает ту же canonical `Pair`, которую меняет Like. Profile API не вызывается и запись не выполняется.

Успех — `200 OK`:

```json
{"target_id":"u2","liked_by_me":true,"liked_me":false,"likes":1,"matched":false}
```

Отсутствие пары — начальное состояние, а не 404:

```json
{"target_id":"u2","liked_by_me":false,"liked_me":false,"likes":0,"matched":false}
```

Этот маршрут не проверяет существование профиля. Поэтому syntactically valid, но неизвестный actor/target также может получить начальное состояние, если пары нет.

## Служебные маршруты

- `GET /healthz` приложения возвращает `200 OK` и `{"status":"ok"}`; demo identity не требуется.
- `GET /v1/profiles/{id}` Profile API возвращает профиль в режиме `ok`.
- `PUT /_admin/mode/{mode}` Profile API переключает `ok`, `missing`, `error`, `slow`, `large`. Это служебный интерфейс учебной зависимости, не бизнес-API LinkUp.

Режимы Profile API:

| Режим | Ответ зависимости | Результат Like |
|---|---|---|
| `ok` | Корректный профиль | Выполняется по предметным правилам |
| `missing` | 404 | 404 `target_not_found`, записи нет |
| `error` | 500 | 502 `profile_unavailable`, записи нет |
| `slow` | Задержка больше command budget | 504 `profile_timeout`, записи нет |
| `large` | Тело больше лимита клиента | 502 `profile_response_too_large`, записи нет |

## Ошибки

| Причина | HTTP | `code` |
|---|---:|---|
| Неверный ID | 400 | `invalid_id` |
| Неверная/неизвестная/повторённая query | 400 | `invalid_query` |
| Нет или неверен `X-Demo-User` | 400 | `invalid_identity` |
| Непустое body | 400 | `body_not_allowed` |
| Actor отсутствует в локальных данных | 404 | `actor_not_found` |
| Target отсутствует (включая missing Profile API) | 404 | `target_not_found` |
| Самолайк | 409 | `self_like` |
| Неактивный actor или target | 409 | `inactive_profile` |
| Блокировка | 409 | `blocked` |
| Profile API вернул 500 или сетевая недоступность | 502 | `profile_unavailable` |
| Неверный JSON/обязательные поля/ID ответа Profile API | 502 | `profile_invalid_response` |
| Ответ Profile API превысил лимит | 502 | `profile_response_too_large` |
| Истёк общий бюджет или timeout клиента Profile API | 504 | `profile_timeout` |
| Неожиданная ошибка store/приложения | 500 | `internal_error` |

Повторный Like не является ошибкой `already_liked`. Если условия допуска изменились, повтор может получить соответствующий предметный отказ. Отдельного правила предметного права, которое давало бы 403, в текущей модели LinkUp нет.

Причины распознаются через `errors.Is/errors.As`, включая обёрнутые ошибки. При отмене клиентского соединения доставка JSON не гарантируется, но отменяемая работа прекращается.

## Ресурсы и завершение

| Настройка | Default | Назначение |
|---|---|---|
| `LINKUP_ADDR` | `127.0.0.1:8080` | Адрес приложения |
| `PROFILE_BASE_URL` | `http://127.0.0.1:8081` | Base URL Profile API |
| `LINKUP_COMMAND_TIMEOUT` | `300ms` | Единый бюджет команды Like |
| `PROFILE_CLIENT_TIMEOUT` | `1s` | Отдельный предел `http.Client` |
| `PROFILE_DIAL_TIMEOUT` | `500ms` | Timeout установления сетевого соединения |
| `PROFILE_RESPONSE_HEADER_TIMEOUT` | `500ms` | Сетевой timeout ожидания заголовков ответа |
| `PROFILE_MAX_RESPONSE_BYTES` | `16384` | Максимум тела ответа Profile API |
| `LINKUP_READ_HEADER_TIMEOUT` | `5s` | Чтение заголовков запроса |
| `LINKUP_WRITE_TIMEOUT` | `2s` | Запись ответа; больше штатного command budget |
| `LINKUP_IDLE_TIMEOUT` | `60s` | Idle keep-alive соединения |
| `LINKUP_MAX_HEADER_BYTES` | `1048576` | Ограничение заголовков |
| `LINKUP_SHUTDOWN_TIMEOUT` | `5s` | Независимый grace period Shutdown |
| `PROFILE_ADDR` | `127.0.0.1:8081` | Адрес отдельного Profile API |
| `PROFILE_MODE` | `ok` | Начальный режим зависимости |
| `PROFILE_SLOW_DELAY` | `2s` | Задержка slow-ответа |

Все durations, размеры и лимиты должны быть положительными; URL и адреса проверяются при старте. Для Like используется один context с command budget, унаследованный от request context. HTTP client timeout и response-header timeout задаются отдельно и не начинают независимый бюджет внутри application.

Memory store защищён mutex, а Like выполняет read→decide→save под одной write-блокировкой пары; сетевой вызов происходит до блокировки. Данные хранятся только в памяти одного процесса, при перезапуске сбрасываются в fixtures. Несколько экземпляров приложения не делят состояние. Персистентность, распределённая идемпотентность и откат после уже совершённой записи не обещаются.
