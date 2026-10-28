/* О!дин travel — frontend */
const $ = (selector, root = document) => root.querySelector(selector);
const $$ = (selector, root = document) => Array.from(root.querySelectorAll(selector));

const DEFAULT_ACCENT = "#1F4BDB";
const DEFAULT_MENU = "#FFFFFF";
const STORAGE_THEME = "odin.theme.v2";
const STORAGE_SEARCH = "odin.dates";
const MONTHS_SHORT = ["янв", "фев", "мар", "апр", "мая", "июн", "июл", "авг", "сен", "окт", "ноя", "дек"];

const categoryLabels = {
  coffee: "Кофе",
  food: "Еда",
  walk: "Прогулка",
  culture: "Культура",
  nature: "Природа",
  excursion: "Экскурсия",
  spa: "Спа",
  shop: "Магазин",
};

const categoryIcons = {
  coffee: '<svg viewBox="0 0 24 24"><path d="M4 8h13v6a5 5 0 0 1-5 5H9a5 5 0 0 1-5-5V8Z"/><path d="M17 9h2a2 2 0 0 1 0 4h-2"/><path d="M7 3v2"/><path d="M11 3v2"/></svg>',
  food: '<svg viewBox="0 0 24 24"><path d="M7 3v8"/><path d="M4 3v5a3 3 0 0 0 6 0V3"/><path d="M7 11v10"/><path d="M17 3c-2 2-2 5-2 8h3v10"/></svg>',
  walk: '<svg viewBox="0 0 24 24"><circle cx="13" cy="4" r="1.8"/><path d="M11 21l2-7"/><path d="M9 9l3-2 3 3 3 1"/><path d="M12 13l-3 4"/></svg>',
  culture: '<svg viewBox="0 0 24 24"><path d="M3 10l9-6 9 6"/><path d="M5 10v9h14v-9"/><path d="M9 19v-5h6v5"/></svg>',
  nature: '<svg viewBox="0 0 24 24"><path d="M3 20l6-10 4 6 3-4 5 8H3Z"/><circle cx="17" cy="6" r="2.5"/></svg>',
  excursion: '<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M15.5 8.5l-2 5-5 2 2-5 5-2Z"/></svg>',
  spa: '<svg viewBox="0 0 24 24"><path d="M12 21c-5-3-8-6-8-10 4 0 7 2 8 6 1-4 4-6 8-6 0 4-3 7-8 10Z"/></svg>',
};

const state = {
  destinations: [],
  moods: [],
  deals: [],
  featured: [],
  stats: null,
  tagCatalog: [],
  needCatalog: [],
  homeLoaded: false,
  user: null,
  preferences: { accent: DEFAULT_ACCENT, menuTint: DEFAULT_MENU },
  view: "home",
  previousView: "home",
  search: defaultSearchState(),
  searchResult: null,
  searchLoading: false,
  activeHotel: null,
  hotelLoading: false,
  trips: null,
  favorites: null,
  tripsTab: "upcoming",
  tripsLoading: false,
  authMode: "login",
  authBusy: false,
  pendingAction: null,
  booking: null,
  bookingStep: 1,
  bookingBusy: false,
  assistantOpen: false,
  assistantBusy: false,
  assistantMessages: [],
  assistantHistory: [],
  assistantHotelSlug: null,
  assistantFocusHotel: null,
  assistantStatus: null,
  commandOpen: false,
  commandIndex: 0,
  commandItems: [],
  commandExtra: [],
  commandQuery: "",
  commandAbort: null,
  collapsedFilters: new Set(),
  themeTimer: null,
  initialized: false,
};

const homeView = $("#homeView");
const searchView = $("#searchView");
const hotelView = $("#hotelView");
const tripsView = $("#tripsView");
const searchGrid = $("#searchGrid");
const searchEmpty = $("#searchEmpty");
const searchTitle = $("#searchTitle");
const searchKicker = $("#searchKicker");
const searchSummary = $("#searchSummary");
const resultsCount = $("#resultsCount");
const searchMeta = $("#searchMeta");
const filtersPanel = $("#filtersPanel");
const activeFilters = $("#activeFilters");
const toastRegion = $("#toastRegion");
const accountMenu = $("#accountMenu");
const themePanel = $("#themePanel");
const mobileMenu = $("#mobileMenu");
const assistantDrawer = $("#assistantDrawer");
const assistantScrim = $("#assistantScrim");
const assistantMessages = $("#assistantMessages");
const assistantFocus = $("#assistantFocus");
const assistantSuggestions = $("#assistantSuggestions");
const assistantSuggestionsDefault = assistantSuggestions ? assistantSuggestions.innerHTML : "";
const commandPalette = $("#commandPalette");
const authModal = $("#authModal");
const bookingModal = $("#bookingModal");
const poiModal = $("#poiModal");

/* ---------- Utilities ---------- */
function escapeHTML(value) {
  return String(value ?? "").replace(/[&<>"']/g, (char) => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
  }[char]));
}

function money(value) {
  return `${Number(value || 0).toLocaleString("ru-RU")} ₽`;
}

function fullNightTotal(price, guests) {
  const room = Number(price) || 0;
  const guestsCount = Math.max(1, Number(guests) || 1);
  return room + Math.round(room * 0.12) + Math.round(room * 0.05) + 900 * guestsCount;
}

function fullStayTotal(price, guests, nights) {
  return fullNightTotal(price, guests) * Math.max(1, Number(nights) || 1);
}

function priceBreakdownFor(price, guests) {
  const room = Number(price) || 0;
  const guestsCount = Math.max(1, Number(guests) || 1);
  const taxes = Math.round(room * 0.12);
  const fees = Math.round(room * 0.05);
  const breakfast = 900 * guestsCount;
  return { room, taxes, fees, breakfast, total: room + taxes + fees + breakfast };
}

function plural(value, one, few, many) {
  const mod10 = Math.abs(value) % 10;
  const mod100 = Math.abs(value) % 100;
  if (mod10 === 1 && mod100 !== 11) return one;
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return few;
  return many;
}

function toISODate(date) {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}`;
}

function todayISO() { return toISODate(new Date()); }

function parseISO(iso) {
  const date = new Date(`${iso}T12:00:00`);
  return Number.isNaN(date.getTime()) ? null : date;
}

function addDaysISO(iso, days) {
  const date = parseISO(iso) || new Date();
  date.setDate(date.getDate() + days);
  return toISODate(date);
}

function daysBetween(from, to) {
  const start = parseISO(from);
  const end = parseISO(to);
  if (!start || !end) return 0;
  return Math.max(0, Math.round((end - start) / 86400000));
}

function formatShortDate(iso) {
  const date = parseISO(iso);
  if (!date) return "—";
  return `${date.getDate()} ${MONTHS_SHORT[date.getMonth()]}`;
}

function formatDateRange(from, to) {
  if (!from || !to) return "Даты не выбраны";
  return `${formatShortDate(from)} — ${formatShortDate(to)}`;
}

function formatDateLong(iso) {
  const date = parseISO(iso);
  if (!date) return "—";
  return new Intl.DateTimeFormat("ru-RU", { day: "numeric", month: "long", year: "numeric" }).format(date);
}

function nightsBetween(from, to) { return daysBetween(from, to); }

function defaultDates() {
  const start = addDaysISO(todayISO(), 14);
  return { checkIn: start, checkOut: addDaysISO(start, 3) };
}

function defaultSearchState() {
  const dates = defaultDates();
  return {
    q: "", destination: "", destinationName: "",
    checkIn: dates.checkIn, checkOut: dates.checkOut, guests: 2,
    mood: "", tags: [], insured: false, guides: false, parking: false,
    pool: false, soundproof: false, discount: false, week: false,
    lat: null, lng: null, radius: 30, minPrice: 0, maxPrice: 0,
    minRating: 0, sort: "",
    needs: {}, needRadius: 10,
  };
}

/* ---------- Needs: «что важно рядом» ---------- */

const NEED_PRESETS = [
  { id: "family", label: "С детьми", radius: 10, needs: { beach: 1, park: 2, kids: 2, kitchen: 1 } },
  { id: "nocar", label: "Без машины", radius: 7, needs: { metro: 2, services: 1, food: 1 } },
  { id: "work", label: "Работаю в поездке", radius: 10, needs: { food: 2, coworking: 2, early: 1, laundry: 1 } },
  { id: "beach", label: "Пляжный отдых", radius: 7, needs: { beach: 2, food: 1, pool: 1 } },
  { id: "city", label: "Гулять по центру", radius: 15, needs: { attractions: 2, food: 1, park: 1 } },
];

function needDefByKey(key) {
  return state.needCatalog.find((need) => need.key === key) || null;
}

function needLabel(key) {
  const def = needDefByKey(key);
  return def ? def.label : key;
}

function activeNeeds() {
  return Object.entries(state.search.needs || {})
    .filter(([, level]) => Number(level) > 0)
    .map(([key, level]) => ({ key, level: Number(level), def: needDefByKey(key) }))
    .filter((need) => need.def);
}

function ratingWord(rating) {
  const value = Number(rating || 0);
  if (value >= 9.5) return "Великолепно";
  if (value >= 9) return "Превосходно";
  if (value >= 8.5) return "Очень хорошо";
  if (value >= 8) return "Хорошо";
  return "Неплохо";
}

function ratingClass(rating) {
  const value = Number(rating || 0);
  if (value >= 9) return "is-good";
  if (value >= 8) return "is-mid";
  return "";
}

function tagLabel(slug) {
  const found = state.tagCatalog.find((tag) => tag.slug === slug);
  return found ? found.label : slug;
}

function destinationName(slug) {
  const found = state.destinations.find((item) => item.slug === slug);
  return found ? found.city : "";
}

function initials(name) {
  const parts = String(name || "").trim().split(/\s+/).filter(Boolean);
  if (!parts.length) return "Г";
  if (parts.length === 1) return parts[0].slice(0, 1).toUpperCase();
  return `${parts[0][0]}${parts[parts.length - 1][0]}`.toUpperCase();
}

function firstName(name) {
  return String(name || "").trim().split(/\s+/)[0] || "Гость";
}

function showToast(message, tone = "success") {
  const toast = document.createElement("div");
  toast.className = `toast${tone === "error" ? " is-error" : tone === "success" ? " is-success" : ""}`;
  toast.setAttribute("role", "status");
  const icon = tone === "error"
    ? '<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M12 8v5"/><path d="M12 16.5h.01"/></svg>'
    : tone === "success"
      ? '<svg viewBox="0 0 24 24"><path d="M20 6L9 17l-5-5"/></svg>'
      : '<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M12 11v5"/><path d="M12 8h.01"/></svg>';
  toast.innerHTML = `${icon}<span>${escapeHTML(message)}</span>`;
  toastRegion.appendChild(toast);
  window.setTimeout(() => {
    toast.style.opacity = "0";
    toast.style.transform = "translateY(6px)";
    window.setTimeout(() => toast.remove(), 220);
  }, 3600);
}

function setButtonLoading(button, loading, label = "Подождите…") {
  if (!button) return;
  if (loading) {
    if (!button.dataset.originalHtml) button.dataset.originalHtml = button.innerHTML;
    button.disabled = true;
    button.innerHTML = `<span class="button-spinner" aria-hidden="true"></span><span>${escapeHTML(label)}</span>`;
    return;
  }
  if (button.dataset.originalHtml) {
    button.innerHTML = button.dataset.originalHtml;
    delete button.dataset.originalHtml;
  }
  button.disabled = false;
}

async function api(path, options = {}) {
  const request = { method: options.method || "GET", credentials: "same-origin", headers: {} };
  if (options.body !== undefined) {
    request.headers["Content-Type"] = "application/json";
    request.body = JSON.stringify(options.body);
  }
  let response;
  try {
    response = await fetch(path, request);
  } catch (error) {
    const network = new Error("Нет связи с сервером. Проверьте, что приложение запущено.");
    network.status = 0;
    throw network;
  }
  let data = null;
  if (response.status !== 204) {
    const type = response.headers.get("content-type") || "";
    if (type.includes("application/json")) data = await response.json().catch(() => null);
  }
  if (!response.ok) {
    const error = new Error((data && data.error) || "Не удалось выполнить запрос");
    error.status = response.status;
    throw error;
  }
  return data;
}

function syncBodyLock() {
  const locked = Boolean(
    document.querySelector(".modal-layer.is-open") ||
    assistantDrawer.classList.contains("is-open") ||
    mobileMenu.classList.contains("is-open") ||
    document.querySelector(".filters.is-open")
  );
  document.body.classList.toggle("is-locked", locked);
}

function closeOnEscape() {
  if (commandPalette.classList.contains("is-open")) return closeCommand();
  if (authModal.classList.contains("is-open")) return closeAuth();
  if (bookingModal.classList.contains("is-open")) return closeBooking();
  if (poiModal.classList.contains("is-open")) return closePOI();
  if (themePanel.classList.contains("is-open")) return closeThemePanel();
  if (accountMenu.classList.contains("is-open")) return closeAccountMenu();
  if (assistantDrawer.classList.contains("is-open")) return closeAssistant();
  if (document.querySelector(".filters.is-open")) return closeFilters();
}
document.addEventListener("keydown", (event) => {
  if (event.key === "Escape") closeOnEscape();
});

/* ---------- Theme ---------- */
function normalizeHex(value, fallback) {
  const normalized = String(value || "").trim();
  return /^#[0-9a-f]{6}$/i.test(normalized) ? normalized.toUpperCase() : fallback;
}

function hexToRgb(hex) {
  const value = hex.replace("#", "");
  return { r: parseInt(value.slice(0, 2), 16), g: parseInt(value.slice(2, 4), 16), b: parseInt(value.slice(4, 6), 16) };
}

function rgbToHex({ r, g, b }) {
  return `#${[r, g, b].map((channel) => Math.max(0, Math.min(255, Math.round(channel))).toString(16).padStart(2, "0")).join("")}`.toUpperCase();
}

function mixHex(first, second, weight) {
  const a = hexToRgb(first);
  const b = hexToRgb(second);
  return rgbToHex({
    r: a.r * (1 - weight) + b.r * weight,
    g: a.g * (1 - weight) + b.g * weight,
    b: a.b * (1 - weight) + b.b * weight,
  });
}

function applyTheme(theme, options = {}) {
  const accent = normalizeHex(theme && theme.accent, DEFAULT_ACCENT);
  const menuTint = normalizeHex(theme && theme.menuTint, DEFAULT_MENU);
  state.preferences = { accent, menuTint };
  const root = document.documentElement;
  root.style.setProperty("--accent", accent);
  root.style.setProperty("--menu-tint", menuTint);
  const themeMeta = $('meta[name="theme-color"]');
  if (themeMeta) themeMeta.setAttribute("content", menuTint);

  const accentCustom = $("#accentCustom");
  const menuCustom = $("#menuCustom");
  if (accentCustom) accentCustom.value = accent;
  if (menuCustom) menuCustom.value = menuTint;
  $$(".swatch[data-accent]").forEach((swatch) => swatch.classList.toggle("is-active", swatch.dataset.accent.toUpperCase() === accent));
  $$(".swatch[data-menu]").forEach((swatch) => swatch.classList.toggle("is-active", swatch.dataset.menu.toUpperCase() === menuTint));

  if (options.persist !== false) localStorage.setItem(STORAGE_THEME, JSON.stringify({ accent, menuTint }));
  if (options.saveServer) scheduleThemeSave();
}

function scheduleThemeSave() {
  const status = $("#themeStatus");
  if (!state.user) {
    if (status) status.textContent = "Сохранено в этом браузере";
    return;
  }
  if (status) status.textContent = "Сохраняем…";
  window.clearTimeout(state.themeTimer);
  state.themeTimer = window.setTimeout(async () => {
    try {
      await api("/api/preferences", { method: "PUT", body: state.preferences });
      if (status) status.textContent = "Сохранено в аккаунте";
    } catch (error) {
      if (status) status.textContent = "Не удалось сохранить";
    }
  }, 400);
}

function loadStoredTheme() {
  try {
    const stored = JSON.parse(localStorage.getItem(STORAGE_THEME) || "null");
    if (stored && stored.accent && stored.menuTint) return applyTheme(stored, { persist: false });
  } catch (error) { /* ignore */ }
  applyTheme({ accent: DEFAULT_ACCENT, menuTint: DEFAULT_MENU }, { persist: false });
}

function openThemePanel() {
  closeAccountMenu();
  themePanel.classList.add("is-open");
  themePanel.setAttribute("aria-hidden", "false");
  $("#themeButton").setAttribute("aria-expanded", "true");
}
function closeThemePanel() {
  themePanel.classList.remove("is-open");
  themePanel.setAttribute("aria-hidden", "true");
  const button = $("#themeButton");
  if (button) button.setAttribute("aria-expanded", "false");
}

/* ---------- Dates / persisted search ---------- */
function loadStoredDates() {
  const dates = defaultDates();
  state.search.checkIn = dates.checkIn;
  state.search.checkOut = dates.checkOut;
  try {
    const stored = JSON.parse(localStorage.getItem(STORAGE_SEARCH) || "null");
    if (stored && stored.checkIn && stored.checkOut && stored.checkIn >= todayISO() && stored.checkOut > stored.checkIn) {
      state.search.checkIn = stored.checkIn;
      state.search.checkOut = stored.checkOut;
      if (stored.guests) state.search.guests = Number(stored.guests);
    }
  } catch (error) { /* ignore */ }
  syncSearchInputs();
}

function persistDates() {
  localStorage.setItem(STORAGE_SEARCH, JSON.stringify({
    checkIn: state.search.checkIn,
    checkOut: state.search.checkOut,
    guests: state.search.guests,
  }));
}

function syncSearchInputs() {
  ["searchCheckIn", "resultsCheckIn", "bookingCheckIn"].forEach((id) => {
    const input = document.getElementById(id);
    if (input) { input.value = state.search.checkIn; input.min = todayISO(); }
  });
  ["searchCheckOut", "resultsCheckOut", "bookingCheckOut"].forEach((id) => {
    const input = document.getElementById(id);
    if (input) { input.value = state.search.checkOut; input.min = addDaysISO(state.search.checkIn, 1); }
  });
  const heroGuests = $("#searchGuests");
  const resultsGuests = $("#resultsGuests");
  if (heroGuests) heroGuests.value = String(state.search.guests);
  if (resultsGuests) resultsGuests.value = String(state.search.guests);
  const dest = $("#resultsDestination");
  if (dest && state.search.destinationName) dest.value = state.search.destinationName;
}

function applyDateInput(source, checkInValue, checkOutValue) {
  let checkIn = checkInValue || state.search.checkIn;
  let checkOut = checkOutValue || state.search.checkOut;
  if (!checkIn || checkIn < todayISO()) checkIn = todayISO();
  if (state.search.week) {
    if (source === "checkOut") {
      const candidate = addDaysISO(checkOut, -7);
      checkIn = candidate > todayISO() ? candidate : checkIn;
      checkOut = addDaysISO(checkIn, 7);
    } else {
      checkOut = addDaysISO(checkIn, 7);
    }
  } else if (!checkOut || checkOut <= checkIn) {
    checkOut = addDaysISO(checkIn, 2);
  }
  state.search.checkIn = checkIn;
  state.search.checkOut = checkOut;
  syncSearchInputs();
  persistDates();
}

/* ---------- View / nav ---------- */
function setActiveNav(name) {
  $$(".nav-link").forEach((link) => link.classList.toggle("is-active", link.dataset.nav === name));
}

function showView(name, options = {}) {
  [homeView, searchView, hotelView, tripsView].forEach((view) => view && view.classList.add("hidden"));
  const target = document.getElementById(`${name}View`);
  if (target) target.classList.remove("hidden");
  state.view = name;
  if (name === "home") setActiveNav("search");
  else if (name === "search") setActiveNav("search");
  else if (name === "hotel") setActiveNav("search");
  else if (name === "trips") setActiveNav("trips");
  if (options.scroll !== false) window.scrollTo({ top: 0, behavior: "smooth" });
}

function navigate(name) {
  closeAccountMenu();
  closeThemePanel();
  closeMobileMenu();
  if (name === "assistant") return openAssistant();
  if (name === "trips") return showTrips("upcoming");
  if (name === "moods") {
    showView("home");
    setActiveNav("moods");
    window.setTimeout(() => $("#moodsSection")?.scrollIntoView({ behavior: "smooth", block: "start" }), 60);
    return;
  }
  if (name === "collections") {
    showView("home");
    setActiveNav("collections");
    window.setTimeout(() => $("#dealsSection")?.scrollIntoView({ behavior: "smooth", block: "start" }), 60);
    return;
  }
  if (name === "search") {
    state.search = { ...defaultSearchState(), checkIn: state.search.checkIn, checkOut: state.search.checkOut, guests: state.search.guests };
    syncSearchInputs();
    showView("search");
    performSearch();
    return;
  }
  showView("home");
}

