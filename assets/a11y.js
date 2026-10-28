/* N9 — доступность: focus-trap оверлеев, возврат фокуса, Esc, skip-link, aria на лету.
   Автономный модуль: не меняет app.js, работает через MutationObserver и делегирование. */
(function () {
  "use strict";
  if (window.__odinA11y) return;
  window.__odinA11y = true;

  var FOCUSABLE = 'a[href],button:not([disabled]),input:not([disabled]):not([type="hidden"]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])';

  /* ---------- Оверлеи ---------- */
  // own: Esc обрабатываем сами (app.js не умеет или конфликтует). Остальные закрывает app.js.
  var DEFS = [
    { sel: "#mobileMenu", open: function (e) { return e.classList.contains("is-open"); }, close: "#mobileMenuClose", own: true, role: "dialog", label: "Меню" },
    { sel: "#assistantDrawer", open: function (e) { return e.classList.contains("is-open"); }, role: "dialog", label: "Ассистент" },
    { sel: "#bookingModal", open: function (e) { return e.classList.contains("is-open"); }, box: ".booking-panel" },
    { sel: "#authModal", open: function (e) { return e.classList.contains("is-open"); } },
    { sel: "#poiModal", open: function (e) { return e.classList.contains("is-open"); } },
    { sel: "#commandPalette", open: function (e) { return e.classList.contains("is-open"); } },
    { sel: "#tripPlanner", open: function (e) { return !e.classList.contains("hidden"); }, close: "#tripClose", own: true },
    { sel: "#shareOverlay", open: function (e) { return !e.classList.contains("hidden"); }, close: "#shareClose", own: true },
    { sel: "#galleryZoom", open: function (e) { return e.isConnected; }, own: true, lightbox: true }
  ];
  var stack = []; // {def, el, opener}
  window.__odinA11yStack = stack;

  function visible(el) {
    if (!el || !el.isConnected) return false;
    var r = el.getClientRects();
    if (!r.length) return false;
    var cs = getComputedStyle(el);
    return cs.visibility !== "hidden" && cs.display !== "none";
  }
  function focusables(root) {
    return Array.prototype.filter.call(root.querySelectorAll(FOCUSABLE), visible);
  }
  function boxOf(item) {
    return (item.def.box && item.el.querySelector(item.def.box)) || item.el;
  }

  function prepare(def, el) {
    if (def.role && !el.getAttribute("role")) el.setAttribute("role", def.role);
    if (def.role === "dialog") {
      el.setAttribute("aria-modal", "true");
      if (!el.hasAttribute("aria-label") && !el.hasAttribute("aria-labelledby") && def.label) el.setAttribute("aria-label", def.label);
    }
    if (def.lightbox) {
      el.setAttribute("role", "dialog");
      el.setAttribute("aria-modal", "true");
      el.setAttribute("aria-label", "Просмотр фото");
      el.setAttribute("tabindex", "-1");
      if (!el.querySelector(".a11y-lightbox-close")) {
        var b = document.createElement("button");
        b.type = "button";
        b.className = "a11y-lightbox-close";
        b.setAttribute("aria-label", "Закрыть фото");
        b.textContent = "×";
        b.addEventListener("click", function (ev) { ev.stopPropagation(); el.remove(); });
        el.appendChild(b);
      }
    }
  }

  function focusInto(item) {
    var box = boxOf(item);
    if (box.contains(document.activeElement) && document.activeElement !== box) return;
    var target = null;
    if (item.def.lightbox) target = box.querySelector(".a11y-lightbox-close");
    if (!target) {
      var list = focusables(box).filter(function (n) { return !n.closest("[aria-hidden='true']"); });
      var close = item.def.close && box.querySelector(item.def.close);
      // Первый фокусируемый; если это только крестик — тоже ок.
      target = list.find(function (n) { return n !== close; }) || list[0] || null;
    }
    if (!target) {
      var h = box.querySelector("h1,h2,h3,[role='heading']") || box;
      if (!h.hasAttribute("tabindex")) h.setAttribute("tabindex", "-1");
      target = h;
    }
    try { target.focus({ preventScroll: true }); } catch (e) { target.focus(); }
  }

  function restore(item) {
    var o = item.opener;
    var cur = document.activeElement;
    var lost = !cur || cur === document.body || !cur.isConnected || item.el.contains(cur) || !visible(cur);
    if (!lost) return;
    if (o && o.isConnected && visible(o)) { try { o.focus({ preventScroll: true }); return; } catch (e) {} }
    // Если есть ещё оверлей — вернём фокус в него, иначе на main.
    var top = stack[stack.length - 1];
    if (top) return focusInto(top);
    var m = document.querySelector("main");
    if (m) { if (!m.hasAttribute("tabindex")) m.setAttribute("tabindex", "-1"); m.focus({ preventScroll: true }); }
  }

  function scan() {
    var i, d, el, item, idx;
    // закрытые
    for (i = stack.length - 1; i >= 0; i--) {
      item = stack[i];
      if (!item.el.isConnected || !item.def.open(item.el)) {
        stack.splice(i, 1);
        // фокус возвращаем с задержкой на кадр: app.js мог ещё что-то перерисовать
        (function (it) { setTimeout(function () { restore(it); }, 30); })(item);
      }
    }
    // открытые
    DEFS.forEach(function (def) {
      el = document.querySelector(def.sel);
      if (!el || !def.open(el)) return;
      if (stack.some(function (s) { return s.el === el; })) return;
      var opener = document.activeElement;
      if (opener && (opener === document.body || el.contains(opener))) opener = null;
      item = { def: def, el: el, opener: opener };
      stack.push(item);
      prepare(def, el);
      setTimeout(function () { if (stack.indexOf(item) >= 0) focusInto(item); }, 60);
    });
  }

  // Перерисовка внутри оверлея убила элемент с фокусом -> фокус на заголовок оверлея, а не в <body>.
  function rescue() {
    var top = stack[stack.length - 1];
    if (!top) return;
    var a = document.activeElement;
    if (a && a !== document.body && a.isConnected) return;
    var box = boxOf(top);
    var h = box.querySelector("h1,h2,h3");
    if (h && visible(h)) {
      if (!h.hasAttribute("tabindex")) h.setAttribute("tabindex", "-1");
      h.focus({ preventScroll: true });
    } else focusInto(top);
  }

  // Смена экрана (главная -> отель): app.js прячет .view, фокус падает в <body>. Переносим на заголовок.
  var lastView = null;
  var viewTries = 0;
  function viewFocus() {
    var v = document.querySelector("main .view:not(.hidden)");
    if (!v) return;
    if (lastView === null) { lastView = v.id; return; }
    if (v.id === lastView) return;
    var a = document.activeElement;
    if (stack.length || (a && a !== document.body && a.isConnected && visible(a))) { lastView = v.id; viewTries = 0; return; }
    var h = v.querySelector("h1,h2");
    if (!h && viewTries++ < 40) return; // контент отеля ещё грузится — ждём заголовок
    lastView = v.id; viewTries = 0;
    h = h || v;
    if (!h.hasAttribute("tabindex")) h.setAttribute("tabindex", "-1");
    h.focus({ preventScroll: true });
  }

  var pending = false;
  function schedule() {
    if (pending) return;
    pending = true;
    setTimeout(function () { pending = false; scan(); enhance(); rescue(); viewFocus(); }, 16);
  }

  /* ---------- Клавиатура ---------- */
  window.addEventListener("keydown", function (e) {
    var top = stack[stack.length - 1];
    if (e.key === "Escape" && top && top.def.own && !e.defaultPrevented) {
      e.preventDefault();
      e.stopImmediatePropagation(); // не даём app.js закрыть планировщик под поделиться
      if (top.def.lightbox) top.el.remove();
      else { var c = top.el.querySelector(top.def.close); if (c) c.click(); }
      return;
    }
    if (e.key === "Tab" && top) {
      var box = boxOf(top);
      var list = focusables(box);
      if (!list.length) { e.preventDefault(); focusInto(top); return; }
      var first = list[0], last = list[list.length - 1], a = document.activeElement;
      if (!box.contains(a) || a === box) { e.preventDefault(); (e.shiftKey ? last : first).focus(); }
      else if (e.shiftKey && a === first) { e.preventDefault(); last.focus(); }
      else if (!e.shiftKey && a === last) { e.preventDefault(); first.focus(); }
    }
  }, true);

  // Фокус не должен уезжать за оверлей (клик мимо, программный focus()).
  document.addEventListener("focusin", function (e) {
    var top = stack[stack.length - 1];
    if (!top) return;
    var box = boxOf(top);
    if (box.contains(e.target) || top.el.contains(e.target)) return;
    // уведомления/тосты не перехватываем
    if (e.target.closest && e.target.closest("[role='status'],[role='alert'],.toast")) return;
    focusInto(top);
  });

  // Enter/Space для не-нативных «кнопок» и «ссылок» (карточки, фото галереи).
  document.addEventListener("keydown", function (e) {
    if (e.defaultPrevented || (e.key !== "Enter" && e.key !== " ")) return;
    var t = e.target;
    if (!t || !t.matches || !t.matches('[role="button"][tabindex],[role="link"][tabindex]')) return;
    if (/^(A|BUTTON|INPUT|SELECT|TEXTAREA|SUMMARY)$/.test(t.tagName)) return;
    e.preventDefault();
    t.click();
  });

  /* ---------- Skip-link ---------- */
  function ensureSkip() {
    if (document.getElementById("a11ySkip")) return;
    var main = document.querySelector("main");
    if (!main) return;
    if (!main.id) main.id = "main";
    var a = document.createElement("a");
    a.id = "a11ySkip";
    a.className = "a11y-skip";
    a.href = "#" + main.id;
    a.textContent = "Перейти к содержимому";
    a.addEventListener("click", function (e) {
      e.preventDefault();
      var v = document.querySelector("main .view:not(.hidden)") || main;
      var h = v.querySelector("h1,h2") || v;
      if (!h.hasAttribute("tabindex")) h.setAttribute("tabindex", "-1");
      h.focus();
      h.scrollIntoView({ block: "start" });
    });
    document.body.insertBefore(a, document.body.firstChild);
  }

  /* ---------- aria на лету ---------- */
  var LABELS = { "#mobileMenuButton": "Открыть меню" };
  function nameOf(el) {
    return (el.getAttribute("aria-label") || el.getAttribute("aria-labelledby") || (el.textContent || "").trim() || el.getAttribute("title") || "");
  }
  function enhance() {
    // img без alt: декоративные (внутри карточек с подписью) -> alt=""; прочие берём из title/aria
    document.querySelectorAll("img:not([alt])").forEach(function (img) {
      var host = img.closest("[aria-label]");
      img.setAttribute("alt", host && host.getAttribute("role") !== "button" ? "" : (img.title || ""));
    });
    // icon-button без имени
    document.querySelectorAll("button, [role='button'], a[href]").forEach(function (b) {
      if (b.hasAttribute("data-a11y-ok")) return;
      if (nameOf(b)) { b.setAttribute("data-a11y-ok", ""); return; }
      var svgTitle = b.querySelector("svg title");
      var lbl = (svgTitle && svgTitle.textContent) || b.getAttribute("data-label") || b.getAttribute("data-tooltip");
      if (!lbl) {
        var id = b.id;
        if (/close|Close/.test(id) || b.matches(".modal-close,.close")) lbl = "Закрыть";
      }
      if (lbl) { b.setAttribute("aria-label", lbl); b.setAttribute("data-a11y-ok", ""); }
    });
    // decorative svg
    document.querySelectorAll("button svg:not([aria-hidden]):not([role])").forEach(function (s) { s.setAttribute("aria-hidden", "true"); s.setAttribute("focusable", "false"); });
    // what-if (app.js): is-active -> aria-pressed
    document.querySelectorAll(".whatif-option").forEach(function (b) {
      b.setAttribute("aria-pressed", b.classList.contains("is-active") ? "true" : "false");
    });
    // Пилюли выбора (настроение, темп, «быстрый старт»): aria-pressed по is-active
    document.querySelectorAll("#tripPlanner .rating-pill").forEach(function (b) {
      b.setAttribute("aria-pressed", b.classList.contains("is-active") ? "true" : "false");
    });
    document.querySelectorAll(".nav-link").forEach(function (b) {
      if (b.classList.contains("is-active")) b.setAttribute("aria-current", "page"); else b.removeAttribute("aria-current");
    });
    document.querySelectorAll(".segment,.swatch").forEach(function (b) {
      b.setAttribute("aria-pressed", b.classList.contains("is-active") ? "true" : "false");
    });
    var fa = document.getElementById("filtersAside");
    document.querySelectorAll(".filter-toggle").forEach(function (b) {
      b.setAttribute("aria-expanded", fa && fa.classList.contains("is-open") ? "true" : "false");
      if (fa && !b.hasAttribute("aria-controls")) b.setAttribute("aria-controls", "filtersAside");
    });
    // Прочие тумблеры, у которых есть is-active, но нет aria-pressed
    document.querySelectorAll("button.chip.is-active:not([aria-pressed]),button.filter-chip.is-active:not([aria-pressed])").forEach(function (b) { b.setAttribute("aria-pressed", "true"); });
    // live-регион результата what-if
    var res = document.getElementById("whatIfResult");
    if (res && !res.hasAttribute("aria-live")) res.setAttribute("aria-live", "polite");
    // скрытые от AT свёрнутые панели не должны ловить фокус
    document.querySelectorAll('[aria-hidden="true"]').forEach(function (n) {
      if (n.matches(".mobile-menu,.assistant-drawer,.modal-layer")) n.toggleAttribute("inert", true);
    });
    document.querySelectorAll('.mobile-menu[aria-hidden="false"],.assistant-drawer[aria-hidden="false"],.modal-layer[aria-hidden="false"]').forEach(function (n) { n.removeAttribute("inert"); });
  }

  /* ---------- Старт ---------- */
  function start() {
    ensureSkip();
    var v0 = document.querySelector("main .view:not(.hidden)");
    lastView = v0 ? v0.id : null;
    scan();
    enhance();
    new MutationObserver(schedule).observe(document.body, { subtree: true, childList: true, attributes: true, attributeFilter: ["class", "hidden", "aria-hidden"] });
  }
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", start);
  else start();
})();
