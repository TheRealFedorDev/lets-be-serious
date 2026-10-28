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

## Хуки от напарника → N5

Публичная страница маршрута: `assets/share.html?s=<slug>` (автономная, без входа, с QR и печатью в A4; slug также читается из `#slug`). Чтобы ссылка и QR вели на неё, владельцу `app.js` и `share.go` нужно:

**app.js** — в `shareTrip(plan)` строить ссылку из slug, а не брать `response.url`:

```js
const response = await api("/api/trips/share", { method: "POST", body: { plan } });
// slug есть в ответе; если нет — берём последний сегмент url (/share/{slug})
const slug = response.slug || String(response.url || "").split("/").filter(Boolean).pop();
const link = `${location.origin}/assets/share.html?s=${encodeURIComponent(slug)}`;
renderShareOverlay(link);            // ссылка и QR в overlay ведут на новую страницу
window.ODIN?.emit?.("trip:shared", { plan, url: link, slug });
```

**share.go** — по желанию: в `handleSharePage` сделать редирект на красивую страницу, а `shareURL` оставить как есть для обратной совместимости:

```go
http.Redirect(w, r, "/assets/share.html?s="+url.QueryEscape(slug), http.StatusFound)
```

Страница использует только `GET /api/share/{slug}` (`{plan, views}`), 404 показывает «Маршрут не найден».

## Хуки от напарника → N9 (a11y)

Подключение: `<link rel="stylesheet" href="/assets/a11y.css">` и `<script src="/assets/a11y.js" defer></script>` (после остальных модулей). Модуль сам делает: skip-link, focus-trap и возврат фокуса для `#tripPlanner`, `#shareOverlay`, `#bookingModal`, `#authModal`, `#poiModal`, `#commandPalette`, мобильного меню, ассистента и лайтбокса `#galleryZoom`; Esc для мобильного меню, share (поверх планировщика закрывается только share, не оба), планировщика и лайтбокса; Enter/Space на `[role=button|link][tabindex]` (карточки, фото галереи); `aria-pressed` для `.whatif-option`, `.rating-pill`, `.segment`, `.swatch`; `aria-current` для `.nav-link`; перенос фокуса на заголовок при смене экрана и при перерисовке внутри оверлея. Модуль работает и без правок app.js; сниппеты ниже — «правильный» вариант в самом app.js (по желанию), тогда fallback в a11y.js просто не сработает.

**1. Лайтбокс фото (renderHotelDetail)** — сейчас div без role, без Esc, без кнопки закрытия, без возврата фокуса:

```js
const overlay = document.createElement("div");
overlay.id = "galleryZoom";
overlay.setAttribute("role", "dialog");
overlay.setAttribute("aria-modal", "true");
overlay.setAttribute("aria-label", "Просмотр фото");
overlay.innerHTML = `<img src="${img}" alt="${escapeHTML(hotel.name)}" style="...">
  <button type="button" class="a11y-lightbox-close" aria-label="Закрыть фото">×</button>`;
const close = () => { overlay.remove(); shot.focus(); };
overlay.addEventListener("click", close);
overlay.addEventListener("keydown", (e) => { if (e.key === "Escape") { e.stopPropagation(); close(); } });
document.body.appendChild(overlay);
overlay.querySelector("button").focus();
```

**2. Карточки `role="link" tabindex="0"`** (deal/destination/hotel/result) — нет клавиатурной активации. В делегированном обработчике кликов добавить:

```js
document.addEventListener("keydown", (e) => {
  if ((e.key === "Enter" || e.key === " ") && e.target.matches('[role="link"][tabindex]')) { e.preventDefault(); e.target.click(); }
});
```

**3. «Что если» (bindWhatIf)** — кнопки `.whatif-option` переключают только класс `is-active`. В обработчике клика:

```js
box.querySelectorAll("[data-whatif]").forEach((item) => item.setAttribute("aria-pressed", String(item === button && active === variant.key)));
// и в ветке сброса: button.setAttribute("aria-pressed", "false");
// плюс <div class="whatif-result" id="whatIfResult" aria-live="polite">
```

**4. closeOnEscape()** — не знает про мобильное меню и share-overlay; закрывается только то, что перечислено. Если хотите убрать зависимость от a11y.js: добавить `if (mobileMenu.classList.contains("is-open")) return closeMobileMenu();` и `if (!document.getElementById("shareOverlay")?.classList.contains("hidden")) return closeShareOverlay();` перед проверкой `tripPlanner`; и удалить дублирующий обработчик Esc в `bindTripPlannerEntry` (иначе при открытом share Esc закрывает планировщик под ним — a11y.js это перехватывает в capture-фазе).

**5. Фокус после открытия экрана отеля** — в `openHotel`/`showView` после показа вида: `document.querySelector("#hotelView h1")?.setAttribute("tabindex","-1"); …focus({preventScroll:true})` (a11y.js делает то же при потере фокуса).
