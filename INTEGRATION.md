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

## Найдено QA — для владельца бэкенда и app.js

Источники: `demo/qa/functional.md`, `demo/qa/visual.md`, `demo/qa/security.md` (2026-10-28). Только зона Федора (`*.go`, `assets/app.js`). Находки модулей напарника (trip.js, share.js, match.js, audit.js, блок N7 в app.css) сюда не входят.

### До демо (видно судьям)

1. Цена за ночь: карточка и план считают завтрак/сборы на `Capacity` (4), а не на гостей (3): план 114 552 vs карточка 110 952 (+3,2%; romance +8%, budget +15%); детали отеля считают по `guests`, номера по `room.Capacity` (Sunset Pier: 24 615 vs 25 515); главная «21 400 ₽ за ночь» без налогов (реально 26 838) → `store.go:837` Search, `:905` HotelBySlug, `:975` rooms(), `:1152` buildPriceBreakdown; `trip.go:373` buildTripBudget → один N = гости запроса, одна функция цены везде, на карточке цена с налогами.
2. Завтрак оплачен дважды: `buildPriceBreakdown` всегда добавляет 900×N в «Проживание», а `trip.go:buildTripDays` кладёт «Завтрак и кофе» 650/700×гостей в «Еду», `buildTripTips:417` пишет «Завтрак не включён» → решить: завтрак включён (убрать блок и подсказку) или убрать 900×N; одна ставка вместо 650/700/900.
3. «К оплате» ≠ бронь: «Итого к оплате» 73 845 (Sunset Pier, 3 ночи, 2 гостя), а `CreateBooking` сохраняет `price*nights` = 58 500 (-21%) → `store.go:1268` (`total := price * nights`), `app.js` bookingTotal → «К оплате» = сумма брони, считать общей функцией из п.1; `audit.js` (напарник) должен показывать то же.
4. Балл плана обрезан до 99: план 99, карточка 100 (Marea); рядом «99 Идеальное совпадение» и «Выше бюджета на 13 842 ₽» (Aqua Family) → `trip.go:193` `clampInt(score, 1, 99)`: убрать clamp (или ограничивать везде, как `store.go:642-645`), согласовать балл с `fits`.
5. Паспорт без фильтров: карточка 76–95, паспорт 94–99 (Sunset Pier 76 vs 97; 14 из 25 сочетаний) → `main.go:496` `BuildMatchBreakdown(SearchParams{Guests, Mood, Tags})` не передаёт `Needs` и `MaxPrice` → передавать, как в `store.go:509` Search.
6. Три расчёта балла одного отеля: Volga Loft — план 99, карточка 97, паспорт 86 → `store.go:Search` смешивает MoodScore, overlap и logistic через max; `trip.go` BuildTrip берёт max(card, logistic) → план берёт `hotel.match` из Search, своей формулы не считает (`toggleWhy` без mood/tags — напарник).
7. Склонение города: «Kvartal 47 в Москва», «Volga Loft в Казань», «Baltic Boutique в Санкт-Петербург» → `trip.go:458` `"%s в %s"` → использовать склонятель из `assistant.go` (~845).
8. Прогулки до 2 081 км: 28 из 168 маршрутов > 20 км, Marea 19 ноября — 83 км / 1 196 мин → `walk.go:66` `planWalkingRoute`: фильтр 3/5 км работает, только если подходящих точек больше `count`, нет верхнего предела; в дождь проходит «Скайпарк» (Красная Поляна, 40 км, `seed.go:783`) → жёсткий потолок (5 км / 150 мин), брать только точки в радиусе.
9. Неверные координаты POI: «Кофейня «Молоко»» 56.14,101.64 (Братск, 480 км от Иркутска) → `poi_coords.go:59`; «Байкал рядом» 55.92,88.20 → `:27`; «Площадь Кирова» 57.15,103.93 → `:122`; Алтай: POI в Горно-Алтайске при отелях в 120 км → выверить по городу, проверить все POI в радиусе ≤ 15 км.
10. «Что рядом»: «Тисо-самшитовая роща — 90 мин пешком», «Олимпийский парк — 70 мин», «~11.2 км · 150 мин» → `app.js` openNearbyMap (~3540): не выводить минуты пешком при большой дистанции; категория «park» выводится по-английски → перевести на «парк».
11. Молчаливая подмена отеля: `hotelSlug=marea-resort, budget=60000` → план для `aqua-family` (то же для sunset-pier, cedar-sea; неизвестный slug — тоже первый отель) → `trip.go` BuildTrip: поиск по `MaxPrice = budget/nights*0.55` берёт первый попавшийся; вернуть запрошенный отель с `fits=false` или явную ошибку, неизвестный slug → 400.
12. `Math.random` в what-if: при `delta=0` показывается случайное «+N баллов» → `app.js:1744` `renderWhatIf` → заменить на `variant.delta` (0 → «балл не меняется»).
13. «+20% к бюджету» врёт: подпись «Лимит станет 26 750» = +25% → `whatif.go:87` `basePrice*0.25` при `budget=0` → считать +20% или не показывать вариант без бюджета.
14. «+1 500 ₽ за ночь на всех гостей» → реально +1 755 (3 гостя; 1 000 vs 1 170 при 2) → `whatif.go:98` `breakfastPrice = basePrice + 500*guests` плюс `nightTotal` (налоги), а базовая цена уже включает завтрак → дельта через `nightTotal`, подпись из результата.
15. Противоречие отмены: сайдбар «Бесплатная отмена до заезда» vs what-if «Гибкая отмена +1 712 ₽/ночь» → `app.js` renderHotel (сайдбар) и `whatif.go` → убрать одно из двух.
16. Ссылка «Открыть в Яндекс.Картах» (`app.js:3576`, в отчёте «двойной escapeHTML»): при проверке `mapLink` (строка 3552) — это URL, экранируется один раз в атрибуте, дефект в коде не найден → проверить клик в браузере; если ссылка рабочая, пункт снять.
17. Небезопасные src/href: `app.js:1700` `<img src="${img}">` без `escapeHTML`; `app.js:3464` `walk.mapUrl` без проверки схемы → `escapeHTML(img)` и `^https?://`, как в `share.js`.
18. Английские подписи: «N% match» → `app.js:943` (→ «совпадение»); «Спросить AI об отеле» → `app.js:1644` (→ «ИИ»); «21400 ₽» → `match.go` Note фактора price (→ «21 400 ₽»); «King + sofa bed» → `seed.go:896` (→ «King + диван»).
19. «Совпадение по запросу — 0 из 0 тегов» в блоке «Против», когда запроса не было → `match.go:71` `hotelMatchFeatures`: добавлять фактор только при `len(inferredTags) > 0`.
20. Цены на главной без «за ночь» («7 200 ₽ 9 200 ₽», «от 7 400 ₽») → `app.js` рендер dealsGrid и destinationsTrack: добавить «за ночь».
21. Ссылка шаринга: `shareURL` (`share.go:146`) даёт `/share/{slug}`, отдельная страница `/assets/share.html` даёт вторую версию плана (E-13) → сошлись на сниппет из раздела «Хуки от напарника → N5» выше: `app.js` shareTrip строит `/assets/share.html?s=<slug>`; `share.go` `handleSharePage` → `http.Redirect` на эту страницу.
22. CTA `/?hotel=<slug>` на странице шаринга ведёт на главную → `app.js` не читает `location.search` → в init читать `hotel` и вызывать `openHotel(slug)` (или напарник убирает CTA в `share.js`).
23. Анонимный шаринг: `POST /api/trips/share` без входа принимает ~900 КБ (2 МБ → 400), сервер хранит присланный план как есть (`budget.total=1`, любой `hotel.name`) → `share.go:111-131`, `:57-90` CreateShare: требовать вход или IP-лимит (10/час), лимит ~64 КБ и длин строк, план пересобирать на сервере по `hotelSlug`.
24. Нет Content-Security-Policy → `main.go:934` securityHeaders: добавить CSP (`default-src 'self'; img-src 'self' data: https:; frame-ancestors 'none'; base-uri 'none'`); инлайн-скрипт `/share/` (`share.go:~346`) вынести в `/assets/`, иначе временно `'unsafe-inline'` для script.

