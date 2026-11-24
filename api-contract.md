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
| `DATABASE_URL` | `postgres://linkup:linkup@127.0.0.1:5432/linkup?sslmode=disable` | PostgreSQL source of truth |
| `LINKUP_DB_MAX_CONNS` | `10` | Максимум соединений pool |
| `LINKUP_DB_CONNECT_TIMEOUT` | `5s` | Создание и проверка pool |
| `LINKUP_MIGRATION_TIMEOUT` | `30s` | Общий предел миграций и fixtures |
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

В ЛР3 memory store защищён mutex, а Like выполняет read→decide→save под одной write-блокировкой пары; сетевой вызов происходит до блокировки. Эти сведения описывают исходную реализацию ЛР3. В ЛР4 production-состояние выбранного сценария переносится в PostgreSQL, а memory остаётся только в unit-тестах; гарантии persistence и конкуренции приведены в разделе ЛР4 ниже.

## Архитектурные границы ЛР4

HTTP API обслуживается слоем `internal/transport/http`; application-операции находятся в `internal/service` и разбиты по сущностям. Хранилища расположены в `internal/repository/{memory,postgres}`, доменные модели и общие ошибки — в `internal/domain/models` и `internal/domain/errors`. Transport не содержит предметных правил, service не содержит HTTP/SQL, а domain не зависит от остальных слоёв.

Архитектурный перенос не меняет описанные выше маршруты, DTO, статусы и гарантии ЛР3. Production-runtime использует PostgreSQL; memory store сохранён для изолированных unit-тестов. Внешний Profile API остаётся отдельной HTTP-зависимостью.

# Контракт ЛР4: PostgreSQL и конкурирующие операции

Новые гарантии и маршруты вводятся последовательно в PR3–PR6. После интеграции все операции выбранного сценария используют PostgreSQL как единственный источник истины; memory остаётся только в unit-тестах.

## Состояния, actor и права

Actor по-прежнему задаётся заголовком `X-Demo-User`. Like и Withdraw разрешены actor в отношении другого существующего активного профиля при отсутствии блокировки в любую сторону. Block разрешён существующему активному actor в отношении другого существующего активного профиля. Профиль редактирует только его владелец.

| Состояние | Команда | Успех / повтор | Отказ |
|---|---|---|---|
| Нет реакции actor | `PUT /v1/me/likes/{targetID}` | Добавлена реакция; при встречной реакции активирован match | 404 отсутствует профиль; 409 self/inactive/blocked |
| Реакция actor уже есть | Like | Прежний результат, `changed:false`, дубля нет | Права и блокировка проверяются заново |
| Нет блокировки | `PUT /v1/me/blocks/{targetID}` | Создан block, активный match деактивирован | 404 отсутствует профиль; 409 self/inactive |
| Block уже есть | Block | `changed:false`, `match_deactivated:false` | Права проверяются заново |
| Есть реакция actor | `DELETE /v1/me/likes/{targetID}` | Реакция actor удалена, match деактивирован | 404 actor/target; 409 self/inactive/blocked |
| Реакции actor нет | Withdraw | `changed:false`, `match_deactivated:false` | Права и блокировка проверяются заново |
| Профиль имеет version N | `PUT /v1/profiles/{userID}` | Владелец получает version N+1 | 404, 403 или 409 `version_conflict` |

Реакции и историческая строка match после Block сохраняются, но match становится неактивным. Последующий Like запрещён; снятия блокировки нет. Withdraw удаляет только собственную реакцию. В pair response `matched` означает активный match, поэтому после Block возможны `likes:2` и `matched:false`. Повторный Like после Withdraw допустим по обычным правилам.

## Новые DTO

Block — `200 OK`:

```json
{"changed":true,"blocked":true,"match_deactivated":true}
```

Withdraw — `200 OK`:

```json
{"changed":true,"likes":1,"matched":false,"match_deactivated":true}
```

`GET /v1/profiles/{userID}` — `200 OK`:

```json
{"id":"u1","name":"Анна","age":25,"city":"Москва","interests":["music","travel"],"version":1}
```

`PUT /v1/profiles/{userID}` принимает полное представление и ожидаемую версию:

```json
{"name":"Анна","age":26,"city":"Москва","interests":["music","travel"],"version":1}
```

Успех возвращает обновлённый профиль с `version:2`. Редактируются только name, age, city и interests; active через этот API не изменяется. Тело ограничено 64 KiB, требуется `Content-Type: application/json`, неизвестные поля и дополнительные JSON-документы запрещены. После `version_conflict` клиент заново читает профиль и согласовывает правку; автоматически подставлять новую версию в старый payload нельзя.

## Транзакционная граница и ресурсы

PostgreSQL хранит users, canonical pairs, направленные reactions, matches и blocks. Discovery, Like, Inspect, Block и Withdraw читают это же состояние. Результат сохраняется после перезапуска приложения. Memory не является вторым production-источником этих фактов.

