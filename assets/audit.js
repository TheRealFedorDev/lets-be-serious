/* Карточка «Проверка цены» для страницы отеля.
 * Подключается после app.js (defer). Использует только мост window.ODIN
 * и не меняет логику app.js: карточка вставляется сразу после .price-pie. */
(function () {
  "use strict";

  const ROOM_FIELDS = ["detailCheckIn", "detailCheckOut", "detailRoom", "detailGuests"];
  // Отель, для которого пользователь менял даты/номер/гостей. Нужен, чтобы понять,
  // какой breakdown сейчас показывает диаграмма: исходный или пересчитанный по номеру.
  let changedHotel = null;

  const $ = (id) => document.getElementById(id);

  function num(value) {
    const n = Number(value);
    return Number.isFinite(n) && n > 0 ? n : 0;
  }

  function nightsBetween(from, to) {
    const a = Date.parse(`${from}T00:00:00Z`);
    const b = Date.parse(`${to}T00:00:00Z`);
    if (!Number.isFinite(a) || !Number.isFinite(b)) return 0;
    return Math.round((b - a) / 86400000);
  }

  // Та же формула, что priceBreakdownFor в app.js: налоги 12%, сборы 5%, завтрак 900 на гостя.
  function breakdownForRoom(price, guests) {
    const room = num(price);
    const g = Math.max(1, Number(guests) || 1);
    return {
      room,
      taxes: Math.round(room * 0.12),
      fees: Math.round(room * 0.05),
      breakfast: 900 * g,
    };
  }

  function currentBreakdown(hotel) {
    if (!hotel || changedHotel !== hotel) return hotel && hotel.priceBreakdown;
    const rooms = hotel.rooms || [];
    const select = $("detailRoom");
    const cheapest = rooms.reduce((min, r) => (!min || r.price < min.price ? r : min), null);
    const room = rooms.find((r) => select && String(r.id) === String(select.value)) || cheapest;
    if (!room) return hotel.priceBreakdown;
    const guestsInput = $("detailGuests");
    return breakdownForRoom(room.price, Number(guestsInput ? guestsInput.value : hotel.guests) || 2);
  }

  function currentNights(hotel) {
    const inInput = $("detailCheckIn");
    const outInput = $("detailCheckOut");
    const from = (inInput && inInput.value) || hotel.checkIn || (ODIN.state.search || {}).checkIn;
    const to = (outInput && outInput.value) || hotel.checkOut || (ODIN.state.search || {}).checkOut;
    return Math.max(1, nightsBetween(from, to) || 1);
  }

  function currentGuests(hotel) {
    const guestsInput = $("detailGuests");
    return Number((guestsInput && guestsInput.value) || hotel.guests || (ODIN.state.search || {}).guests) || 2;
  }

  /**
   * Рисует карточку «Проверка цены» в container.
   * breakdown: { room, taxes, fees, breakfast } — цена за ОДНУ ночь.
   * nights: число ночей, guests: число гостей.
   */
  function renderPriceAudit(container, breakdown, nights, guests) {
    if (!container) return;
    const O = window.ODIN;
    const n = Math.max(1, Math.floor(Number(nights) || 1));
    const g = Math.max(1, Math.floor(Number(guests) || 1));
    const nightsWord = O.plural(n, "ночь", "ночи", "ночей");
    const guestsWord = O.plural(g, "гость", "гостя", "гостей");

    container.classList.add("audit-card");

    const parts = {
      room: num(breakdown && breakdown.room),
      taxes: num(breakdown && breakdown.taxes),
      fees: num(breakdown && breakdown.fees),
      breakfast: num(breakdown && breakdown.breakfast),
    };
    const perNight = parts.room + parts.taxes + parts.fees + parts.breakfast;

    if (!breakdown || perNight <= 0) {
      container.innerHTML = `
        <h3 class="audit-title">Проверка цены</h3>
        <p class="audit-empty">Цена уточняется</p>`;
      return;
    }

    const stayTotal = perNight * n;
    const rows = [
      ["Номер", parts.room],
      ["Налоги", parts.taxes],
      ["Сборы", parts.fees],
      ["Завтрак", parts.breakfast],
    ];
    const tableRows = rows.map(([label, value]) => `
      <tr>
        <th scope="row">${O.escapeHTML(label)}</th>
        <td>${value > 0 ? O.money(value) : "—"}</td>
        <td>${value > 0 ? O.money(value * n) : "—"}</td>
      </tr>`).join("");

    const included = [];
    const notIncluded = [];
    if (parts.breakfast > 0) included.push("Завтрак");
    else notIncluded.push({ mark: "✗", text: "Завтрак не включён" });
    if (parts.taxes > 0) included.push("Налоги");
    else notIncluded.push({ mark: "✗", text: "Налоги не включены в цену" });
    if (parts.fees > 0) included.push("Сборы уже в цене");
    else notIncluded.push({ mark: "⚠", text: "Курортный / туристический сбор может взиматься на месте" });

    const listMarkup = (items, cls) => items.map((item) => {
      const mark = typeof item === "string" ? "✓" : item.mark;
      const text = typeof item === "string" ? item : item.text;
      const kind = mark === "⚠" ? "warn" : cls;
      return `<li class="audit-item audit-item-${kind}"><span class="audit-mark" aria-hidden="true">${mark}</span><span>${O.escapeHTML(text)}</span></li>`;
    }).join("");

    container.innerHTML = `
      <div class="audit-head">
        <h3 class="audit-title">Проверка цены</h3>
        <span class="audit-meta">${n} ${nightsWord} · ${g} ${guestsWord}</span>
      </div>

      <div class="audit-tiles">
        <div class="audit-tile">
          <span class="audit-cap">Цена за 1 ночь</span>
          <strong class="audit-val">${O.money(perNight)}</strong>
        </div>
        <div class="audit-tile">
          <span class="audit-cap">Итого за ${n} ${nightsWord}</span>
          <strong class="audit-val">${O.money(stayTotal)}</strong>
        </div>
      </div>

      <div class="audit-table-wrap">
        <table class="audit-table">
          <thead>
            <tr><th scope="col">Статья</th><th scope="col">за ночь</th><th scope="col">за ${n} ${nightsWord}</th></tr>
          </thead>
          <tbody>${tableRows}</tbody>
          <tfoot>
            <tr><th scope="row">Всего</th><td>${O.money(perNight)}</td><td>${O.money(stayTotal)}</td></tr>
          </tfoot>
        </table>
      </div>

      <div class="audit-lists">
        <div class="audit-list-block">
          <h4 class="audit-list-title">Что включено</h4>
          <ul class="audit-list">${listMarkup(included, "yes")}</ul>
        </div>
        ${notIncluded.length ? `
        <div class="audit-list-block">
          <h4 class="audit-list-title">Не включено</h4>
          <ul class="audit-list">${listMarkup(notIncluded, "no")}</ul>
        </div>` : ""}
      </div>

      <div class="audit-final">
        <span>Итого к оплате:</span>
        <strong>${O.money(stayTotal)}</strong>
        <small>за ${n} ${nightsWord}, ${g} ${guestsWord}</small>
      </div>`;
  }

  function mountAfterPies(root) {
    const O = window.ODIN;
    const hotel = O.state && O.state.activeHotel;
    if (!hotel) return;
    root.querySelectorAll(".price-pie").forEach((pie) => {
      if (pie.nextElementSibling && pie.nextElementSibling.classList.contains("audit-card")) return;
      const box = document.createElement("div");
      pie.insertAdjacentElement("afterend", box);
      renderPriceAudit(box, currentBreakdown(hotel), currentNights(hotel), currentGuests(hotel));
    });
  }

  function init() {
    if (!window.ODIN) return;
    const host = $("hotelContent");
    if (!host) return;

    // Capture-фаза: флаг ставится раньше, чем app.js перерисует диаграмму.
    document.addEventListener("change", (event) => {
      if (!event.target || !ROOM_FIELDS.includes(event.target.id)) return;
      changedHotel = window.ODIN.state.activeHotel || null;
    }, true);

    const observer = new MutationObserver(() => mountAfterPies(host));
    observer.observe(host, { childList: true, subtree: true });
    mountAfterPies(host);
  }

  if (window.ODIN) {
    window.ODIN.renderPriceAudit = renderPriceAudit;
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