### Желательно

1. Избранное на несуществующем отеле: `POST /api/favorites/999999` → 500 «Не удалось обновить избранное» (нарушение FK) → `store.go:1321` ToggleFavorite: проверять отель до записи, 404 «Отель не найден».
2. Бронь при guests > вместимости → 409 «Номер уже занят» вместо 400 (Studio cap 2, guests 3) → `store.go:1253` `ErrUnavailable` смешан с вместимостью → отдельная проверка, 400 «слишком много гостей».
3. Даты не валидируются: `from=garbage` → 200 с `checkIn="garbage"`; 2020-01-01 → 200; `to<=from` → молча `from+3`; выезд раньше заезда в поиске → `trip.go` BuildTrip, `store.go:509` Search, `:843` HotelBySlug → `time.Parse("2006-01-02")`, 400 при прошлом, мусоре и to≤from.
4. Гости и бюджет: `guests=5/100` → 404 «Не нашли отели» (вместимость 4); `guests=0/-3` молча = 2; `search?guests=abc` → 0 без ошибки; `budget=-1000` игнорируется, `budget=10^15` → delta -999 999 999 891 355 → guests 1–4 с 400, budget ≥ 0 с верхней границей (`main.go:395` handleTripPlan и парсинг search).
5. Переполнение what-if: `nights=1000000000, guests=100` → total 112 815 000 000 000; `nights=2^62` → отрицательный total (int64); отрицательные значения молча заменяются → `whatif.go`: nights 1–30, guests 1–4, budget ≥ 0, иначе 400.
6. Ассистент, неизвестный город («Атлантида») → подборка из разных городов под одним названием (Suzdal Estate, The Senses Moscow, Altai Ridge) → `assistant.go`: вежливое «не нашёл, уточните» без подборки.
7. Ассистент игнорирует бюджет и цену: «недорого в Казани» → Kazan House 10 400 ₽ (Volga Loft 8 600 дешевле), бюджет 100 000 и «бассейн» не учитываются → `assistant.go`: сортировка по цене и фильтры из запроса.
8. Ассистент, склонение: «Вот отели в Суздаль», «В Мурманск обратите внимание» → `assistant.go` (~845), тот же склонятель, что в п. 7 «До демо».
9. Нет rate limit: 12 неверных логинов подряд — без блокировки; регистрация, `POST /api/trips/share`, `POST /api/assistant/chat` без лимита → лимитер в памяти по IP+email (5 попыток / 15 мин) на `main.go:657`, `:692` (register), `:710` (login), `:266` (assistant).
10. Host-header в ссылке: `curl -H "Host: evil.example"` → ссылка и QR `https://evil.example/share/...` → `share.go:146-159` shareURL строит базу из `r.Host` и `X-Forwarded-Proto` → базовый URL из `ODIN_PUBLIC_URL`, `X-Forwarded-Proto` только за доверенным прокси.
11. Прогулки повторяются: дни 1/3/5 идентичны, дни 2/4 идентичны → `walk.go:66` ротация `seed % len(candidates)` только при > 3 кандидатах → ротация всегда; в день выезда не ставить полную прогулку 150 мин при блоке «50 мин» (`trip.go`).
12. «Нет данных» подменяется выдумкой: `park`, `services`, `attractions` без данных получают `Known=true` и минуты `3+ID%12`, `2+ID%9`, `12+ID%15` → `needs.go:260` nearbyMinutes: `known=false` (правило INTEGRATION п.2).
13. «В день» занижено: 36 830 вместо 46 038 (делитель `nights+1`) → `trip.go:395` buildTripBudget: делить на `nights`.
14. Один ввод — разный отель: сценарий передаёт `tags`, форма не шлёт → `app.js` submitTripPlan: добавить `tags` и `hotelSlug` в payload.
15. Одинаковые фото: все номера Sunset Pier — одно фото; The Senses Moscow и Baltic Boutique — одно; Казань — фото Чинкве-Терре → `seed.go` (поле images).
16. Описание не совпадает с названием: «Культурная точка — Кофе и море — Смотровая площадка…»; в дождь «Прогулка под крышей: 8,8 км · 230 мин» → `trip.go` подбор активностей (`buildTripDays`): брать описание по категории точки.
17. Одинаковая плашка «Комфортная погода для прогулок — Прогулки, Веранда, Велопрокат…» на каждой карточке → `app.js` ~1028 (`.weather-match`): выводить только при реальном совпадении с погодой.

