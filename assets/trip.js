/* Экран «Маршрут» (N2). Перерисовывает #tripBody на событие odin:trip:ready. */
(function () {
  "use strict";
  var O = window.ODIN;
  if (!O) return;

  var esc = O.escapeHTML;
  var money = O.money;
  var MOCK_URL = "/assets/mocks/trip.json";

  var TYPES = {
    food: { label: "Еда", icon: '<path d="M7 3v8M4 3v5a3 3 0 0 0 3 3M10 3v5a3 3 0 0 1-3 3M7 11v10"/><path d="M17 3c-2 1.5-3 4-3 7h3v11"/>' },
    activity: { label: "Активность", icon: '<path d="M12 3l2.6 5.6 6 .7-4.5 4.1 1.2 6L12 16.5 6.7 19.4l1.2-6L3.4 9.3l6-.7z"/>' },
    walk: { label: "Прогулка", icon: '<circle cx="13" cy="4.5" r="1.8"/><path d="M10 21l2-6-3-3 2-5 3 2 3 1M12 15l3 2 1 4"/>' },
    transfer: { label: "Переезд", icon: '<path d="M4 16V9a2 2 0 0 1 2-2h8l4 4v5"/><circle cx="8" cy="17" r="1.8"/><circle cx="16" cy="17" r="1.8"/>' },
    rest: { label: "Отдых", icon: '<path d="M4 12h16v6H4zM6 12V8a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v4M4 18v2M20 18v2"/>' },
    hotel: { label: "Отель", icon: '<path d="M3 20V7M3 12h18v8M21 12a3 3 0 0 0-3-3h-7v3"/>' }
  };
  var SPLIT = [
    ["stay", "Проживание"],
    ["food", "Еда"],
    ["activities", "Занятия"],
    ["transport", "Транспорт"]
  ];

  var S = null; // { plan, tab, pace, origBudget, busy }
  var abort = null; // AbortController для слушателей текущего рендера

  function ico(path) {
    return '<svg viewBox="0 0 24 24" aria-hidden="true">' + path + "</svg>";
  }
  function typeOf(t) { return TYPES[t] || TYPES.activity; }
  function num(v) { return Number(v) || 0; }
  function plural(n, a, b, c) { return O.plural(n, a, b, c); }

  function dayTotal(day) {
    return (day.blocks || []).reduce(function (s, b) { return s + num(b.price); }, 0);
  }

  /* ---------- разметка ---------- */

  function hotelMarkup(plan) {
    var h = plan.hotel || {};
    return '<button type="button" class="tx-hotel" data-tx-hotel="' + esc(h.slug || "") + '" aria-label="Открыть отель ' + esc(h.name || "") + '">' +
      '<img src="' + esc(h.image || plan.image || "") + '" alt="" loading="lazy">' +
      '<span class="tx-hotel-copy">' +
      '<span class="tx-kicker">' + esc(h.city || plan.city) + " · " + esc(O.formatDateRange(plan.checkIn, plan.checkOut)) + "</span>" +
      "<strong>" + esc(h.name || "") + "</strong>" +
      "<span class=\"tx-hotel-desc\">" + esc(h.shortDescription || "") + "</span>" +
      '<span class="tx-hotel-meta"><span>★ ' + num(h.rating).toFixed(1) + "</span><span>" + money(num(h.priceFrom)) + " за ночь</span><span>" +
      num(plan.guests) + " " + plural(num(plan.guests), "гость", "гостя", "гостей") + "</span></span>" +
      "</span>" +
      '<span class="tx-hotel-go" aria-hidden="true">' + ico('<path d="M9 6l6 6-6 6"/>') + "</span></button>";
  }

  function whyMarkup(plan) {
    var h = plan.hotel || {};
    if (!h.slug) return "";
    return '<section class="tx-why" id="txWhy" aria-label="Почему этот отель">' +
      '<button type="button" class="tx-why-btn" id="txWhyBtn" aria-expanded="false" aria-controls="txWhyBody">' +
      "<span>Почему этот отель</span>" + ico('<path d="M6 9l6 6 6-6"/>') + "</button>" +
      '<div class="tx-why-body" id="txWhyBody" hidden>' +
      '<div class="tx-why-mount" id="txWhyMount" aria-live="polite"></div>' +
      '<p class="tx-why-note">По параметрам карточки отеля. Балл здесь может отличаться от балла маршрута вверху.</p></div></section>';
  }

  function toggleWhy(host) {
    var btn = host.querySelector("#txWhyBtn");
    var body = host.querySelector("#txWhyBody");
    var box = host.querySelector("#txWhy");
    if (!btn || !body || !box) return;
    var open = btn.getAttribute("aria-expanded") !== "true";
    btn.setAttribute("aria-expanded", open ? "true" : "false");
    body.hidden = !open;
    box.classList.toggle("is-open", open);
    if (!open || S.whyState) return;
    S.whyState = "loading";
    var plan = S.plan;
    var slug = (plan.hotel || {}).slug;
    var q = new URLSearchParams();
    if (plan.checkIn) q.set("checkIn", plan.checkIn);
    if (plan.checkOut) q.set("checkOut", plan.checkOut);
    if (plan.guests) q.set("guests", String(plan.guests));
    var fail = function () {
      S.whyState = "failed";
      if (box.isConnected) box.hidden = true;
    };
    if (typeof O.renderMatch !== "function") return fail();
    body.setAttribute("aria-busy", "true");
    O.api("/api/hotels/" + encodeURIComponent(slug) + "?" + q.toString()).then(function (hotel) {
      var mount = host.querySelector("#txWhyMount");
      var detail = hotel && hotel.matchDetail;
      if (!mount || !box.isConnected) return;
      if (!detail || !O.renderMatch(mount, detail, { compact: true })) return fail();
      body.removeAttribute("aria-busy");
      S.whyState = "done";
    }).catch(fail);
  }

  function tabsMarkup(plan) {
    return '<div class="tx-tabs" role="tablist" aria-label="Дни поездки">' + (plan.days || []).map(function (d, i) {
      var w = d.weather || {};
      var on = i === S.tab;
      return '<button type="button" role="tab" class="tx-tab' + (on ? " is-active" : "") + '" id="txTab' + i + '" aria-selected="' + on +
        '" aria-controls="txPanel" tabindex="' + (on ? 0 : -1) + '" data-tx-tab="' + i + '">' +
        '<span class="tx-tab-n">День ' + (i + 1) + "</span>" +
        '<span class="tx-tab-date">' + esc(O.formatShortDate(d.date)) + "</span>" +
        '<span class="tx-chip">' + esc(w.icon || "") + " " + num(w.tempC) + "°</span></button>";
    }).join("") + "</div>";
  }

  function blockMarkup(b) {
    var t = typeOf(b.type);
    var price = num(b.price) > 0 ? money(num(b.price)) : (b.type === "food" ? "—" : "бесплатно");
    return '<li class="tx-block tx-t-' + esc(b.type || "activity") + '">' +
      '<span class="tx-dot" aria-hidden="true">' + ico(t.icon) + "</span>" +
      '<div class="tx-card"><div class="tx-card-top"><span class="tx-time">' + esc(b.time) + '</span><span class="tx-type">' + esc(t.label) +
      (num(b.minutes) ? " · " + num(b.minutes) + " мин" : "") + "</span>" +
      '<span class="tx-price' + (num(b.price) > 0 ? "" : " is-free") + '">' + price + "</span></div>" +
      "<strong>" + esc(b.title) + "</strong>" +
      (b.place ? '<span class="tx-place">' + esc(b.place) + "</span>" : "") +
      (b.note ? '<span class="tx-note">' + esc(b.note) + "</span>" : "") + "</div></li>";
  }

  function walkMarkup(w) {
    if (!w) return "";
    // Данные прогулки иногда приходят битыми (десятки км, сотни минут) — такую прогулку не показываем.
    if (num(w.distanceMeters) > 15000 || num(w.totalMinutes) > 300) return "";
    var stops = (w.stops || []).length;
    return '<div class="tx-walk"><span class="tx-walk-ico" aria-hidden="true">' + ico(TYPES.walk.icon) + "</span>" +
      '<div class="tx-walk-copy"><strong>' + esc(w.title || "Пешая прогулка") + "</strong><span>" +
      stops + " " + plural(stops, "остановка", "остановки", "остановок") +
      (num(w.distanceMeters) ? " · " + (num(w.distanceMeters) / 1000).toFixed(1).replace(".", ",") + " км" : "") +
      (num(w.totalMinutes) ? " · " + num(w.totalMinutes) + " мин" : "") + "</span></div>" +
      (w.mapUrl ? '<a class="tx-walk-link" href="' + esc(w.mapUrl) + '" target="_blank" rel="noopener">На карте</a>' : "") + "</div>";
  }

  function panelMarkup() {
    var d = S.plan.days[S.tab];
    if (!d) return "";
    var rest = ((S.dayPace && S.dayPace[S.tab]) || S.plan.pace) === "active" ? "насыщенный" : "спокойный";
    return '<div class="tx-day-head"><div><span class="tx-kicker">' + esc(O.formatShortDate(d.date)) + " · " + rest + " темп</span>" +
      "<h3>" + esc(d.title) + "</h3></div>" +
      '<button type="button" class="tx-rebuild" id="txRebuild"' + (S.busy ? " disabled" : "") + ">" +
      ico('<path d="M20 11a8 8 0 1 0-2.3 5.7M20 4v7h-7"/>') + "<span>" + (S.busy ? "Перестраиваем…" : "Перестроить день") + "</span></button></div>" +
      '<ol class="tx-timeline">' + (d.blocks || []).map(blockMarkup).join("") + "</ol>" +
      walkMarkup(d.walk) +
      '<div class="tx-day-total"><span>Итого за день</span><strong>' + money(num(d.total)) + "</strong></div>";
  }

  function budgetMarkup(plan) {
    var b = plan.budget || { split: {} };
    var sp = b.split || {};
    var total = num(b.total) || 1;
    var fits = b.fits !== false;
    return '<section class="tx-budget" aria-label="Бюджет поездки"><div class="tx-budget-top"><div>' +
      '<span class="tx-kicker">Честная стоимость поездки</span><strong class="tx-total">' + money(num(b.total)) + "</strong>" +
      "<small>" + money(num(b.perPerson)) + " на человека · " + money(num(b.perDay)) + " в день</small></div>" +
      '<span class="tx-fit ' + (fits ? "is-ok" : "is-over") + '">' + (fits ? "В бюджет" : "+" + money(num(b.delta)) + " сверх бюджета") + "</span></div>" +
      '<ul class="tx-split">' + SPLIT.map(function (r) {
        var v = num(sp[r[0]]);
        var pct = Math.round((v / total) * 100);
        return '<li><div class="tx-split-row"><span>' + r[1] + "</span><span>" + money(v) + " · " + pct + "%</span></div>" +
          '<div class="tx-bar" role="presentation"><i class="tx-bar-' + r[0] + '" style="width:' + Math.max(pct, v > 0 ? 2 : 0) + '%"></i></div></li>';
      }).join("") + "</ul></section>";
  }

  /* ---------- рендер ---------- */

  function render(host, buttons) {
    var plan = S.plan;
    var m = plan.match || {};
    var score = num(m.score) || num((plan.hotel || {}).match);
    var title = document.getElementById("tripTitle");
    if (title) title.textContent = plan.city + " · " + plan.nights + " " + plural(plan.nights, "ночь", "ночи", "ночей");

    host.innerHTML =
      '<div class="tx-root">' +
      '<header class="tx-head"><div class="tx-match"><span class="tx-score" aria-label="Совпадение ' + score + ' процентов">' + score + '<small>%</small></span>' +
      "<div><strong>" + esc(m.verdict || "Подходит") + "</strong><span>" + esc(m.reason || "") + "</span></div></div>" +
      '<div class="tx-actions"><button type="button" class="tx-btn" id="txShare">' + ico('<path d="M12 15V3M8 7l4-4 4 4M5 12v7h14v-7"/>') + "Поделиться</button>" +
      '<span id="txSlotRestart"></span><span id="txSlotBook"></span></div></header>' +
      hotelMarkup(plan) +
      whyMarkup(plan) +
      '<section class="tx-days" aria-label="План по дням">' + tabsMarkup(plan) +
      '<div class="tx-panel" id="txPanel" role="tabpanel" tabindex="0" aria-labelledby="txTab' + S.tab + '">' + panelMarkup() + "</div></section>" +
      budgetMarkup(plan) +
      ((plan.tips || []).length ? '<section class="tx-tips"><h3>Советы</h3><ul>' + plan.tips.map(function (t) { return "<li>" + esc(t) + "</li>"; }).join("") + "</ul></section>" : "") +
      "</div>";

    // исходные кнопки app.js переносим в наш layout (их обработчики сохраняются)
    if (buttons.restart) {
      buttons.restart.className = "tx-btn";
      host.querySelector("#txSlotRestart").replaceWith(buttons.restart);
    }
    if (buttons.book) {
      buttons.book.className = "tx-btn tx-btn-primary";
      host.querySelector("#txSlotBook").replaceWith(buttons.book);
    }
    bind(host);
  }

  function bind(host) {
    if (abort) abort.abort();
    abort = typeof AbortController === "function" ? new AbortController() : null;
    var opt = abort ? { signal: abort.signal } : false;
    host.addEventListener("click", function (e) {
      var t = e.target.closest("[data-tx-tab]");
      if (t) return selectTab(host, num(t.getAttribute("data-tx-tab")), false);
      var h = e.target.closest("[data-tx-hotel]");
      if (h) {
        var slug = h.getAttribute("data-tx-hotel");
        if (slug) { O.closeTripPlanner(); O.openHotel(slug); }
        return;
      }
      if (e.target.closest("#txWhyBtn")) return toggleWhy(host);
      if (e.target.closest("#txShare")) return O.shareTrip(S.plan);
      if (e.target.closest("#txRebuild")) return rebuild(host);
    }, opt);
    host.addEventListener("keydown", function (e) {
      var tab = e.target.closest && e.target.closest("[data-tx-tab]");
      if (!tab) return;
      var n = S.plan.days.length, i = S.tab, k = e.key;
      if (k === "ArrowRight") i = (i + 1) % n;
      else if (k === "ArrowLeft") i = (i - 1 + n) % n;
      else if (k === "Home") i = 0;
      else if (k === "End") i = n - 1;
      else return;
      e.preventDefault();
      selectTab(host, i, true);
    }, opt);
  }

  function selectTab(host, i, focus) {
    if (i === S.tab || i < 0 || i >= S.plan.days.length) return;
    S.tab = i;
    host.querySelectorAll("[data-tx-tab]").forEach(function (b, idx) {
      var on = idx === i;
      b.classList.toggle("is-active", on);
      b.setAttribute("aria-selected", on);
      b.tabIndex = on ? 0 : -1;
      if (on && focus) b.focus();
    });
    paintPanel(host);
  }

  function paintPanel(host) {
    var p = host.querySelector("#txPanel");
    if (!p) return;
    p.setAttribute("aria-labelledby", "txTab" + S.tab);
    p.innerHTML = panelMarkup();
    p.classList.remove("is-enter");
    void p.offsetWidth;
    p.classList.add("is-enter");
  }

  /* ---------- перестроить день ---------- */

  function oppositePace() {
    var cur = (S.dayPace && S.dayPace[S.tab]) || S.plan.pace || "calm";
    return cur === "calm" ? "active" : "calm";
  }

  function toast(msg, tone) { if (O.showToast) O.showToast(msg, tone); }

  function fetchDay(pace) {
    var p = S.plan;
    var req = {
      destination: p.destination,
      from: p.checkIn,
      to: p.checkOut,
      guests: p.guests,
      budget: S.origBudget,
      hotelSlug: (p.hotel || {}).slug,
      pace: pace
    };
    return O.api("/api/trip/plan", { method: "POST", body: req }).then(function (r) {
      if (!r || !r.days || !r.days[S.tab]) throw new Error("empty");
      return r.days[S.tab];
    }).catch(function () {
      return fetch(MOCK_URL).then(function (r) {
        if (!r.ok) throw new Error("mock");
        return r.json();
      }).then(function (mock) {
        var src = mock.days[S.tab % mock.days.length];
        var cur = S.plan.days[S.tab];
        var copy = JSON.parse(JSON.stringify(src));
        copy.date = cur.date;
        copy.weather = cur.weather || copy.weather;
        toast("Сервер недоступен — показан пример дня", "info");
        return copy;
      });
    });
  }

  function recomputeBudget(oldDay, newDay) {
    var b = S.plan.budget;
    if (!b) return;
    function sums(d) {
      var food = 0, act = 0;
      (d.blocks || []).forEach(function (x) {
        if (x.type === "food") food += num(x.price);
        else if (x.type === "activity" || x.type === "rest") act += num(x.price);
      });
      return [food, act];
    }
    var o = sums(oldDay), n = sums(newDay);
    b.split = b.split || {};
    b.split.food = num(b.split.food) + n[0] - o[0];
    b.split.activities = num(b.split.activities) + n[1] - o[1];
    b.total = num(b.total) + (n[0] - o[0]) + (n[1] - o[1]);
    var days = S.plan.days.length || 1;
    b.perDay = Math.floor(b.total / days);
    b.perPerson = Math.floor(b.total / Math.max(1, num(S.plan.guests)));
    if (S.origBudget > 0) { b.delta = b.total - S.origBudget; b.fits = b.delta <= 0; }
  }

  function rebuild(host) {
    if (S.busy) return;
    var idx = S.tab;
    var pace = oppositePace();
    S.busy = true;
    paintPanel(host);
    fetchDay(pace).then(function (day) {
      if (!S || S.plan.days[idx] === undefined) return;
      var old = S.plan.days[idx];
      day.total = dayTotal(day);
      recomputeBudget(old, day);
      S.plan.days[idx] = day;
      S.dayPace = S.dayPace || {};
      S.dayPace[idx] = pace;
      if (O.tripPlan && O.tripPlan() && O.tripPlan().days) { /* исходный план не трогаем */ }
    }).catch(function () {
      toast("Не удалось перестроить день", "error");
    }).then(function () {
      S.busy = false;
      if (!host.isConnected || !host.querySelector(".tx-root")) return;
      var t = host.querySelector(".tx-budget");
      if (t) t.outerHTML = budgetMarkup(S.plan);
      paintPanel(host);
      var tabs = host.querySelectorAll("[data-tx-tab]");
      var chip = tabs[idx] && tabs[idx].querySelector(".tx-chip");
      var w = S.plan.days[idx].weather || {};
      if (chip) chip.textContent = (w.icon || "") + " " + num(w.tempC) + "°";
      var r = host.querySelector("#txRebuild");
      if (r) r.focus();
    });
  }

  /* ---------- вход ---------- */

  function onReady(e) {
    var plan = e && e.detail && e.detail.plan;
    var host = document.getElementById("tripBody");
    if (!plan || !host || !plan.days || !plan.days.length) return;
    var buttons = {
      restart: host.querySelector("#tripRestart"),
      book: host.querySelector("#tripBook")
    };
    if (buttons.restart) buttons.restart.remove();
    if (buttons.book) buttons.book.remove();
    var copy = JSON.parse(JSON.stringify(plan));
    var b = copy.budget || {};
    S = {
      plan: copy,
      tab: 0,
      pace: copy.pace || "calm",
      dayPace: {},
      busy: false,
      origBudget: Math.max(0, num(b.total) - num(b.delta))
    };
    render(host, buttons);
  }

  O.on("trip:ready", onReady);
})();