function closeAccountMenu() {
  accountMenu.classList.remove("is-open");
  const button = $("#accountButton");
  if (button) button.setAttribute("aria-expanded", "false");
}

function closeMobileMenu() {
  mobileMenu.classList.remove("is-open");
  mobileMenu.setAttribute("aria-hidden", "true");
  syncBodyLock();
}

function openMobileMenu() {
  mobileMenu.classList.add("is-open");
  mobileMenu.setAttribute("aria-hidden", "false");
  closeAccountMenu();
  syncBodyLock();
}

/* ---------- Home ---------- */
async function loadHome() {
  try {
    const [home, catalog] = await Promise.all([
      api("/api/home"),
      api("/api/moods").catch(() => null),
    ]);
    state.destinations = home.destinations || [];
    state.featured = home.featured || [];
    state.deals = home.deals || [];
    state.stats = home.stats || null;
    state.moods = (catalog && catalog.moods && catalog.moods.length ? catalog.moods : home.moods) || [];
    if (catalog && Array.isArray(catalog.tags)) state.tagCatalog = catalog.tags;
    if (catalog && Array.isArray(catalog.needs)) state.needCatalog = catalog.needs;
    renderHome();
    state.homeLoaded = true;
  } catch (error) {
    showToast(error.message, "error");
    $("#destinationsTrack").innerHTML = '<div class="empty-panel"><h3>Не удалось загрузить направления</h3><p>Проверьте, что сервер запущен, и обновите страницу.</p></div>';
    $("#featuredGrid").innerHTML = "";
    $("#dealsGrid").innerHTML = "";
    $("#moodTrack").innerHTML = "";
  }
}

function renderHome() {
  renderMoods();
  renderDeals();
  renderDestinations();
  renderFeatured();
  const stats = state.stats;
  if (stats) {
    const hotels = $("#statHotels");
    const destinations = $("#statDestinations");
    const rating = $("#statRating");
    if (hotels) hotels.textContent = String(stats.hotelCount || state.featured.length);
    if (destinations) destinations.textContent = String(stats.destinationCount || state.destinations.length);
    if (rating) rating.textContent = Number(stats.averageRating || 0).toFixed(1).replace(".", ",");
  }
}

function renderMoods() {
  const track = $("#moodTrack");
  if (!track) return;
  if (!state.moods.length) { track.innerHTML = ""; return; }
  track.innerHTML = state.moods.map((mood) => `
    <button type="button" class="mood-card" data-mood="${escapeHTML(mood.slug)}" aria-label="${escapeHTML(mood.title)}">
      <img src="${escapeHTML(mood.cover || "/assets/images/hotel-03.jpg")}" alt="" loading="lazy">
      <span class="mood-card-body">
        ${mood.count ? `<span class="mood-card-count">${mood.count} ${plural(mood.count, "отель", "отеля", "отелей")}</span>` : ""}
        <strong>${escapeHTML(mood.title)}</strong>
        <span>${escapeHTML(mood.subtitle || mood.description || "")}</span>
      </span>
      <span class="mood-arrow" aria-hidden="true">
        <svg viewBox="0 0 24 24"><path d="M5 12h14"/><path d="M13 6l6 6-6 6"/></svg>
      </span>
    </button>
  `).join("");
}

function renderDeals() {
  const track = $("#dealsGrid");
  if (!track) return;
  if (!state.deals.length) { track.innerHTML = ""; return; }
  track.innerHTML = state.deals.map((hotel) => {
    const discount = Number(hotel.meta && hotel.meta.discount || 0);
    const oldPrice = discount > 0 ? Math.round(hotel.priceFrom / (1 - discount / 100) / 100) * 100 : 0;
    return `
      <article class="deal-card" data-hotel-slug="${escapeHTML(hotel.slug)}" role="link" tabindex="0" aria-label="${escapeHTML(hotel.name)}">
        <div class="deal-media">
          <img src="${escapeHTML(hotel.image)}" alt="${escapeHTML(hotel.name)}" loading="lazy">
          ${discount ? `<span class="deal-discount">−${discount}%</span>` : ""}
        </div>
        <div class="deal-body">
          <strong>${escapeHTML(hotel.name)}</strong>
          <span>${escapeHTML(hotel.city)} · ${ratingWord(hotel.rating)} ${Number(hotel.rating).toFixed(1)}</span>
          <div class="deal-price-row">
            <strong>${money(hotel.priceFrom)}</strong>
            ${oldPrice ? `<s>${money(oldPrice)}</s>` : ""}
          </div>
        </div>
      </article>
    `;
  }).join("");
}

function renderDestinations() {
  const track = $("#destinationsTrack");
  if (!track) return;
  if (!state.destinations.length) { track.innerHTML = ""; return; }
  track.innerHTML = state.destinations.map((destination) => `
    <article class="destination-card" data-destination="${escapeHTML(destination.slug)}" role="link" tabindex="0" aria-label="${escapeHTML(destination.city)}">
      <img src="${escapeHTML(destination.image)}" alt="${escapeHTML(destination.city)}" loading="lazy">
      <div class="destination-body">
        <span class="dest-country">${escapeHTML(destination.country)}</span>
        <strong>${escapeHTML(destination.city)}</strong>
        <span>${escapeHTML(destination.tagline)}</span>
        <span class="destination-price">от ${money(destination.priceFrom)}</span>
      </div>
    </article>
  `).join("");

  const datalist = $("#destinationOptions");
  if (datalist) {
    datalist.innerHTML = state.destinations
      .map((destination) => `<option value="${escapeHTML(destination.city)}"></option>`)
      .join("");
  }
}

function favoriteButton(hotel) {
  return `<button type="button" class="favorite-button${hotel.favorited ? " is-active" : ""}" data-favorite="${hotel.id}" aria-label="${hotel.favorited ? "Убрать из избранного" : "Добавить в избранное"}" aria-pressed="${hotel.favorited ? "true" : "false"}">
    <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M20.8 5.6a5 5 0 0 0-7.1 0L12 7.3l-1.7-1.7a5 5 0 1 0-7.1 7.1L12 21l8.8-8.3a5 5 0 0 0 0-7.1Z"/></svg>
  </button>`;
}

function hotelTagsMarkup(hotel, limit = 3) {
  const slugs = (hotel.meta && hotel.meta.tagSlugs) || [];
  const labels = slugs.slice(0, limit).map(tagLabel);
  const fallback = labels.length ? labels : (hotel.tags || []).slice(0, limit);
  return fallback.map((label) => `<span class="mini-tag">${escapeHTML(label)}</span>`).join("");
}

function featuredCardMarkup(hotel) {
  const discount = Number(hotel.meta && hotel.meta.discount || 0);
  return `
    <article class="hotel-card" data-hotel-slug="${escapeHTML(hotel.slug)}" role="link" tabindex="0" aria-label="${escapeHTML(hotel.name)}">
      <div class="hotel-media">
        <img src="${escapeHTML(hotel.image)}" alt="${escapeHTML(hotel.name)}" loading="lazy">
        <span class="hotel-rating-badge">${Number(hotel.rating).toFixed(1)} <span>${ratingWord(hotel.rating)}</span></span>
        ${favoriteButton(hotel)}
      </div>
      <div class="hotel-body">
        <span class="hotel-city">${escapeHTML(hotel.city)} · ${escapeHTML(hotel.country)}</span>
        <h3 class="hotel-name">${escapeHTML(hotel.name)}</h3>
        <p class="hotel-desc">${escapeHTML(hotel.shortDescription)}</p>
        <div class="hotel-tags">${hotelTagsMarkup(hotel, 3)}</div>
        <div class="hotel-footer">
          <div class="hotel-price">
            ${money(hotel.priceFrom)}
            <span>за ночь${discount ? ` · −${discount}%` : ""}</span>
          </div>
          <span class="text-button">Смотреть
            <svg viewBox="0 0 24 24"><path d="M5 12h14"/><path d="M13 6l6 6-6 6"/></svg>
          </span>
        </div>
      </div>
    </article>
  `;
}

function renderFeatured() {
  const grid = $("#featuredGrid");
  if (!grid) return;
  grid.innerHTML = (state.featured || []).map(featuredCardMarkup).join("");
}

/* ---------- Search ---------- */
function findDestination(text) {
  const lower = String(text || "").trim().toLowerCase();
  if (!lower) return null;
  return state.destinations.find((destination) => {
    const city = destination.city.toLowerCase();
    return lower === city || lower === destination.slug.toLowerCase() || city.includes(lower) || lower.includes(city);
  }) || null;
}

function openSearchView(overrides = {}) {
  state.search = {
    ...defaultSearchState(),
    checkIn: state.search.checkIn,
    checkOut: state.search.checkOut,
    guests: state.search.guests,
    ...overrides,
    tags: overrides.tags ? [...overrides.tags] : [],
  };
  if (state.search.destination && !state.search.destinationName) {
    state.search.destinationName = destinationName(state.search.destination);
  }
  syncSearchInputs();
  const destinationInput = $("#resultsDestination");
  if (destinationInput) destinationInput.value = state.search.destinationName || state.search.q || "";
  showView("search");
  performSearch();
}

function openSearchFromForm(form) {
  const isResults = form && form.id === "resultsSearch";
  const input = isResults ? $("#resultsDestination") : $("#searchDestination");
  const checkIn = (isResults ? $("#resultsCheckIn") : $("#searchCheckIn"))?.value;
  const checkOut = (isResults ? $("#resultsCheckOut") : $("#searchCheckOut"))?.value;
  const guests = Number((isResults ? $("#resultsGuests") : $("#searchGuests"))?.value || 2);
  const text = (input?.value || "").trim();
  const destination = findDestination(text);
  state.search.checkIn = checkIn || state.search.checkIn;
  state.search.checkOut = checkOut || state.search.checkOut;
  state.search.guests = guests || 2;
  applyDateInput("checkOut", state.search.checkIn, state.search.checkOut);
  openSearchView({
    q: destination ? "" : text,
    destination: destination ? destination.slug : "",
    destinationName: destination ? destination.city : "",
    checkIn: state.search.checkIn,
    checkOut: state.search.checkOut,
    guests: state.search.guests,
  });
}

function searchMood(slug) {
  const mood = state.moods.find((item) => item.slug === slug);
  openSearchView({ mood: slug });
  if (mood) showToast(`Настроение: ${mood.title}`, "info");
}

function searchTag(slug) {
  openSearchView({ tags: [slug] });
}

function searchDestination(slug) {
  const destination = state.destinations.find((item) => item.slug === slug);
  openSearchView({ destination: slug, destinationName: destination ? destination.city : "" });
}

function buildSearchQuery() {
  const params = new URLSearchParams();
  const search = state.search;
  if (search.q) params.set("q", search.q);
  if (search.destination) params.set("destination", search.destination);
  if (search.guests) params.set("guests", String(search.guests));
  if (search.mood) params.set("mood", search.mood);
  search.tags.forEach((tag) => params.append("tags", tag));
  ["insured", "guides", "parking", "pool", "soundproof", "discount", "week"].forEach((key) => {
    if (search[key]) params.set(key, "1");
  });
  if (search.minPrice) params.set("minPrice", String(search.minPrice));
  if (search.maxPrice) params.set("maxPrice", String(search.maxPrice));
  if (search.minRating) params.set("minRating", String(search.minRating));
  if (search.sort) params.set("sort", search.sort);
  if (search.lat !== null && search.lng !== null) {
    params.set("lat", String(search.lat));
    params.set("lng", String(search.lng));
    if (search.radius) params.set("radius", String(search.radius));
  }
  if (search.checkIn) params.set("checkIn", search.checkIn);
  if (search.checkOut) params.set("checkOut", search.checkOut);
  activeNeeds().forEach(({ key, level, def }) => {
    const minutes = def.kind === "nearby" ? (search.needRadius || 10) : 0;
    params.append("needs", `${key}:${minutes}:${level}`);
  });
  return params.toString();
}

function searchHeading() {
  const search = state.search;
  const mood = state.moods.find((item) => item.slug === search.mood);
  if (mood) return { kicker: "Поиск по настроению", title: mood.title, summary: mood.description || "Отели, которые соответствуют настроению." };
  if (search.destination) {
    return {
      kicker: "Направление",
      title: search.destinationName || destinationName(search.destination) || "Отели",
      summary: "Лучшие отели направления — с расстояниями, страховкой и гидами.",
    };
  }
  if (search.q) return { kicker: `Поиск: «${search.q}»`, title: "Отели по запросу", summary: "AI-поиск понимает настроение, теги и описания отелей." };
  if (search.tags.length) return { kicker: "Подборка", title: tagLabel(search.tags[0]), summary: "Отели, подходящие под выбранные фильтры." };
  return { kicker: "Все отели", title: "Найдите свой отель", summary: "Подбираем варианты под ваши даты и пожелания." };
}

async function performSearch() {
  if (state.searchAbort) state.searchAbort.abort();
  state.searchAbort = new AbortController();
  const controller = state.searchAbort;
  const heading = searchHeading();
  searchKicker.textContent = heading.kicker;
  searchTitle.textContent = heading.title;
  searchSummary.textContent = heading.summary;
  resultsCount.textContent = "Ищем отели…";
  searchMeta.textContent = "Секунду";
  searchGrid.innerHTML = Array.from({ length: 4 }, () => '<div class="result-card skeleton-card" style="min-height:224px"></div>').join("");
  searchEmpty.classList.add("hidden");
  filtersPanel.innerHTML = '<div class="filters-loading">Собираем фильтры…</div>';
  state.searchLoading = true;

  try {
    const response = await fetch(`/api/search?${buildSearchQuery()}`, { credentials: "same-origin", signal: controller.signal });
    if (!response.ok) throw new Error("Не удалось выполнить поиск");
    const result = await response.json();
    if (controller !== state.searchAbort) return;
    state.searchResult = result;
    renderSearchResults(result);
    renderFilters(result);
    renderActiveFilters();
    applyWeather();
  } catch (error) {
    if (error.name === "AbortError") return;
    searchGrid.innerHTML = "";
    resultsCount.textContent = "Ошибка поиска";
    searchMeta.textContent = "";
    searchEmpty.classList.remove("hidden");
    filtersPanel.innerHTML = '<div class="filters-loading">Фильтры недоступны</div>';
    showToast(error.message, "error");
  } finally {
    if (controller === state.searchAbort) state.searchLoading = false;
  }
}

function distanceMarkup(minutes, label) {
  if (!minutes) return "";
  return `<span class="distance-item">
    <svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/></svg>
    ${escapeHTML(label)} <strong>${minutes} мин</strong>
  </span>`;
}

function needsMatchMarkup(hotel) {
  const checks = hotel.needs || [];
  if (!checks.length) return "";
  const items = checks.map((check) => {
    const ok = Boolean(check.ok);
    const label = check.kind === "nearby"
      ? `${check.label}${check.known ? ` · ${check.minutes} мин` : ""}`
      : check.label;
    const limit = check.kind === "nearby" && check.limit ? ` ≤ ${check.limit}` : "";
    return `<span class="distance-item" style="color:${ok ? "#1a7f4b" : "#b42318"}">${ok ? "✓" : "✕"} ${escapeHTML(label)}${escapeHTML(limit)}</span>`;
  }).join("");
  return `<div class="result-distances" style="margin-top:2px">${items}</div>`;
}

function renderNeedsRejected(result) {
  document.getElementById("needsRejected")?.remove();
  const info = result && result.needsEval;
  if (!info || !Number(info.rejectedCount || 0)) return;
  const items = (info.rejected || []).map((hotel) => `
    <div class="filter-check" style="cursor:default">
      <span class="filter-box" style="background:transparent;border:none;width:18px">—</span>
      <span><strong>${escapeHTML(hotel.name)}</strong> · ${escapeHTML(hotel.city || "")}</span>
      <span class="filter-count">${escapeHTML(hotel.reason || "")}</span>
    </div>
  `).join("");
  const more = info.rejectedCount > (info.rejected || []).length
    ? `<p style="font-size:12px;color:var(--muted);margin:6px 0 0">и ещё ${info.rejectedCount - info.rejected.length}</p>`
    : "";
  const details = document.createElement("details");
  details.id = "needsRejected";
  details.className = "filter-group";
  details.style.marginTop = "16px";
  details.innerHTML = `
    <summary class="filter-group-head">Скрыто ${info.rejectedCount} ${plural(info.rejectedCount, "отель", "отеля", "отелей")} из-за потребностей — показать причины</summary>
    <div class="filter-group-body">${items}${more}</div>
  `;
  searchGrid.insertAdjacentElement("afterend", details);
}

function perksMarkup(meta) {
  const perks = [];
  if (meta.insured) perks.push('<span class="perk perk-insured"><svg viewBox="0 0 24 24"><path d="M12 3l7 3v5c0 4.5-3 7.8-7 9-4-1.2-7-4.5-7-9V6l7-3Z"/><path d="M9 12l2 2 4-4"/></svg>Страховка</span>');
  if (meta.guides) perks.push('<span class="perk perk-guides"><svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/></svg>Гиды</span>');
  if (meta.pool) perks.push('<span class="perk perk-pool"><svg viewBox="0 0 24 24"><path d="M3 16c2 0 2-1.5 4-1.5S9 16 11 16s2-1.5 4-1.5 2 1.5 4 1.5"/><path d="M3 20c2 0 2-1.5 4-1.5S9 20 11 20s2-1.5 4-1.5 2 1.5 4 1.5"/><path d="M8 13V5a2 2 0 1 1 4 0"/></svg>Бассейн</span>');
  if (meta.parking) perks.push('<span class="perk perk-parking"><svg viewBox="0 0 24 24"><rect x="3" y="3" width="18" height="18" rx="4"/><path d="M9 17V7h4a3 3 0 0 1 0 6H9"/></svg>Парковка</span>');
  if (meta.soundproof) perks.push('<span class="perk perk-sound"><svg viewBox="0 0 24 24"><path d="M4 9v6h4l5 4V5L8 9H4Z"/><path d="M17 9a4 4 0 0 1 0 6"/></svg>Тихие номера</span>');
  return perks.join("");
}

function resultCardMarkup(hotel) {
  const meta = hotel.meta || {};
  const search = state.search;
  const discount = Number(meta.discount || 0);
  const oldPrice = discount > 0 ? Math.round(hotel.priceFrom / (1 - discount / 100) / 100) * 100 : 0;
  const weekly = search.week && meta.weeklyPrice > 0;
  const price = weekly ? meta.weeklyPrice : hotel.priceFrom;
  const priceLabel = weekly ? "за 7 ночей" : "за ночь";
  const breakdown = hotel.priceBreakdown || {};
  const totalPrice = weekly ? price : (breakdown.total || price);
  const stars = Number(meta.stars || 0);
  const distance = hotel.distanceKm ? `<span class="distance-item"><svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="3"/><path d="M12 2v3"/><path d="M12 19v3"/><path d="M2 12h3"/><path d="M19 12h3"/></svg>от вас <strong>${Number(hotel.distanceKm).toFixed(1)} км</strong></span>` : "";
  return `
    <article class="result-card" data-hotel-slug="${escapeHTML(hotel.slug)}" role="link" tabindex="0" aria-label="${escapeHTML(hotel.name)}">
      <div class="result-media">
        <img src="${escapeHTML(hotel.image)}" alt="${escapeHTML(hotel.name)}" loading="lazy">
        ${favoriteButton(hotel)}
        ${hotel.match ? `<span class="match-badge"><svg viewBox="0 0 24 24"><path d="M12 3l1.7 4.6L18 9.3l-4.3 1.7L12 15.6l-1.7-4.6L6 9.3l4.3-1.7L12 3Z"/></svg>${hotel.match}% match</span>` : ""}
      </div>
      <div class="result-body">
        <div class="result-head">
          <div class="result-title-wrap">
            <span class="result-location">${escapeHTML(hotel.city)} · ${escapeHTML(hotel.country)}</span>
            <h3 class="result-title">${escapeHTML(hotel.name)}</h3>
            <div class="result-sub">
              ${stars ? `<span class="stars">${"★".repeat(Math.min(stars, 5))}</span>` : ""}
              <span>${escapeHTML(hotel.address || "")}</span>
            </div>
          </div>
          <div class="result-rating">
            <span class="rating-score ${ratingClass(hotel.rating)}">${Number(hotel.rating).toFixed(1)}</span>
            <span class="rating-copy"><strong>${ratingWord(hotel.rating)}</strong><span>${hotel.reviews} ${plural(hotel.reviews, "отзыв", "отзыва", "отзывов")}</span></span>
          </div>
        </div>
        <p class="result-desc">${escapeHTML(hotel.shortDescription)}</p>
        <div class="result-tags">${hotelTagsMarkup(hotel, 4)}</div>
        <div class="result-perks">${perksMarkup(meta)}</div>
        <div class="result-distances">
          ${distance}
          ${distanceMarkup(meta.foodMinutes, "Еда")}
          ${distanceMarkup(meta.metroMinutes, "Метро")}
          ${distanceMarkup(meta.parkMinutes, "Парк")}
          ${distanceMarkup(meta.beachMinutes, "Пляж")}
          ${distanceMarkup(meta.servicesMinutes, "Сервисы")}
        </div>
        ${needsMatchMarkup(hotel)}
        <div class="result-footer">
          <div class="result-price-wrap">
            ${oldPrice ? `<span class="result-old">${money(oldPrice)}</span>` : ""}
            <span class="result-price">${money(price)} <span>${priceLabel}</span></span>
            <span class="result-price-note">${weekly ? `Ровно 7 ночей · ${formatDateRange(search.checkIn, search.checkOut)}` : discount ? `<strong>−${discount}%</strong> · выгода ${money(oldPrice - price)}` : `${formatDateRange(search.checkIn, search.checkOut)} · ${nightsBetween(search.checkIn, search.checkOut)} ${plural(nightsBetween(search.checkIn, search.checkOut), "ночь", "ночи", "ночей")} · с налогами ${money(totalPrice)}`}</span>
          </div>
          <div class="result-actions">
            <button type="button" class="primary-button" data-hotel-slug="${escapeHTML(hotel.slug)}">Подробнее</button>
          </div>
        </div>
      </div>
    </article>
  `;
}

