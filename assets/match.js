/* match: «Паспорт совпадения» (N3) и What-if симулятор (N4). Только реальные числа из API. */
(function () {
  "use strict";
  if (window.__matchModule) return;
  window.__matchModule = true;

  const ODIN = (window.ODIN = window.ODIN || {});

  /* ---------- helpers ---------- */
  const esc = (v) => (ODIN.escapeHTML ? ODIN.escapeHTML(v) : String(v == null ? "" : v).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c])));
  const money = (v) => (ODIN.money ? ODIN.money(v) : `${Number(v || 0).toLocaleString("ru-RU")} ₽`);
  const plural = (n, a, b, c) => {
    if (ODIN.plural) return ODIN.plural(n, a, b, c);
    const x = Math.abs(n) % 100, y = x % 10;
    return x > 10 && x < 20 ? c : y > 1 && y < 5 ? b : y === 1 ? a : c;
  };
  const reduced = () => !!(window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches);
  const MINUS = "−";
  const signed = (n) => (n > 0 ? "+" : n < 0 ? MINUS : "") + Math.abs(n);
  const signedMoney = (n) => (n > 0 ? "+" : n < 0 ? MINUS : "") + money(Math.abs(n));

  // Плавный пересчёт числа. Текущее отображаемое значение хранится в el._cur.
  function tween(el, to, ms, fmt) {
    fmt = fmt || String;
    cancelAnimationFrame(el._raf);
    const from = typeof el._cur === "number" ? el._cur : 0;
    if (reduced() || from === to || ms <= 0) {
      el._cur = to;
      el.textContent = fmt(to);
      return;
    }
    const t0 = performance.now();
    const step = (now) => {
      const p = Math.min(1, (now - t0) / ms);
      const eased = 1 - Math.pow(1 - p, 3);
      el._cur = p >= 1 ? to : Math.round(from + (to - from) * eased);
      el.textContent = fmt(el._cur);
      if (p < 1) el._raf = requestAnimationFrame(step);
    };
    el._raf = requestAnimationFrame(step);
  }

  function whenVisible(el, cb) {
    if (!("IntersectionObserver" in window)) { cb(); return; }
    const io = new IntersectionObserver((entries) => {
      if (entries.some((e) => e.isIntersecting)) { io.disconnect(); cb(); }
    }, { threshold: 0.15 });
    io.observe(el);
  }

  /* ---------- N3: renderMatch ---------- */
  function topLists(m) {
    const factors = Array.isArray(m.factors) ? m.factors : [];
    const fmt = (f) => (f.note ? `${f.label} — ${f.note}` : f.label);
    let pros = Array.isArray(m.pros) ? m.pros.filter(Boolean) : [];
    let cons = Array.isArray(m.cons) ? m.cons.filter(Boolean) : [];
    if (!pros.length) pros = factors.filter((f) => f.tone === "plus").sort((a, b) => b.points - a.points).map(fmt);
    if (!cons.length) cons = factors.filter((f) => f.tone === "minus").map(fmt);
    return { pros: pros.slice(0, 3), cons: cons.slice(0, 2) };
  }

  function renderMatch(container, m, opts) {
    opts = opts || {};
    if (!container || !m || !Array.isArray(m.factors) || !m.factors.length) return null;
    const compact = !!opts.compact;
    const base = Number(m.score) || 0;
    const factors = m.factors.map((f) => ({ ...f, points: Number(f.points) || 0 }));
    const sum = factors.reduce((s, f) => s + f.points, 0);
    const denom = base > 0 ? base : sum || 1; // ширина = points / score
    const share = (p) => Math.max(0, Math.min(100, (p / denom) * 100));
    const toneOf = (f) => (f.tone === "plus" || f.tone === "minus" ? f.tone : "neutral");
    const { pros, cons } = topLists(m);
    const contributing = factors.filter((f) => f.points > 0);
    const eq = contributing.map((f) => f.points).join(" + ");

    const list = (title, items, cls) => items.length ? `
      <div class="match-list match-list-${cls}">
        <h3>${title}</h3>
        <ul>${items.map((t) => `<li>${esc(t)}</li>`).join("")}</ul>
      </div>` : "";

    container.innerHTML = `
      <div class="match-card${compact ? " is-compact" : ""}">
        <div class="match-top">
          <div class="match-score" role="img" aria-label="Совпадение ${base}%">
            <span class="match-score-num" data-match-num>0</span><span class="match-score-pct">%</span>
          </div>
          <div class="match-head">
            ${compact ? "" : `<span class="section-kicker">Почему этот отель</span>`}
            <h2 class="match-title">${compact ? "Совпадение" : "Паспорт совпадения"}</h2>
            <p class="match-verdict">${esc(m.verdict || "")}</p>
            <p class="match-scenario" data-match-scenario hidden></p>
          </div>
        </div>
        <div class="match-stack-wrap">
          <div class="match-stack" role="img" aria-label="Вклады факторов: ${esc(eq)} = ${sum}">
            ${contributing.map((f) => `<i class="match-seg is-${toneOf(f)}" data-i="${factors.indexOf(f)}" style="--w:${share(f.points).toFixed(2)}%" title="${esc(f.label)}: ${f.points}"></i>`).join("")}
          </div>
          <div class="match-eq">${esc(eq)} = <strong>${sum}</strong></div>
        </div>
        ${compact ? "" : `<ul class="match-factors-list">
          ${factors.map((f, i) => `
            <li class="match-fx is-${toneOf(f)}${f.points ? "" : " is-zero"}" data-i="${i}" tabindex="0">
              <div class="match-fx-row">
                <span class="match-fx-label">${esc(f.label)}</span>
                <strong class="match-fx-pts">${f.points ? "+" : ""}${f.points}</strong>
              </div>
              <div class="match-bar"><i style="--w:${share(f.points).toFixed(2)}%"></i></div>
              ${f.note ? `<small>${esc(f.note)}</small>` : ""}
            </li>`).join("")}
        </ul>`}
        ${pros.length || cons.length ? `<div class="match-lists">
          ${list("За", pros, "pro")}${list("Против", cons, "con")}
        </div>` : ""}
        ${compact ? "" : `<p class="match-note">Вклады факторов складываются в итоговый балл. Никакой магии — только ваши параметры и данные отеля.</p>`}
      </div>`;

    const root = container.querySelector(".match-card");
    const num = root.querySelector("[data-match-num]");
    const scenario = root.querySelector("[data-match-scenario]");
    num.textContent = reduced() ? String(base) : "0";
    num._cur = reduced() ? base : 0;

    // Подсветка: фактор <-> сегмент стопки
    const hl = (i, on) => root.querySelectorAll(`[data-i="${i}"]`).forEach((n) => n.classList.toggle("is-hl", on));
    root.querySelectorAll(".match-fx, .match-seg").forEach((n) => {
      const i = n.dataset.i;
      n.addEventListener("mouseenter", () => hl(i, true));
      n.addEventListener("mouseleave", () => hl(i, false));
      n.addEventListener("focus", () => hl(i, true));
      n.addEventListener("blur", () => hl(i, false));
    });

    // Появление: count-up ≤600 мс и рост полос в зоне видимости
    const reveal = () => {
      root.classList.add("is-in");
      tween(num, base, 600);
    };
    if (reduced()) root.classList.add("is-in"), (num.textContent = String(base));
    else whenVisible(root, reveal);

    return {
      el: root,
      // Сценарий what-if: меняем только число; вклады факторов API для сценария не отдаёт.
      setScore(score, label) {
        const s = Number(score);
        tween(num, s, 250);
        num.closest(".match-score").setAttribute("aria-label", `Совпадение ${s}%`);
        if (label && s !== base) {
          scenario.hidden = false;
          scenario.textContent = `Сценарий «${label}»: ${s}% вместо ${base}%. Вклады ниже показаны для текущих параметров.`;
        } else if (label) {
          scenario.hidden = false;
          scenario.textContent = `Сценарий «${label}»: балл не меняется.`;
        } else {
          scenario.hidden = true;
          scenario.textContent = "";
        }
      },
    };
  }

  /* ---------- N4: renderWhatIf ---------- */
  function scoreBadge(delta) {
    if (delta === 0) return { cls: "is-zero", text: "балл не меняется" };
    const a = Math.abs(delta);
    return {
      cls: delta > 0 ? "is-up" : "is-down",
      text: `${delta > 0 ? "↑" : "↓"} ${signed(delta)} ${plural(a, "балл", "балла", "баллов")}`,
    };
  }
  // Для денег рост = хуже (красный), снижение = лучше (зелёный).
  function moneyBadge(delta) {
    if (!delta) return { cls: "is-zero", text: "без изменений" };
    return { cls: delta > 0 ? "is-cost" : "is-save", text: `${delta > 0 ? "↑" : "↓"} ${signedMoney(delta)}` };
  }

  function renderWhatIf(container, data, onChange) {
    if (!container || !data) return null;
    const variants = Array.isArray(data.variants) ? data.variants : [];
    if (!variants.length) { container.textContent = ""; return null; }
    const base = { score: Number(data.score) || 0, price: Number(data.price) || 0, total: Number(data.total) || 0 };
    let active = null;

    container.innerHTML = `
      <div class="wi" data-wi>
        <div class="wi-head">
          <strong>Что если?</strong>
          <span>Сравниваем по одному: включён один сценарий, он считается от базовых параметров.</span>
        </div>
        <div class="wi-switches" role="group" aria-label="Сценарии «что если»">
          ${variants.map((v) => {
            const sb = scoreBadge(Number(v.delta) || 0);
            const td = Number(v.totalDelta) || 0;
            return `
            <button type="button" class="wi-switch" role="switch" aria-checked="false" data-key="${esc(v.key)}">
              <span class="wi-track" aria-hidden="true"><span class="wi-knob"></span></span>
              <span class="wi-text">
                <strong>${esc(v.label)}</strong>
                <small>${esc(v.description)}</small>
                <em class="wi-hint">${esc(sb.text)}${td ? ` · ${esc(signedMoney(td))} за поездку` : ""}</em>
              </span>
            </button>`;
          }).join("")}
        </div>
        <div class="wi-summary" aria-live="polite">
          <div class="wi-metric">
            <span class="wi-mlabel">Совпадение</span>
            <span class="wi-val"><b data-wi-score>0</b>%</span>
            <span class="wi-badge" data-wi-score-badge hidden></span>
          </div>
          <div class="wi-metric">
            <span class="wi-mlabel">За ночь</span>
            <span class="wi-val"><b data-wi-price>0</b></span>
            <span class="wi-badge" data-wi-price-badge hidden></span>
          </div>
          <div class="wi-metric">
            <span class="wi-mlabel">За поездку</span>
            <span class="wi-val"><b data-wi-total>0</b></span>
            <span class="wi-badge" data-wi-total-badge hidden></span>
          </div>
        </div>
        <p class="wi-caption" data-wi-caption>Базовые параметры. Нажмите на сценарий, повторное нажатие вернёт базу.</p>
      </div>`;

    const root = container.querySelector("[data-wi]");
    const q = (s) => root.querySelector(s);
    const els = {
      score: q("[data-wi-score]"), price: q("[data-wi-price]"), total: q("[data-wi-total]"),
      sb: q("[data-wi-score-badge]"), pb: q("[data-wi-price-badge]"), tb: q("[data-wi-total-badge]"),
      caption: q("[data-wi-caption]"),
    };
    const switches = Array.from(root.querySelectorAll(".wi-switch"));
    const MS = 240;

    function setBadge(el, b) {
      if (!b) { el.hidden = true; return; }
      el.hidden = false;
      el.className = `wi-badge ${b.cls}`;
      el.textContent = b.text;
      if (!reduced()) {
        el.classList.remove("wi-pop");
        void el.offsetWidth;
        el.classList.add("wi-pop");
      }
    }

    function apply(v) {
      const target = v || { ...base };
      tween(els.score, Number(target.score), MS);
      tween(els.price, Number(target.price), MS, money);
      tween(els.total, Number(target.total), MS, money);
      if (v) {
        setBadge(els.sb, scoreBadge(Number(v.delta) || 0));
        setBadge(els.pb, moneyBadge(Number(v.priceDelta) || 0));
        setBadge(els.tb, moneyBadge(Number(v.totalDelta) || 0));
        els.caption.textContent = `Включено: «${v.label}». Остальные сценарии выключены, чтобы не складывать независимые поправки.`;
      } else {
        setBadge(els.sb, null); setBadge(els.pb, null); setBadge(els.tb, null);
        els.caption.textContent = "Базовые параметры. Нажмите на сценарий, повторное нажатие вернёт базу.";
      }
    }

    function select(key) {
      active = key && active !== key ? key : null;
      const v = variants.find((x) => x.key === active) || null;
      switches.forEach((b) => b.setAttribute("aria-checked", String(b.dataset.key === active)));
      apply(v);
      if (typeof onChange === "function") {
        const cur = v || base;
        onChange({ key: active, variant: v, base, score: Number(cur.score), price: Number(cur.price), total: Number(cur.total) });
      }
    }

    switches.forEach((b) => b.addEventListener("click", () => select(b.dataset.key)));

    // Начальные значения: показываем сразу, без анимации «из нуля»
    els.score._cur = base.score; els.score.textContent = String(base.score);
    els.price._cur = base.price; els.price.textContent = money(base.price);
    els.total._cur = base.total; els.total.textContent = money(base.total);

    return { el: root, select, reset: () => active && select(active) };
  }

  /* ---------- Автовстраивание в карточку отеля ---------- */
  function nightsOf(a, b) {
    const d = Math.round((new Date(b) - new Date(a)) / 864e5);
    return Number.isFinite(d) && d > 0 ? d : 1;
  }

  async function loadWhatIf(hotel) {
    const st = ODIN.state || {};
    const s = st.search || {};
    return ODIN.api("/api/match/what-if", {
      method: "POST",
      body: {
        slug: hotel.slug,
        guests: Number(hotel.guests || s.guests || 2),
        nights: nightsOf(hotel.checkIn || s.checkIn, hotel.checkOut || s.checkOut),
        budget: s.maxPrice || 0,
        mood: s.mood || "",
        tags: s.tags || [],
      },
    });
  }

  function enhance(section) {
    if (section.dataset.matchEnhanced) return;
    const hotel = ODIN.state && ODIN.state.activeHotel;
    const detail = hotel && hotel.matchDetail;
    if (!detail || !Array.isArray(detail.factors) || !detail.factors.length) return;
    const box = section.querySelector("#whatIfBox");
    if (box && box.dataset.whatifSlug && box.dataset.whatifSlug !== hotel.slug) return;
    section.dataset.matchEnhanced = "1";
    section.classList.add("match-enhanced");

    if (box) box.remove();
    section.textContent = "";
    const mount = document.createElement("div");
    mount.className = "match-mount";
    section.appendChild(mount);
    const ctrl = renderMatch(mount, detail);
    if (!ctrl) return;
    if (!box) return;
    ctrl.el.appendChild(box); // тот же элемент, не пересоздаём

    let cached = null;
    let wi = null;
    const mountWhatIf = () => {
      if (!cached || !box.isConnected) return;
      wi = renderWhatIf(box, cached, (e) => ctrl.setScore(e.score, e.variant ? e.variant.label : ""));
      if (ctrl.el.classList.contains("is-in")) ctrl.setScore(cached.score, "");
    };
    // app.js может дорисовать свой renderWhatIf позже: возвращаем наш UI
    new MutationObserver(() => {
      if (cached && box.isConnected && !box.querySelector("[data-wi]")) mountWhatIf();
    }).observe(box, { childList: true });

    loadWhatIf(hotel).then((data) => {
      if (!box.isConnected || !data || !Array.isArray(data.variants)) return;
      cached = data;
      mountWhatIf();
    }).catch(() => { /* оставляем то, что нарисует app.js */ });
  }

  function scan() {
    const host = document.getElementById("hotelContent");
    if (!host) return;
    host.querySelectorAll(".match-passport:not([data-match-enhanced])").forEach(enhance);
  }

  function boot() {
    const host = document.getElementById("hotelContent");
    if (!host) return;
    new MutationObserver(scan).observe(host, { childList: true, subtree: true });
    scan();
  }

  ODIN.renderMatch = renderMatch;
  ODIN.renderWhatIf = renderWhatIf;
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", boot);
  else boot();
})();