Все писатели Like/Block/Withdraw работают под `READ COMMITTED` и соблюдают один протокол: `INSERT pair ... ON CONFLICT` → `SELECT pair FOR UPDATE` → повторное чтение reactions/match/block → проверка actor/прав/перехода → связанные записи через один tx → проверенный Commit. Пара канонизируется `low_id < high_id`; операция захватывает только одну pair row, поэтому встречный порядок ресурсов исключён. Пустой предварительный SELECT и локальный mutex защитой не считаются.

Сеть Profile API выполняется до Begin. Внешние изменяемые факты не входят в транзакционную гарантию PostgreSQL. Поле local `active`, влияющее на допуск, не редактируется Profile API редактирования. Ожидание pair lock и все SQL входят в общий command context. Rollback выполняется на каждом незавершённом выходе; успех возвращается только после Commit.

Block атомарно создаёт запрет и меняет активность match. Withdraw атомарно удаляет реакцию и меняет активность match. Ошибка между SQL-операциями откатывает всю новую команду; прежние реакции другого пользователя сохраняются.

Допустимые исходы после сериализации на pair lock:

- Like первым → встречная реакция и активный match фиксируются, затем Block деактивирует match. Block первым → ожидающий Like получает `blocked`, новой реакции и активного match нет.
- Встречный Like первым → match создаётся, затем Withdraw другой стороны деактивирует его и удаляет только её реакцию. Withdraw первым → ожидающий Like видит отсутствие встречной реакции и завершается как `pending`, активного match нет.
- Withdraw первым → match деактивирован, затем Block создаёт запрет с `match_deactivated:false`. Block первым → ожидающий Withdraw получает `blocked`; обе прежние реакции и неактивный исторический match сохраняются.
- Встречные допустимые Like оба сохраняются, но `matched_now:true` получает только второй по порядку блокировки. Из повторных Like одной направленной реакции `changed:true` получает только первый.

## Ошибки и неизвестный исход

| Причина | HTTP | `code` |
|---|---:|---|
| Некорректный JSON/поле/версия | 400 | `invalid_request` |
| Некорректное JSON-тело | 400 | `invalid_json` |
| Тело профиля больше 64 KiB | 413 | `body_too_large` |
| Неверный Content-Type профиля | 415 | `unsupported_media_type` |
| Профиль не найден | 404 | `profile_not_found` |
| Редактирование чужого профиля | 403 | `profile_forbidden` |
| Старая версия | 409 | `version_conflict` |
| Self-block | 409 | `self_block` |
| Запрещённый переход/отзыв | 409 | `interaction_conflict` |
| Блокировка | 409 | `blocked` |
| Timeout БД | 503 | `database_timeout` |
| Serialization/deadlock/временный отказ tx | 503 | `transaction_retryable` |
| Неизвестный результат Commit | 503 | `commit_unconfirmed` |

Ошибки PostgreSQL различаются по типу, SQLSTATE и имени constraint, а не по тексту. При `transaction_retryable` автоматический retry в базовой версии не выполняется; клиент может повторить всю команду позже в пределах нового бюджета. После `commit_unconfirmed` или потери HTTP-ответа нельзя утверждать, что записи нет: для Like/Block/Withdraw сначала безопасно прочитать pair, затем при необходимости повторить ту же идемпотентную команду; для профиля перечитать version и согласовать правку заново.

## Ограничения схемы

- PK reactions `(actor_id,target_id)` запрещает дубли направленной реакции.
- PK pairs и matches по канонической паре запрещает расходящиеся идентичности и дубли match.
- FK users/pairs/reactions/matches/blocks запрещает ссылки на отсутствующие объекты.
- CHECK запрещает self-like, self-block, неканоническую пару и неположительную version.
- NOT NULL закрывает обязательные факты профиля и состояния; допустимые age и непустые name/city проверяются CHECK.
- Многострочный инвариант «активный match только при двух реакциях и без block» обеспечивается общим pair lock; UNIQUE сам по себе его не доказывает.

## Карта операций и проверок

| Операция | Инвариант | Проверка | Участник |
|---|---|---|---|
| Like/match | Нет потерянных реакций и дублей match | Успех, no-op, rollback после reaction SQL, встречная конкуренция, restart | @acyushka (A) |
| Block | Запрет и активность match согласованы | Rollback, повтор, Like против Block в обоих порядках | @d3e3df (B) |
| Profile edit | Старая форма не затирает новую | Две правки одной version, 403, 404, сохранность первой | @acyushka (C) |
| Withdraw | Нет активного match без двух реакций | Rollback, повтор, конфликт с Like/Block | @d3e3df (D) |
| Все писатели | Общий протокол и ограниченное ожидание | Test-only barrier, наблюдаемый lock wait, timeout и последующий успех | Совместно |