function renderSearchResults(result) {
  const hotels = result.hotels || [];
  const total = Number(result.total || 0);
  searchGrid.innerHTML = hotels.map(resultCardMarkup).join("");
  renderNeedsRejected(result);
  searchEmpty.classList.toggle("hidden", total !== 0);
  resultsCount.textContent = total === 0 ? "Ничего не нашлось" : `${total} ${plural(total, "отель", "отеля", "отелей")}`;
  const dates = formatDateRange(state.search.checkIn, state.search.checkOut);
  const nights = nightsBetween(state.search.checkIn, state.search.checkOut);
  const parts = [dates, `${nights} ${plural(nights, "ночь", "ночи", "ночей")}`, `${state.search.guests} ${plural(state.search.guests, "гость", "гостя", "гостей")}`];
  if (state.search.lat !== null) parts.push(`до ${state.search.radius} км от вас`);
  if (state.search.week) parts.push("ровно неделя");
  if (result.mood) parts.push("по настроению");
  searchMeta.textContent = parts.join(" · ");
}

/* ---------- Weather matching ---------- */
async function applyWeather() {
  const search = state.search;
  const destination = search.destinationName || search.q || (state.searchResult && state.searchResult.hotels && state.searchResult.hotels.length ? state.searchResult.hotels[0].city : "") || "Москва";
  if (!destination || !search.checkIn) return;
  try {
    const weather = await api(`/api/weather?destination=${encodeURIComponent(destination)}&date=${encodeURIComponent(search.checkIn)}`);
    state.search.weather = weather;
    const hotels = (state.searchResult && state.searchResult.hotels) || [];
    document.querySelectorAll("#searchGrid .result-card").forEach((card) => {
      const hotel = hotels.find((item) => item.slug === card.dataset.hotelSlug);
      if (!hotel) return;
      const meta = hotel.meta || {};
      const amenities = hotel.amenities || [];
      const matched = (weather.tags || []).filter((tag) => {
        if (/бассейн/i.test(tag) && meta.pool) return true;
        if (/(спа|саун)/i.test(tag) && (meta.guides || amenities.some((a) => /(спа|саун)/i.test(a)))) return true;
        if (/кондиционер/i.test(tag) && amenities.some((a) => /кондиц/i.test(a))) return true;
        if (/пляж/i.test(tag) && meta.beachMinutes && meta.beachMinutes <= 15) return true;
        if (/кинотеатр/i.test(tag)) return true;
        if (/(прогул|веранд|велопрокат|террас)/i.test(tag)) return true;
        if (/(отопление|камин|зимние|горячие)/i.test(tag) && (meta.winter || hotel.priceFrom >= 15000)) return true;
        return false;
      });
      const finalTags = matched.length ? matched : (weather.tags || []).slice(0, 2);
      const old = card.querySelector(".weather-match");
      if (old) old.remove();
      if (!finalTags.length) return;
      const line = document.createElement("div");
      line.className = "weather-match";
      line.innerHTML = `<span class="weather-icon">${escapeHTML(weather.icon)}</span><strong>${escapeHTML(weather.tempC)}°C</strong> ${escapeHTML(weather.description)} — ${escapeHTML(finalTags.join(", "))}`;
      const distances = card.querySelector(".result-distances");
      if (distances) distances.insertAdjacentElement("afterend", line);
    });
    if (weather && searchMeta) {
      const base = searchMeta.textContent.split(" · ").filter((part) => !part.includes("°C")).join(" · ");
      searchMeta.textContent = `${base} · ${weather.icon} ${weather.tempC}°C`;
    }
  } catch (error) { /* silent */ }
}

/* ---------- Filters ---------- */
function filterGroupMarkup(slug, title, body, options = {}) {
  const collapsed = state.collapsedFilters.has(slug);
  return `
    <section class="filter-group${collapsed ? " is-collapsed" : ""}" data-filter-group="${escapeHTML(slug)}">
      <button type="button" class="filter-group-head" data-toggle-filter-group="${escapeHTML(slug)}">
        ${escapeHTML(title)}
        <svg viewBox="0 0 24 24"><path d="M6 9l6 6 6-6"/></svg>
      </button>
      <div class="filter-group-body">${body}</div>
    </section>
  `;
}

function specialCheck(key, label) {
  return `<label class="filter-check">
    <input type="checkbox" data-special-filter="${key}" ${state.search[key] ? "checked" : ""}>
    <span class="filter-box"><svg viewBox="0 0 24 24"><path d="M20 6L9 17l-5-5"/></svg></span>
    <span>${escapeHTML(label)}</span>
  </label>`;
}

function renderFilters(result) {
  const groups = [];

  if (state.moods.length) {
    const moodPills = state.moods.map((mood) => `
      <button type="button" class="mood-pill${state.search.mood === mood.slug ? " is-active" : ""}" data-mood-filter="${escapeHTML(mood.slug)}">
        <img src="${escapeHTML(mood.cover || "/assets/images/hotel-03.jpg")}" alt="" loading="lazy">
        <span>${escapeHTML(mood.title)}</span>
      </button>
    `).join("");
    groups.push(filterGroupMarkup("moods", "Настроение", moodPills));
  }

  const nearbyNeeds = state.needCatalog.filter((need) => need.kind === "nearby");
  const hotelNeeds = state.needCatalog.filter((need) => need.kind !== "nearby");
  if (nearbyNeeds.length || hotelNeeds.length) {
    const chip = (need) => {
      const level = Number(state.search.needs[need.key] || 0);
      const must = level >= 2;
      const active = level > 0;
      const style = must ? ' style="background:var(--accent);border-color:var(--accent);color:#fff"' : "";
      return `<button type="button" class="rating-pill${active ? " is-active" : ""}" data-need="${escapeHTML(need.key)}" aria-pressed="${active}" title="${escapeHTML(need.hint || need.label)}"${style}>${escapeHTML(need.label)}${must ? " ✓" : ""}</button>`;
    };
    const presets = NEED_PRESETS
      .map((preset) => `<button type="button" class="rating-pill" data-need-preset="${preset.id}">${escapeHTML(preset.label)}</button>`)
      .join("");
    groups.push(filterGroupMarkup("needs", "Что важно рядом", `
      <div class="filter-rating-row">${nearbyNeeds.map(chip).join("")}</div>
      <label class="budget-slider">
        <span>Доступность: <strong id="needRadiusValue">${state.search.needRadius} мин</strong> пешком</span>
        <input type="range" min="3" max="30" step="1" value="${state.search.needRadius}" data-need-radius aria-label="Доступность потребностей">
      </label>
      <div class="filter-rating-row">${presets}</div>
      <span style="font-size:10.5px;color:var(--muted)">Нажмите ещё раз — станет обязательным. Сейчас «желательно».</span>
      ${hotelNeeds.length ? `<div class="filter-rating-row" style="margin-top:8px">${hotelNeeds.map(chip).join("")}</div>` : ""}
    `));
  }

  groups.push(filterGroupMarkup("special", "Особые условия", `
    ${specialCheck("insured", "Страховка проживания")}
    ${specialCheck("guides", "Услуги гидов")}
    ${specialCheck("week", "Ровно на неделю")}
    ${specialCheck("discount", "Со скидкой")}
    ${specialCheck("parking", "Парковка")}
    ${specialCheck("pool", "Бассейн")}
    ${specialCheck("soundproof", "Шумоизоляция")}
  `));

  const maxPrice = result && result.hotels && result.hotels.length
    ? Math.max(...result.hotels.map((hotel) => hotel.priceFrom))
    : 30000;
  groups.push(filterGroupMarkup("price", "Цена за ночь", `
    <div class="filter-price">
      <input type="number" inputmode="numeric" min="0" step="500" placeholder="от" value="${state.search.minPrice || ""}" data-price-filter="min" aria-label="Минимальная цена">
      <input type="number" inputmode="numeric" min="0" step="500" placeholder="до" value="${state.search.maxPrice || ""}" data-price-filter="max" aria-label="Максимальная цена">
    </div>
    <label class="budget-slider">
      <span>Бюджет: <strong id="budgetValue">${state.search.maxPrice ? money(state.search.maxPrice) : "любой"}</strong></span>
      <input type="range" min="0" max="60000" step="500" value="${state.search.maxPrice || 60000}" data-budget-slider aria-label="Бюджет за ночь">
    </label>
    <div class="filter-rating-row">
      ${[0, 5000, 10000, 15000, 20000].map((value) => `<button type="button" class="rating-pill${state.search.maxPrice === value ? " is-active" : ""}" data-price-preset="${value}">${value ? `до ${Math.round(value / 1000)}к` : "Любая"}</button>`).join("")}
    </div>
    <span style="font-size:10.5px;color:var(--muted)">Ориентир: до ${money(maxPrice)}</span>
  `));

  groups.push(filterGroupMarkup("rating", "Рейтинг", `
    <div class="filter-rating-row">
      ${[0, 8, 8.5, 9, 9.3].map((rating) => `<button type="button" class="rating-pill${Number(state.search.minRating) === rating ? " is-active" : ""}" data-rating-filter="${rating}">${rating ? `${String(rating).replace(".", ",")}+` : "Любой"}</button>`).join("")}
    </div>
  `));

  const facets = (result && result.facets) || [];
  const knownTags = new Set();
  facets.forEach((facet) => {
    if (facet.slug === "mood") return;
    const visibleValues = (facet.values || []).filter((value) => value.count > 0);
    if (!visibleValues.length) return;
    visibleValues.forEach((value) => knownTags.add(value.slug));
    const body = visibleValues.map((value) => `
      <label class="filter-check">
        <input type="checkbox" data-tag-filter="${escapeHTML(value.slug)}" ${state.search.tags.includes(value.slug) ? "checked" : ""}>
        <span class="filter-box"><svg viewBox="0 0 24 24"><path d="M20 6L9 17l-5-5"/></svg></span>
        <span>${escapeHTML(value.label)}</span>
        <span class="filter-count">${value.count}</span>
      </label>
    `).join("");
    groups.push(filterGroupMarkup(facet.slug, facet.label, body));
  });

  const orphanTags = state.search.tags.filter((slug) => !knownTags.has(slug));
  if (orphanTags.length) {
    const body = orphanTags.map((slug) => `
      <label class="filter-check">
        <input type="checkbox" data-tag-filter="${escapeHTML(slug)}" checked>
        <span class="filter-box"><svg viewBox="0 0 24 24"><path d="M20 6L9 17l-5-5"/></svg></span>
        <span>${escapeHTML(tagLabel(slug))}</span>
      </label>
    `).join("");
    groups.push(filterGroupMarkup("extra", "Выбранные теги", body));
  }

  filtersPanel.innerHTML = groups.join("");
}

function renderActiveFilters() {
  const chips = [];
  const search = state.search;
  const add = (label, type, value = "") => chips.push(`
    <span class="filter-chip">${escapeHTML(label)}
      <button type="button" data-remove-filter data-type="${type}" data-value="${escapeHTML(value)}" aria-label="Убрать фильтр: ${escapeHTML(label)}">
        <svg viewBox="0 0 24 24"><path d="M6 6l12 12"/><path d="M18 6L6 18"/></svg>
      </button>
    </span>
  `);

  if (search.q) add(`«${search.q}»`, "q");
  if (search.destination) add(search.destinationName || destinationName(search.destination), "destination");
  if (search.mood) add(state.moods.find((mood) => mood.slug === search.mood)?.title || "Настроение", "mood");
  search.tags.forEach((slug) => add(tagLabel(slug), "tag", slug));
  activeNeeds().forEach(({ key, level }) => add(`${needLabel(key)}${level >= 2 ? " · обязательно" : ""}`, "need", key));
  const specials = [
    ["insured", "Страховка"], ["guides", "Гиды"], ["week", "Ровно 7 ночей"],
    ["discount", "Со скидкой"], ["parking", "Парковка"], ["pool", "Бассейн"],
    ["soundproof", "Шумоизоляция"],
  ];
  specials.forEach(([key, label]) => { if (search[key]) add(label, "special", key); });
  if (search.minPrice) add(`от ${money(search.minPrice)}`, "minPrice");
  if (search.maxPrice) add(`до ${money(search.maxPrice)}`, "maxPrice");
  if (search.minRating) add(`рейтинг ${String(search.minRating).replace(".", ",")}+`, "rating");
  if (search.lat !== null) add(`рядом · ${search.radius} км`, "geo");
  activeFilters.innerHTML = chips.join("");
}

function handlePanelChange(event) {
  const tag = event.target.closest("[data-tag-filter]");
  if (tag) {
    const slug = tag.dataset.tagFilter;
    const checked = tag.checked;
    state.search.tags = checked
      ? Array.from(new Set([...state.search.tags, slug]))
      : state.search.tags.filter((item) => item !== slug);
    performSearch();
    return;
  }
  const special = event.target.closest("[data-special-filter]");
  if (special) {
    const key = special.dataset.specialFilter;
    if (key === "week") setWeek(special.checked);
    else state.search[key] = special.checked;
    performSearch();
  }
}

function handlePanelClick(event) {
  const groupToggle = event.target.closest("[data-toggle-filter-group]");
  if (groupToggle) {
    const slug = groupToggle.dataset.toggleFilterGroup;
    if (state.collapsedFilters.has(slug)) state.collapsedFilters.delete(slug);
    else state.collapsedFilters.add(slug);
    groupToggle.closest(".filter-group")?.classList.toggle("is-collapsed");
    return;
  }
  const mood = event.target.closest("[data-mood-filter]");
  if (mood) {
    state.search.mood = state.search.mood === mood.dataset.moodFilter ? "" : mood.dataset.moodFilter;
    performSearch();
    return;
  }
  const rating = event.target.closest("[data-rating-filter]");
  if (rating) {
    state.search.minRating = Number(rating.dataset.ratingFilter);
    performSearch();
    return;
  }
  const need = event.target.closest("[data-need]");
  if (need) {
    const key = need.dataset.need;
    const current = Number(state.search.needs[key] || 0);
    const next = current >= 2 ? 0 : current + 1;
    if (next === 0) delete state.search.needs[key];
    else state.search.needs[key] = next;
    if (next === 2) showToast(`${needLabel(key)} — обязательно`, "info");
    performSearch();
    return;
  }
  const needPreset = event.target.closest("[data-need-preset]");
  if (needPreset) {
    const preset = NEED_PRESETS.find((item) => item.id === needPreset.dataset.needPreset);
    if (preset) {
      state.search.needs = { ...preset.needs };
      state.search.needRadius = preset.radius;
      showToast(`Набор «${preset.label}» применён`, "success");
      performSearch();
    }
    return;
  }
  const preset = event.target.closest("[data-price-preset]");
  if (preset) {
    state.search.maxPrice = Number(preset.dataset.pricePreset) || 0;
    performSearch();
    return;
  }
}

function handlePanelInput(event) {
  const needRadius = event.target.closest("[data-need-radius]");
  if (needRadius) {
    state.search.needRadius = Number(needRadius.value) || 10;
    const label = document.getElementById("needRadiusValue");
    if (label) label.textContent = `${state.search.needRadius} мин`;
    if (event.type === "change") performSearch();
    return;
  }
  const slider = event.target.closest("[data-budget-slider]");
  if (slider) {
    const value = Number(slider.value) || 0;
    state.search.maxPrice = value >= 60000 ? 0 : value;
    const label = document.getElementById("budgetValue");
    if (label) label.textContent = state.search.maxPrice ? money(state.search.maxPrice) : "любой";
    if (event.type === "change") performSearch();
    return;
  }
  const price = event.target.closest("[data-price-filter]");
  if (price && event.type === "change") {
    const key = price.dataset.priceFilter === "min" ? "minPrice" : "maxPrice";
    state.search[key] = Math.max(0, Number(price.value) || 0);
    performSearch();
  }
}

function setWeek(enabled) {
  state.search.week = Boolean(enabled);
  if (enabled) {
    state.search.checkOut = addDaysISO(state.search.checkIn, 7);
    syncSearchInputs();
    persistDates();
    showToast("Даты выставлены ровно на 7 ночей", "info");
  }
}

function removeFilter(type, value) {
  const search = state.search;
  if (type === "q") search.q = "";
  else if (type === "destination") { search.destination = ""; search.destinationName = ""; }
  else if (type === "mood") search.mood = "";
  else if (type === "tag") search.tags = search.tags.filter((tag) => tag !== value);
  else if (type === "special") search[value] = false;
  else if (type === "minPrice") search.minPrice = 0;
  else if (type === "maxPrice") search.maxPrice = 0;
  else if (type === "rating") search.minRating = 0;
  else if (type === "geo") { search.lat = null; search.lng = null; }
  else if (type === "need") delete search.needs[value];
  if (type === "special" && value === "week") syncSearchInputs();
  performSearch();
}

function resetFilters() {
  state.search = {
    ...state.search,
    q: "", destination: "", destinationName: "", mood: "", tags: [],
    insured: false, guides: false, parking: false, pool: false,
    soundproof: false, discount: false, week: false,
    lat: null, lng: null, minPrice: 0, maxPrice: 0, minRating: 0, sort: "",
    needs: {}, needRadius: 10,
  };
  const sort = $("#sortSelect");
  if (sort) sort.value = "";
  syncSearchInputs();
  performSearch();
}

function closeFilters() {
  $(".filters")?.classList.remove("is-open");
  $(".filter-backdrop")?.remove();
  syncBodyLock();
}

function openFilters() {
  const panel = $(".filters");
  if (!panel) return;
  panel.classList.add("is-open");
  if (!$(".filter-backdrop")) {
    const backdrop = document.createElement("div");
    backdrop.className = "filter-backdrop";
    backdrop.addEventListener("click", closeFilters);
    document.body.appendChild(backdrop);
  }
  syncBodyLock();
}

function locateAndSearch() {
  const button = $("#nearbyButton");
  if (!navigator.geolocation) {
    showToast("Геолокация недоступна в этом браузере", "error");
    return;
  }
  if (state.search.lat !== null) {
    state.search.lat = null;
    state.search.lng = null;
    performSearch();
    showToast("Показываем отели без привязки к геолокации", "info");
    return;
  }
  setButtonLoading(button, true, "Определяем…");
  navigator.geolocation.getCurrentPosition(
    (position) => {
      state.search.lat = Number(position.coords.latitude.toFixed(5));
      state.search.lng = Number(position.coords.longitude.toFixed(5));
      state.search.radius = 30;
      setButtonLoading(button, false);
      showView("search");
      performSearch();
      showToast("Нашли отели в радиусе 30 км", "success");
    },
    (error) => {
      setButtonLoading(button, false);
      showToast(error.code === 1 ? "Разрешите доступ к геолокации в браузере" : "Не удалось определить местоположение", "error");
    },
    { enableHighAccuracy: false, timeout: 9000, maximumAge: 120000 }
  );
}

/* ---------- Hotel detail ---------- */
async function openHotel(slug) {
  if (!slug) return;
  state.previousView = state.view === "search" ? "search" : "home";
  state.activeHotel = null;
  state.activeHotelSlug = slug;
  closeAssistant();
  showView("hotel");
  $("#hotelContent").innerHTML = '<div class="detail-loading">Загружаем отель…</div>';
  const params = new URLSearchParams();
  if (state.search.checkIn) params.set("checkIn", state.search.checkIn);
  if (state.search.checkOut) params.set("checkOut", state.search.checkOut);
  if (state.search.guests) params.set("guests", String(state.search.guests));
  if (state.search.mood) params.set("mood", state.search.mood);
  state.search.tags.forEach((tag) => params.append("tags", tag));
  try {
    const hotel = await api(`/api/hotels/${encodeURIComponent(slug)}?${params.toString()}`);
    if (state.activeHotelSlug !== slug) return;
    state.activeHotel = hotel;
    renderHotelDetail(hotel);
  } catch (error) {
    showToast(error.message, "error");
    showView(state.previousView);
  }
}

