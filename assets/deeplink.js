(function () {
  "use strict";
  // Открывает карточку отеля по /?hotel=<slug>. Не зависит от других скриптов:
  // ждёт window.ODIN (до 5 с) и вызывает ODIN.openHotel(slug).
  var POLL_MS = 100;
  var TIMEOUT_MS = 5000;

  function removeParam() {
    try {
      var u = new URL(location.href);
      u.searchParams.delete("hotel");
      var qs = u.searchParams.toString();
      history.replaceState(history.state, "", u.pathname + (qs ? "?" + qs : "") + u.hash);
    } catch (e) { /* адрес остаётся как есть */ }
  }

  try {
    var slug = new URLSearchParams(location.search).get("hotel");
    slug = slug ? slug.trim() : "";
    if (!slug) return;

    var waited = 0;
    var timer = setInterval(function () {
      try {
        if (window.ODIN && typeof window.ODIN.openHotel === "function") {
          clearInterval(timer);
          window.ODIN.openHotel(slug);
          removeParam();
          return;
        }
      } catch (e) {
        clearInterval(timer);
        return;
      }
      waited += POLL_MS;
      if (waited >= TIMEOUT_MS) clearInterval(timer);
    }, POLL_MS);
  } catch (e) { /* без параметра или без ODIN скрипт ничего не делает */ }
})();