### После хакатона

1. Cookie без `Secure` (по умолчанию) → `main.go:201`: `ODIN_SECURE_COOKIE=true` на публичном деплое, плюс HSTS.
2. Сессии: TTL 7 дней без ротации, смена пароля не гасит сессии → `main.go:34`, `:773-787`: гасить сессии при смене пароля, sliding TTL.
3. bcrypt `DefaultCost`, пароль `12345678` принят (201) → `main.go:692`, `seed.go:~925`: cost 12, правила пароля.
4. Перечисление по времени: неверный пароль 46 мс, несуществующий email 0,4 мс → `main.go:722-740`: сравнивать с фиктивным bcrypt-хешем при `ErrNotFound`.
5. Мутации без Origin проходят (`POST /api/favorites/2` с кукой → 200) → `main.go:908-920` sameOrigin: отвергать без Origin и без `Sec-Fetch-Site`.
6. Листинг `/assets/` и `/assets/images/` отдаёт 200 со списком файлов → `main.go:279` FileServer: 404 для путей, оканчивающихся на `/`.
7. Share: `<img src>` принимает любой хост (трекинг-пиксель) → `share.go:~330`: только пути `/assets/images/…`.
8. Share: `width:{{.Percent}}%` без ограничения (`99999999%`, `-4%`) → `share.go:~301` shareBudgetRows: percent 0–100.
9. Кэш без верхней границы: `tripCache` растёт на уникальных запросах → `trip.go:490-498`: лимит (~256) с вытеснением старейшей записи.
10. Беспарольный демо-вход (`POST /api/auth/demo`): общий стейт и PII при публичном деплое → `main.go:755`: отключить вне демо-окружения.
11. Отмена чужой, несуществующей или повторной брони → 409 везде (E-14) → `store.go:1301` CancelBooking: 404/403 по смыслу.
12. POST без `application/json` → 400 вместо 415 (E-15) → decodeJSON в `main.go`.
13. Неверный метод на `/api/*` → 404 «API-метод не найден» вместо 405 (E-26) → catch-all в `main.go`: 405 с `Allow`.
14. Имя пользователя `<script>…` принимается и хранится (E-17) → ограничить символы при регистрации (`main.go`, register).
15. Цена не влияет на балл: +1 500 ₽/ночь даёт delta 0 (N-12) → `match.go` фактор price (`1 - PriceFrom/40000`, вес 1.0 из ~10): видимая шкала или вес.