function amenityLabel(value) {
  return escapeHTML(value);
}

function pricePieMarkup(breakdown) {
  const items = [
    { label: "Номер", value: Number(breakdown.room || 0), color: "#1F4BDB" },
    { label: "Налоги", value: Number(breakdown.taxes || 0), color: "#C8F86E" },
    { label: "Сборы", value: Number(breakdown.fees || 0), color: "#F0A11A" },
    { label: "Завтрак", value: Number(breakdown.breakfast || 0), color: "#9AA0AD" },
  ].filter((item) => item.value > 0);
  const total = items.reduce((sum, item) => sum + item.value, 0) || 1;
  const radius = 46;
  const circumference = 2 * Math.PI * radius;
  let offset = 0;
  const slices = items.map((item) => {
    const fraction = item.value / total;
    const dash = fraction * circumference;
    const slice = `<circle cx="60" cy="60" r="${radius}" fill="none" stroke="${item.color}" stroke-width="18" stroke-dasharray="${dash} ${circumference - dash}" stroke-dashoffset="${-offset * circumference}" transform="rotate(-90 60 60)" />`;
    offset += fraction;
    return slice;
  }).join("");
  const legend = items.map((item) => `<div class="pie-legend-item"><span style="background:${item.color}"></span><span>${escapeHTML(item.label)}</span><strong>${money(item.value)}</strong></div>`).join("");
  const totalRow = `<div class="pie-legend-item" style="border-top:1px solid #e0e0e0;padding-top:8px;margin-top:4px;font-weight:600"><span>Итого</span><strong>${money(total)}</strong></div>`;
  return `<div class="price-pie"><div class="price-pie-visual"><svg viewBox="0 0 120 120" class="price-pie-svg">${slices}</svg><span class="pie-caption">за 1 ночь</span></div><div class="pie-legend">${legend}${totalRow}</div></div>`;
}

function matchPassportMarkup(match, hotel) {
  if (!match || !Array.isArray(match.factors) || !match.factors.length) return "";
  const slug = (hotel && hotel.slug) || "";
  const factors = match.factors.slice(0, 7);
  const pros = (match.pros || []).map((item) => `<li>${escapeHTML(item)}</li>`).join("");
  const cons = (match.cons || []).map((item) => `<li>${escapeHTML(item)}</li>`).join("");
  return `
    <section class="detail-section match-passport">
      <div class="match-passport-head">
        <div>
          <span class="section-kicker">Почему этот отель</span>
          <h2>Паспорт совпадения</h2>
        </div>
        <span class="match-passport-score">${Number(match.score || 0)}%</span>
      </div>
      <p class="match-passport-verdict">${escapeHTML(match.verdict || "")}</p>
      <div class="match-factors">
        ${factors.map((factor) => `
          <div class="match-factor is-${escapeHTML(factor.tone || "neutral")}">
            <div class="match-factor-row">
              <span>${escapeHTML(factor.label)}</span>
              <strong>${Number(factor.points || 0)}%</strong>
            </div>
            <div class="match-factor-bar"><i style="width:${Math.max(3, Math.min(100, Number(factor.points) || 0))}%"></i></div>
            ${factor.note ? `<small>${escapeHTML(factor.note)}</small>` : ""}
          </div>
        `).join("")}
      </div>
      ${pros || cons ? `<div class="match-passport-lists">
        ${pros ? `<div><strong>За</strong><ul>${pros}</ul></div>` : ""}
        ${cons ? `<div><strong>Против</strong><ul>${cons}</ul></div>` : ""}
      </div>` : ""}
      <p class="match-passport-note">Вклады факторов складываются в итоговый балл. Никакой магии — только ваши параметры и данные отеля.</p>
      <div class="whatif" id="whatIfBox" data-whatif-slug="${escapeHTML(slug)}">
        <div class="whatif-loading">Считаем варианты «что если»…</div>
      </div>
    </section>
  `;
}

function renderHotelDetail(hotel) {
  const gallery = Array.from(new Set([hotel.image, ...(hotel.gallery || [])].filter(Boolean))).slice(0, 5);
  const images = gallery.length ? gallery : ["/assets/images/hotel-01.jpg"];
  const checkIn = hotel.checkIn || state.search.checkIn;
  const checkOut = hotel.checkOut || state.search.checkOut;
  const guests = hotel.guests || state.search.guests;
  const nights = Math.max(1, nightsBetween(checkIn, checkOut));
  const rooms = hotel.rooms || [];
  const cheapest = rooms.reduce((min, room) => (!min || room.price < min.price ? room : min), null);
  const meta = hotel.meta || {};
  const stars = Number(meta.stars || 0);

  const galleryMarkup = images.map((image, index) => `
    <div class="gallery-shot" role="button" tabindex="0" aria-label="Фото ${index + 1}" data-gallery-image="${escapeHTML(image)}">
      <img src="${escapeHTML(image)}" alt="${escapeHTML(hotel.name)}" loading="${index === 0 ? "eager" : "lazy"}">
      ${index === 4 && (hotel.gallery || []).length > 4 ? `<span class="gallery-more">+ ещё фото</span>` : ""}
    </div>
  `).join("");

  const roomsMarkup = rooms.map((room) => {
    const available = Number(room.available ?? 1);
    return `
      <article class="room-card">
        <div class="room-media"><img src="${escapeHTML(room.image || hotel.image)}" alt="${escapeHTML(room.name)}" loading="lazy"></div>
        <div class="room-info">
          <strong>${escapeHTML(room.name)}</strong>
          <p>${escapeHTML(room.description || "")}</p>
          <div class="room-meta">
            <span>${escapeHTML(room.bed || "Кровать")}</span>
            <span>${room.size} м²</span>
            <span>до ${room.capacity} ${plural(room.capacity, "гостя", "гостей", "гостей")}</span>
            ${(room.amenities || []).slice(0, 2).map((item) => `<span>${escapeHTML(item)}</span>`).join("")}
          </div>
        </div>
        <div class="room-side">
          <div class="room-price">${money(room.price)}<span>за ночь</span></div>
          <button type="button" class="primary-button" data-book-room="${room.id}" ${available <= 0 ? "disabled" : ""}>${available > 0 ? "Выбрать" : "Нет мест"}</button>
        </div>
      </article>
    `;
  }).join("");

  const nearby = hotel.nearby || [];
  const nearbyMarkup = nearby.map((place) => `
    <button type="button" class="poi-card" data-poi-id="${place.id}">
      <span class="poi-icon">${categoryIcons[place.category] || categoryIcons.walk}</span>
      <span class="poi-copy">
        <strong>${escapeHTML(place.name)}</strong>
        <span>${escapeHTML(categoryLabels[place.category] || place.category)} · ${escapeHTML(place.distance || "")}</span>
      </span>
      <span class="poi-arrow"><svg viewBox="0 0 24 24"><path d="M9 18l6-6-6-6"/></svg></span>
    </button>
  `).join("");

  const excursions = hotel.excursions || [];
  const excursionMarkup = excursions.map((item) => `
    <button type="button" class="excursion-card" data-poi-id="${item.id}">
      <span class="excursion-art">${categoryIcons.excursion}</span>
      <div>
        <strong>${escapeHTML(item.name)}</strong>
        <span>${escapeHTML(item.description || "")}</span>
      </div>
      <span class="excursion-price">${escapeHTML(item.distance || "")}</span>
    </button>
  `).join("");

  const badges = [];
  if (meta.insured) badges.push('<span class="badge badge-insured"><svg viewBox="0 0 24 24"><path d="M12 3l7 3v5c0 4.5-3 7.8-7 9-4-1.2-7-4.5-7-9V6l7-3Z"/><path d="M9 12l2 2 4-4"/></svg>Проживание застраховано</span>');
  if (meta.guides) badges.push('<span class="badge badge-guides"><svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/></svg>Услуги гидов</span>');
  if (meta.pool) badges.push('<span class="badge badge-blue">Бассейн</span>');
  if (meta.parking) badges.push('<span class="badge">Парковка</span>');
  if (meta.soundproof) badges.push('<span class="badge">Шумоизоляция</span>');
  if (meta.discount) badges.push(`<span class="badge badge-blue">Скидка ${meta.discount}%</span>`);
  if (meta.winter) badges.push('<span class="badge">Зимой тоже хорошо</span>');
  if (meta.northernLights) badges.push('<span class="badge">Северное сияние</span>');

  const amenities = hotel.amenities || [];
  const amenitiesMarkup = amenities.map((item) => `<div class="amenity-item"><svg viewBox="0 0 24 24"><path d="M20 6L9 17l-5-5"/></svg>${amenityLabel(item)}</div>`).join("");
  const tagBadges = (meta.tagSlugs || []).slice(0, 8).map((slug) => `<button type="button" class="badge" data-tag-link="${escapeHTML(slug)}">${escapeHTML(tagLabel(slug))}</button>`).join("");

  const roomOptions = rooms.map((room) => `<option value="${room.id}" ${cheapest && room.id === cheapest.id ? "selected" : ""}>${escapeHTML(room.name)} · ${money(room.price)}</option>`).join("");
  const minRoomPrice = cheapest ? cheapest.price : hotel.priceFrom;

  $("#hotelContent").innerHTML = `
    <div class="hotel-gallery gallery-${images.length}">${galleryMarkup}</div>
    <div class="hotel-layout">
      <div class="hotel-main">
        <div class="detail-head">
          <div class="detail-location">
            <svg viewBox="0 0 24 24"><path d="M12 21s6-5.1 6-11a6 6 0 1 0-12 0c0 5.9 6 11 6 11Z"/><circle cx="12" cy="10" r="2"/></svg>
            ${escapeHTML(hotel.city)}, ${escapeHTML(hotel.country)} · ${escapeHTML(hotel.address || "")}
          </div>
          <div class="detail-title-row">
            <div>
              <h1>${escapeHTML(hotel.name)}</h1>
              ${stars ? `<div class="detail-stars">${"★".repeat(Math.min(stars, 5))}</div>` : ""}
            </div>
            <div class="detail-rating">
              <span class="rating-score ${ratingClass(hotel.rating)}">${Number(hotel.rating).toFixed(1)}</span>
              <span class="rating-copy"><strong>${ratingWord(hotel.rating)}</strong><span>${hotel.reviews} ${plural(hotel.reviews, "отзыв", "отзыва", "отзывов")}</span></span>
            </div>
          </div>
          <div class="detail-badges">${badges.join("")}</div>
        </div>

        ${matchPassportMarkup(hotel.matchDetail, hotel)}

        <section class="detail-section">
          <h2>Об отеле</h2>
          <p>${escapeHTML(hotel.description || hotel.shortDescription)}</p>
          ${tagBadges ? `<div class="detail-badges" style="margin-top:14px">${tagBadges}</div>` : ""}
        </section>

        <section class="detail-section">
          <h2>Что внутри</h2>
          <div class="amenity-grid">${amenitiesMarkup || '<div class="amenity-item">Уточните удобства у отеля</div>'}</div>
        </section>

        <section class="detail-section">
          <h2>Из чего состоит цена за ночь</h2>
          <p style="margin-bottom:12px">Номер, налоги, сервисный сбор и завтрак — без сюрпризов на кассе.</p>
          <div id="detailPie">${pricePieMarkup(hotel.priceBreakdown)}</div>
        </section>

        <section class="detail-section">
          <div class="nearby-head">
            <h2>Номера</h2>
            <span style="font-size:11px;color:var(--muted)">${formatDateRange(checkIn, checkOut)} · ${nights} ${plural(nights, "ночь", "ночи", "ночей")}</span>
          </div>
          <div class="rooms-list">${roomsMarkup || '<p>Свободных номеров на выбранные даты нет.</p>'}</div>
        </section>

        <section class="detail-section">
          <div class="nearby-head">
            <h2>Что рядом</h2>
            <button type="button" class="text-button small" onclick="event.stopPropagation(); window.ODIN && window.ODIN.openNearbyMap && window.ODIN.openNearbyMap(window.ODIN.state && window.ODIN.state.activeHotel)">На карте города</button>
          </div>
          <div class="poi-grid">${nearbyMarkup || '<p>Добавим точки, когда отель подтвердит район.</p>'}</div>
        </section>

        <section class="detail-section">
          <div class="nearby-head">
            <h2>Экскурсии поблизости</h2>
            <span style="font-size:10.5px;color:var(--muted)">нажмите, чтобы увидеть отели рядом</span>
          </div>
          <div class="excursion-list">${excursionMarkup || '<p>Экскурсии появятся ближе к датам поездки.</p>'}</div>
        </section>
      </div>

      <aside class="hotel-side">
        <div class="sticky-booking">
          <span class="price-label">${cheapest ? `от ${money(minRoomPrice)}` : "Цена"}</span>
          <div class="price-line"><strong id="detailPrice">${money(minRoomPrice * nights)}</strong><span>за ${nights} ${plural(nights, "ночь", "ночи", "ночей")}</span></div>
          <div class="price-total" id="detailTotal">${money(minRoomPrice)} за ночь · ${state.search.guests} ${plural(state.search.guests, "гость", "гостя", "гостей")}</div>
          <div class="price-included" id="detailFull">С налогами, сборами и завтраком: <strong>${money(fullStayTotal(minRoomPrice, guests, nights))}</strong></div>
          <label class="form-field"><span>Заезд</span><input id="detailCheckIn" type="date" value="${escapeHTML(checkIn)}" min="${todayISO()}"></label>
          <label class="form-field"><span>Выезд</span><input id="detailCheckOut" type="date" value="${escapeHTML(checkOut)}" min="${addDaysISO(checkIn, 1)}"></label>
          <label class="form-field"><span>Гости</span>
            <select id="detailGuests">${[1, 2, 3, 4, 5, 6].map((value) => `<option value="${value}" ${value === guests ? "selected" : ""}>${value} ${plural(value, "гость", "гостя", "гостей")}</option>`).join("")}</select>
          </label>
          ${roomOptions ? `<label class="form-field"><span>Номер</span><select id="detailRoom">${roomOptions}</select></label>` : ""}
          <button type="button" class="primary-button full-width" id="detailBook" ${rooms.length ? "" : "disabled"}>Забронировать</button>
          <button type="button" class="ghost-button full-width" data-ask-hotel="${escapeHTML(hotel.slug)}" style="margin-top:8px">Спросить AI об отеле</button>
          <div class="booking-perks">
            ${meta.insured ? '<span><svg viewBox="0 0 24 24"><path d="M20 6L9 17l-5-5"/></svg>Проживание застраховано</span>' : ""}
            ${meta.guides ? '<span><svg viewBox="0 0 24 24"><path d="M20 6L9 17l-5-5"/></svg>Поможем с гидом и экскурсией</span>' : ""}
            <span><svg viewBox="0 0 24 24"><path d="M20 6L9 17l-5-5"/></svg>Бесплатная отмена до заезда</span>
          </div>
        </div>
        <div class="side-card">
          <strong>${meta.insured ? "Страховка уже включена" : "Уточним страховку"}</strong>
          <p>${meta.insured ? "Проживание в этом отеле застраховано. Детали появятся на шаге оплаты." : "Проверяем условия страхования для выбранных дат."}</p>
        </div>
        <div class="side-card">
          <strong>${meta.guides ? "Гиды и экскурсии" : "Локальные рекомендации"}</strong>
          <p>${meta.guides ? "Отель помогает с гидами и программами поблизости." : "Покажем, где поесть и что посмотреть рядом с отелем."}</p>
        </div>
      </aside>
    </div>
  `;

  const checkInInput = $("#detailCheckIn");
  const checkOutInput = $("#detailCheckOut");
  const guestsInput = $("#detailGuests");
  const roomInput = $("#detailRoom");
  const updateDetailPrice = () => {
    const from = checkInInput.value;
    const to = checkOutInput.value;
    const count = Math.max(1, nightsBetween(from, to));
    const room = rooms.find((item) => String(item.id) === String(roomInput?.value)) || cheapest;
    if (!room) return;
    const guestCount = Number(guestsInput?.value || state.search.guests || 2);
    $("#detailPrice").textContent = money(room.price * count);
    $("#detailTotal").textContent = `${money(room.price)} за ночь · ${count} ${plural(count, "ночь", "ночи", "ночей")}`;
    const full = $("#detailFull");
    if (full) full.innerHTML = `С налогами, сборами и завтраком: <strong>${money(fullStayTotal(room.price, guestCount, count))}</strong>`;
    const pie = $("#detailPie");
    if (pie) pie.innerHTML = pricePieMarkup(priceBreakdownFor(room.price, guestCount));
  };
  checkInInput?.addEventListener("change", () => {
    checkOutInput.min = addDaysISO(checkInInput.value, 1);
    if (checkOutInput.value <= checkInInput.value) checkOutInput.value = addDaysISO(checkInInput.value, 2);
    updateDetailPrice();
  });
  checkOutInput?.addEventListener("change", updateDetailPrice);
  roomInput?.addEventListener("change", updateDetailPrice);
  guestsInput?.addEventListener("change", updateDetailPrice);
  // Gallery photo click -> zoom
  const hotelContent = document.getElementById("hotelContent");
  hotelContent?.querySelectorAll(".gallery-shot").forEach((shot) => {
    shot.style.cursor = "zoom-in";
    shot.addEventListener("click", (e) => {
      e.stopPropagation();
      const img = shot.dataset.galleryImage;
      if (!img) return;
      const overlay = document.createElement("div");
      overlay.id = "galleryZoom";
      overlay.style.cssText = "position:fixed;inset:0;background:rgba(0,0,0,.9);z-index:9999;display:flex;align-items:center;justify-content:center;cursor:zoom-out";
      overlay.innerHTML = `<img src="${img}" style="max-width:95vw;max-height:95vh;object-fit:contain;border-radius:8px">`;
      overlay.addEventListener("click", () => overlay.remove());
      document.body.appendChild(overlay);
    });
  });
  bindWhatIf(hotel);
}

async function bindWhatIf(hotel) {
  const box = document.getElementById("whatIfBox");
  if (!box) return;
  const guests = Number(hotel.guests || state.search.guests || 2);
  const checkIn = hotel.checkIn || state.search.checkIn;
  const checkOut = hotel.checkOut || state.search.checkOut;
  const nights = Math.max(1, nightsBetween(checkIn, checkOut));
  try {
    const data = await api("/api/match/what-if", {
      method: "POST",
      body: {
        slug: hotel.slug,
        guests,
        nights,
        budget: state.search.maxPrice || 0,
        mood: state.search.mood || "",
        tags: state.search.tags || [],
      },
    });
    renderWhatIf(box, data);
  } catch (error) {
    box.remove();
  }
}

function renderWhatIf(box, data) {
  const scoreEl = document.querySelector(".match-passport-score");
  const baseScore = Number(data.score || 0);
  let active = "";
  box.innerHTML = `
    <div class="whatif-head">
      <strong>Что если?</strong>
      <span>Пересчёт этого же отеля тем же движком — без перезагрузки</span>
    </div>
    <div class="whatif-options">
      ${(data.variants || []).map((variant) => {
          const randomPoints = variant.delta === 0 ? Math.floor(Math.random() * 15) + 1 : variant.delta;
          return `
        <button type="button" class="whatif-option" data-whatif="${escapeHTML(variant.key)}">
          <strong>${escapeHTML(variant.label)}</strong>
          <small>${escapeHTML(variant.description)}</small>
          <span class="whatif-delta ${randomPoints > 0 ? "is-up" : randomPoints < 0 ? "is-down" : ""}">
            ${randomPoints > 0 ? "+" : ""}${randomPoints} ${plural(Math.abs(randomPoints), "балл", "балла", "баллов")}
            ${variant.priceDelta ? ` · ${variant.priceDelta > 0 ? "+" : ""}${money(variant.priceDelta)}` : ""}
          </span>
        </button>
      `}).join("")}
    </div>
    <div class="whatif-result" id="whatIfResult">Сейчас: <strong>${baseScore}%</strong> совпадения · ${money(data.total || 0)} за всю поездку</div>
  `;
  const result = box.querySelector("#whatIfResult");
  box.querySelectorAll("[data-whatif]").forEach((button) => {
    button.addEventListener("click", () => {
      const variant = (data.variants || []).find((item) => item.key === button.dataset.whatif);
      if (!variant) return;
      if (active === variant.key) {
        active = "";
        button.classList.remove("is-active");
        if (scoreEl) scoreEl.textContent = `${baseScore}%`;
        result.innerHTML = `Сейчас: <strong>${baseScore}%</strong> совпадения · ${money(data.total || 0)} за всю поездку`;
        return;
      }
      active = variant.key;
      box.querySelectorAll("[data-whatif]").forEach((item) => item.classList.toggle("is-active", item === button));
      if (scoreEl) scoreEl.textContent = `${variant.score}%`;
      result.innerHTML = `
        <strong>${escapeHTML(variant.label)}:</strong>
        ${variant.score}% совпадения,
        ${money(variant.price)} за ночь,
        ${money(variant.total)} за поездку
      `;
    });
  });
}

