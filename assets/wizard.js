/* Мастер поездки: параметры → предложение → одна оплата.
   Шаги 1–3 собирают предпочтения, шаг 4 — план (рисует trip.js), шаг 5 — корзина и оплата.
   Работает через мост window.ODIN; подключается после app.js. */
(function () {
  "use strict";
  var O = window.ODIN;
  if (!O) return;

  var WEATHER = [
    { id: "", icon: "🤷", label: "Не важно" },
    { id: "warm", icon: "☀️", label: "Тепло" },
    { id: "cool", icon: "🍃", label: "Без жары" },
    { id: "dry", icon: "🌂", label: "Без дождя" },
    { id: "snow", icon: "❄️", label: "Снег и зима" }
  ];
  var INTERESTS = [
    ["museums", "🏛", "Музеи и история"], ["nature", "🌲", "Природа"], ["gastro", "🍽", "Гастрономия"],
    ["night", "🌙", "Вечерняя жизнь"], ["kids", "🧸", "С детьми"], ["active", "🚴", "Активный отдых"],
    ["shopping", "🛍", "Шопинг"], ["views", "🌅", "Виды и панорамы"]
  ];
  var BEDS = [["", "Не важно"], ["double", "Двуспальная"], ["twin", "Две отдельные"]];
  var VIEWS = [["", "Не важно"], ["sea", "Море"], ["city", "Город"], ["mountain", "Горы"], ["nature", "Природа"]];
  var SIZES = [[0, "Не важно"], [30, "от 30 м²"], [40, "от 40 м²"], [55, "от 55 м²"]];
  var WALKS = [[0, "Не важно"], [10, "до 10 мин"], [15, "до 15 мин"], [20, "до 20 мин"], [30, "до 30 мин"]];
  var STEPS = ["Куда и когда", "Что важно", "Номер", "Программа", "Оплата"];

  var W = null; // состояние мастера

  function esc(v) { return O.escapeHTML(v); }
  function money(v) { return O.money(Number(v) || 0); }
  function body() { return document.getElementById("tripBody"); }
  function title(text) { var t = document.getElementById("tripTitle"); if (t) t.textContent = text; }

  function freshState(prefill) {
    var st = O.state.search || {};
    var today = O.todayISO();
    var from = st.checkIn && st.checkIn >= today ? st.checkIn : O.addDaysISO(today, 14);
    return {
      step: 1,
      req: {
        destination: prefill.destination || st.destination || ((O.state.destinations || [])[0] || {}).slug || "sochi",
        from: from,
        to: st.checkOut && st.checkOut > from ? st.checkOut : O.addDaysISO(from, 3),
        guests: Number(st.guests) || 2,
        budget: 80000,
        mood: st.mood || "",
        pace: "calm",
        hotelSlug: prefill.hotelSlug || "",
        weather: "",
        interests: [],
        maxWalkMin: 0,
        room: { bed: "", view: "", minSize: 0, kitchen: false, kidsCot: false }
      },
      needs: {}, // key -> 1 (желательно) | 2 (обязательно)
      plan: null,
      cart: {},
      busy: false,
      mode: "",
      exclude: [],
      envelope: false,
      surprise: null
    };
  }

  function needsParam() {
    return Object.keys(W.needs).map(function (k) { return k + "::" + W.needs[k]; }).join(",");
  }

  function tripRequest() {
    var r = W.req;
    return {
      destination: r.destination, from: r.from, to: r.to, guests: r.guests, budget: r.budget,
      mood: r.mood, pace: r.pace, hotelSlug: r.hotelSlug, weather: r.weather, interests: r.interests.slice(),
      needs: needsParam(), room: r.room, maxWalkMin: r.maxWalkMin
    };
  }

  /* ---------- разметка ---------- */

  function stepper() {
    return '<ol class="wz-steps" aria-label="Шаги">' + STEPS.map(function (s, i) {
      var n = i + 1, cls = n === W.step ? " is-active" : (n < W.step ? " is-done" : "");
      return '<li class="wz-step' + cls + '"' + (n === W.step ? ' aria-current="step"' : "") + "><span>" + n + "</span>" + esc(s) + "</li>";
    }).join("") + "</ol>";
  }

  function pills(name, options, current) {
    return options.map(function (o) {
      var on = String(o[0]) === String(current);
      return '<button type="button" class="rating-pill' + (on ? " is-active" : "") + '" aria-pressed="' + on + '" data-wz-set="' + name + '" data-wz-val="' + esc(o[0]) + '">' + esc(o[1]) + "</button>";
    }).join("");
  }

  function footer(back, nextLabel) {
    return '<div class="wz-footer">' +
      (back ? '<button type="button" class="ghost-button" data-wz-go="' + (W.step - 1) + '">Назад</button>' : "<span></span>") +
      '<button type="button" class="primary-button" data-wz-next>' + esc(nextLabel) + "</button></div>";
  }

  function step1() {
    var r = W.req, dests = O.state.destinations || [];
    return '<div class="wz-scenarios"><span>Быстрый старт:</span>' +
      '<button type="button" class="rating-pill" data-wz-demo="family">🧸 Семья</button>' +
      '<button type="button" class="rating-pill" data-wz-demo="romance">🥂 Романтика</button>' +
      '<button type="button" class="rating-pill" data-wz-demo="budget">🎒 Разумно</button>' +
      '<button type="button" class="rating-pill wz-ai" data-wz-ai>✨ Рассказать ИИ своими словами</button>' +
      '<button type="button" class="rating-pill wz-lucky" data-wz-lucky>🎲 Мне повезёт</button></div>' +
      '<div class="trip-form-grid">' +
      '<label class="trip-field"><span>Куда</span><select data-wz-field="destination">' +
      dests.map(function (d) { return '<option value="' + esc(d.slug) + '"' + (d.slug === r.destination ? " selected" : "") + ">" + esc(d.city) + "</option>"; }).join("") +
      "</select></label>" +
      '<label class="trip-field"><span>Заезд</span><input type="date" data-wz-field="from" value="' + esc(r.from) + '" min="' + O.todayISO() + '"></label>' +
      '<label class="trip-field"><span>Выезд</span><input type="date" data-wz-field="to" value="' + esc(r.to) + '" min="' + O.addDaysISO(O.todayISO(), 1) + '"></label>' +
      '<label class="trip-field"><span>Гости</span><select data-wz-field="guests">' +
      [1, 2, 3, 4, 5, 6].map(function (n) { return '<option value="' + n + '"' + (n === r.guests ? " selected" : "") + ">" + n + " " + O.plural(n, "гость", "гостя", "гостей") + "</option>"; }).join("") +
      "</select></label>" +
      '<label class="trip-field"><span>Бюджет на всё, ₽</span><input type="number" min="0" step="5000" inputmode="numeric" data-wz-field="budget" value="' + esc(r.budget) + '"></label>' +
      '<label class="trip-field"><span>Настроение</span><select data-wz-field="mood"><option value="">Любое</option>' +
      (O.state.moods || []).map(function (m) { return '<option value="' + esc(m.slug) + '"' + (m.slug === r.mood ? " selected" : "") + ">" + esc(m.title) + "</option>"; }).join("") +
      "</select></label></div>" +
      '<p class="wz-error hidden" id="wzError" role="alert"></p>' + footer(false, "Дальше: что важно");
  }

  function needChip(n) {
    var level = W.needs[n.key] || 0;
    var label = level === 2 ? "обязательно" : (level === 1 ? "желательно" : "");
    var mins = n.kind === "nearby" && n.defaultLimit ? " · до " + n.defaultLimit + " мин" : "";
    return '<button type="button" class="wz-need lvl-' + level + '" data-wz-need="' + esc(n.key) + '" aria-pressed="' + (level > 0) + '">' +
      esc(n.label) + esc(mins) + (label ? "<small>" + label + "</small>" : "") + "</button>";
  }

  function step2() {
    var r = W.req, cat = O.state.needCatalog || [];
    var nearby = cat.filter(function (n) { return n.kind === "nearby"; });
    var inHotel = cat.filter(function (n) { return n.kind !== "nearby"; });
    return '<section class="wz-group"><h3>Какая погода нужна</h3><div class="wz-tiles">' + WEATHER.map(function (w) {
      var on = w.id === r.weather;
      return '<button type="button" class="wz-tile' + (on ? " is-active" : "") + '" aria-pressed="' + on + '" data-wz-set="weather" data-wz-val="' + w.id + '"><span aria-hidden="true">' + w.icon + "</span>" + esc(w.label) + "</button>";
    }).join("") + '</div><p class="wz-hint">Под погоду подстроим программу: в дождь — музеи и крытые места, в тепло — улица. Если даты неудачные, подскажем лучшие.</p></section>' +
      '<section class="wz-group"><h3>Что хочется увидеть и делать</h3><div class="wz-chips">' + INTERESTS.map(function (i) {
        var on = r.interests.indexOf(i[0]) >= 0;
        return '<button type="button" class="rating-pill' + (on ? " is-active" : "") + '" aria-pressed="' + on + '" data-wz-interest="' + i[0] + '">' + i[1] + " " + esc(i[2]) + "</button>";
      }).join("") + "</div></section>" +
      '<section class="wz-group"><h3>Сколько готовы идти пешком до мест</h3><div class="wz-chips">' + pills("maxWalkMin", WALKS, r.maxWalkMin) + "</div></section>" +
      (nearby.length ? '<section class="wz-group"><h3>Что должно быть рядом с отелем</h3><p class="wz-hint">Нажмите один раз — «желательно», второй — «обязательно», третий — снять.</p><div class="wz-chips">' + nearby.map(needChip).join("") + "</div></section>" : "") +
      (inHotel.length ? '<section class="wz-group"><h3>Что важно в самом отеле</h3><div class="wz-chips">' + inHotel.map(needChip).join("") + "</div></section>" : "") +
      '<section class="wz-group"><h3>Темп</h3><div class="wz-chips">' + pills("pace", [["calm", "Спокойный"], ["active", "Насыщенный"]], r.pace) + "</div></section>" +
      footer(true, "Дальше: номер");
  }

  function step3() {
    var room = W.req.room;
    var toggle = function (key, label) {
      return '<button type="button" class="rating-pill' + (room[key] ? " is-active" : "") + '" aria-pressed="' + !!room[key] + '" data-wz-room-toggle="' + key + '">' + esc(label) + "</button>";
    };
    return '<section class="wz-group"><h3>Кровать</h3><div class="wz-chips">' + pills("room.bed", BEDS, room.bed) + "</div></section>" +
      '<section class="wz-group"><h3>Вид из окна</h3><div class="wz-chips">' + pills("room.view", VIEWS, room.view) + "</div></section>" +
      '<section class="wz-group"><h3>Площадь</h3><div class="wz-chips">' + pills("room.minSize", SIZES, room.minSize) + "</div></section>" +
      '<section class="wz-group"><h3>Ещё</h3><div class="wz-chips">' + toggle("kitchen", "Кухня") + toggle("kidsCot", "Детская кроватка") + "</div>" +
      '<p class="wz-hint">Подберём номер, где всем хватит места, и честно покажем, какие пожелания выполнить не удалось.</p></section>' +
      '<p class="wz-error hidden" id="wzError" role="alert"></p>' + footer(true, "Собрать поездку");
  }

  function paint() {
    var host = body();
    if (!host) return;
    if (W.mode === "surprise") {
      title("Поездка-сюрприз");
      host.innerHTML = '<div class="wz-root"><div class="wz-body">' + surpriseMarkup() + "</div></div>";
      return;
    }
    title("Соберём поездку под вас");
    var content = W.step === 1 ? step1() : W.step === 2 ? step2() : step3();
    host.innerHTML = '<div class="wz-root">' + stepper() + '<div class="wz-body">' + content + "</div></div>";
  }

  function showError(text) {
    var box = document.getElementById("wzError");
    if (box) { box.textContent = text; box.classList.remove("hidden"); }
    else O.showToast(text, "error");
  }

  /* ---------- поездка-сюрприз ---------- */

  function surpriseMarkup() {
    var r = W.req, dests = O.state.destinations || [];
    return '<section class="wz-group wz-lucky-head"><h3>🎲 Поездка-сюрприз</h3><p class="wz-hint">Задайте бюджет и что важно — город и отель выпадут случайно, но только из тех, что подходят под все условия.</p></section>' +
      '<div class="trip-form-grid">' +
      '<label class="trip-field"><span>Заезд</span><input type="date" data-wz-field="from" value="' + esc(r.from) + '" min="' + O.todayISO() + '"></label>' +
      '<label class="trip-field"><span>Выезд</span><input type="date" data-wz-field="to" value="' + esc(r.to) + '" min="' + O.addDaysISO(O.todayISO(), 1) + '"></label>' +
      '<label class="trip-field"><span>Гости</span><select data-wz-field="guests">' +
      [1, 2, 3, 4, 5, 6].map(function (n) { return '<option value="' + n + '"' + (n === r.guests ? " selected" : "") + ">" + n + " " + O.plural(n, "гость", "гостя", "гостей") + "</option>"; }).join("") +
      "</select></label>" +
      '<label class="trip-field"><span>Бюджет на всё, ₽</span><input type="number" min="0" step="5000" inputmode="numeric" data-wz-field="budget" value="' + esc(r.budget) + '"></label></div>' +
      '<section class="wz-group"><h3>Погода</h3><div class="wz-chips">' + WEATHER.map(function (w) {
        var on = w.id === r.weather;
        return '<button type="button" class="rating-pill' + (on ? " is-active" : "") + '" aria-pressed="' + on + '" data-wz-set="weather" data-wz-val="' + w.id + '">' + w.icon + " " + esc(w.label) + "</button>";
      }).join("") + "</div></section>" +
      '<section class="wz-group"><h3>Что хочется</h3><div class="wz-chips">' + INTERESTS.map(function (i) {
        var on = r.interests.indexOf(i[0]) >= 0;
        return '<button type="button" class="rating-pill' + (on ? " is-active" : "") + '" aria-pressed="' + on + '" data-wz-interest="' + i[0] + '">' + i[1] + " " + esc(i[2]) + "</button>";
      }).join("") + "</div></section>" +
      (dests.length ? '<section class="wz-group"><h3>Куда точно не хочу</h3><div class="wz-chips">' + dests.map(function (d) {
        var on = W.exclude.indexOf(d.slug) >= 0;
        return '<button type="button" class="rating-pill wz-exclude' + (on ? " is-active" : "") + '" aria-pressed="' + on + '" data-wz-exclude="' + esc(d.slug) + '">' + (on ? "✕ " : "") + esc(d.city) + "</button>";
      }).join("") + "</div></section>" : "") +
      '<label class="wz-envelope"><input type="checkbox" data-wz-envelope' + (W.envelope ? " checked" : "") + '> <span><strong>Закрытый конверт</strong> — город и отель узнаете только после оплаты</span></label>' +
      '<div class="wz-reel hidden" id="wzReel" aria-live="polite"></div>' +
      '<div id="wzReveal"></div>' +
      '<p class="wz-error hidden" id="wzError" role="alert"></p>' +
      '<div class="wz-footer"><button type="button" class="ghost-button" data-wz-exit-lucky>Обычный подбор</button>' +
      '<button type="button" class="primary-button" data-wz-spin>🎲 Крутить</button></div>';
  }

  function readSurpriseFields() {
    body().querySelectorAll("[data-wz-field]").forEach(function (el) {
      var k = el.getAttribute("data-wz-field");
      W.req[k] = k === "guests" || k === "budget" ? Math.max(0, Number(el.value) || 0) : el.value;
    });
    var env = body().querySelector("[data-wz-envelope]");
    if (env) W.envelope = env.checked;
  }

  function spin(extraExclude) {
    if (W.busy) return;
    readSurpriseFields();
    if (!W.req.from || !W.req.to || W.req.to <= W.req.from) return showError("Выезд должен быть позже заезда.");
    W.busy = true;
    var prev = W.surprise && W.surprise.plan.hotel.slug;
    var req = tripRequest();
    req.destination = "";
    req.hotelSlug = "";
    req.exclude = W.exclude.concat(extraExclude || []);
    var reel = document.getElementById("wzReel");
    var reveal = document.getElementById("wzReveal");
    var err = document.getElementById("wzError");
    if (err) err.classList.add("hidden");
    if (reveal) reveal.innerHTML = "";
    var attempt = function (left) {
      req.seed = 0; // сервер выбирает seed сам и возвращает его — результат воспроизводим
      return O.api("/api/trip/surprise", { method: "POST", body: req }).then(function (res) {
        if (res.plan.hotel.slug === prev && left > 0 && res.candidates > 1) return attempt(left - 1);
        return res;
      });
    };
    attempt(3).then(function (res) {
      return animateReel(reel, res.cities, W.envelope ? "🔒 Секретный город" : res.plan.city).then(function () { return res; });
    }).then(function (res) {
      W.surprise = res;
      W.req.destination = res.plan.destination;
      W.req.hotelSlug = res.plan.hotel.slug;
      if (reveal) reveal.innerHTML = revealMarkup(res);
    }).catch(function (e) {
      if (reel) reel.classList.add("hidden");
      showError((e && e.message) || "Не удалось собрать сюрприз");
    }).then(function () { if (W) W.busy = false; });
  }

  function animateReel(reel, cities, finalText) {
    return new Promise(function (resolve) {
      if (!reel) return resolve();
      reel.classList.remove("hidden");
      var list = (cities && cities.length ? cities : ["?"]).slice();
      var reduce = window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
      if (reduce || list.length < 2) { reel.textContent = finalText; return resolve(); }
      var i = 0, delay = 60, elapsed = 0;
      (function tick() {
        reel.textContent = list[i % list.length];
        i += 1;
        elapsed += delay;
        if (elapsed > 1500) { reel.textContent = finalText; reel.classList.add("is-landed"); return resolve(); }
        delay = Math.min(220, delay * 1.12);
        setTimeout(tick, delay);
      })();
    });
  }

  function revealMarkup(res) {
    var p = res.plan, h = p.hotel || {}, env = W.envelope;
    var hidden = function (open, closed) { return env ? closed : open; };
    return '<article class="wz-reveal">' +
      (env ? '<div class="wz-envelope-art" aria-hidden="true">✉️</div>' : '<img src="' + esc(h.image || p.image || "") + '" alt="" loading="lazy">') +
      '<div class="wz-reveal-copy"><span class="tx-kicker">' + esc(hidden(p.city, "Город — сюрприз")) + " · " + esc(O.formatDateRange(p.checkIn, p.checkOut)) + "</span>" +
      "<strong>" + esc(hidden(h.name, (h.meta && h.meta.stars ? h.meta.stars + "★ отель" : "Отель") + ", рейтинг " + Number(h.rating || 0).toFixed(1))) + "</strong>" +
      '<ul class="wz-reveal-why">' + (res.why || []).map(function (w) { return "<li>✓ " + esc(w) + "</li>"; }).join("") + "</ul>" +
      '<span class="wz-reveal-total">Вся поездка ≈ ' + money(p.budget && p.budget.total) + " · выбрано из " + res.candidates + " " + O.plural(res.candidates, "варианта", "вариантов", "вариантов") + "</span></div>" +
      '<div class="wz-reveal-actions"><button type="button" class="primary-button" data-wz-take>Беру!</button>' +
      '<button type="button" class="ghost-button" data-wz-spin>Крутить ещё</button>' +
      (env ? "" : '<button type="button" class="ghost-button" data-wz-not-city>Не этот город</button>') + "</div></article>";
  }

  function takeSurprise() {
    var res = W.surprise;
    if (!res) return;
    W.mode = "";
    if (W.envelope) {
      // конверт: сразу к оплате, программа и город откроются после неё
      W.plan = res.plan;
      W.cart = {};
      (res.plan.days || []).forEach(function (d) { (d.blocks || []).forEach(function (b) { if (b.bookable) W.cart[b.itemId] = true; }); });
      return openCheckout();
    }
    showPlan(res.plan);
  }

  /* ---------- план (шаг 4) ---------- */

  function buildPlan() {
    if (W.busy) return;
    W.busy = true;
    var btn = body().querySelector("[data-wz-next]");
    if (btn) { btn.disabled = true; btn.textContent = "Собираем поездку…"; }
    O.api("/api/trip/plan", { method: "POST", body: tripRequest() }).then(function (plan) {
      showPlan(plan);
    }).catch(function (err) {
      if (btn) { btn.disabled = false; btn.textContent = "Собрать поездку"; }
      var msg = err && err.message ? err.message : "Не удалось собрать поездку";
      if (err && err.status === 404) msg += ". Ослабьте одно из условий: «обязательно» → «желательно» или другой город.";
      showError(msg);
    }).then(function () { if (W) W.busy = false; });
  }

  function showPlan(plan) {
    W.plan = plan;
    W.step = 4;
    W.cart = {};
    (plan.days || []).forEach(function (d) {
      (d.blocks || []).forEach(function (b) { if (b.bookable) W.cart[b.itemId] = true; });
    });
    var host = body();
    if (host) host.innerHTML = "";
    O.emit("trip:ready", { plan: plan, wizard: true });
  }

  /* ---------- корзина и оплата (шаг 5) ---------- */

  function cartItems() {
    var out = [];
    (W.plan.days || []).forEach(function (d) {
      (d.blocks || []).forEach(function (b) {
        if (b.bookable && W.cart[b.itemId]) out.push({ id: b.itemId, date: d.date, time: b.time, title: b.title, place: b.place, price: Number(b.price) || 0 });
      });
    });
    return out;
  }

  function checkoutMarkup() {
    var p = W.plan, room = p.room || {}, pb = room.priceBreakdown || {}, n = p.nights;
    var items = cartItems();
    var itemsSum = items.reduce(function (a, i) { return a + i.price; }, 0);
    var stay = (Number(pb.total) || 0) * n;
    var user = O.state.user || {};
    var row = function (l, v) { return '<div class="wz-row"><span>' + esc(l) + "</span><span>" + money(v) + "</span></div>"; };
    var env = W.envelope;
    return '<div class="wz-checkout"><section class="wz-cart"><h3>Ваш заказ' + (env ? " · ✉️ закрытый конверт" : "") + "</h3>" +
      '<div class="wz-cart-hotel"><strong>' + esc(env ? "Секретный отель, рейтинг " + Number(p.hotel.rating || 0).toFixed(1) : p.hotel.name) + "</strong><span>" + esc(room.name || "") + " · " +
      esc(O.formatDateRange(p.checkIn, p.checkOut)) + " · " + p.guests + " " + O.plural(p.guests, "гость", "гостя", "гостей") + "</span></div>" +
      row("Номер · " + n + " " + O.plural(n, "ночь", "ночи", "ночей"), (Number(pb.room) || 0) * n) +
      row("Налоги и сборы", ((Number(pb.taxes) || 0) + (Number(pb.fees) || 0)) * n) +
      row("Завтрак (включён)", (Number(pb.breakfast) || 0) * n) +
      "<h4>Программа · " + items.length + " " + O.plural(items.length, "позиция", "позиции", "позиций") + "</h4>" +
      (items.length ? items.map(function (i) {
        return '<div class="wz-row wz-item"><span>' + esc(O.formatShortDate(i.date)) + ", " + esc(i.time) + " · " + esc(env ? i.title : (i.place || i.title)) + "</span><span>" + money(i.price) + "</span></div>";
      }).join("") : '<p class="wz-hint">Платные активности не выбраны — оплатите только проживание.</p>') +
      '<div class="wz-row wz-total"><span>Итого к оплате</span><strong>' + money(stay + itemsSum) + "</strong></div>" +
      '<p class="wz-hint">Еда и транспорт не входят — это расходы на месте, они учтены в бюджете плана.</p></section>' +
      '<form class="wz-pay" id="wzPay" novalidate><h3>Гость и оплата</h3>' +
      '<label class="trip-field"><span>Имя</span><input name="name" autocomplete="name" value="' + esc(user.name || "") + '"></label>' +
      '<label class="trip-field"><span>Email</span><input name="email" type="email" autocomplete="email" value="' + esc(user.email || "") + '"></label>' +
      '<label class="trip-field"><span>Телефон</span><input name="phone" type="tel" autocomplete="tel" placeholder="+7 900 000-00-00"></label>' +
      '<label class="trip-field"><span>Номер карты</span><input name="card" inputmode="numeric" autocomplete="cc-number" placeholder="0000 0000 0000 0000"></label>' +
      '<div class="wz-pay-row"><label class="trip-field"><span>Срок</span><input name="exp" inputmode="numeric" autocomplete="cc-exp" placeholder="ММ/ГГ"></label>' +
      '<label class="trip-field"><span>CVC</span><input name="cvc" inputmode="numeric" autocomplete="cc-csc" placeholder="123"></label></div>' +
      '<p class="wz-hint">Демо-оплата: деньги не списываются, данные карты не отправляются на сервер.</p>' +
      '<p class="wz-error hidden" id="wzError" role="alert"></p>' +
      '<div class="wz-footer"><button type="button" class="ghost-button" ' + (env ? "data-wz-back-lucky>К сюрпризу" : "data-wz-back-plan>К программе") + "</button>" +
      '<button type="submit" class="primary-button" id="wzPayBtn">Оплатить ' + money(stay + itemsSum) + "</button></div></form></div>";
  }

  function openCheckout() {
    if (!W || !W.plan) return;
    if (!W.plan.room) { O.showToast("Нет свободного номера под ваши параметры — измените даты или гостей", "error"); return; }
    W.step = 5;
    title("Оплата поездки");
    body().innerHTML = '<div class="wz-root">' + stepper() + '<div class="wz-body">' + checkoutMarkup() + "</div></div>";
  }

  function validCard(form) {
    var digits = String(form.card.value).replace(/\D/g, "");
    if (digits.length < 16 || digits.length > 19) return "Проверьте номер карты";
    var m = String(form.exp.value).match(/^(\d{2})\s*\/\s*(\d{2})$/);
    if (!m || Number(m[1]) < 1 || Number(m[1]) > 12) return "Срок карты в формате ММ/ГГ";
    var now = new Date(), y = 2000 + Number(m[2]);
    if (y < now.getFullYear() || (y === now.getFullYear() && Number(m[1]) < now.getMonth() + 1)) return "Срок действия карты истёк";
    if (!/^\d{3,4}$/.test(String(form.cvc.value).trim())) return "CVC — 3 цифры на обороте";
    return "";
  }

  function pay(form) {
    var err = validCard(form);
    if (err) return showError(err);
    if (!O.state.user) {
      O.state.pendingAction = { type: "callback", run: openCheckout };
      O.openAuth("login", "Войдите, чтобы оплатить поездку — она появится в «Поездках».");
      return;
    }
    var btn = document.getElementById("wzPayBtn");
    var label = btn ? btn.textContent : "";
    if (btn) { btn.disabled = true; btn.textContent = "Оплачиваем…"; }
    var req = tripRequest();
    req.hotelSlug = W.plan.hotel.slug;
    O.api("/api/trip/checkout", {
      method: "POST",
      body: {
        trip: req, roomId: W.plan.room.id, itemIds: cartItems().map(function (i) { return i.id; }),
        guestName: form.name.value, guestEmail: form.email.value, guestPhone: form.phone.value
      }
    }).then(success).catch(function (e) {
      if (btn) { btn.disabled = false; btn.textContent = label; }
      showError((e && e.message) || "Не удалось оплатить");
    });
  }

  function success(res) {
    var t = res.trip || {};
    var count = (res.items || []).length;
    title("Поездка оплачена");
    var envelope = W && W.envelope;
    body().innerHTML = '<div class="wz-root"><div class="wz-success" role="status"><span class="wz-success-ico" aria-hidden="true">' + (envelope ? "✉️" : "✓") + "</span>" +
      (envelope ? '<p class="wz-opened">Конверт открыт! Вы едете в</p><h2 class="wz-city">' + esc(t.city || "") + "</h2>" : "") +
      "<h3>Готово! Код брони " + esc(t.code || "") + "</h3>" +
      "<p>" + esc(t.hotelName || "") + " · " + esc(t.roomName || "") + "</p>" +
      "<p>Оплачено " + money(res.total) + ": проживание " + money(t.total) +
      (count ? " и " + count + " " + O.plural(count, "позиция", "позиции", "позиций") + " программы" : "") + ".</p>" +
      '<div class="wz-footer"><span></span><button type="button" class="primary-button" data-wz-trips>Мои поездки</button></div></div></div>';
    W = null;
  }

  /* ---------- события ---------- */

  function setValue(path, raw) {
    var value = /maxWalkMin|minSize/.test(path) ? Number(raw) || 0 : raw;
    if (path.indexOf("room.") === 0) W.req.room[path.slice(5)] = value; else W.req[path] = value;
  }

  function readStep1() {
    body().querySelectorAll("[data-wz-field]").forEach(function (el) {
      var k = el.getAttribute("data-wz-field");
      W.req[k] = k === "guests" || k === "budget" ? Math.max(0, Number(el.value) || 0) : el.value;
    });
    if (!W.req.from || !W.req.to || W.req.to <= W.req.from) return "Выезд должен быть позже заезда.";
    if (W.req.from < O.todayISO()) return "Дата заезда не может быть в прошлом.";
    return "";
  }

  function next() {
    if (W.step === 1) {
      var err = readStep1();
      if (err) return showError(err);
    }
    if (W.step === 3) return buildPlan();
    W.step += 1;
    paint();
  }

  function startDemo(id) {
    O.api("/api/demo/scenario/" + encodeURIComponent(id)).then(function (sc) {
      var p = (sc && sc.prefill) || {};
      ["destination", "from", "to", "mood", "pace"].forEach(function (k) { if (p[k]) W.req[k] = p[k]; });
      if (Number(p.guests) > 0) W.req.guests = Number(p.guests);
      if (Number(p.budget) > 0) W.req.budget = Number(p.budget);
      if (id === "family") { W.req.interests = ["kids", "nature"]; W.req.room.kidsCot = true; W.req.weather = "warm"; }
      if (id === "romance") { W.req.interests = ["gastro", "views"]; W.req.room.bed = "double"; }
      if (id === "budget") { W.req.interests = ["museums"]; W.req.maxWalkMin = 20; }
      W.step = 3;
      paint();
      buildPlan();
    }).catch(function (e) { showError((e && e.message) || "Сценарий недоступен"); });
  }

  document.addEventListener("click", function (e) {
    var t = e.target.closest && e.target.closest("button");
    if (!t) return;
    if (t.hasAttribute("data-wz-trips")) { O.closeTripPlanner(); O.showTrips("upcoming"); return; }
    var host = body();
    if (!W || !host || !host.contains(t)) return;
    if (t.hasAttribute("data-wz-next")) return next();
    if (t.hasAttribute("data-wz-go")) { W.step = Number(t.getAttribute("data-wz-go")); return paint(); }
    if (t.hasAttribute("data-wz-demo")) return startDemo(t.getAttribute("data-wz-demo"));
    if (t.hasAttribute("data-wz-ai")) { O.closeTripPlanner(); return O.openAssistant(); }
    if (t.hasAttribute("data-wz-lucky")) { readStep1(); W.mode = "surprise"; return paint(); }
    if (t.hasAttribute("data-wz-exit-lucky")) { readSurpriseFields(); W.mode = ""; W.step = 1; return paint(); }
    if (t.hasAttribute("data-wz-spin")) return spin();
    if (t.hasAttribute("data-wz-not-city") && W.surprise) { W.exclude.push(W.surprise.plan.destination); return spin(); }
    if (t.hasAttribute("data-wz-take")) return takeSurprise();
    if (t.hasAttribute("data-wz-back-lucky")) { W.mode = "surprise"; paint(); var rv = document.getElementById("wzReveal"); if (rv && W.surprise) rv.innerHTML = revealMarkup(W.surprise); return; }
    if (t.hasAttribute("data-wz-exclude")) {
      readSurpriseFields();
      var slug = t.getAttribute("data-wz-exclude"), ix = W.exclude.indexOf(slug);
      if (ix >= 0) W.exclude.splice(ix, 1); else W.exclude.push(slug);
      return paint();
    }
    if (t.hasAttribute("data-wz-back-plan")) { W.step = 4; return O.emit("trip:ready", { plan: W.plan, wizard: true }); }
    if (t.hasAttribute("data-wz-set")) {
      if (W.mode === "surprise") readSurpriseFields(); else if (W.step === 1) readStep1();
      setValue(t.getAttribute("data-wz-set"), t.getAttribute("data-wz-val"));
      return paint();
    }
    if (t.hasAttribute("data-wz-interest")) {
      if (W.mode === "surprise") readSurpriseFields();
      var id = t.getAttribute("data-wz-interest"), list = W.req.interests, at = list.indexOf(id);
      if (at >= 0) list.splice(at, 1); else list.push(id);
      return paint();
    }
    if (t.hasAttribute("data-wz-need")) {
      var key = t.getAttribute("data-wz-need");
      W.needs[key] = ((W.needs[key] || 0) + 1) % 3;
      if (!W.needs[key]) delete W.needs[key];
      return paint();
    }
    if (t.hasAttribute("data-wz-room-toggle")) {
      var k = t.getAttribute("data-wz-room-toggle");
      W.req.room[k] = !W.req.room[k];
      return paint();
    }
  });

  document.addEventListener("submit", function (e) {
    if (e.target && e.target.id === "wzPay") { e.preventDefault(); pay(e.target); }
  });

  /* ---------- API для app.js и trip.js ---------- */

  O.renderTripWizard = function (prefill) {
    W = freshState(prefill || {});
    // Ассистент передаёт готовые параметры: сразу собираем предложение.
    var p = prefill && prefill.prefs;
    if (p) {
      ["destination", "from", "to", "mood", "pace", "weather"].forEach(function (k) { if (p[k]) W.req[k] = p[k]; });
      if (Number(p.guests) > 0) W.req.guests = Number(p.guests);
      if (Number(p.budget) > 0) W.req.budget = Number(p.budget);
      if (Array.isArray(p.interests)) W.req.interests = p.interests.slice();
      if (Number(p.maxWalkMin) > 0) W.req.maxWalkMin = Number(p.maxWalkMin);
      if (p.room) W.req.room = Object.assign(W.req.room, p.room);
      if (p.needs) String(p.needs).split(",").forEach(function (c) {
        var parts = c.split(":");
        if (parts[0]) W.needs[parts[0]] = Number(parts[2]) === 2 ? 2 : 1;
      });
      if (W.req.from < O.todayISO() || W.req.to <= W.req.from) { W.req.from = O.addDaysISO(O.todayISO(), 14); W.req.to = O.addDaysISO(W.req.from, 3); }
      W.step = 3;
      paint();
      buildPlan();
      return;
    }
    paint();
  };

  O.openSurprise = function (prefs) {
    O.openTripPlanner();
    if (!W) return;
    if (prefs) {
      if (Number(prefs.budget) > 0) W.req.budget = Number(prefs.budget);
      if (prefs.weather) W.req.weather = prefs.weather;
      if (Array.isArray(prefs.interests)) W.req.interests = prefs.interests.slice();
      if (Number(prefs.guests) > 0) W.req.guests = Number(prefs.guests);
    }
    W.mode = "surprise";
    paint();
  };

  O.wizard = {
    active: function () { return !!(W && W.plan && W.step >= 4); },
    inCart: function (itemId) { return !!(W && W.cart[itemId]); },
    toggle: function (itemId) { if (W) W.cart[itemId] = !W.cart[itemId]; return !!(W && W.cart[itemId]); },
    cartTotal: function () { return W && W.plan ? cartItems().reduce(function (a, i) { return a + i.price; }, 0) : 0; },
    checkout: openCheckout,
    edit: function () { if (W) { W.step = 2; paint(); } }
  };
})();
