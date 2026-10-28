# Интеграция с второй частью (ветка `fedor`)

Вся наша логика уже живёт в одном приложении. Чтобы вторая часть команды
подключалась без конфликтов, мы держим стабильный мост `window.ODIN` и
документируем зоны ответственности.

## Зоны файлов

| Зона | Файлы | Владелец |
| --- | --- | --- |
| Бэкенд и логика | `needs.go`, `trip.go`, `match.go`, `whatif.go`, `demo.go`, `share.go`, `actions.go`, `assistant.go`, `main.go`, `store.go`, `models.go` | ассистент |
| `assets/app.js` | вся логика UI и мост `window.ODIN` | ассистент |
| `assets/app.css`, `index.html` | дизайн-система и разметка | напарник |
| Новые UI-модули | отдельные `assets/*.js`, подключаются в `index.html` после `app.js` | напарник |

Если нужно поменять `index.html` или `assets/app.css` — меняем точечно, не
переписываем файл целиком, чтобы не потерять чужие правки.

## Мост `window.ODIN`

Доступен после `init()` на любом экране.

```js
window.ODIN = {
  state,            // общее состояние (search, user, activeHotel, ...)
  api,              // fetch-обёртка: сам сериализует body в JSON
  showToast,        // toast(message, "success" | "info" | "error")
  openHotel,        // openHotel(slug)
  openAssistant,    // openAssistant(hotelSlug?)
  openTripPlanner,  // openTripPlanner({ hotelSlug? })
  closeTripPlanner,
  shareTrip,        // shareTrip(plan) — ссылка + QR
  closeShareOverlay,
  openSearchView,   // openSearchView({ destination, tags, mood, needs, ... })
  performSearch,
  destinationName,
  money, plural, formatDateRange, formatShortDate, escapeHTML,
  needLabel, activeNeeds,
  tripPlan: () => TRIP_UI.lastPlan,
  featureFlags: () => state.featureFlags || {},
  on: (event, handler) => document.addEventListener(`odin:${event}`, handler),
  emit: (event, detail) => document.dispatchEvent(new CustomEvent(`odin:${event}`, { detail })),
};
```

## События

Мост поддерживает `odin:*` события через `ODIN.on/emit`. Пример:

```js
ODIN.on("trip:ready", ({ detail }) => console.log(detail.plan));
```

## API, на которое можно опираться

- `GET /api/search` — отели + `needsEval` (причины отсева).
- `GET /api/hotels/{slug}` — карточка отеля + `matchDetail` (паспорт совпадения).
- `POST /api/match/what-if` — варианты «что если» с пересчётом балла.
- `POST /api/trip/plan` — маршрут по дням, бюджет, погода.
- `GET /api/demo/scenarios`, `GET /api/demo/scenario/{id}` — демо-сценарии.
- `POST /api/trips/share`, `GET /api/share/{slug}`, `GET /share/{slug}` — шаринг и QR.
- `POST /api/assistant/chat` — ответ + `actions` (кнопки следующего шага).

## Правила, которые нельзя ломать

1. Вклады факторов в `matchDetail.factors[].points` **суммируются в `score`**.
2. Потребность со значением `0` минут означает «нет данных», а не отказ.
3. Уровни потребностей: `1` — желательно (влияет на балл), `2` — обязательно (жёсткий фильтр).
4. `api(path, { body })` в `app.js` сам делает `JSON.stringify` — не передавайте строку.
5. Обработчики кликов передавайте обёрткой `() => fn()`, а не ссылкой на функцию.

## Проверка перед демо

```powershell
$env:ODIN_DB_PATH="$env:TEMP\odin-test.db"
$env:PORT="8081"
$env:ODIN_OPEN_BROWSER="false"
.\odin-new.exe
```

Демо-логин: `demo@odin.travel` / `odin-demo-2026`.