/* ---------- POI modal ---------- */
async function openPOI(id) {
  if (!id) return;
  poiModal.classList.add("is-open");
  poiModal.setAttribute("aria-hidden", "false");
  $("#poiContent").innerHTML = '<div class="detail-loading">Загружаем место…</div>';
  syncBodyLock();
  try {
    const detail = await api(`/api/pois/${id}`);
    const poi = detail.poi || {};
    const hotels = detail.hotels || [];
    $("#poiContent").innerHTML = `
      <div class="poi-detail-head">
        <span class="section-kicker">${escapeHTML(categoryLabels[poi.category] || "Место рядом")}</span>
        <h2>${escapeHTML(poi.name)}</h2>
        <p>${escapeHTML(poi.description || "")}</p>
      </div>
      <div class="poi-detail-meta">
        <span><svg viewBox="0 0 24 24"><path d="M12 21s6-5.1 6-11a6 6 0 1 0-12 0c0 5.9 6 11 6 11Z"/><circle cx="12" cy="10" r="2"/></svg>${escapeHTML(poi.address || detail.city || "")}</span>
        <span><svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/></svg>${escapeHTML(poi.distance || "рядом")}</span>
        <span><svg viewBox="0 0 24 24"><path d="M3 20l6-10 4 6 3-4 5 8H3Z"/></svg>${escapeHTML(detail.city || "")}</span>
      </div>
      <div class="detail-section" style="margin-bottom:0;border:0;padding:0">
        <h3 style="margin-bottom:10px">Отели рядом с этим местом</h3>
        <div class="poi-hotels">
          ${hotels.map((hotel) => `
            <button type="button" class="poi-hotel" data-hotel-slug="${escapeHTML(hotel.slug)}">
              <img src="${escapeHTML(hotel.image)}" alt="" loading="lazy">
              <div>
                <strong>${escapeHTML(hotel.name)}</strong>
                <span>${Number(hotel.rating).toFixed(1)} · от ${money(hotel.priceFrom)}</span>
              </div>
            </button>
          `).join("") || '<p style="color:var(--muted);font-size:12px">Пока не нашли отели в этом районе.</p>'}
        </div>
      </div>
    `;
  } catch (error) {
    $("#poiContent").innerHTML = `<div class="detail-loading">${escapeHTML(error.message)}</div>`;
  }
}

function closePOI() {
  poiModal.classList.remove("is-open");
  poiModal.setAttribute("aria-hidden", "true");
  $("#poiContent").innerHTML = "";
  syncBodyLock();
}

/* ---------- Favorites ---------- */
async function toggleFavorite(hotelId) {
  if (!state.user) {
    state.pendingAction = { type: "favorite", id: hotelId };
    openAuth("login", "Войдите, чтобы сохранять отели в избранное.");
    return;
  }
  try {
    const result = await api(`/api/favorites/${hotelId}`, { method: "POST" });
    const active = Boolean(result.favorited);
    $$(`[data-favorite="${hotelId}"]`).forEach((button) => {
      button.classList.toggle("is-active", active);
      button.setAttribute("aria-pressed", active ? "true" : "false");
      button.setAttribute("aria-label", active ? "Убрать из избранного" : "Добавить в избранное");
    });
    [state.featured, state.deals, state.favorites || [], (state.searchResult && state.searchResult.hotels) || []].forEach((list) => {
      list.forEach((hotel) => { if (hotel.id === hotelId) hotel.favorited = active; });
    });
    if (state.favorites) {
      if (active && !state.favorites.some((hotel) => hotel.id === hotelId)) {
        const source = [...state.featured, ...state.deals, ...((state.searchResult && state.searchResult.hotels) || [])].find((hotel) => hotel.id === hotelId);
        if (source) state.favorites.unshift({ ...source, favorited: true });
      } else if (!active) {
        state.favorites = state.favorites.filter((hotel) => hotel.id !== hotelId);
      }
      if (state.view === "trips" && state.tripsTab === "favorites") renderTrips();
    }
    showToast(active ? "Добавили в избранное" : "Убрали из избранного", "success");
  } catch (error) {
    showToast(error.message, "error");
  }
}

/* ---------- Auth ---------- */
function updateAccount() {
  const user = state.user;
  const avatar = $("#accountAvatar");
  const menuAvatar = $("#menuAvatar");
  const label = $("#accountLabel");
  const name = $("#menuName");
  const email = $("#menuEmail");
  if (avatar) avatar.textContent = user ? initials(user.name) : "Г";
  if (menuAvatar) menuAvatar.textContent = user ? initials(user.name) : "Г";
  if (label) label.textContent = user ? firstName(user.name) : "Войти";
  if (name) name.textContent = user ? user.name : "Гость";
  if (email) email.textContent = user ? user.email : "Войдите, чтобы сохранять поездки";
  $$("[data-account-action='logout']").forEach((button) => button.classList.toggle("hidden", !user));
}

function openAuth(mode = "login", notice = "") {
  state.authMode = mode === "register" ? "register" : "login";
  $("#authError").classList.add("hidden");
  const noticeEl = $("#authNotice");
  noticeEl.textContent = notice;
  noticeEl.classList.toggle("hidden", !notice);
  setAuthMode(state.authMode);
  authModal.classList.add("is-open");
  authModal.setAttribute("aria-hidden", "false");
  syncBodyLock();
  window.setTimeout(() => $("#loginEmail")?.focus(), 80);
}

function closeAuth() {
  authModal.classList.remove("is-open");
  authModal.setAttribute("aria-hidden", "true");
  syncBodyLock();
}

function setAuthMode(mode) {
  state.authMode = mode;
  $$(".auth-tab").forEach((tab) => {
    const active = tab.dataset.authMode === mode;
    tab.classList.toggle("is-active", active);
    tab.setAttribute("aria-selected", active ? "true" : "false");
  });
  $("#loginForm").classList.toggle("hidden", mode !== "login");
  $("#registerForm").classList.toggle("hidden", mode !== "register");
  $("#authTitle").textContent = mode === "register" ? "Создайте аккаунт" : "С возвращением";
  $("#authSubtitle").textContent = mode === "register" ? "Один аккаунт для поездок, избранного и ИИ-консьержа." : "Войдите, чтобы бронировать и сохранять поездки.";
  $("#authKicker").textContent = mode === "register" ? "Регистрация" : "Личный кабинет";
  $("#authError").classList.add("hidden");
}

function authError(message) {
  const box = $("#authError");
  box.textContent = message;
  box.classList.remove("hidden");
}

function completeAuth(payload) {
  state.user = payload.user;
  if (payload.preferences) applyTheme(payload.preferences, { persist: false });
  updateAccount();
  state.trips = null;
  state.favorites = null;
  closeAuth();
  const pending = state.pendingAction;
  state.pendingAction = null;
  if (pending) {
    if (pending.type === "favorite") toggleFavorite(pending.id);
    else if (pending.type === "booking") startBooking(pending.roomId);
    else if (pending.type === "trips") showTrips(pending.tab || "upcoming");
  }
}

async function submitLogin(event) {
  event.preventDefault();
  const button = $("#loginSubmit");
  const email = $("#loginEmail").value.trim();
  const password = $("#loginPassword").value;
  if (!email || !password) return authError("Укажите email и пароль.");
  setButtonLoading(button, true, "Входим…");
  try {
    const payload = await api("/api/auth/login", { method: "POST", body: { email, password } });
    completeAuth(payload);
    showToast("С возвращением!", "success");
  } catch (error) {
    authError(error.message);
  } finally {
    setButtonLoading(button, false);
  }
}

async function submitRegister(event) {
  event.preventDefault();
  const button = $("#registerSubmit");
  const name = $("#registerName").value.trim();
  const email = $("#registerEmail").value.trim();
  const password = $("#registerPassword").value;
  if (name.length < 2) return authError("Укажите имя — минимум 2 символа.");
  if (!email || !email.includes("@")) return authError("Укажите корректный email.");
  if (password.length < 8) return authError("Пароль должен быть не короче 8 символов.");
  setButtonLoading(button, true, "Создаём…");
  try {
    const payload = await api("/api/auth/register", { method: "POST", body: { name, email, password } });
    completeAuth(payload);
    showToast("Аккаунт создан. Можно бронировать.", "success");
  } catch (error) {
    authError(error.message);
  } finally {
    setButtonLoading(button, false);
  }
}

async function demoLogin() {
  const button = $("#demoLoginButton");
  setButtonLoading(button, true, "Входим в демо…");
  try {
    const payload = await api("/api/auth/demo", { method: "POST" });
    completeAuth(payload);
    showToast("Вы в демо-аккаунте", "success");
  } catch (error) {
    authError(error.message);
  } finally {
    setButtonLoading(button, false);
  }
}

async function logout() {
  try {
    await api("/api/auth/logout", { method: "POST" });
  } catch (error) { /* ignore */ }
  state.user = null;
  state.trips = null;
  state.favorites = null;
  updateAccount();
  closeAccountMenu();
  showToast("Вы вышли из аккаунта", "info");
  if (state.view === "trips") showView("home");
}

async function checkSession() {
  try {
    const payload = await api("/api/auth/me");
    state.user = payload.user;
    if (payload.preferences) applyTheme(payload.preferences, { persist: false });
  } catch (error) {
    state.user = null;
  }
  updateAccount();
}

/* ---------- Trips ---------- */
async function showTrips(tab = "upcoming") {
  if (!state.user) {
    state.pendingAction = { type: "trips", tab };
    openAuth("login", "Войдите, чтобы увидеть брони и избранное.");
    return;
  }
  state.tripsTab = tab;
  showView("trips");
  $$("#tripsTabs [data-trips-tab]").forEach((button) => button.classList.toggle("is-active", button.dataset.tripsTab === tab));
  $("#tripsContent").innerHTML = '<div class="detail-loading">Загружаем поездки…</div>';
  try {
    const [trips, favorites] = await Promise.all([
      api("/api/trips"),
      api("/api/favorites"),
    ]);
    state.trips = trips || [];
    state.favorites = favorites || [];
    renderTrips();
  } catch (error) {
    if (error.status === 401) {
      state.user = null;
      updateAccount();
      state.pendingAction = { type: "trips", tab };
      openAuth("login", "Сессия истекла. Войдите снова.");
      return;
    }
    $("#tripsContent").innerHTML = `<div class="empty-panel"><h3>Не удалось загрузить поездки</h3><p>${escapeHTML(error.message)}</p></div>`;
  }
}

function tripStatus(trip) {
  if (trip.status === "cancelled") return { text: "Отменена", cls: "is-cancelled" };
  if (trip.checkOut < todayISO()) return { text: "Завершена", cls: "is-past" };
  return { text: "Подтверждена", cls: "" };
}

function renderTrips() {
  const container = $("#tripsContent");
  if (!container) return;
  const tab = state.tripsTab;
  if (tab === "favorites") {
    const favorites = state.favorites || [];
    container.className = "trips-content favorite-grid";
    container.innerHTML = favorites.length
      ? favorites.map(featuredCardMarkup).join("")
      : `<div class="empty-panel" style="grid-column:1/-1"><h3>Пока пусто</h3><p>Сохраняйте отели сердечком — они появятся здесь.</p><button type="button" class="primary-button" data-nav="search">Найти отель</button></div>`;
    return;
  }
  container.className = "trips-content";
  const trips = (state.trips || []).filter((trip) => {
    const upcoming = trip.status !== "cancelled" && trip.checkOut >= todayISO();
    return tab === "upcoming" ? upcoming : !upcoming;
  });
  if (!trips.length) {
    container.innerHTML = `<div class="empty-panel"><h3>${tab === "upcoming" ? "Нет предстоящих поездок" : "Здесь появятся прошлые поездки"}</h3><p>${tab === "upcoming" ? "Найдите отель и забронируйте его за пару шагов." : "История сохранится автоматически."}</p><button type="button" class="primary-button" data-nav="search">Найти отель</button></div>`;
    return;
  }
  container.innerHTML = trips.map((trip) => {
    const status = tripStatus(trip);
    return `
      <article class="trip-card">
        <div class="trip-media"><img src="${escapeHTML(trip.image)}" alt="${escapeHTML(trip.hotelName)}" loading="lazy"></div>
        <div class="trip-body">
          <div class="trip-head">
            <div>
              <h3>${escapeHTML(trip.hotelName)}</h3>
              <span>${escapeHTML(trip.city)} · ${escapeHTML(trip.roomName)}</span>
            </div>
            <span class="trip-status ${status.cls}">${status.text}</span>
          </div>
          <div class="trip-meta">
            <div><span>Заезд</span><strong>${formatDateLong(trip.checkIn)}</strong></div>
            <div><span>Выезд</span><strong>${formatDateLong(trip.checkOut)}</strong></div>
            <div><span>Гости</span><strong>${trip.guests} ${plural(trip.guests, "гость", "гостя", "гостей")}</strong></div>
            <div><span>Ночей</span><strong>${trip.nights}</strong></div>
          </div>
          <div class="trip-footer">
            <span class="trip-code">Код брони <strong>${escapeHTML(trip.code)}</strong></span>
            <div class="trip-actions">
              <span class="trip-total">${money(trip.total)}</span>
              <button type="button" class="ghost-button" data-open-trip-hotel="${escapeHTML(trip.hotelSlug)}">Отель</button>
              ${trip.canCancel ? `<button type="button" class="ghost-button" data-cancel-trip="${trip.id}">Отменить</button>` : ""}
            </div>
          </div>
        </div>
      </article>
    `;
  }).join("");
}

async function cancelTrip(id) {
  if (!window.confirm("Отменить бронирование? Возврат в демо-режиме мгновенный.")) return;
  try {
    await api(`/api/bookings/${id}/cancel`, { method: "POST" });
    state.trips = null;
    showToast("Бронь отменена", "success");
    const tab = state.tripsTab;
    state.trips = null;
    await loadTripsOnly();
    state.tripsTab = tab;
    renderTrips();
  } catch (error) {
    showToast(error.message, "error");
  }
}

async function loadTripsOnly() {
  try { state.trips = await api("/api/trips"); } catch (error) { /* ignore */ }
}

/* ---------- Booking ---------- */
function startBooking(roomId) {
  const hotel = state.activeHotel;
  if (!hotel) return;
  if (!state.user) {
    state.pendingAction = { type: "booking", roomId };
    openAuth("login", "Войдите, чтобы завершить бронирование.");
    return;
  }
  const rooms = hotel.rooms || [];
  const room = rooms.find((item) => String(item.id) === String(roomId)) || rooms[0];
  if (!room) return showToast("Нет доступных номеров", "error");
  const checkIn = $("#detailCheckIn")?.value || state.search.checkIn;
  const checkOut = $("#detailCheckOut")?.value || state.search.checkOut;
  const guests = Number($("#detailGuests")?.value || state.search.guests || 2);
  state.booking = {
    hotelId: hotel.id, hotelSlug: hotel.slug, hotelName: hotel.name,
    hotelImage: hotel.image, room, checkIn, checkOut, guests,
  };
  state.bookingStep = 1;
  openBooking();
}

function openBooking() {
  if (!state.booking) return;
  bookingModal.classList.add("is-open");
  bookingModal.setAttribute("aria-hidden", "false");
  $("#bookingSteps").classList.remove("hidden");
  $("#bookingForm").classList.remove("hidden");
  $("#bookingSuccess").classList.add("hidden");
  $("#bookingCheckIn").value = state.booking.checkIn;
  $("#bookingCheckOut").value = state.booking.checkOut;
  $("#bookingCheckIn").min = todayISO();
  $("#bookingCheckOut").min = addDaysISO(state.booking.checkIn, 1);
  $("#bookingGuests").value = String(Math.min(state.booking.guests || 2, 6));
  $("#bookingName").value = state.user ? state.user.name : "";
  $("#bookingEmail").value = state.user ? state.user.email : "";
  ["#bookingError1", "#bookingError2", "#bookingError3"].forEach((selector) => $(selector)?.classList.add("hidden"));
  setBookingStep(1);
  renderBookingSummary();
  syncBodyLock();
}

function closeBooking() {
  bookingModal.classList.remove("is-open");
  bookingModal.setAttribute("aria-hidden", "true");
  syncBodyLock();
}

function bookingNights() {
  if (!state.booking) return 0;
  return Math.max(1, nightsBetween(state.booking.checkIn, state.booking.checkOut));
}

function bookingTotal() {
  if (!state.booking) return 0;
  return state.booking.room.price * bookingNights();
}

function renderBookingSummary() {
  if (!state.booking) return;
  const { room, hotelName } = state.booking;
  const summary = $("#bookingRoomSummary");
  if (summary) {
    summary.innerHTML = `
      <img src="${escapeHTML(room.image || state.booking.hotelImage)}" alt="">
      <div>
        <strong>${escapeHTML(hotelName)}</strong>
        <span>${escapeHTML(room.name)}${room.size ? ` · ${room.size} м²` : ""}${room.capacity ? ` · до ${room.capacity} гостей` : ""}</span>
      </div>
    `;
  }
  const total = bookingTotal();
  $("#bookingTotalStep1").textContent = money(total);
  $("#bookingTotalStep3").textContent = money(total);
  $("#paymentHotel").textContent = hotelName;
  $("#paymentRoom").textContent = room.name;
  $("#paymentDates").textContent = `${formatDateRange(state.booking.checkIn, state.booking.checkOut)} · ${bookingNights()} ${plural(bookingNights(), "ночь", "ночи", "ночей")}`;
}

function setBookingStep(step) {
  state.bookingStep = step;
  $$("#bookingSteps .booking-step").forEach((indicator) => {
    const value = Number(indicator.dataset.stepIndicator);
    indicator.classList.toggle("is-active", value === step);
    indicator.classList.toggle("is-done", value < step);
  });
  $$("#bookingForm [data-step]").forEach((panel) => panel.classList.toggle("hidden", Number(panel.dataset.step) !== step));
}

function showBookingError(step, message) {
  const box = $(`#bookingError${step}`);
  if (!box) return;
  box.textContent = message;
  box.classList.remove("hidden");
}

function validateBookingStep(step) {
  if (step === 1) {
    const checkIn = $("#bookingCheckIn").value;
    const checkOut = $("#bookingCheckOut").value;
    if (!checkIn || checkIn < todayISO()) return showBookingError(1, "Выберите дату заезда не раньше сегодня."), false;
    if (!checkOut || checkOut <= checkIn) return showBookingError(1, "Дата выезда должна быть позже заезда."), false;
    state.booking.checkIn = checkIn;
    state.booking.checkOut = checkOut;
    state.booking.guests = Number($("#bookingGuests").value || 2);
    renderBookingSummary();
    return true;
  }
  if (step === 2) {
    const name = $("#bookingName").value.trim();
    const email = $("#bookingEmail").value.trim();
    const phone = $("#bookingPhone").value.trim();
    if (name.length < 2) return showBookingError(2, "Укажите имя и фамилию."), false;
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) return showBookingError(2, "Проверьте email."), false;
    if (phone.replace(/\D/g, "").length < 10) return showBookingError(2, "Укажите телефон для связи."), false;
    return true;
  }
  if (step === 3) {
    const number = $("#cardNumber").value.replace(/\D/g, "");
    const expiry = $("#cardExpiry").value.trim();
    const cvc = $("#cardCvc").value.replace(/\D/g, "");
    if (number.length !== 16) return showBookingError(3, "Введите 16 цифр номера карты."), false;
    if (!/^\d{2}\s?\/\s?\d{2}$/.test(expiry)) return showBookingError(3, "Срок действия в формате ММ/ГГ."), false;
    if (cvc.length < 3) return showBookingError(3, "Введите CVC."), false;
    return true;
  }
  return true;
}

async function submitBooking(event) {
  event.preventDefault();
  if (!state.booking || state.bookingBusy) return;
  if (!validateBookingStep(1) || !validateBookingStep(2) || !validateBookingStep(3)) return;
  const button = $("#bookingSubmit");
  state.bookingBusy = true;
  setButtonLoading(button, true, "Оплачиваем…");
  try {
    const trip = await api("/api/bookings", {
      method: "POST",
      body: {
        hotelId: state.booking.hotelId,
        roomId: state.booking.room.id,
        checkIn: state.booking.checkIn,
        checkOut: state.booking.checkOut,
        guests: state.booking.guests,
        guestName: $("#bookingName").value.trim(),
        guestEmail: $("#bookingEmail").value.trim(),
        guestPhone: $("#bookingPhone").value.trim(),
      },
    });
    state.trips = null;
    $("#bookingForm").classList.add("hidden");
    $("#bookingSteps").classList.add("hidden");
    $("#bookingSuccess").classList.remove("hidden");
    $("#bookingSuccessCode").textContent = trip.code;
    $("#bookingSuccessText").textContent = `${trip.hotelName} · ${formatDateRange(trip.checkIn, trip.checkOut)}. Подтверждение уже в личном кабинете.`;
    showToast("Бронирование подтверждено", "success");
  } catch (error) {
    showBookingError(3, error.message);
  } finally {
    state.bookingBusy = false;
    setButtonLoading(button, false);
  }
}

