(function () {
  "use strict";
  var app = document.getElementById("app");
  var rub = new Intl.NumberFormat("ru-RU");

  function escapeHTML(v) {
    return String(v == null ? "" : v).replace(/[&<>"']/g, function (c) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c];
    });
  }
  function money(n) { return rub.format(Math.round(Number(n) || 0)) + " ₽"; }
  function parseDate(s) {
    var m = /^(\d{4})-(\d{2})-(\d{2})/.exec(s || "");
    return m ? new Date(+m[1], +m[2] - 1, +m[3]) : null;
  }
  function fmtDate(s, opts) {
    var d = parseDate(s);
    return d ? d.toLocaleDateString("ru-RU", opts) : "";
  }
  function plural(n, f) { // f: [1, 2-4, 5+]
    var a = Math.abs(n) % 100, b = a % 10;
    if (a > 10 && a < 20) return f[2];
    if (b > 1 && b < 5) return f[1];
    return b === 1 ? f[0] : f[2];
  }
  function getSlug() {
    var q = new URLSearchParams(location.search).get("s");
    if (q) return q.trim();
    var h = decodeURIComponent((location.hash || "").replace(/^#\/?/, "")).trim();
    return h;
  }
  function safeImg(src) { return /^(\/|https?:\/\/)/.test(src || "") ? src : ""; }

  var ICONS = {
    food: '<path d="M7 3v8M4 3v5a3 3 0 0 0 6 0V3M7 11v10M17 3c-2 2-3 5-3 8h3v10"/>',
    activity: '<path d="M12 21s7-6.1 7-11a7 7 0 1 0-14 0c0 4.9 7 11 7 11z"/><circle cx="12" cy="10" r="2.5"/>',
    rest: '<path d="M4 9h12v5a5 5 0 0 1-5 5H9a5 5 0 0 1-5-5V9zM16 10h2a2 2 0 0 1 0 4h-2M7 3v3M11 3v3"/>',
    transfer: '<rect x="5" y="8" width="14" height="12" rx="2"/><path d="M9 8V5h6v3"/>',
    walk: '<circle cx="13" cy="4.5" r="1.8"/><path d="M10 21l2-6-2-3 1-4 3 2 3 1M8 12l3-3"/>',
    dot: '<circle cx="12" cy="12" r="3"/>'
  };
  function icon(type) {
    var k = ICONS[type] ? type : "dot";
    return '<span class="sp-ico sp-ico--' + escapeHTML(k) + '" aria-hidden="true"><svg viewBox="0 0 24 24">' + ICONS[k] + "</svg></span>";
  }
  var SVG_COPY = '<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="9" y="9" width="11" height="11" rx="2"/><path d="M5 15V6a2 2 0 0 1 2-2h9"/></svg>';
  var SVG_PRINT = '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 4v11M7 10l5 5 5-5M5 20h14"/></svg>';

  function skeleton() {
    app.innerHTML = '<div class="sp-skel" aria-busy="true" aria-label="Загружаем маршрут">' +
      '<div class="sk sk-hero"></div><div class="sp-wrap sp-skel"><div class="sk sk-block"></div><div class="sk sk-block tall"></div><div class="sk sk-block tall"></div></div></div>';
  }
  function stateView(ico, title, text, actionHTML) {
    app.innerHTML = '<div class="sp-state"><div class="sp-state-ico" aria-hidden="true">' + ico + "</div><h1>" + escapeHTML(title) +
      "</h1><p>" + escapeHTML(text) + "</p>" + actionHTML + "</div>";
  }
  function notFound() {
    document.title = "Маршрут не найден — О!дин";
    stateView("🧭", "Маршрут не найден", "Возможно, ссылка устарела или скопирована не полностью.",
      '<a class="sp-btn sp-btn--primary" href="/">Вернуться на главную</a>');
  }
  function netError() {
    stateView("📡", "Не удалось загрузить", "Проверьте соединение и попробуйте ещё раз.",
      '<button type="button" class="sp-btn sp-btn--primary" id="spRetry">Повторить</button>');
    var b = document.getElementById("spRetry");
    if (b) b.addEventListener("click", load);
  }

  function renderBudget(b) {
    b = b || {};
    var split = b.split || {};
    var labels = [["stay", "Проживание"], ["food", "Еда"], ["activities", "Активности"], ["transport", "Транспорт"]];
    var total = Number(b.total) || 0;
    var rows = labels.map(function (l, i) {
      var v = Number(split[l[0]]) || 0;
      var pct = total > 0 ? Math.max(0, Math.min(100, Math.round(v / total * 100))) : 0;
      return '<li><div class="sp-row-head"><span>' + l[1] + '</span><span>' + money(v) + '</span></div>' +
        '<div class="sp-bar" role="img" aria-label="' + l[1] + ": " + pct + '% бюджета"><i class="c' + i + '" style="width:' + pct + '%"></i></div></li>';
    }).join("");
    var verdict = b.fits
      ? '<div class="sp-verdict ok">Укладываетесь в бюджет' + (b.delta ? ", запас " + money(Math.abs(b.delta)) : "") + "</div>"
      : '<div class="sp-verdict over">Выше бюджета на ' + money(Math.abs(b.delta || 0)) + "</div>";
    return '<section class="sp-card" aria-labelledby="spBudget"><h2 id="spBudget">Бюджет</h2>' +
      '<div class="sp-total"><span>Итого за поездку</span><strong>' + money(total) + "</strong></div>" +
      '<ul class="sp-rows">' + rows + "</ul>" + verdict +
      '<p class="sp-pp">' + money(b.perDay) + " в день · " + money(b.perPerson) + " на человека</p></section>";
  }

  // Прогулка с битыми данными (десятки км, сотни минут) не показывается — как в trip.js walkMarkup.
  function usableWalk(walk) {
    if (!walk) return null;
    if (Number(walk.distanceMeters) > 15000 || Number(walk.totalMinutes) > 300) return null;
    return walk;
  }

  function renderDay(day, i) {
    var w = day.weather || {};
    var walk = usableWalk(day.walk);
    var weather = w.icon || w.tempC != null
      ? '<div class="sp-weather" title="' + escapeHTML(w.description || "") + '"><span aria-hidden="true">' + escapeHTML(w.icon || "") +
        "</span><span>" + (w.tempC != null ? (w.tempC > 0 ? "+" : "") + escapeHTML(w.tempC) + "°" : "") + "</span></div>"
      : "";
    var items = (day.blocks || []).map(function (bl) {
      var sub = [bl.place, bl.minutes ? bl.minutes + " мин" : ""].filter(Boolean).map(escapeHTML).join(" · ");
      return '<li class="sp-item"><span class="sp-time">' + escapeHTML(bl.time) + "</span>" + icon(bl.type) +
        '<div class="sp-item-text"><div class="sp-item-title">' + escapeHTML(bl.title) + "</div>" +
        (sub ? '<div class="sp-item-sub">' + sub + "</div>" : "") +
        (bl.note ? '<div class="sp-item-note">' + escapeHTML(bl.note) + "</div>" : "") +
        (bl.price > 0 ? '<div class="sp-item-price">' + money(bl.price) + "</div>" : "") + "</div></li>";
    }).join("");
    var mapUrl = walk && /^https?:\/\//.test(walk.mapUrl || "") ? walk.mapUrl : "";
    var foot = '<div class="sp-day-foot"><span>Итого за день: ' + money(day.total) + "</span>" +
      (mapUrl ? '<a class="sp-link sp-no-print" href="' + escapeHTML(mapUrl) + '" target="_blank" rel="noopener">Пешая карта →</a>' : "") + "</div>";
    var date = fmtDate(day.date, { weekday: "long", day: "numeric", month: "long" });
    return '<article class="sp-card sp-day"><header class="sp-day-head"><div><div class="sp-day-no">День ' + (i + 1) +
      '</div><h3>' + escapeHTML(day.title) + '</h3><div class="sp-day-date">' + escapeHTML(date) + "</div></div>" + weather + "</header>" +
      (w.description ? '<p class="sp-weather-note">' + escapeHTML(w.description) + "</p>" : "") +
      '<ol class="sp-tl">' + items + "</ol>" + foot + "</article>";
  }

  function render(data) {
    var p = data.plan || {};
    var h = p.hotel || {};
    var m = p.match || {};
    var city = p.city || h.city || "Ваша поездка";
    document.title = "Маршрут: " + city + " — О!дин";
    var nights = Number(p.nights) || 0, guests = Number(p.guests) || 0;
    var range = fmtDate(p.checkIn, { day: "numeric", month: "long" }) + " — " + fmtDate(p.checkOut, { day: "numeric", month: "long", year: "numeric" });
    var cover = safeImg(h.image || p.image);
    var views = Number(data.views) || 0;
    var cta = h.slug ? "/?hotel=" + encodeURIComponent(h.slug) : "/";
    var score = Math.max(0, Math.min(100, Number(m.score) || 0));
    var circ = 2 * Math.PI * 42;

    var hero = '<header class="sp-hero"' + (cover ? ' style="background-image:url(\'' + escapeHTML(cover) + '\')"' : "") + ">" +
      '<div class="sp-top"><a class="sp-brand" href="/">О<b>!</b>дин</a><span class="sp-eyebrow">Маршрут поездки</span></div>' +
      '<div class="sp-wrap sp-hero-body"><h1>' + escapeHTML(city) + "</h1>" +
      (h.name ? '<p class="sp-hero-sub">Отель ' + escapeHTML(h.name) + "</p>" : "") +
      '<div class="sp-facts"><span class="sp-fact">' + escapeHTML(range) + "</span>" +
      (nights ? '<span class="sp-fact">' + nights + " " + plural(nights, ["ночь", "ночи", "ночей"]) + "</span>" : "") +
      (guests ? '<span class="sp-fact">' + guests + " " + plural(guests, ["гость", "гостя", "гостей"]) + "</span>" : "") +
      "</div></div></header>";

    var match = m.score != null ? '<section class="sp-card sp-match" aria-label="Совпадение с вашими желаниями"><div class="sp-score" role="img" aria-label="Совпадение ' + score + ' из 100">' +
      '<svg viewBox="0 0 100 100"><circle class="bg" cx="50" cy="50" r="42"/><circle class="fg" cx="50" cy="50" r="42" stroke-dasharray="' + circ.toFixed(1) + '" stroke-dashoffset="' + (circ * (1 - score / 100)).toFixed(1) + '"/></svg><b>' + score + "</b></div>" +
      "<div><h2>" + escapeHTML(m.verdict || "Совпадение") + "</h2><p>" + escapeHTML(m.reason || "") + "</p></div></section>" : "";

    var hotel = h.name ? '<section class="sp-card sp-hotel" aria-label="Отель">' + (safeImg(h.image) ? '<img src="' + escapeHTML(h.image) + '" alt="' + escapeHTML(h.name) + '" loading="lazy">' : "") +
      '<div class="sp-hotel-body"><div class="sp-label">Где живём</div><h3>' + escapeHTML(h.name) + "</h3>" +
      (h.shortDescription ? "<p>" + escapeHTML(h.shortDescription) + "</p>" : "") +
      '<div class="sp-meta">' + (h.rating ? '<span class="sp-chip sp-chip--lime">★ ' + escapeHTML(h.rating) + "</span>" : "") +
      (h.city ? '<span class="sp-chip">' + escapeHTML(h.city) + "</span>" : "") +
      (h.priceFrom ? '<span class="sp-chip">от ' + money(h.priceFrom) + " / ночь</span>" : "") + "</div></div></section>" : "";

    var days = (p.days || []).length ? '<div class="sp-days">' + p.days.map(renderDay).join("") + "</div>" : "";
    var tips = (p.tips || []).length ? '<section class="sp-card"><h2>Советы</h2><ul class="sp-tips">' + p.tips.map(function (t) { return "<li>" + escapeHTML(t) + "</li>"; }).join("") + "</ul></section>" : "";

    var side = '<section class="sp-card sp-qr-card" aria-labelledby="spQrT"><h2 id="spQrT" style="margin:0">Открыть на телефоне</h2>' +
      '<div class="sp-qr" id="spQr" role="img" aria-label="QR-код со ссылкой на этот маршрут"></div><p>Наведите камеру — маршрут откроется без входа и установки.</p>' +
      '<div class="sp-actions sp-no-print" style="width:100%">' +
      '<button type="button" class="sp-btn sp-btn--primary" id="spPrint">' + SVG_PRINT + "Скачать</button>" +
      '<button type="button" class="sp-btn" id="spCopy">' + SVG_COPY + "Скопировать ссылку</button></div>" +
      '<div class="sp-views">' + views + " " + plural(views, ["просмотр", "просмотра", "просмотров"]) + "</div></section>";

    var ctaBlock = '<section class="sp-card sp-cta"><h2>Хотите так же?</h2><p>О!дин подберёт отель и маршрут под ваши привычки за минуту.</p>' +
      '<a class="sp-btn sp-btn--lime" href="' + escapeHTML(cta) + '">Открыть в О!дин</a></section>';

    app.innerHTML = hero + '<div class="sp-wrap"><div class="sp-main"><div class="sp-col">' + match + hotel + days + tips +
      '</div><aside class="sp-col sp-side">' + renderBudget(p.budget) + side + ctaBlock + "</aside></div>" +
      '<p class="sp-footer">Собрано в О!дин · <a class="sp-link" href="/">odin.travel</a></p></div>';

    drawQR();
    var print = document.getElementById("spPrint");
    if (print) print.addEventListener("click", function () { window.print(); });
    var copy = document.getElementById("spCopy");
    if (copy) copy.addEventListener("click", function () { copyLink(copy); });
  }

  function drawQR() {
    var box = document.getElementById("spQr");
    if (!box) return;
    try {
      var qr = qrcode(0, "M");
      qr.addData(location.href);
      qr.make();
      box.innerHTML = qr.createSvgTag({ cellSize: 4, margin: 0, scalable: true });
    } catch (e) {
      box.textContent = "QR недоступен";
    }
  }

  function copyLink(btn) {
    var url = location.href;
    var label = btn.innerHTML;
    function done(ok) {
      btn.innerHTML = ok ? "Скопировано" : "Выделите и скопируйте";
      setTimeout(function () { btn.innerHTML = label; }, 2200);
    }
    function fallback() {
      var ta = document.createElement("textarea");
      ta.value = url; ta.setAttribute("readonly", "");
      ta.style.cssText = "position:fixed;top:0;left:0;opacity:0;font-size:16px";
      document.body.appendChild(ta);
      ta.select();
      var ok = false;
      try { ok = document.execCommand("copy"); } catch (e) {}
      document.body.removeChild(ta);
      if (!ok) { try { window.prompt("Скопируйте ссылку", url); } catch (e) {} }
      done(ok);
    }
    if (navigator.clipboard && window.isSecureContext) {
      navigator.clipboard.writeText(url).then(function () { done(true); }, fallback);
    } else fallback();
  }

  function load() {
    var slug = getSlug();
    if (!slug) { notFound(); return; }
    skeleton();
    fetch("/api/share/" + encodeURIComponent(slug), { headers: { Accept: "application/json" } })
      .then(function (r) {
        if (r.status === 404) { notFound(); return null; }
        if (!r.ok) throw new Error("http " + r.status);
        return r.json();
      })
      .then(function (data) { if (data) render(data); })
      .catch(function () { netError(); });
  }

  window.addEventListener("hashchange", function () { if (!new URLSearchParams(location.search).get("s")) load(); });
  load();
})();