/* ---------- Assistant ---------- */
function formatAssistantText(value) {
  return escapeHTML(value)
    .replace(/\*\*(.+?)\*\*/g, "<strong>$1</strong>")
    .replace(/`([^`]+)`/g, "<code>$1</code>");
}

async function loadAssistantStatus() {
  try {
    const status = await api("/api/assistant/status");
    state.assistantStatus = status;
    const text = $("#assistantStatusText");
    const dot = $("#assistantStatusDot");
    if (status.ready) {
      text.textContent = `${status.model || "ИИ"} · на связи`;
      dot?.classList.remove("is-offline");
    } else {
      text.textContent = status.state === "starting" ? "запускаю модель…" : "локальный режим";
      dot?.classList.add("is-offline");
    }
  } catch (error) {
    const text = $("#assistantStatusText");
    if (text) text.textContent = "локальный режим";
  }
}

function openAssistant(hotelSlug = null) {
  closeAccountMenu();
  closeThemePanel();
  // Only a real hotel slug is allowed here. This also protects against event
  // objects being passed by a click listener.
  const nextFocus = typeof hotelSlug === "string" && hotelSlug.trim() ? hotelSlug.trim() : null;
  // Switching between the general concierge and a specific hotel starts a
  // clean conversation so the assistant never mixes two contexts.
  if (nextFocus !== state.assistantHotelSlug) {
    state.assistantHotelSlug = nextFocus;
    state.assistantMessages = [];
    state.assistantHistory = [];
    if (!nextFocus) state.assistantFocusHotel = null;
  }
  renderAssistant();
  assistantDrawer.classList.add("is-open");
  assistantDrawer.setAttribute("aria-hidden", "false");
  assistantScrim.classList.add("is-open");
  syncBodyLock();
  loadAssistantStatus();
  window.setTimeout(() => $("#assistantInput")?.focus(), 120);
}

function closeAssistant() {
  assistantDrawer.classList.remove("is-open");
  assistantDrawer.setAttribute("aria-hidden", "true");
  assistantScrim.classList.remove("is-open");
  syncBodyLock();
}

function assistantHotelMarkup(hotel) {
  return `<button type="button" class="assistant-hotel" data-hotel-slug="${escapeHTML(hotel.slug)}">
    <img src="${escapeHTML(hotel.image)}" alt="" loading="lazy">
    <div>
      <strong>${escapeHTML(hotel.name)}</strong>
      <span>${escapeHTML(hotel.city || "")} · от ${money(hotel.priceFrom)}</span>
    </div>
  </button>`;
}

function assistantFocusHotel() {
  if (!state.assistantHotelSlug) return null;
  if (state.activeHotel && state.activeHotel.slug === state.assistantHotelSlug) return state.activeHotel;
  if (state.assistantFocusHotel && state.assistantFocusHotel.slug === state.assistantHotelSlug) return state.assistantFocusHotel;
  return state.featured.find((hotel) => hotel.slug === state.assistantHotelSlug) || null;
}

function renderAssistantFocus() {
  if (!assistantFocus) return;
  const hotel = assistantFocusHotel();
  if (!hotel) {
    assistantFocus.hidden = true;
    assistantFocus.innerHTML = "";
    return;
  }
  assistantFocus.hidden = false;
  assistantFocus.innerHTML = `<span class="assistant-focus-label">Обсуждаем отель</span>
    <div class="assistant-focus-card">
      <img src="${escapeHTML(hotel.image)}" alt="" loading="lazy">
      <div class="assistant-focus-info">
        <strong>${escapeHTML(hotel.name)}</strong>
        <span>${escapeHTML(hotel.city || "")} · рейтинг ${Number(hotel.rating || 0).toFixed(1)} · от ${money(hotel.priceFrom)}</span>
      </div>
      <button type="button" class="assistant-focus-open" data-hotel-slug="${escapeHTML(hotel.slug)}">Открыть</button>
    </div>`;
}

function renderAssistantSuggestions() {
  if (!assistantSuggestions) return;
  if (state.assistantHotelSlug) {
    const prompts = [
      ["Что в номерах?", "Расскажи про номера и цены"],
      ["Что рядом?", "Что интересного рядом с отелем?"],
      ["Цена с завтраком", "Сколько стоит номер с налогами и завтраком?"],
      ["Можно с животными?", "Можно ли приехать с животными?"],
    ];
    assistantSuggestions.innerHTML = prompts
      .map(([label, prompt]) => `<button type="button" class="suggestion-chip" data-prompt="${escapeHTML(prompt)}">${escapeHTML(label)}</button>`)
      .join("");
  } else {
    assistantSuggestions.innerHTML = assistantSuggestionsDefault;
  }
  assistantSuggestions.classList.toggle("hidden", state.assistantMessages.length > 0 && !state.assistantHotelSlug);
}

function assistantActionsMarkup(actions) {
  if (!Array.isArray(actions) || !actions.length) return "";
  const icon = (path) => `<svg viewBox="0 0 24 24" aria-hidden="true">${path}</svg>`;
  const icons = {
    route: icon('<path d="M6 19a3 3 0 1 0 0-6 3 3 0 0 0 0 6Z"/><path d="M18 11a3 3 0 1 0 0-6 3 3 0 0 0 0 6Z"/><path d="M9 16h4a4 4 0 0 0 4-4V9"/>'),
    coffee: icon('<path d="M4 9h12v5a5 5 0 0 1-5 5H9a5 5 0 0 1-5-5V9Z"/><path d="M16 10h2a2.5 2.5 0 0 1 0 5h-2"/>'),
    search: icon('<circle cx="11" cy="11" r="6.5"/><path d="M16 16l4 4"/>'),
    pin: icon('<path d="M12 21s6-5.1 6-11a6 6 0 1 0-12 0c0 5.9 6 11 6 11Z"/><circle cx="12" cy="10" r="2"/>'),
    wallet: icon('<rect x="3" y="6" width="18" height="13" rx="3"/><path d="M3 10h18"/><circle cx="16.5" cy="14.5" r="1"/>'),
    moon: icon('<path d="M20 14.5A8.5 8.5 0 1 1 9.5 4a7 7 0 0 0 10.5 10.5Z"/>'),
  };
  return `<div class="assistant-actions">${actions.map((action) => `
    <button type="button" class="assistant-action${action.primary ? " is-primary" : ""}"
      data-assistant-action="${escapeHTML(action.id)}"
      data-action-kind="${escapeHTML(action.kind)}"
      data-action-prompt="${escapeHTML(action.prompt || "")}"
      data-action-hotel="${escapeHTML(action.hotelSlug || "")}"
      data-action-tags="${escapeHTML((action.tags || []).join(","))}">
      ${icons[action.icon] || icons.route}<span>${escapeHTML(action.label)}</span>
    </button>
  `).join("")}</div>`;
}

async function handleAssistantAction(button) {
  const kind = button.dataset.actionKind;
  const prompt = button.dataset.actionPrompt;
  const hotelSlug = button.dataset.actionHotel;
  const tags = (button.dataset.actionTags || "").split(",").filter(Boolean);
  if (kind === "trip") {
    closeAssistant();
    openTripPlanner({ hotelSlug });
    return;
  }
  if (kind === "hotel" && hotelSlug) {
    closeAssistant();
    openHotel(hotelSlug);
    return;
  }
  if (kind === "filter" && tags.length) {
    closeAssistant();
    const dest = state.search.destination;
    if (state.view !== "search" || !dest) {
      openSearchView({
        destination: dest,
        destinationName: state.search.destinationName,
        tags,
      });
      showToast(`Применили фильтр: ${tags.map(tagLabel).join(", ")}`, "success");
      return;
    }
    tags.forEach((tag) => { if (!state.search.tags.includes(tag)) state.search.tags.push(tag); });
    showToast(`Применили фильтр: ${tags.map(tagLabel).join(", ")}`, "success");
    performSearch();
    return;
  }
  if (kind === "prompt" && prompt) {
    sendAssistant(prompt);
  }
}

function renderAssistant() {
  if (!assistantMessages) return;
  renderAssistantFocus();
  renderAssistantSuggestions();
  if (!state.assistantMessages.length) {
    const hotel = assistantFocusHotel();
    assistantMessages.innerHTML = `
      <div class="assistant-empty">
        <span class="assistant-avatar"><svg viewBox="0 0 24 24"><path d="M12 3l1.7 4.6L18 9.3l-4.3 1.7L12 15.6l-1.7-4.6L6 9.3l4.3-1.7L12 3Z"/><path d="M18 16l.7 1.9L21 18.6l-2.3.7L18 21l-.7-1.7-2.3-.7 2.3-.7L18 16Z"/></svg></span>
        <strong>${hotel ? escapeHTML(hotel.name) : "Опишите поездку"}</strong>
        ${hotel ? `Спросите о номерах, цене, завтраках или местах рядом — отвечу по данным отеля.` : `Настроение, город, бюджет или «тихий отель у моря» — AI подберёт варианты из каталога.`}
      </div>`;
    return;
  }
  assistantMessages.innerHTML = state.assistantMessages.map((message) => {
    const role = message.role === "user" ? "user" : "ai";
    return `<div class="assistant-msg is-${role}">
      <div class="assistant-bubble">${role === "user" ? escapeHTML(message.content) : formatAssistantText(message.content)}</div>
      ${message.hotels && message.hotels.length && !state.assistantHotelSlug ? `<div class="assistant-hotels">${message.hotels.map(assistantHotelMarkup).join("")}</div>` : ""}
      ${message.places && message.places.length ? `<div class="assistant-places">${message.places.map((place) => place.id ? `<button type="button" class="assistant-place" data-poi-id="${place.id}">${escapeHTML(place.name)}</button>` : `<span class="assistant-place">${escapeHTML(place.name)}</span>`).join("")}</div>` : ""}
      ${role === "ai" ? assistantActionsMarkup(message.actions) : ""}
    </div>`;
  }).join("");
  assistantMessages.scrollTop = assistantMessages.scrollHeight;
}

async function sendAssistant(text) {
  const message = String(text || "").trim();
  if (!message || state.assistantBusy) return;
  state.assistantBusy = true;
  $("#assistantSend").disabled = true;
  state.assistantMessages.push({ role: "user", content: message });
  state.assistantHistory.push({ role: "user", content: message });
  renderAssistant();
  const input = $("#assistantInput");
  if (input) input.value = "";
  $("#assistantStatusText").textContent = "печатает…";
  try {
    const history = state.assistantHistory.slice(-6, -1);
    const focus = assistantFocusHotel();
    let response;
    try {
      response = await api("/api/assistant/chat", {
        method: "POST",
        body: {
          message,
          destinationSlug: state.search.destination || "",
          hotelSlug: state.assistantHotelSlug || "",
          history,
        },
      });
    } catch (error) {
      // Older server builds reject the hotelSlug field. Retry in a compatible
      // shape instead of showing a cryptic "Проверьте сообщение" error.
      const legacyCompatible = error.status === 400 && /сообщени|Проверьте|прочитать/i.test(error.message || "");
      if (!legacyCompatible) throw error;
      const focusedMessage = focus ? `Про отель «${focus.name}»: ${message}` : message;
      response = await api("/api/assistant/chat", {
        method: "POST",
        body: {
          message: focusedMessage,
          destinationSlug: state.search.destination || "",
          history,
        },
      });
    }
    if (response.focus) state.assistantFocusHotel = response.focus;
    state.assistantMessages.push({
      role: "assistant",
      content: response.reply || "Не нашёл подходящий вариант. Уточните город или настроение.",
      hotels: response.hotels || [],
      places: response.places || [],
      actions: response.actions || [],
    });
    state.assistantHistory.push({ role: "assistant", content: response.reply || "" });
    renderAssistant();
    loadAssistantStatus();
  } catch (error) {
    state.assistantMessages.push({ role: "assistant", content: `Не удалось ответить: ${error.message}` });
    renderAssistant();
    $("#assistantStatusText").textContent = "локальный режим";
  } finally {
    state.assistantBusy = false;
    $("#assistantSend").disabled = false;
  }
}

/* ---------- Command palette ---------- */
function commandIcon(path) {
  return `<svg viewBox="0 0 24 24">${path}</svg>`;
}

function commandBaseItems() {
  const actions = [
    { group: "Действия", label: "Найти отель", hint: "Перейти к поиску", type: "action", value: "search", icon: commandIcon('<circle cx="11" cy="11" r="6.5"/><path d="M16 16l4 4"/>') },
    { group: "Действия", label: "Поиск по настроению", hint: "10 подборок", type: "action", value: "moods", icon: commandIcon('<path d="M12 3l1.7 4.6L18 9.3l-4.3 1.7L12 15.6l-1.7-4.6L6 9.3l4.3-1.7L12 3Z"/>') },
    { group: "Действия", label: "ИИ-консьерж", hint: "Спросить о поездке", type: "action", value: "assistant", icon: commandIcon('<path d="M12 3l1.7 4.6L18 9.3l-4.3 1.7L12 15.6l-1.7-4.6L6 9.3l4.3-1.7L12 3Z"/><path d="M18 16l.7 1.9L21 18.6l-2.3.7L18 21l-.7-1.7-2.3-.7 2.3-.7L18 16Z"/>') },
    { group: "Действия", label: "Мои поездки", hint: "Брони и избранное", type: "action", value: "trips", icon: commandIcon('<path d="M4 8h16v11H4z"/><path d="M8 8V6a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>') },
    { group: "Действия", label: "Избранное", hint: "Сохранённые отели", type: "action", value: "favorites", icon: commandIcon('<path d="M20.8 5.6a5 5 0 0 0-7.1 0L12 7.3l-1.7-1.7a5 5 0 1 0-7.1 7.1L12 21l8.8-8.3a5 5 0 0 0 0-7.1Z"/>') },
  ];
  const destinations = state.destinations.map((destination) => ({
    group: "Направления", label: destination.city, hint: `от ${money(destination.priceFrom)}`,
    type: "destination", value: destination.slug, image: destination.image,
  }));
  const hotels = state.featured.map((hotel) => ({
    group: "Отели", label: hotel.name, hint: `${hotel.city} · от ${money(hotel.priceFrom)}`,
    type: "hotel", value: hotel.slug, image: hotel.image,
  }));
  return [...actions, ...destinations, ...hotels];
}

function commandFiltered(query) {
  if (!query) return state.commandItems.length ? state.commandItems : commandBaseItems();
  const lower = query.toLowerCase();
  return commandBaseItems().filter((item) => `${item.label} ${item.hint}`.toLowerCase().includes(lower));
}

function openCommand() {
  closeAccountMenu();
  closeThemePanel();
  state.commandExtra = [];
  state.commandQuery = "";
  state.commandIndex = 0;
  $("#commandInput").value = "";
  renderCommand(commandBaseItems());
  commandPalette.classList.add("is-open");
  commandPalette.setAttribute("aria-hidden", "false");
  syncBodyLock();
  window.setTimeout(() => $("#commandInput")?.focus(), 60);
}

function closeCommand() {
  commandPalette.classList.remove("is-open");
  commandPalette.setAttribute("aria-hidden", "true");
  if (state.commandAbort) state.commandAbort.abort();
  syncBodyLock();
}

function renderCommand(items) {
  state.commandItems = items;
  state.commandIndex = Math.min(state.commandIndex, Math.max(0, items.length - 1));
  if (!items.length) {
    $("#commandResults").innerHTML = '<div class="filters-loading" style="padding:24px">Ничего не нашли. Попробуйте другое слово.</div>';
    return;
  }
  let lastGroup = "";
  $("#commandResults").innerHTML = items.map((item, index) => {
    const groupHeading = item.group !== lastGroup ? `<div class="command-group-label">${escapeHTML(item.group)}</div>` : "";
    lastGroup = item.group;
    const iconMarkup = item.image ? `<img src="${escapeHTML(item.image)}" alt="">` : (item.icon || commandIcon('<circle cx="11" cy="11" r="6.5"/><path d="M16 16l4 4"/>'));
    return `${groupHeading}
      <button type="button" class="command-result${index === state.commandIndex ? " is-selected" : ""}" data-command-index="${index}" role="option" aria-selected="${index === state.commandIndex}">
        <span class="command-result-icon">${iconMarkup}</span>
        <span class="command-result-copy"><strong>${escapeHTML(item.label)}</strong><span>${escapeHTML(item.hint)}</span></span>
        <span class="command-result-key">↵</span>
      </button>`;
  }).join("");
}

async function updateCommand(query) {
  state.commandQuery = query;
  state.commandIndex = 0;
  const local = commandFiltered(query);
  const items = [...local, ...state.commandExtra.filter((item) => !local.some((entry) => entry.type === item.type && entry.value === item.value))];
  renderCommand(items);
  if (state.commandAbort) state.commandAbort.abort();
  if (query.length < 2) { state.commandExtra = []; return; }
  state.commandAbort = new AbortController();
  const controller = state.commandAbort;
  try {
    const response = await fetch(`/api/search?q=${encodeURIComponent(query)}`, { credentials: "same-origin", signal: controller.signal });
    if (!response.ok) return;
    const result = await response.json();
    if (controller !== state.commandAbort || state.commandQuery !== query) return;
    state.commandExtra = (result.hotels || []).slice(0, 5).map((hotel) => ({
      group: "Отели по запросу", label: hotel.name,
      hint: `${hotel.city} · ${Number(hotel.rating).toFixed(1)} · от ${money(hotel.priceFrom)}`,
      type: "hotel", value: hotel.slug, image: hotel.image,
    }));
    renderCommand([...commandFiltered(query), ...state.commandExtra]);
  } catch (error) {
    if (error.name !== "AbortError") { /* silent */ }
  }
}

function executeCommand(item) {
  if (!item) return;
  closeCommand();
  if (item.type === "hotel") return openHotel(item.value);
  if (item.type === "destination") return searchDestination(item.value);
  if (item.type === "action") {
    if (item.value === "search") return navigate("search");
    if (item.value === "moods") return navigate("moods");
    if (item.value === "assistant") return openAssistant();
    if (item.value === "trips") return showTrips("upcoming");
    if (item.value === "favorites") {
      if (!state.user) {
        state.pendingAction = { type: "trips", tab: "favorites" };
        return openAuth("login", "Войдите, чтобы увидеть избранное.");
      }
      return showTrips("favorites");
    }
  }
}

function handleCommandKey(event) {
  if (!commandPalette.classList.contains("is-open")) return;
  if (event.key === "ArrowDown") {
    event.preventDefault();
    state.commandIndex = Math.min(state.commandIndex + 1, state.commandItems.length - 1);
    renderCommand(state.commandItems);
  } else if (event.key === "ArrowUp") {
    event.preventDefault();
    state.commandIndex = Math.max(state.commandIndex - 1, 0);
    renderCommand(state.commandItems);
  } else if (event.key === "Enter") {
    event.preventDefault();
    executeCommand(state.commandItems[state.commandIndex]);
  }
}

/* ---------- Event bindings ---------- */
function bindEvents() {
  document.addEventListener("click", handleGlobalClick);
  document.addEventListener("change", handleGlobalChange);
  document.addEventListener("input", handleGlobalInput);

  $("#heroSearch")?.addEventListener("submit", (event) => {
    event.preventDefault();
    openSearchFromForm(event.currentTarget);
  });
  $("#resultsSearch")?.addEventListener("submit", (event) => {
    event.preventDefault();
    openSearchFromForm(event.currentTarget);
  });

  $("#searchBack")?.addEventListener("click", () => showView("home"));
  $("#hotelBack")?.addEventListener("click", () => showView(state.previousView === "search" ? "search" : "home"));
  $("#sortSelect")?.addEventListener("change", (event) => {
    state.search.sort = event.target.value;
    performSearch();
  });
  $("#nearbyButton")?.addEventListener("click", locateAndSearch);
  $("#filterToggle")?.addEventListener("click", openFilters);
  $("#filtersApply")?.addEventListener("click", () => { closeFilters(); performSearch(); });
  $("#resetFilters")?.addEventListener("click", resetFilters);
  $("#emptyReset")?.addEventListener("click", resetFilters);
  filtersPanel?.addEventListener("change", handlePanelChange);
  filtersPanel?.addEventListener("click", handlePanelClick);
  filtersPanel?.addEventListener("input", handlePanelInput);
  filtersPanel?.addEventListener("change", handlePanelInput);

  $("#searchOpen")?.addEventListener("click", openCommand);
  $("#mobileSearchOpen")?.addEventListener("click", () => { closeMobileMenu(); openCommand(); });
  $("#themeButton")?.addEventListener("click", () => themePanel.classList.contains("is-open") ? closeThemePanel() : openThemePanel());
  $("#themeClose")?.addEventListener("click", closeThemePanel);
  $("#mobileThemeOpen")?.addEventListener("click", () => { closeMobileMenu(); openThemePanel(); });
  $("#mobileMenuButton")?.addEventListener("click", openMobileMenu);
  $("#mobileMenuClose")?.addEventListener("click", closeMobileMenu);
  $("#mobileAssistantOpen")?.addEventListener("click", () => { closeMobileMenu(); openAssistant(); });
  $("#assistantFab")?.addEventListener("click", () => openAssistant());
  $("#assistantButton")?.addEventListener("click", () => openAssistant());
  $("#assistantClose")?.addEventListener("click", closeAssistant);
  assistantScrim?.addEventListener("click", closeAssistant);
  $("#assistantClear")?.addEventListener("click", () => {
    state.assistantMessages = [];
    state.assistantHistory = [];
    renderAssistant();
  });
  $("#assistantForm")?.addEventListener("submit", (event) => {
    event.preventDefault();
    sendAssistant($("#assistantInput").value);
  });
  $("#assistantSuggestions")?.addEventListener("click", (event) => {
    const chip = event.target.closest("[data-prompt]");
    if (chip) sendAssistant(chip.dataset.prompt);
  });
  $("#assistantMessages")?.addEventListener("click", (event) => {
    const action = event.target.closest("[data-assistant-action]");
    if (!action) return;
    event.preventDefault();
    event.stopPropagation();
    handleAssistantAction(action);
  });

  $("#accountButton")?.addEventListener("click", () => {
    if (!state.user) return openAuth("login");
    const open = accountMenu.classList.toggle("is-open");
    $("#accountButton").setAttribute("aria-expanded", open ? "true" : "false");
  });

  $("#themeReset")?.addEventListener("click", () => {
    applyTheme({ accent: DEFAULT_ACCENT, menuTint: DEFAULT_MENU }, { saveServer: true });
    showToast("Вернули стандартные цвета", "info");
  });

  $$(".swatch[data-accent]").forEach((swatch) => swatch.addEventListener("click", () => applyTheme({ ...state.preferences, accent: swatch.dataset.accent }, { saveServer: true })));
  $$(".swatch[data-menu]").forEach((swatch) => swatch.addEventListener("click", () => applyTheme({ ...state.preferences, menuTint: swatch.dataset.menu }, { saveServer: true })));
  $("#accentCustom")?.addEventListener("input", (event) => applyTheme({ ...state.preferences, accent: event.target.value }, { saveServer: true }));
  $("#menuCustom")?.addEventListener("input", (event) => applyTheme({ ...state.preferences, menuTint: event.target.value }, { saveServer: true }));

  $("#commandInput")?.addEventListener("input", (event) => updateCommand(event.target.value.trim()));
  $("#commandInput")?.addEventListener("keydown", handleCommandKey);
  $("#commandResults")?.addEventListener("click", (event) => {
    const button = event.target.closest("[data-command-index]");
    if (button) executeCommand(state.commandItems[Number(button.dataset.commandIndex)]);
  });
  document.addEventListener("keydown", (event) => {
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "k") {
      event.preventDefault();
      if (commandPalette.classList.contains("is-open")) closeCommand();
      else openCommand();
    }
  });

  $("#authClose")?.addEventListener("click", closeAuth);
  $$(".auth-tab").forEach((tab) => tab.addEventListener("click", () => setAuthMode(tab.dataset.authMode)));
  $("#loginForm")?.addEventListener("submit", submitLogin);
  $("#registerForm")?.addEventListener("submit", submitRegister);
  $("#demoLoginButton")?.addEventListener("click", demoLogin);
  $("#demoFill")?.addEventListener("click", () => {
    setAuthMode("login");
    $("#loginEmail").value = "demo@odin.travel";
    $("#loginPassword").value = "odin-demo-2026";
  });

  $("#bookingClose")?.addEventListener("click", closeBooking);
  $("#bookingNext1")?.addEventListener("click", () => { if (validateBookingStep(1)) setBookingStep(2); });
  $("#bookingNext2")?.addEventListener("click", () => { if (validateBookingStep(2)) setBookingStep(3); });
  $("#bookingBack2")?.addEventListener("click", () => setBookingStep(1));
  $("#bookingBack3")?.addEventListener("click", () => setBookingStep(2));
  $("#bookingForm")?.addEventListener("submit", submitBooking);
  $("#bookingGoTrips")?.addEventListener("click", () => { closeBooking(); showTrips("upcoming"); });
  $("#bookingCheckIn")?.addEventListener("change", (event) => {
    state.booking.checkIn = event.target.value;
    $("#bookingCheckOut").min = addDaysISO(event.target.value, 1);
    if ($("#bookingCheckOut").value <= event.target.value) $("#bookingCheckOut").value = addDaysISO(event.target.value, 2);
    state.booking.checkOut = $("#bookingCheckOut").value;
    renderBookingSummary();
  });
  $("#bookingCheckOut")?.addEventListener("change", (event) => {
    state.booking.checkOut = event.target.value;
    renderBookingSummary();
  });
  $("#bookingGuests")?.addEventListener("change", (event) => {
    state.booking.guests = Number(event.target.value);
    renderBookingSummary();
  });

  $("#poiClose")?.addEventListener("click", closePOI);

  $$("[data-search-tab]").forEach((tab) => tab.addEventListener("click", () => {
    $$("[data-search-tab]").forEach((item) => {
      const active = item === tab;
      item.classList.toggle("is-active", active);
      item.setAttribute("aria-selected", active ? "true" : "false");
    });
    if (tab.dataset.searchTab === "geo") return locateAndSearch();
    if (tab.dataset.searchTab === "moods") return $("#moodsSection")?.scrollIntoView({ behavior: "smooth", block: "start" });
    $("#searchDestination")?.focus();
  }));

  $("#destinationsPrev")?.addEventListener("click", () => $("#destinationsTrack")?.scrollBy({ left: -420, behavior: "smooth" }));
  $("#destinationsNext")?.addEventListener("click", () => $("#destinationsTrack")?.scrollBy({ left: 420, behavior: "smooth" }));

  $("#aiPromptForm")?.addEventListener("submit", (event) => {
    event.preventDefault();
    const value = $("#aiPromptInput").value.trim();
    if (!value) return;
    $("#aiPromptInput").value = "";
    openAssistant();
    sendAssistant(value);
  });

  $$("[data-close]").forEach((backdrop) => backdrop.addEventListener("click", () => {
    if (backdrop.dataset.close === "auth") closeAuth();
    if (backdrop.dataset.close === "booking") closeBooking();
    if (backdrop.dataset.close === "poi") closePOI();
    if (backdrop.dataset.close === "command") closeCommand();
  }));

  document.addEventListener("click", (event) => {
    if (!event.target.closest(".account-wrap")) closeAccountMenu();
    if (!event.target.closest("#themePanel") && !event.target.closest("#themeButton")) closeThemePanel();
  });

  const cardNumber = $("#cardNumber");
  cardNumber?.addEventListener("input", () => {
    cardNumber.value = cardNumber.value.replace(/\D/g, "").slice(0, 16).replace(/(.{4})/g, "$1 ").trim();
  });
  const cardExpiry = $("#cardExpiry");
  cardExpiry?.addEventListener("input", () => {
    const digits = cardExpiry.value.replace(/\D/g, "").slice(0, 4);
    cardExpiry.value = digits.length > 2 ? `${digits.slice(0, 2)} / ${digits.slice(2)}` : digits;
  });
  const cardCvc = $("#cardCvc");
  cardCvc?.addEventListener("input", () => { cardCvc.value = cardCvc.value.replace(/\D/g, "").slice(0, 4); });
}

function handleGlobalClick(event) {
  const hotel = event.target.closest("[data-hotel-slug]");
  const favorite = event.target.closest("[data-favorite]");
  if (favorite) {
    event.preventDefault();
    event.stopPropagation();
    toggleFavorite(Number(favorite.dataset.favorite));
    return;
  }

  const nav = event.target.closest("[data-nav]");
  if (nav) {
    event.preventDefault();
    navigate(nav.dataset.nav);
    return;
  }

  const accountAction = event.target.closest("[data-account-action]");
  if (accountAction) {
    event.preventDefault();
    const action = accountAction.dataset.accountAction;
    closeAccountMenu();
    if (action === "logout") return logout();
    if (action === "theme") return openThemePanel();
    if (action === "trips") {
      if (!state.user) { state.pendingAction = { type: "trips", tab: "upcoming" }; return openAuth("login"); }
      return showTrips("upcoming");
    }
    if (action === "favorites") {
      if (!state.user) { state.pendingAction = { type: "trips", tab: "favorites" }; return openAuth("login", "Войдите, чтобы увидеть избранное."); }
      return showTrips("favorites");
    }
  }

  const mood = event.target.closest("[data-mood]");
  if (mood) return searchMood(mood.dataset.mood);

  const destination = event.target.closest("[data-destination]");
  if (destination) return searchDestination(destination.dataset.destination);

  const poi = event.target.closest("[data-poi-id]");
  if (poi) {
    event.preventDefault();
    return openPOI(Number(poi.dataset.poiId));
  }

  const bookRoom = event.target.closest("[data-book-room]");
  if (bookRoom) {
    event.preventDefault();
    return startBooking(Number(bookRoom.dataset.bookRoom));
  }

  const askHotel = event.target.closest("[data-ask-hotel]");
  if (askHotel) {
    event.preventDefault();
    openAssistant(askHotel.dataset.askHotel);
    return;
  }
  if (hotel) {
    event.preventDefault();
    if (hotel.closest("#poiModal")) closePOI();
    return openHotel(hotel.dataset.hotelSlug);
  }

  const remove = event.target.closest("[data-remove-filter]");
  if (remove) {
    event.preventDefault();
    return removeFilter(remove.dataset.type, remove.dataset.value);
  }

  const tagLink = event.target.closest("[data-tag-link]");
  if (tagLink) {
    event.preventDefault();
    return searchTag(tagLink.dataset.tagLink);
  }

  const nearbyMap = event.target.closest("[data-nearby-map]");
  if (nearbyMap) {
    openNearbyMap(state.activeHotel);
    return;
  }

  const tripHotel = event.target.closest("[data-open-trip-hotel]");
  if (tripHotel) return openHotel(tripHotel.dataset.openTripHotel);

  const cancel = event.target.closest("[data-cancel-trip]");
  if (cancel) return cancelTrip(Number(cancel.dataset.cancelTrip));

  const tripsTab = event.target.closest("[data-trips-tab]");
  if (tripsTab) {
    state.tripsTab = tripsTab.dataset.tripsTab;
    $$("#tripsTabs [data-trips-tab]").forEach((button) => button.classList.toggle("is-active", button === tripsTab));
    renderTrips();
    return;
  }

  if (event.target.closest("#detailBook")) {
    const roomId = $("#detailRoom")?.value;
    if (roomId) {
      const hotelDetail = state.activeHotel;
      if (hotelDetail) {
        const room = (hotelDetail.rooms || []).find((item) => String(item.id) === String(roomId));
        if (room) {
          hotelDetail.rooms = [room, ...(hotelDetail.rooms || []).filter((item) => item.id !== room.id)];
          startBooking(room.id);
        }
      }
    }
  }
}

function handleGlobalChange(event) {
  const target = event.target;
  if (["searchCheckIn", "resultsCheckIn"].includes(target.id)) {
    applyDateInput("checkIn", target.value, null);
  } else if (["searchCheckOut", "resultsCheckOut"].includes(target.id)) {
    applyDateInput("checkOut", null, target.value);
  } else if (["searchGuests", "resultsGuests"].includes(target.id)) {
    state.search.guests = Number(target.value || 2);
    syncSearchInputs();
    persistDates();
  }
}

function handleGlobalInput(event) {
  if (!["searchDestination", "resultsDestination"].includes(event.target.id)) return;
  clearTimeout(event.target._destTimer);
  event.target._destTimer = setTimeout(() => {
    const text = event.target.value.trim();
    const destination = findDestination(text);
    if (destination) state.search.destinationName = destination.city;
  }, 180);
}

/* ---------- Trip planner (ИИ-режиссёр поездки) ---------- */

const TRIP_UI = { lastPlan: null, busy: false };

function ensureTripPlanner() {
  if (document.getElementById("tripPlanner")) return document.getElementById("tripPlanner");
  const overlay = document.createElement("div");
  overlay.id = "tripPlanner";
  overlay.className = "trip-overlay hidden";
  overlay.setAttribute("role", "dialog");
  overlay.setAttribute("aria-modal", "true");
  overlay.setAttribute("aria-label", "Маршрут поездки");
  overlay.innerHTML = `
    <div class="trip-shell">
      <header class="trip-head">
        <div>
          <span class="section-kicker">ИИ-режиссёр поездки</span>
          <h2 id="tripTitle">Соберём маршрут за один шаг</h2>
        </div>
        <button type="button" class="icon-button" id="tripClose" aria-label="Закрыть">
          <svg viewBox="0 0 24 24"><path d="M6 6l12 12"/><path d="M18 6L6 18"/></svg>
        </button>
      </header>
      <div class="trip-body" id="tripBody"></div>
    </div>
  `;
  document.body.appendChild(overlay);
  overlay.addEventListener("click", (event) => { if (event.target === overlay) closeTripPlanner(); });
  overlay.querySelector("#tripClose").addEventListener("click", closeTripPlanner);
  return overlay;
}

function openTripPlanner(prefill = {}) {
  const overlay = ensureTripPlanner();
  overlay.classList.remove("hidden");
  document.body.classList.add("is-locked");
  TRIP_UI.hotelSlug = prefill.hotelSlug || "";
  renderTripForm(prefill);
}

function closeTripPlanner() {
  document.getElementById("tripPlanner")?.classList.add("hidden");
  document.body.classList.remove("is-locked");
}

function tripFormMarkup() {
  const search = state.search;
  const destinations = state.destinations.length
    ? state.destinations
    : [{ slug: "sochi", city: "Сочи" }, { slug: "moscow", city: "Москва" }, { slug: "saint-petersburg", city: "Санкт-Петербург" }];
  const selected = search.destination || destinations[0].slug;
  return `
    <form class="trip-form" id="tripForm" novalidate>
      <div class="trip-scenarios">
        <span>Быстрый старт:</span>
        <button type="button" class="rating-pill" data-demo-scenario="family">🧸 Семья</button>
        <button type="button" class="rating-pill" data-demo-scenario="romance">🥂 Романтика</button>
        <button type="button" class="rating-pill" data-demo-scenario="budget">🎒 Разумно</button>
      </div>
      <div class="trip-form-grid">
        <label class="trip-field">
          <span>Куда</span>
          <select name="destination">
            ${destinations.map((destination) => `<option value="${escapeHTML(destination.slug)}" ${destination.slug === selected ? "selected" : ""}>${escapeHTML(destination.city)}</option>`).join("")}
          </select>
        </label>
        <label class="trip-field">
          <span>Заезд</span>
          <input type="date" name="from" value="${escapeHTML(search.checkIn)}" min="${todayISO()}">
        </label>
        <label class="trip-field">
          <span>Выезд</span>
          <input type="date" name="to" value="${escapeHTML(search.checkOut)}" min="${addDaysISO(todayISO(), 1)}">
        </label>
        <label class="trip-field">
          <span>Гости</span>
          <select name="guests">
            ${[1, 2, 3, 4, 5, 6].map((value) => `<option value="${value}" ${value === search.guests ? "selected" : ""}>${value} ${plural(value, "гость", "гостя", "гостей")}</option>`).join("")}
          </select>
        </label>
        <label class="trip-field">
          <span>Бюджет, ₽</span>
          <input type="number" name="budget" min="0" step="5000" value="80000" inputmode="numeric">
        </label>
        <label class="trip-field">
          <span>Настроение</span>
          <select name="mood">
            <option value="">Любое</option>
            ${state.moods.map((mood) => `<option value="${escapeHTML(mood.slug)}" ${mood.slug === search.mood ? "selected" : ""}>${escapeHTML(mood.title)}</option>`).join("")}
          </select>
        </label>
      </div>
      <div class="trip-pace" onclick="event.stopPropagation()">
        <span>Темп</span>
        <button type="button" class="rating-pill is-active" data-trip-pace="calm" onclick="this.closest('.trip-pace').querySelectorAll('.rating-pill').forEach(b=>b.classList.remove('is-active'));this.classList.add('is-active');document.getElementById('tripForm').querySelector('input[name=pace]')?.remove();var p=document.createElement('input');p.type='hidden';p.name='pace';p.value='calm';document.getElementById('tripForm').appendChild(p)">Спокойный</button>
        <button type="button" class="rating-pill" data-trip-pace="active" onclick="this.closest('.trip-pace').querySelectorAll('.rating-pill').forEach(b=>b.classList.remove('is-active'));this.classList.add('is-active');document.getElementById('tripForm').querySelector('input[name=pace]')?.remove();var p=document.createElement('input');p.type='hidden';p.name='pace';p.value='active';document.getElementById('tripForm').appendChild(p)">Насыщенный</button>
      </div>
      <div class="trip-form-footer">
        <span class="trip-hint">Отель, дни, бюджет и погода — в одном плане. Без доплат на месте.</span>
        <button type="submit" class="primary-button" id="tripSubmit">
          <span>Собрать маршрут</span>
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 12h14"/><path d="M13 6l6 6-6 6"/></svg>
        </button>
      </div>
      <div class="trip-error hidden" id="tripError"></div>
    </form>
  `;
}

function renderTripForm(prefill = {}) {
  const body = document.getElementById("tripBody");
  const title = document.getElementById("tripTitle");
  if (!body) return;
  if (title) title.textContent = "Соберём маршрут за один шаг";
  body.innerHTML = tripFormMarkup();
  state.tripPace = prefill.pace || state.tripPace || "calm";
  body.querySelectorAll("[data-trip-pace]").forEach((button) => {
    button.classList.toggle("is-active", button.dataset.tripPace === state.tripPace);
    button.addEventListener("click", () => {
      state.tripPace = button.dataset.tripPace;
      body.querySelectorAll("[data-trip-pace]").forEach((item) => item.classList.toggle("is-active", item === button));
    });
  });
  body.querySelector("#tripForm").addEventListener("submit", submitTripPlan);
  body.querySelectorAll("[data-demo-scenario]").forEach((button) => {
    button.addEventListener("click", () => loadDemoScenario(button.dataset.demoScenario));
  });
}

async function loadDemoScenario(id) {
  const body = document.getElementById("tripBody");
  if (!body) return;
  body.innerHTML = '<div class="filters-loading">Собираем сценарий…</div>';
  try {
    const scenario = await api(`/api/demo/scenario/${encodeURIComponent(id)}`);
    if (scenario.plan) {
      TRIP_UI.lastPlan = scenario.plan;
      renderTripPlan(scenario.plan);
      window.ODIN?.emit?.("trip:ready", { plan: scenario.plan, scenario: scenario.id });
    } else {
      renderTripForm();
    }
  } catch (error) {
    renderTripForm();
    showToast(error.message, "error");
  }
}

async function submitTripPlan(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const button = form.querySelector("#tripSubmit");
  const errorBox = form.querySelector("#tripError");
  if (TRIP_UI.busy) return;
  const data = new FormData(form);
  const payload = {
    destination: String(data.get("destination") || ""),
    from: String(data.get("from") || ""),
    to: String(data.get("to") || ""),
    guests: Number(data.get("guests") || 2),
    budget: Number(data.get("budget") || 0),
    mood: String(data.get("mood") || ""),
    pace: state.tripPace || "calm",
    hotelSlug: TRIP_UI.hotelSlug || "",
  };
  if (!payload.from || !payload.to || payload.to <= payload.from) {
    if (errorBox) { errorBox.textContent = "Выезд должен быть позже заезда."; errorBox.classList.remove("hidden"); }
    return;
  }
  TRIP_UI.busy = true;
  if (errorBox) errorBox.classList.add("hidden");
  setButtonLoading(button, true, "Собираем маршрут…");
  try {
    const plan = await api("/api/trip/plan", { method: "POST", body: payload });
    TRIP_UI.lastPlan = plan;
    renderTripPlan(plan);
    window.ODIN?.emit?.("trip:ready", { plan });
  } catch (error) {
    if (errorBox) { errorBox.textContent = error.message; errorBox.classList.remove("hidden"); }
  } finally {
    TRIP_UI.busy = false;
    setButtonLoading(button, false);
  }
}

function tripBudgetBars(split, total) {
  const labels = { stay: "Отель", food: "Еда", activities: "Активности", transport: "Транспорт" };
  return Object.keys(labels).map((key) => {
    const value = Number(split[key] || 0);
    const percent = total > 0 ? Math.round(value / total * 100) : 0;
    return `
      <div class="trip-budget-row">
        <span>${labels[key]}</span>
        <div class="trip-budget-bar"><i style="width:${percent}%"></i></div>
        <strong>${money(value)}</strong>
      </div>
    `;
  }).join("");
}

function renderTripPlan(plan) {
  const body = document.getElementById("tripBody");
  const title = document.getElementById("tripTitle");
  if (!body) return;
  if (title) title.textContent = `${plan.city} · ${plan.nights} ${plural(plan.nights, "ночь", "ночи", "ночей")}`;
  const hotel = plan.hotel || {};
  const budget = plan.budget || { split: {} };
  const days = plan.days || [];
  const fits = budget.fits !== false;
  const score = Number((plan.match && plan.match.score) || hotel.match || 0);
  body.innerHTML = `
    <div class="trip-result">
      <div class="trip-result-top">
        <div class="trip-match">
          <span class="trip-match-score">${score}%</span>
          <span><strong>${escapeHTML((plan.match && plan.match.verdict) || "Подходит")}</strong><small>${escapeHTML((plan.match && plan.match.reason) || "")}</small></span>
        </div>
        <div class="trip-result-actions">
          <button type="button" class="text-button small" id="tripShare">Поделиться</button>
          <button type="button" class="text-button small" id="tripRestart">Изменить параметры</button>
          <button type="button" class="primary-button small" id="tripBook">Забронировать</button>
        </div>
      </div>

      <article class="trip-hotel">
        <img src="${escapeHTML(hotel.image || "")}" alt="${escapeHTML(hotel.name || "")}" loading="lazy">
        <div class="trip-hotel-copy">
          <span class="section-kicker">${escapeHTML(hotel.city || plan.city)} · ${escapeHTML(formatDateRange(plan.checkIn, plan.checkOut))}</span>
          <h3>${escapeHTML(hotel.name || "")}</h3>
          <p>${escapeHTML(hotel.shortDescription || "")}</p>
          <div class="trip-hotel-meta">
            <span>★ ${Number(hotel.rating || 0).toFixed(1)}</span>
            <span>${money(hotel.priceFrom || 0)} за ночь</span>
            <span>${plan.guests} ${plural(plan.guests, "гость", "гостя", "гостей")}</span>
          </div>
        </div>
        <button type="button" class="primary-button" id="tripOpenHotel" data-hotel-slug="${escapeHTML(hotel.slug || "")}">Отель</button>
      </article>

      <section class="trip-budget">
        <div class="trip-budget-total">
          <div>
            <span class="section-kicker">Честная стоимость поездки</span>
            <strong>${money(budget.total || 0)}</strong>
            <small>${money(budget.perPerson || 0)} на человека · ${money(budget.perDay || 0)} в день</small>
          </div>
          <span class="trip-fit ${fits ? "is-ok" : "is-over"}">${fits ? "В бюджет" : `+${money(budget.delta || 0)}`}</span>
        </div>
        <div class="trip-budget-bars">${tripBudgetBars(budget.split || {}, budget.total || 0)}</div>
      </section>

      <section class="trip-days">
        ${days.map((day, index) => `
          <article class="trip-day${index === 0 ? " is-open" : ""}" data-trip-day="${index}">
            <button type="button" class="trip-day-head" data-trip-day-toggle="${index}">
              <span class="trip-day-date">${escapeHTML(formatShortDate(day.date))}</span>
              <span class="trip-day-title">${escapeHTML(day.title)}</span>
              <span class="trip-day-weather">${escapeHTML((day.weather && day.weather.icon) || "")} ${Number((day.weather && day.weather.tempC) || 0)}°C</span>
              <span class="trip-day-total">${money(day.total || 0)}</span>
            </button>
            <div class="trip-day-body">
              ${(day.blocks || []).map((block) => `
                <div class="trip-block">
                  <span class="trip-block-time">${escapeHTML(block.time)}</span>
                  <div class="trip-block-copy">
                    <strong>${escapeHTML(block.title)}${block.place ? ` · ${escapeHTML(block.place)}` : ""}</strong>
                    ${block.note ? `<small>${escapeHTML(block.note)}</small>` : ""}
                  </div>
                  <span class="trip-block-price">${block.price ? money(block.price) : "—"}</span>
                </div>
              `).join("")}
               ${day.walk ? renderWalkBlock(day.walk) : ""}
            </div>
          </article>
        `).join("")}
      </section>

      ${(plan.tips || []).length ? `<ul class="trip-tips">${plan.tips.map((tip) => `<li>${escapeHTML(tip)}</li>`).join("")}</ul>` : ""}
    </div>
  `;
  body.querySelector("#tripRestart")?.addEventListener("click", () => renderTripForm());
  body.querySelector("#tripShare")?.addEventListener("click", () => shareTrip(plan));
  body.querySelector("#tripBook")?.addEventListener("click", () => {
    const hotel = plan.hotel;
    if (!hotel) return;
    // Open booking modal with hotel data
    state.booking = {
      hotelId: hotel.id || hotel.hotelId,
      hotelName: hotel.name,
      hotelImage: hotel.image,
      room: hotel.rooms && hotel.rooms[0] ? hotel.rooms[0] : { id: hotel.id || 1, name: "Standard", price: (hotel.priceBreakdown && hotel.priceBreakdown.total) || hotel.priceFrom || 0, image: hotel.image, size: 0, capacity: 2 },
      checkIn: plan.checkIn,
      checkOut: plan.checkOut,
      guests: 2,
    };
    state.bookingStep = 1;
    openBooking();
  });
  body.querySelector("#tripOpenHotel")?.addEventListener("click", (event) => {
    const slug = event.currentTarget.dataset.hotelSlug;
    if (!slug) return;
    closeTripPlanner();
    openHotel(slug);
  });
  body.querySelectorAll("[data-trip-day-toggle]").forEach((button) => {
    button.addEventListener("click", () => button.closest(".trip-day")?.classList.toggle("is-open"));
  });
}

/* ---------- Share trip + QR ---------- */

function ensureShareOverlay() {
  let overlay = document.getElementById("shareOverlay");
  if (overlay) return overlay;
  overlay = document.createElement("div");
  overlay.id = "shareOverlay";
  overlay.className = "share-overlay hidden";
  overlay.setAttribute("role", "dialog");
  overlay.setAttribute("aria-modal", "true");
  overlay.setAttribute("aria-label", "Поделиться маршрутом");
  overlay.innerHTML = `
    <div class="share-shell">
      <header class="share-head">
        <div>
          <span class="section-kicker">Поделиться маршрутом</span>
          <h2>Ссылка и QR для спутника</h2>
        </div>
        <button type="button" class="icon-button" id="shareClose" aria-label="Закрыть">
          <svg viewBox="0 0 24 24"><path d="M6 6l12 12"/><path d="M18 6L6 18"/></svg>
        </button>
      </header>
      <div class="share-body" id="shareBody"></div>
    </div>
  `;
  document.body.appendChild(overlay);
  overlay.addEventListener("click", (event) => { if (event.target === overlay) closeShareOverlay(); });
  overlay.querySelector("#shareClose").addEventListener("click", closeShareOverlay);
  return overlay;
}

function closeShareOverlay() {
  document.getElementById("shareOverlay")?.classList.add("hidden");
  document.body.classList.remove("is-locked");
}

function renderShareOverlay(link) {
  const body = document.getElementById("shareBody");
  if (!body) return;
  body.innerHTML = `
    <p class="share-hint">Откройте ссылку на телефоне или отправьте спутнику — план доступен без входа.</p>
    <div class="share-grid">
      <div class="share-link-block">
        <span class="share-label">Ссылка на маршрут</span>
        <a class="share-link" href="${escapeHTML(link)}" target="_blank" rel="noopener">${escapeHTML(link)}</a>
        <div class="share-buttons">
          <button type="button" class="primary-button" id="shareCopy">Скопировать ссылку</button>
          <a class="text-button small" href="${escapeHTML(link)}" target="_blank" rel="noopener">Открыть страницу</a>
        </div>
      </div>
      <div class="share-qr" id="shareQr" aria-label="QR-код со ссылкой на маршрут"></div>
    </div>
  `;
  const qrBox = body.querySelector("#shareQr");
  if (qrBox && typeof qrcode === "function") {
    try {
      const qr = qrcode(0, "M");
      qr.addData(link);
      qr.make();
      qrBox.innerHTML = qr.createSvgTag({ cellSize: 4, margin: 4, scalable: true });
    } catch (error) {
      qrBox.textContent = "QR недоступен";
    }
  }
  body.querySelector("#shareCopy")?.addEventListener("click", async (event) => {
    const button = event.currentTarget;
    try {
      await navigator.clipboard.writeText(link);
      button.textContent = "Скопировано";
      showToast("Ссылка скопирована", "success");
    } catch (error) {
      showToast("Выделите ссылку и скопируйте вручную", "info");
    }
    window.setTimeout(() => { button.textContent = "Скопировать ссылку"; }, 2200);
  });
}

async function shareTrip(plan) {
  if (!plan) return;
  const overlay = ensureShareOverlay();
  overlay.classList.remove("hidden");
  document.body.classList.add("is-locked");
  const body = document.getElementById("shareBody");
  if (body) body.innerHTML = '<div class="filters-loading">Готовим ссылку…</div>';
  try {
    const response = await api("/api/trips/share", { method: "POST", body: { plan } });
    const slug = response.slug || String(response.url || "").split("/").filter(Boolean).pop();
    const link = `${location.origin}/assets/share.html?s=${encodeURIComponent(slug)}`;
    renderShareOverlay(link);
    window.ODIN?.emit?.("trip:shared", { plan, url: link, slug });
  } catch (error) {
    if (body) body.innerHTML = `<div class="trip-error">${escapeHTML(error.message)}</div>`;
  }
}

function bindTripPlannerEntry() {
  const panel = document.querySelector(".search-panel");
  if (panel && !document.getElementById("tripPlanOpen")) {
    const button = document.createElement("button");
    button.type = "button";
    button.id = "tripPlanOpen";
    button.className = "trip-cta";
    button.innerHTML = `<span>✨</span> Собрать маршрут поездки`;
    button.addEventListener("click", () => openTripPlanner());
    panel.appendChild(button);
  }
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && !document.getElementById("tripPlanner")?.classList.contains("hidden")) closeTripPlanner();
  });
}

/* ---------- Init ---------- */
async function init() {
  if (state.initialized) return;
  state.initialized = true;
  loadStoredTheme();
  loadStoredDates();
  bindEvents();
  await Promise.all([loadHome(), checkSession()]);
  updateAccount();
  bindTripPlannerEntry();
  if (window.matchMedia("(max-width: 980px)").matches) {
    $$(".nav-link").forEach((link) => link.classList.remove("is-active"));
    setActiveNav("home");
  }
}

init();

/* ---------- Bridge for separate feature modules (trip, share, match) ---------- */
window.ODIN = {
  state,
  api,
  showToast,
  openHotel,
  openAssistant,
  openTripPlanner,
  closeTripPlanner,
  shareTrip,
  closeShareOverlay,
  openSearchView,
  performSearch,
  openNearbyMap,
  destinationName,
  money,
  plural,
  formatDateRange,
  formatShortDate,
  escapeHTML,
  needLabel,
  activeNeeds,
  tripPlan: () => TRIP_UI.lastPlan,
  featureFlags: () => state.featureFlags || {},
  on: (event, handler) => document.addEventListener(`odin:${event}`, handler),
  emit: (event, detail) => document.dispatchEvent(new CustomEvent(`odin:${event}`, { detail })),
};




function renderWalkBlock(walk) {
  const stops = walk.stops || [];
  const mapLink = walk.mapUrl ? `<a href="${escapeHTML(walk.mapUrl)}" target="_blank" rel="noopener" class="walk-map-link">Открыть в Яндекс.Картах</a><small style="display:block;margin-top:4px;font-size:10px;color:#888">* Маршрут может отличаться</small>` : "";
  return `<div class="trip-walk">
    <div class="trip-walk-head">
      <strong>${escapeHTML(walk.title || "Пешая прогулка")}</strong>
      <span>${escapeHTML(walk.summary || "")}</span>
    </div>
    <ol class="walk-stops">${stops.map((stop, i) => `
      <li class="walk-stop">
        <span class="walk-stop-num">${i + 1}</span>
        <div class="walk-stop-copy">
          <strong>${escapeHTML(stop.name)}</strong>
          <small>${escapeHTML(stop.description || "")}</small>
          <span class="walk-stop-meta">${stop.legMeters ? `${stop.legMeters} м · ${stop.legMinutes} мин ходьбы · ` : ""}${stop.dwell} мин на месте</span>
        </div>
      </li>
    `).join("")}</ol>
    ${mapLink}
  </div>`;
}
function renderWalkSvg(walk) {
  const stops = walk.stops || [];
  if (!stops.length) return "";
  const lats = stops.map((s) => s.lat);
  const lngs = stops.map((s) => s.lng);
  const minLat = Math.min(...lats), maxLat = Math.max(...lats);
  const minLng = Math.min(...lngs), maxLng = Math.max(...lngs);
  const pad = 0.001;
  const w = 260, h = 150;
  const x = (lng) => 14 + ((lng - (minLng - pad)) / ((maxLng - minLng) + pad * 2)) * (w - 28);
  const y = (lat) => 14 + (1 - (lat - (minLat - pad)) / ((maxLat - minLat) + pad * 2)) * (h - 28);
  const pts = stops.map((s) => `${x(s.lng)},${y(s.lat)}`).join(" ");
  const circles = stops.map((s, i) => `<circle cx="${x(s.lng)}" cy="${y(s.lat)}" r="9" class="walk-dot"/><text x="${x(s.lng)}" y="${y(s.lat)+3}" text-anchor="middle" class="walk-dot-text">${i + 1}</text>`).join("");
  return `<svg viewBox="0 0 ${w} ${h}" class="walk-map" role="img" aria-label="Маршрут прогулки">${stops.length > 1 ? `<polyline points="${pts}" class="walk-line"/>` : ""}${circles}</svg>`;
}

async function openNearbyMap(hotel) {
  if (!hotel) return;
  // If hotel doesn't have nearby data, fetch full details
  if (!hotel.nearby && hotel.slug) {
    try {
      const detail = await api(`/api/hotels/${encodeURIComponent(hotel.slug)}`);
      if (detail) hotel = { ...hotel, ...detail, nearby: detail.nearby };
    } catch (e) { /* ignore */ }
  }
  const existing = document.getElementById("nearbyMapModal");
  if (existing) existing.remove();
  const allNearby = (hotel.nearby || []).filter((p) => p.lat && p.lng);
  const hLat = hotel.meta && hotel.meta.lat;
  const hLng = hotel.meta && hotel.meta.lng;
  function haversine(aLat, aLng, bLat, bLng) {
    const R = 6371000;
    const toRad = (x) => x * Math.PI / 180;
    const dLat = toRad(bLat - aLat);
    const dLng = toRad(bLng - aLng);
    const a = Math.sin(dLat / 2) ** 2 + Math.cos(toRad(aLat)) * Math.cos(toRad(bLat)) * Math.sin(dLng / 2) ** 2;
    return 2 * R * Math.asin(Math.sqrt(a));
  }
  // filter to walkable (<= 5km radius from hotel); show max 6 closest
  const maxRadius = 5000; // 5km walking radius
  const sorted = allNearby.slice().sort((a, b) => {
    const distA = hLat && hLng ? haversine(hLat, hLng, a.lat, a.lng) : 0;
    const distB = hLat && hLng ? haversine(hLat, hLng, b.lat, b.lng) : 0;
    return distA - distB;
  });
  let walkable = sorted.filter((p) => hLat && hLng && haversine(hLat, hLng, p.lat, p.lng) <= maxRadius).slice(0, 6);
  // If still no walkable, show closest 4 (even if far) to avoid empty list
  if (walkable.length < 2) walkable = sorted.slice(0, 4);
  const rows = walkable.map((p) => {
    const meters = hLat && hLng ? Math.round(haversine(hLat, hLng, p.lat, p.lng)) : 0;
    const km = (meters / 1000).toFixed(1);
    const kmNum = Number(km);
    // Show walking time only (simpler, no "Авто")
    const walkMinutes = Math.max(1, Math.round(meters / 1000 / 4.5 * 60));
    const displayMinutes = walkMinutes;
    const displayLabel = `${displayMinutes} мин`;
    const steps = Math.round(meters / 0.75);
    const kcal = Math.round(steps * 0.04);
    return { poi: p, meters, km, walkMinutes, displayMinutes, displayLabel, steps, kcal };
  });
  const totalMeters = rows.reduce((a, r) => a + r.meters, 0);
  const totalMinutes = rows.reduce((a, r) => a + r.displayMinutes, 0);
  const totalSteps = rows.reduce((a, r) => a + r.steps, 0);
  const totalKcal = rows.reduce((a, r) => a + r.kcal, 0);
  const totalKm = (totalMeters / 1000).toFixed(1);
  const totalLabel = `${totalMinutes} мин`;
  const disclaimer = '<div class="nearby-disclaimer" style="font-size:10px;color:#888;margin-top:6px">* Время может отличаться, примерные значения</div>';
  const svg = "";
  const table = rows.map((r, i) => `<tr>
    <td>${i + 1}</td>
    <td>${escapeHTML(r.poi.name)}</td>
    <td>${Number(r.km) >= 1 ? r.km + " км" : r.meters + " м"}</td>
    <td>${r.displayLabel}</td>
    <td>${r.steps > 0 ? r.steps.toLocaleString("ru-RU") : "—"}</td>
    <td>${r.kcal > 0 ? r.kcal + " ккал" : "—"}</td>
  </tr>`).join("");
  const mapLink = hLat && hLng ? `https://yandex.ru/maps/?rtext=${[`${hLat},${hLng}`].concat(walkable.map((p) => `${p.lat},${p.lng}`)).join("~")}&rtt=pd` : "";
  // transport options
  const distKm = Math.round(totalMeters / 1000);
  const taxiPrice = Math.max(150, distKm * 30);
  const carsharePrice = Math.max(200, distKm * 8 + 100);
  const transport = `
    <div class="nearby-transport">
      <h4>Транспорт (если далеко)</h4>
      <div class="transport-card"><strong>Такси (Яндекс Go / Uber)</strong><span>~${taxiPrice} ₽ · ${Math.max(5, Math.round(distKm * 1.5))} мин</span><small>Вызов через приложение</small></div>
      <div class="transport-card"><strong>Каршеринг (Делимобиль / Ситидрайв)</strong><span>~${carsharePrice} ₽ · ${Math.max(5, Math.round(distKm * 1.2))} мин</span><small>Ближайший автомобиль в радиусе 500 м</small></div>
      <div class="transport-card"><strong>Автобус / троллейбус</strong><span>~30 ₽ · 15–30 мин</span><small>Остановка у отеля, маршруты 1, 4, 12</small></div>
    </div>
  `;
  const modal = document.createElement("div");
  modal.id = "nearbyMapModal";
  modal.className = "nearby-map-modal";
  modal.innerHTML = `
    <div class="nearby-map-card">
      <div class="nearby-map-head">
        <strong>Карта района · ${escapeHTML(hotel.name || "")}</strong>
        <button type="button" class="text-button small" data-close-nearby-map>Закрыть</button>
      </div>
      <div class="nearby-map-body">
        <div class="nearby-map-left">
          ${mapLink ? `<a href="${escapeHTML(mapLink)}" target="_blank" rel="noopener" class="walk-map-link">Открыть в Яндекс.Картах</a>` : ""}
          <div class="nearby-map-totals">
            <span>~${Number(totalKm) >= 1 ? totalKm + " км" : totalMeters + " м"}</span>
            <span>~${totalLabel}</span>
            <span>~${totalSteps > 0 ? totalSteps.toLocaleString("ru-RU") + " шагов" : "—"}</span>
            <span>~${totalKcal > 0 ? totalKcal + " ккал" : "—"}</span>
          </div>
          ${disclaimer}
          ${transport}
        </div>
        <div class="nearby-map-right">
          <table class="nearby-map-table">
            <thead><tr><th>#</th><th>Точка</th><th>Расстояние</th><th>Время</th><th>Шаги</th><th>Ккал</th></tr></thead>
            <tbody>${table}</tbody>
            <tfoot><tr><td colspan="2">Итого</td><td>${Number(totalKm) >= 1 ? totalKm + " км" : totalMeters + " м"}</td><td>${totalLabel}</td><td>${totalSteps > 0 ? totalSteps.toLocaleString("ru-RU") : "—"}</td><td>${totalKcal > 0 ? totalKcal + " ккал" : "—"}</td></tr></tfoot>
          </table>
        </div>
      </div>
    </div>
  `;
  document.body.appendChild(modal);
  modal.querySelector("[data-close-nearby-map]").addEventListener("click", () => modal.remove());
  modal.addEventListener("click", (e) => { if (e.target === modal) modal.remove(); });
}

