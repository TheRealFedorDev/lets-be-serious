const $ = (selector, root = document) => root.querySelector(selector);
const $$ = (selector, root = document) => Array.from(root.querySelectorAll(selector));

const DEFAULT_ACCENT = "#D45A3A";
const DEFAULT_MENU = "#FBF7EF";
const STORAGE_THEME = "odin.theme";
const STORAGE_SEARCH = "odin.search";

const state = {
  destinations: [],
  featured: [],
  stats: null,
  user: null,
  preferences: { accent: DEFAULT_ACCENT, menuTint: DEFAULT_MENU },
  currentView: "home",
  previousView: "home",
  search: {
    q: "",
    destination: "",
    destinationName: "",
    checkIn: "",
    checkOut: "",
    guests: 2,
    tag: "",
    maxPrice: 0,
    sort: ""
  },
  searchResults: [],
  searchLoading: false,
  activeHotel: null,
  activeHotelSlug: "",
  trips: null,
  favorites: null,
  tripsTab: "upcoming",
  authMode: "login",
  authBusy: false,
  pendingBooking: null,
  pendingFavoriteId: null,
  booking: null,
  bookingStep: 1,
  assistantOpen: false,
  assistantBusy: false,
  assistantMessages: [],
  assistantHistory: [],
  assistantStatus: null,
  commandItems: [],
  commandExtra: [],
  commandIndex: 0,
  commandQuery: "",
  commandAbort: null,
  themeTimer: null,
  themeInitialized: false
};

const cityAliases = {
  moscow: ["москва", "москве", "москву", "moscow"],
  "saint-petersburg": ["санкт-петербург", "петербург", "петербурге", "питер", "спб", "saint petersburg"],
  sochi: ["сочи", "sochi"],
  kazan: ["казань", "казани", "казанью", "kazan"],
  kaliningrad: ["калининград", "калининграде", "kaliningrad"],
  altai: ["алтай", "алтае", "алтаю", "горный алтай", "altai"]
};

const categoryLabels = {
  coffee: "Кофе",
  food: "Еда",
  walk: "Прогулка",
  culture: "Культура",
  spa: "Спа"
};

const searchDestination = $("#searchDestination");
const searchCheckIn = $("#searchCheckIn");
const searchCheckOut = $("#searchCheckOut");
const searchGuests = $("#searchGuests");
const heroSearch = $("#heroSearch");
const destinationsTrack = $("#destinationsTrack");
const featuredGrid = $("#featuredGrid");
const destinationOptions = $("#destinationOptions");
const siteHeader = $("#siteHeader");
const accountButton = $("#accountButton");
const accountMenu = $("#accountMenu");
const accountAvatar = $("#accountAvatar");
const accountLabel = $("#accountLabel");
const menuAvatar = $("#menuAvatar");
const menuName = $("#menuName");
const menuEmail = $("#menuEmail");
const themeButton = $("#themeButton");
const themePanel = $("#themePanel");
const themeStatus = $("#themeStatus");
const accentCustom = $("#accentCustom");
const menuCustom = $("#menuCustom");
const mobileMenu = $("#mobileMenu");
const commandPalette = $("#commandPalette");
const commandInput = $("#commandInput");
const commandResults = $("#commandResults");
const authModal = $("#authModal");
const bookingModal = $("#bookingModal");
const assistantDrawer = $("#assistantDrawer");
const assistantScrim = $("#assistantScrim");
const assistantMessages = $("#assistantMessages");
const assistantInput = $("#assistantInput");
const assistantSend = $("#assistantSend");
const assistantStatusText = $("#assistantStatusText");
const assistantStatusDot = $("#assistantStatusDot");
const toastRegion = $("#toastRegion");
const searchView = $("#searchView");
const hotelView = $("#hotelView");
const tripsView = $("#tripsView");
const homeView = $("#homeView");
const searchGrid = $("#searchGrid");
const searchMeta = $("#searchMeta");
const searchEmpty = $("#searchEmpty");
const searchTitle = $("#searchTitle");
const searchKicker = $("#searchKicker");
const searchSummary = $("#searchSummary");

function escapeHTML(value) {
  return String(value ?? "").replace(/[&<>"']/g, (character) => {
    const entities = {
      "&": "&amp;",
      "<": "&lt;",
      ">": "&gt;",
      '"': "&quot;",
      "'": "&#39;"
    };
    return entities[character];
  });
}

function formatAssistantText(value) {
  return escapeHTML(value)
    .replace(/\*\*(.+?)\*\*/g, "<strong>$1</strong>")
    .replace(/`([^`]+)`/g, "<code>$1</code>");
}

function money(value) {
  return `${Number(value || 0).toLocaleString("ru-RU")} ₽`;
}

function plural(value, one, few, many) {
  const mod10 = value % 10;
  const mod100 = value % 100;
  if (mod10 === 1 && mod100 !== 11) return one;
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return few;
  return many;
}

function formatShortDate(iso) {
  if (!iso) return "—";
  const date = new Date(`${iso}T12:00:00`);
  if (Number.isNaN(date.getTime())) return "—";
  return new Intl.DateTimeFormat("ru-RU", { day: "numeric", month: "short" }).format(date);
}

function formatDateRange(checkIn, checkOut) {
  if (!checkIn || !checkOut) return "Даты не выбраны";
  return `${formatShortDate(checkIn)} — ${formatShortDate(checkOut)}`;
}

function formatDateLong(iso) {
  if (!iso) return "—";
  const date = new Date(`${iso}T12:00:00`);
  return new Intl.DateTimeFormat("ru-RU", { weekday: "short", day: "numeric", month: "long" }).format(date);
}

function toISODate(date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function todayISO() {
  return toISODate(new Date());
}

function addDays(date, days) {
  const next = new Date(date);
  next.setDate(next.getDate() + days);
  return next;
}

function defaultDates() {
  const start = addDays(new Date(), 14);
  const end = addDays(start, 3);
  return { checkIn: toISODate(start), checkOut: toISODate(end) };
}

function nightsBetween(checkIn, checkOut) {
  const start = new Date(`${checkIn}T12:00:00`);
  const end = new Date(`${checkOut}T12:00:00`);
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) return 0;
  return Math.max(0, Math.round((end - start) / 86400000));
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

function normalizeHex(value, fallback) {
  const normalized = String(value || "").trim();
  return /^#[0-9a-f]{6}$/i.test(normalized) ? normalized.toUpperCase() : fallback;
}

function hexToRgb(hex) {
  const value = hex.replace("#", "");
  return {
    r: parseInt(value.slice(0, 2), 16),
    g: parseInt(value.slice(2, 4), 16),
    b: parseInt(value.slice(4, 6), 16)
  };
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
    b: a.b * (1 - weight) + b.b * weight
  });
}

function luminance(hex) {
  const { r, g, b } = hexToRgb(hex);
  const channels = [r, g, b].map((value) => {
    const normalized = value / 255;
    return normalized <= 0.03928 ? normalized / 12.92 : Math.pow((normalized + 0.055) / 1.055, 2.4);
  });
  return 0.2126 * channels[0] + 0.7152 * channels[1] + 0.0722 * channels[2];
}

function showToast(message, tone = "success") {
  const toast = document.createElement("div");
  toast.className = "toast";
  toast.dataset.tone = tone;
  toast.setAttribute("role", "status");

  const icon = document.createElement("span");
  icon.className = "toast-icon";
  if (tone === "error") {
    icon.innerHTML = '<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M12 8v5"/><path d="M12 16.5h.01"/></svg>';
  } else if (tone === "info") {
    icon.innerHTML = '<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M12 11v5"/><path d="M12 8h.01"/></svg>';
  } else {
    icon.innerHTML = '<svg viewBox="0 0 24 24"><path d="M20 6L9 17l-5-5"/></svg>';
  }

  const text = document.createElement("p");
  text.className = "toast-text";
  text.textContent = message;
  toast.append(icon, text);
  toastRegion.appendChild(toast);

  window.setTimeout(() => {
    toast.classList.add("is-leaving");
    window.setTimeout(() => toast.remove(), 240);
  }, 3600);
}

function setButtonLoading(button, loading, label = "Подождите...") {
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
  const request = {
    method: options.method || "GET",
    credentials: "same-origin",
    headers: {}
  };
  if (options.body !== undefined) {
    request.headers["Content-Type"] = "application/json";
    request.body = JSON.stringify(options.body);
  }

  let response;
  try {
    response = await fetch(path, request);
  } catch (error) {
    const networkError = new Error("Нет связи с сервером ODIN. Проверьте, что приложение запущено.");
    networkError.status = 0;
    throw networkError;
  }

  let data = null;
  if (response.status !== 204) {
    const contentType = response.headers.get("content-type") || "";
    if (contentType.includes("application/json")) {
      data = await response.json().catch(() => null);
    }
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
      mobileMenu.classList.contains("is-open")
  );
  document.body.classList.toggle("is-locked", locked);
}

function applyTheme(theme, options = {}) {
  const accent = normalizeHex(theme && theme.accent, DEFAULT_ACCENT);
  const menuTint = normalizeHex(theme && theme.menuTint, DEFAULT_MENU);
  const persist = options.persist !== false;
  const saveServer = options.saveServer === true;

  state.preferences = { accent, menuTint };

  const root = document.documentElement;
  root.style.setProperty("--accent", accent);
  root.style.setProperty("--accent-strong", mixHex(accent, "#000000", 0.18));
  root.style.setProperty("--accent-soft", mixHex(accent, "#FFFFFF", 0.82));
  root.style.setProperty("--accent-faint", mixHex(accent, "#FFFFFF", 0.93));
  root.style.setProperty("--accent-contrast", luminance(accent) > 0.58 ? "#241F18" : "#FFFFFF");
  root.style.setProperty("--menu-tint", menuTint);
  const menuDark = luminance(menuTint) < 0.48;
  root.style.setProperty("--menu-ink", menuDark ? "#F8F1E4" : "#24211B");
  root.style.setProperty("--menu-muted", menuDark ? "rgba(248, 241, 228, 0.68)" : "#7C7469");

  const themeMeta = $('meta[name="theme-color"]');
  if (themeMeta) themeMeta.setAttribute("content", menuTint);

  accentCustom.value = accent;
  menuCustom.value = menuTint;
  $$(".swatch[data-accent]").forEach((swatch) => {
    swatch.classList.toggle("is-active", swatch.dataset.accent.toUpperCase() === accent);
  });
  $$(".swatch[data-menu]").forEach((swatch) => {
    swatch.classList.toggle("is-active", swatch.dataset.menu.toUpperCase() === menuTint);
  });

  if (persist) {
    localStorage.setItem(STORAGE_THEME, JSON.stringify({ accent, menuTint }));
  }

  if (saveServer) {
    scheduleThemeSave();
  } else if (!state.themeInitialized) {
    state.themeInitialized = true;
  }
}

function scheduleThemeSave() {
  if (!state.user) {
    themeStatus.textContent = "Сохранено в этом браузере";
    return;
  }
  themeStatus.textContent = "Сохраняем...";
  window.clearTimeout(state.themeTimer);
  state.themeTimer = window.setTimeout(async () => {
    try {
      await api("/api/preferences", {
        method: "PUT",
        body: state.preferences
      });
      themeStatus.textContent = "Сохранено в аккаунте";
    } catch (error) {
      themeStatus.textContent = "Не удалось сохранить";
    }
  }, 420);
}

function loadStoredTheme() {
  try {
    const stored = JSON.parse(localStorage.getItem(STORAGE_THEME) || "null");
    if (stored && stored.accent && stored.menuTint) {
      applyTheme(stored, { persist: false });
      return;
    }
  } catch (error) {
  }
  applyTheme({ accent: DEFAULT_ACCENT, menuTint: DEFAULT_MENU }, { persist: false });
}

function setDefaultDates() {
  const dates = defaultDates();
  searchCheckIn.value = dates.checkIn;
  searchCheckOut.value = dates.checkOut;
  searchCheckIn.min = todayISO();
  searchCheckOut.min = dates.checkIn;
  state.search.checkIn = dates.checkIn;
  state.search.checkOut = dates.checkOut;

  try {
    const stored = JSON.parse(localStorage.getItem(STORAGE_SEARCH) || "null");
    if (stored && stored.checkIn && stored.checkOut && stored.checkIn >= todayISO()) {
      searchCheckIn.value = stored.checkIn;
      searchCheckOut.value = stored.checkOut;
      state.search.checkIn = stored.checkIn;
      state.search.checkOut = stored.checkOut;
      if (stored.guests) {
        searchGuests.value = String(stored.guests);
        state.search.guests = Number(stored.guests);
      }
    }
  } catch (error) {
  }
}

function persistSearch() {
  localStorage.setItem(
    STORAGE_SEARCH,
    JSON.stringify({
      checkIn: state.search.checkIn,
      checkOut: state.search.checkOut,
      guests: state.search.guests
    })
  );
}

function findDestinationByText(text) {
  const lower = String(text || "").trim().toLowerCase();
  if (!lower) return null;
  return (
    state.destinations.find((destination) => {
      if (destination.slug === lower || destination.city.toLowerCase() === lower) return true;
      const aliases = cityAliases[destination.slug] || [];
      return aliases.some((alias) => lower.includes(alias));
    }) || null
  );
}

function inferTag(text) {
  const lower = String(text || "").toLowerCase();
  if (lower.includes("тих")) return "тишина";
  if (lower.includes("мор")) return "море";
  if (lower.includes("дизайн")) return "дизайн";
  if (lower.includes("завтрак")) return "завтраки";
  if (lower.includes("гор")) return "горы";
  if (lower.includes("спа")) return "спа";
  if (lower.includes("семь")) return "семья";
  return "";
}

function renderDestinations() {
  destinationsTrack.innerHTML = state.destinations
    .map(
      (destination) => `
        <button type="button" class="destination-card" data-destination="${escapeHTML(destination.slug)}">
          <img src="${escapeHTML(destination.image)}" alt="${escapeHTML(destination.city)}" loading="lazy">
          <span class="destination-content">
            <span>${escapeHTML(destination.country)}</span>
            <h3>${escapeHTML(destination.city)}</h3>
            <p>${escapeHTML(destination.tagline)}</p>
            <span class="destination-meta">
              <strong>от ${money(destination.priceFrom)}</strong>
              <span class="destination-arrow">
                <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 12h14"/><path d="M13 6l6 6-6 6"/></svg>
              </span>
            </span>
          </span>
        </button>
      `
    )
    .join("");

  destinationOptions.innerHTML = state.destinations
    .map((destination) => `<option value="${escapeHTML(destination.city)}"></option>`)
    .join("");
}

function hotelCardMarkup(hotel) {
  return `
    <article class="hotel-card" data-hotel-slug="${escapeHTML(hotel.slug)}" role="link" tabindex="0" aria-label="${escapeHTML(hotel.name)}">
      <div class="hotel-card-image">
        <img src="${escapeHTML(hotel.image)}" alt="${escapeHTML(hotel.name)}" loading="lazy">
        <button type="button" class="favorite-button${hotel.favorited ? " is-active" : ""}" data-favorite="${hotel.id}" aria-label="${hotel.favorited ? "Убрать из избранного" : "Добавить в избранное"}">
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M20.8 5.6a5 5 0 0 0-7.1 0L12 7.3l-1.7-1.7a5 5 0 1 0-7.1 7.1L12 21l8.8-8.3a5 5 0 0 0 0-7.1Z"/></svg>
        </button>
        <span class="hotel-rating">${Number(hotel.rating).toFixed(1)} · ${hotel.reviews} ${plural(hotel.reviews, "отзыв", "отзыва", "отзывов")}</span>
      </div>
      <div class="hotel-card-body">
        <span class="hotel-location">${escapeHTML(hotel.city)} · ${escapeHTML(hotel.country)}</span>
        <h3>${escapeHTML(hotel.name)}</h3>
        <p>${escapeHTML(hotel.shortDescription)}</p>
        <div class="hotel-tags">
          ${(hotel.tags || []).slice(0, 3).map((tag) => `<span class="hotel-tag">${escapeHTML(tag)}</span>`).join("")}
        </div>
        <div class="hotel-card-foot">
          <div class="price">
            <strong>${money(hotel.priceFrom)}</strong>
            <span>за ночь</span>
          </div>
          <span class="card-link">
            Смотреть
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 12h14"/><path d="M13 6l6 6-6 6"/></svg>
          </span>
        </div>
      </div>
    </article>
  `;
}

function renderHotelCards(hotels, container) {
  if (!container) return;
  if (!hotels || !hotels.length) {
    container.innerHTML = "";
    return;
  }
  container.innerHTML = hotels.map(hotelCardMarkup).join("");
}

function renderHome(home) {
  state.destinations = home.destinations || [];
  state.featured = home.featured || [];
  state.stats = home.stats || null;
  renderDestinations();
  renderHotelCards(state.featured, featuredGrid);
  if (home.stats) {
    $("#statHotels").textContent = String(home.stats.hotelCount);
    $("#statDestinations").textContent = String(home.stats.destinationCount);
    $("#statRating").textContent = Number(home.stats.averageRating).toFixed(1).replace(".", ",");
  }
}

async function loadHome() {
  try {
    const home = await api("/api/home");
    renderHome(home);
  } catch (error) {
    showToast(error.message, "error");
    destinationsTrack.innerHTML = '<div class="empty-state"><h3>Не удалось загрузить направления</h3><p>Перезапустите приложение и обновите страницу.</p></div>';
    featuredGrid.innerHTML = "";
  }
}

function setActiveNav(name) {
  $$("[data-nav]").forEach((item) => {
    if (!item.classList.contains("nav-link")) return;
    item.classList.toggle("is-active", item.dataset.nav === name);
  });
}

function showView(name, options = {}) {
  [homeView, searchView, hotelView, tripsView].forEach((view) => view.classList.add("hidden"));
  const view = document.getElementById(`${name}View`);
  if (view) view.classList.remove("hidden");
  state.currentView = name;

  if (name === "home") setActiveNav("home");
  else if (name === "search" || name === "hotel") setActiveNav("search");
  else if (name === "trips") setActiveNav("trips");

  if (options.scroll !== false) {
    window.scrollTo({ top: 0, behavior: "smooth" });
  }
}

function navigate(name) {
  closeAccountMenu();
  closeThemePanel();
  closeMobileMenu();

  if (name === "assistant") {
    openAssistant();
    return;
  }
  if (name === "trips") {
    showTrips("upcoming");
    return;
  }
  if (name === "search") {
    openSearchFromForm();
    return;
  }
  showView("home");
}

function openSearchFromForm() {
  const destinationText = searchDestination.value.trim();
  const destination = findDestinationByText(destinationText);
  const inferredTag = inferTag(destinationText);
  const checkIn = searchCheckIn.value || state.search.checkIn;
  const checkOut = searchCheckOut.value || state.search.checkOut;
  const guests = Number(searchGuests.value || state.search.guests || 2);

  state.search = {
    q: destination ? "" : inferredTag ? "" : destinationText,
    destination: destination ? destination.slug : "",
    destinationName: destination ? destination.city : "",
    checkIn,
    checkOut,
    guests,
    tag: inferredTag,
    maxPrice: 0,
    sort: ""
  };
  persistSearch();
  showView("search");
  performSearch();
}

function openSearchView(options = {}) {
  const destination = options.destination ? state.destinations.find((item) => item.slug === options.destination) : null;
  state.search = {
    q: options.q || "",
    destination: options.destination || "",
    destinationName: destination ? destination.city : options.destinationName || "",
    checkIn: state.search.checkIn || searchCheckIn.value || defaultDates().checkIn,
    checkOut: state.search.checkOut || searchCheckOut.value || defaultDates().checkOut,
    guests: state.search.guests || Number(searchGuests.value || 2),
    tag: options.tag || "",
    maxPrice: options.maxPrice || 0,
    sort: options.sort || ""
  };
  showView("search");
  performSearch();
}

function setFilterUI() {
  $$("[data-filter-tag]").forEach((chip) => {
    chip.classList.toggle("is-active", chip.dataset.filterTag === state.search.tag);
  });
  $("#priceFilter").value = String(state.search.maxPrice || 0);
  $("#sortSelect").value = state.search.sort || "";
}

function buildSearchQuery() {
  const params = new URLSearchParams();
  if (state.search.q) params.set("q", state.search.q);
  if (state.search.destination) params.set("destination", state.search.destination);
  if (state.search.guests) params.set("guests", String(state.search.guests));
  if (state.search.tag) params.set("tag", state.search.tag);
  if (state.search.maxPrice) params.set("maxPrice", String(state.search.maxPrice));
  if (state.search.sort) params.set("sort", state.search.sort);
  return params.toString();
}

async function performSearch() {
  setFilterUI();
  searchGrid.innerHTML = Array.from({ length: 6 }, () => '<div class="hotel-card skeleton-card"></div>').join("");
  searchEmpty.classList.add("hidden");
  searchMeta.textContent = "Ищем подходящие отели...";
  state.searchLoading = true;

  const destination = state.search.destination
    ? state.destinations.find((item) => item.slug === state.search.destination)
    : null;
  if (destination) state.search.destinationName = destination.city;

  const heading = destination
    ? destination.city
    : state.search.q
      ? "Отели по запросу"
      : state.search.tag
        ? `Отели: ${state.search.tag}`
        : "Все отели";
  searchTitle.textContent = heading;
  searchKicker.textContent = state.search.q ? `Поиск: «${state.search.q}»` : "Подборка";
  searchSummary.textContent = `${formatDateRange(state.search.checkIn, state.search.checkOut)} · ${state.search.guests} ${plural(state.search.guests, "гость", "гостя", "гостей")}`;

  try {
    const result = await api(`/api/search?${buildSearchQuery()}`);
    state.searchResults = result.hotels || [];
    renderHotelCards(state.searchResults, searchGrid);
    searchMeta.textContent = `${result.total} ${plural(result.total, "отель", "отеля", "отелей")} · ${formatDateRange(state.search.checkIn, state.search.checkOut)}`;
    searchEmpty.classList.toggle("hidden", result.total !== 0);
  } catch (error) {
    searchGrid.innerHTML = "";
    searchMeta.textContent = "";
    searchEmpty.classList.remove("hidden");
    showToast(error.message, "error");
  } finally {
    state.searchLoading = false;
  }
}

async function searchByDestination(slug) {
  const destination = state.destinations.find((item) => item.slug === slug);
  if (destination) {
    searchDestination.value = destination.city;
  }
  openSearchView({ destination: slug, destinationName: destination ? destination.city : "" });
}

function applyVibe(tag) {
  if (!state.destinations.length) {
    showToast("Подождите, загружаем направления.", "info");
    return;
  }
  openSearchView({ tag });
}

async function openHotel(slug) {
  if (!slug) return;
  state.previousView = state.currentView === "search" ? "search" : "home";
  state.activeHotelSlug = slug;
  state.activeHotel = null;
  closeAssistant();
  showView("hotel");
  $("#hotelContent").innerHTML = `
    <div class="hotel-detail-shell">
      <div class="hotel-gallery">
        <div class="hotel-gallery-image skeleton-card"></div>
        <div class="hotel-gallery-image skeleton-card"></div>
        <div class="hotel-gallery-image skeleton-card"></div>
      </div>
    </div>
  `;

  const params = new URLSearchParams();
  if (state.search.checkIn) params.set("checkIn", state.search.checkIn);
  if (state.search.checkOut) params.set("checkOut", state.search.checkOut);
  if (state.search.guests) params.set("guests", String(state.search.guests));

  try {
    const hotel = await api(`/api/hotels/${encodeURIComponent(slug)}?${params.toString()}`);
    state.activeHotel = hotel;
    renderHotelDetail(hotel);
  } catch (error) {
    showToast(error.message, "error");
    if (state.previousView === "search") showView("search");
    else showView("home");
  }
}

function amenityIcon() {
  return '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M20 6L9 17l-5-5"/></svg>';
}

function renderHotelDetail(hotel) {
  const gallery = [hotel.image, ...(hotel.gallery || [])].filter(Boolean);
  const images = [gallery[0], gallery[1] || gallery[0], gallery[2] || gallery[0]];
  const checkIn = hotel.checkIn || state.search.checkIn || defaultDates().checkIn;
  const checkOut = hotel.checkOut || state.search.checkOut || defaultDates().checkOut;
  const guests = hotel.guests || state.search.guests || 2;
  const nights = nightsBetween(checkIn, checkOut);

  const roomMarkup = (hotel.rooms || [])
    .map((room) => {
      const available = Number(room.available ?? room.totalUnits ?? 1);
      const availabilityClass = available <= 0 ? "is-out" : available <= 2 ? "is-low" : "";
      const availabilityText = available <= 0 ? "Нет свободных номеров" : available === 1 ? "Остался 1 номер" : `Свободно ${available} ${plural(available, "номер", "номера", "номеров")}`;
      return `
        <article class="room-card">
          <div class="room-image"><img src="${escapeHTML(room.image)}" alt="${escapeHTML(room.name)}" loading="lazy"></div>
          <div class="room-info">
            <h3>${escapeHTML(room.name)}</h3>
            <p>${escapeHTML(room.description)}</p>
            <div class="room-features">
              <span>${escapeHTML(room.bed)}</span>
              <span>${room.size} м²</span>
              <span>до ${room.capacity} ${plural(room.capacity, "гостя", "гостей", "гостей")}</span>
              ${(room.amenities || []).slice(0, 2).map((item) => `<span>${escapeHTML(item)}</span>`).join("")}
            </div>
            <span class="room-availability ${availabilityClass}">
              <span class="status-dot"></span>
              ${availabilityText}
            </span>
          </div>
          <div class="room-book">
            <div class="room-price">
              <strong>${money(room.price)}</strong>
              <span>за ночь</span>
            </div>
            <button type="button" class="primary-button" data-book-room="${room.id}" ${available <= 0 ? "disabled" : ""}>Забронировать</button>
          </div>
        </article>
      `;
    })
    .join("");

  const nearbyMarkup = (hotel.nearby || [])
    .map(
      (place) => `
        <article class="nearby-card">
          <span class="nearby-category">${escapeHTML(categoryLabels[place.category] || place.category)}</span>
          <h3>${escapeHTML(place.name)}</h3>
          <p>${escapeHTML(place.description)}</p>
          <div class="nearby-meta">
            <span>${escapeHTML(place.address)}</span>
            <span>${escapeHTML(place.distance)}</span>
          </div>
        </article>
      `
    )
    .join("");

  const favoriteLabel = hotel.favorited ? "В избранном" : "В избранное";

  $("#hotelContent").innerHTML = `
    <div class="hotel-detail-shell">
      <div class="hotel-gallery">
        ${images.map((image, index) => `<div class="hotel-gallery-image"><img src="${escapeHTML(image)}" alt="${escapeHTML(hotel.name)} — фото ${index + 1}" ${index === 0 ? "" : 'loading="lazy"'}></div>`).join("")}
      </div>
      <div class="hotel-detail-layout">
        <div class="hotel-detail-main">
          <div class="hotel-detail-topline">
            <span>${escapeHTML(hotel.city)} · ${escapeHTML(hotel.country)}</span>
            <span class="hotel-rating-inline">★ ${Number(hotel.rating).toFixed(1)}</span>
            <span>${hotel.reviews} ${plural(hotel.reviews, "отзыв", "отзыва", "отзывов")}</span>
          </div>
          <div class="hotel-title-row">
            <h1 class="hotel-detail-title">${escapeHTML(hotel.name)}</h1>
            <button type="button" class="secondary-button detail-favorite${hotel.favorited ? " is-active" : ""}" data-favorite="${hotel.id}">
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M20.8 5.6a5 5 0 0 0-7.1 0L12 7.3l-1.7-1.7a5 5 0 1 0-7.1 7.1L12 21l8.8-8.3a5 5 0 0 0 0-7.1Z"/></svg>
              <span>${favoriteLabel}</span>
            </button>
          </div>
          <p class="hotel-detail-address">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 21s6-5.1 6-11a6 6 0 1 0-12 0c0 5.9 6 11 6 11Z"/><circle cx="12" cy="10" r="2"/></svg>
            ${escapeHTML(hotel.address)}, ${escapeHTML(hotel.city)}
          </p>
          <div class="hotel-detail-tags">
            ${(hotel.tags || []).map((tag) => `<span>${escapeHTML(tag)}</span>`).join("")}
          </div>

          <section class="detail-section">
            <h2>Об отеле</h2>
            <p>${escapeHTML(hotel.description)}</p>
          </section>

          <section class="detail-section">
            <h2>Всё для спокойного отдыха</h2>
            <div class="amenity-grid">
              ${(hotel.amenities || []).map((amenity) => `<div class="amenity-item"><span class="amenity-icon">${amenityIcon()}</span>${escapeHTML(amenity)}</div>`).join("")}
            </div>
          </section>

          <section class="detail-section" id="roomSection">
            <h2>Номера</h2>
            <div class="room-list">${roomMarkup}</div>
          </section>

          <section class="detail-section">
            <h2>Рядом с отелем</h2>
            <div class="nearby-grid">${nearbyMarkup}</div>
          </section>
        </div>

        <aside class="booking-side">
          <div class="booking-side-head">
            <strong>${money(hotel.priceFrom)}</strong>
            <span>за ночь</span>
          </div>
          <div class="booking-side-rows">
            <div class="booking-side-row"><span>Заезд</span><strong>${formatDateLong(checkIn)}</strong></div>
            <div class="booking-side-row"><span>Выезд</span><strong>${formatDateLong(checkOut)}</strong></div>
            <div class="booking-side-row"><span>Гости</span><strong>${guests} ${plural(guests, "гость", "гостя", "гостей")}</strong></div>
            <div class="booking-side-row"><span>Ночей</span><strong>${nights}</strong></div>
          </div>
          <button type="button" class="primary-button full-width" data-room-scroll>Выбрать номер</button>
          <p class="booking-side-note">Бесплатная отмена до заезда. Оплата демонстрационная — данные карты не сохраняются.</p>
        </aside>
      </div>
    </div>
  `;
}

function openBooking(hotel, room) {
  if (!hotel || !room) return;
  if (!state.user) {
    state.pendingBooking = { hotel, room };
    openAuth("login", "Войдите, чтобы завершить бронирование. После входа вернёмся к выбранному номеру.");
    return;
  }

  const dates = defaultDates();
  let checkIn = state.search.checkIn || dates.checkIn;
  let checkOut = state.search.checkOut || dates.checkOut;
  if (checkIn < todayISO()) checkIn = dates.checkIn;
  if (!checkOut || checkOut <= checkIn) checkOut = toISODate(addDays(new Date(`${checkIn}T12:00:00`), 3));

  const guests = Math.min(Math.max(Number(state.search.guests || 2), 1), room.capacity);

  state.booking = { hotel, room, checkIn, checkOut, guests };
  state.bookingStep = 1;

  $("#bookingRoomSummary").innerHTML = `
    <img src="${escapeHTML(room.image)}" alt="${escapeHTML(room.name)}">
    <div>
      <strong>${escapeHTML(room.name)}</strong>
      <span>${escapeHTML(hotel.name)} · ${escapeHTML(hotel.city)} · до ${room.capacity} ${plural(room.capacity, "гостя", "гостей", "гостей")}</span>
    </div>
  `;

  const guestSelect = $("#bookingGuests");
  guestSelect.innerHTML = Array.from({ length: room.capacity }, (_, index) => {
    const count = index + 1;
    return `<option value="${count}" ${count === guests ? "selected" : ""}>${count} ${plural(count, "гость", "гостя", "гостей")}</option>`;
  }).join("");

  const today = todayISO();
  $("#bookingCheckIn").min = today;
  $("#bookingCheckOut").min = checkIn;
  $("#bookingCheckIn").value = checkIn;
  $("#bookingCheckOut").value = checkOut;
  $("#bookingName").value = state.user.name || "";
  $("#bookingEmail").value = state.user.email || "";
  $("#bookingPhone").value = "";
  $("#cardNumber").value = "4242 4242 4242 4242";
  $("#cardExpiry").value = "12 / 30";
  $("#cardCvc").value = "123";
  $("#bookingError1").classList.add("hidden");
  $("#bookingError2").classList.add("hidden");
  $("#bookingError3").classList.add("hidden");
  $("#bookingForm").classList.remove("hidden");
  $("#bookingSuccess").classList.add("hidden");
  $("#bookingSteps").classList.remove("hidden");

  updateBookingTotal();
  setBookingStep(1);
  openModal("bookingModal");
  window.setTimeout(() => $("#bookingCheckIn").focus(), 80);
}

function setBookingStep(step) {
  state.bookingStep = step;
  $$(".booking-step-panel").forEach((panel) => {
    panel.classList.toggle("hidden", Number(panel.dataset.step) !== step);
  });
  $$("[data-step-indicator]").forEach((indicator) => {
    const indicatorStep = Number(indicator.dataset.stepIndicator);
    indicator.classList.toggle("is-active", indicatorStep === step);
    indicator.classList.toggle("is-done", indicatorStep < step);
  });
}

function updateBookingTotal() {
  if (!state.booking) return;
  const checkIn = $("#bookingCheckIn").value;
  const checkOut = $("#bookingCheckOut").value;
  const nights = nightsBetween(checkIn, checkOut);
  const total = nights * state.booking.room.price;
  $("#bookingTotalStep1").textContent = nights > 0 ? `${money(total)} · ${nights} ${plural(nights, "ночь", "ночи", "ночей")}` : "—";
  $("#bookingTotalStep3").textContent = total > 0 ? money(total) : "—";
  $("#paymentHotel").textContent = state.booking.hotel.name;
  $("#paymentRoom").textContent = state.booking.room.name;
  $("#paymentDates").textContent = `${formatDateRange(checkIn, checkOut)} · ${$("#bookingGuests").value} ${plural(Number($("#bookingGuests").value), "гость", "гостя", "гостей")}`;
}

function validateBookingDates() {
  const checkIn = $("#bookingCheckIn").value;
  const checkOut = $("#bookingCheckOut").value;
  const guests = Number($("#bookingGuests").value);
  const error = $("#bookingError1");
  error.classList.add("hidden");

  if (!checkIn || !checkOut) {
    error.textContent = "Выберите даты заезда и выезда.";
    error.classList.remove("hidden");
    return false;
  }
  if (checkIn < todayISO()) {
    error.textContent = "Дата заезда не может быть в прошлом.";
    error.classList.remove("hidden");
    return false;
  }
  if (nightsBetween(checkIn, checkOut) < 1) {
    error.textContent = "Дата выезда должна быть позже заезда.";
    error.classList.remove("hidden");
    return false;
  }
  if (guests > state.booking.room.capacity) {
    error.textContent = `Этот номер вмещает до ${state.booking.room.capacity} гостей.`;
    error.classList.remove("hidden");
    return false;
  }
  state.booking.checkIn = checkIn;
  state.booking.checkOut = checkOut;
  state.booking.guests = guests;
  return true;
}

function validateGuestDetails() {
  const name = $("#bookingName").value.trim();
  const email = $("#bookingEmail").value.trim();
  const phone = $("#bookingPhone").value.trim();
  const error = $("#bookingError2");
  error.classList.add("hidden");

  if (name.length < 2) {
    error.textContent = "Укажите имя гостя.";
    error.classList.remove("hidden");
    $("#bookingName").focus();
    return false;
  }
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
    error.textContent = "Укажите корректный email.";
    error.classList.remove("hidden");
    $("#bookingEmail").focus();
    return false;
  }
  if (phone.length < 5) {
    error.textContent = "Укажите телефон для связи.";
    error.classList.remove("hidden");
    $("#bookingPhone").focus();
    return false;
  }
  return true;
}

function validatePayment() {
  const digits = $("#cardNumber").value.replace(/\D/g, "");
  const expiry = $("#cardExpiry").value.replace(/\D/g, "");
  const cvc = $("#cardCvc").value.replace(/\D/g, "");
  const error = $("#bookingError3");
  error.classList.add("hidden");

  if (digits.length !== 16) {
    error.textContent = "Введите 16 цифр карты. Для демо используйте 4242 4242 4242 4242.";
    error.classList.remove("hidden");
    $("#cardNumber").focus();
    return false;
  }
  if (expiry.length !== 4) {
    error.textContent = "Введите срок действия в формате ММ / ГГ.";
    error.classList.remove("hidden");
    $("#cardExpiry").focus();
    return false;
  }
  const month = Number(expiry.slice(0, 2));
  if (month < 1 || month > 12) {
    error.textContent = "Проверьте месяц окончания карты.";
    error.classList.remove("hidden");
    $("#cardExpiry").focus();
    return false;
  }
  if (cvc.length < 3) {
    error.textContent = "Введите CVC.";
    error.classList.remove("hidden");
    $("#cardCvc").focus();
    return false;
  }
  return true;
}

async function submitBooking() {
  if (!state.booking) return;
  if (!validatePayment()) return;

  const button = $("#bookingSubmit");
  setButtonLoading(button, true, "Оплачиваем...");
  try {
    const trip = await api("/api/bookings", {
      method: "POST",
      body: {
        hotelId: state.booking.hotel.id,
        roomId: state.booking.room.id,
        checkIn: state.booking.checkIn,
        checkOut: state.booking.checkOut,
        guests: state.booking.guests,
        guestName: $("#bookingName").value.trim(),
        guestEmail: $("#bookingEmail").value.trim(),
        guestPhone: $("#bookingPhone").value.trim()
      }
    });

    state.trips = null;
    $("#bookingForm").classList.add("hidden");
    $("#bookingSteps").classList.add("hidden");
    $("#bookingSuccess").classList.remove("hidden");
    $("#bookingSuccessCode").textContent = trip.code;
    $("#bookingSuccessText").textContent = `${trip.hotelName}, ${trip.roomName}. ${formatDateRange(trip.checkIn, trip.checkOut)} · ${trip.nights} ${plural(trip.nights, "ночь", "ночи", "ночей")}.`;
    showToast(`Бронирование ${trip.code} подтверждено.`);
  } catch (error) {
    const errorBox = $("#bookingError3");
    errorBox.textContent = error.message;
    errorBox.classList.remove("hidden");
  } finally {
    setButtonLoading(button, false);
  }
}

function openModal(id) {
  const modal = document.getElementById(id);
  if (!modal) return;
  modal.classList.add("is-open");
  modal.setAttribute("aria-hidden", "false");
  syncBodyLock();
}

function closeModal(id) {
  const modal = document.getElementById(id);
  if (!modal) return;
  modal.classList.remove("is-open");
  modal.setAttribute("aria-hidden", "true");
  syncBodyLock();
}

function openAuth(mode = "login", notice = "") {
  closeAccountMenu();
  closeThemePanel();
  closeAssistant();
  closeMobileMenu();
  setAuthMode(mode);
  $("#authError").classList.add("hidden");
  const noticeElement = $("#authNotice");
  noticeElement.textContent = notice;
  noticeElement.classList.toggle("hidden", !notice);
  openModal("authModal");
  window.setTimeout(() => {
    if (mode === "register") $("#registerName").focus();
    else $("#loginEmail").focus();
  }, 120);
}

function closeAuth(clearPending = true) {
  closeModal("authModal");
  if (clearPending) {
    state.pendingBooking = null;
    state.pendingFavoriteId = null;
  }
  $("#authError").classList.add("hidden");
}

function setAuthMode(mode) {
  state.authMode = mode;
  $$("[data-auth-mode]").forEach((tab) => {
    const active = tab.dataset.authMode === mode;
    tab.classList.toggle("is-active", active);
    tab.setAttribute("aria-selected", String(active));
  });
  $("#loginForm").classList.toggle("hidden", mode !== "login");
  $("#registerForm").classList.toggle("hidden", mode !== "register");
  $("#authKicker").textContent = mode === "login" ? "Личный кабинет" : "Новый аккаунт";
  $("#authTitle").textContent = mode === "login" ? "С возвращением" : "Создать аккаунт";
  $("#authSubtitle").textContent =
    mode === "login"
      ? "Войдите, чтобы бронировать и сохранять поездки."
      : "Пара полей — и ваши поездки будут всегда под рукой.";
}

function setAuthBusy(busy, activeButton) {
  state.authBusy = busy;
  $$("[data-auth-mode]").forEach((tab) => {
    tab.disabled = busy;
  });
  ["#loginSubmit", "#registerSubmit", "#demoLoginButton"].forEach((selector) => {
    const button = $(selector);
    if (!button || button === activeButton) return;
    button.disabled = busy;
  });
  if (activeButton) setButtonLoading(activeButton, busy, "Подключаем...");
  if (!busy && activeButton) setButtonLoading(activeButton, false);
}

function updateAccountUI() {
  const loggedIn = Boolean(state.user);
  const name = loggedIn ? state.user.name : "Гость";
  const email = loggedIn ? state.user.email : "Войдите, чтобы сохранять поездки";
  accountAvatar.textContent = loggedIn ? initials(name) : "Г";
  menuAvatar.textContent = loggedIn ? initials(name) : "Г";
  accountLabel.textContent = loggedIn ? firstName(name) : "Войти";
  menuName.textContent = name;
  menuEmail.textContent = email;
  accountButton.setAttribute("aria-label", loggedIn ? "Меню аккаунта" : "Войти в аккаунт");
}

async function restoreSession() {
  try {
    const payload = await api("/api/auth/me");
    state.user = payload.user;
    state.preferences = payload.preferences || state.preferences;
    applyTheme(payload.preferences || state.preferences, { persist: true, saveServer: false });
    updateAccountUI();
  } catch (error) {
    state.user = null;
    updateAccountUI();
  }
}

async function completeAuth(payload) {
  state.user = payload.user;
  state.preferences = payload.preferences || state.preferences;
  state.trips = null;
  state.favorites = null;
  applyTheme(state.preferences, { persist: true, saveServer: false });
  updateAccountUI();
  const pendingBooking = state.pendingBooking;
  const pendingFavoriteId = state.pendingFavoriteId;
  state.pendingBooking = null;
  state.pendingFavoriteId = null;
  closeAuth(false);
  showToast(`Добро пожаловать, ${firstName(state.user.name)}.`);
  if (pendingBooking) {
    openBooking(pendingBooking.hotel, pendingBooking.room);
    loadHome();
  } else if (pendingFavoriteId) {
    await toggleFavorite(pendingFavoriteId);
    await loadHome();
  } else {
    await loadHome();
  }
}

async function logout() {
  closeAccountMenu();
  try {
    await api("/api/auth/logout", { method: "POST" });
  } catch (error) {
  }
  state.user = null;
  state.trips = null;
  state.favorites = null;
  updateAccountUI();
  showToast("Вы вышли из аккаунта.", "info");
  await loadHome();
  if (state.currentView === "trips") renderTrips();
}

function closeAccountMenu() {
  accountMenu.classList.remove("is-open");
  accountButton.setAttribute("aria-expanded", "false");
}

function openThemePanel() {
  closeAccountMenu();
  closeMobileMenu();
  themePanel.classList.add("is-open");
  themePanel.setAttribute("aria-hidden", "false");
  themeButton.setAttribute("aria-expanded", "true");
}

function closeThemePanel() {
  themePanel.classList.remove("is-open");
  themePanel.setAttribute("aria-hidden", "true");
  themeButton.setAttribute("aria-expanded", "false");
}

function openMobileMenu() {
  closeAccountMenu();
  closeThemePanel();
  mobileMenu.classList.add("is-open");
  mobileMenu.setAttribute("aria-hidden", "false");
  syncBodyLock();
}

function closeMobileMenu() {
  mobileMenu.classList.remove("is-open");
  mobileMenu.setAttribute("aria-hidden", "true");
  syncBodyLock();
}

function openCommandPalette() {
  closeAccountMenu();
  closeThemePanel();
  closeMobileMenu();
  closeAssistant();
  commandPalette.classList.add("is-open");
  commandPalette.setAttribute("aria-hidden", "false");
  commandInput.value = "";
  state.commandQuery = "";
  state.commandExtra = [];
  state.commandIndex = 0;
  renderCommandResults("");
  syncBodyLock();
  window.setTimeout(() => commandInput.focus(), 80);
}

function closeCommandPalette() {
  commandPalette.classList.remove("is-open");
  commandPalette.setAttribute("aria-hidden", "true");
  syncBodyLock();
}

function commandIcon(name) {
  const icons = {
    home: '<svg viewBox="0 0 24 24"><path d="M3 11l9-8 9 8"/><path d="M5 10v10h14V10"/></svg>',
    search: '<svg viewBox="0 0 24 24"><circle cx="11" cy="11" r="6.5"/><path d="M16 16l4 4"/></svg>',
    spark: '<svg viewBox="0 0 24 24"><path d="M12 3l1.7 4.6L18 9.3l-4.3 1.7L12 15.6l-1.7-4.6L6 9.3l4.3-1.7L12 3Z"/></svg>',
    trips: '<svg viewBox="0 0 24 24"><path d="M4 8h16v11H4z"/><path d="M8 8V6a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/><path d="M4 13h16"/></svg>',
    theme: '<svg viewBox="0 0 24 24"><path d="M12 3a9 9 0 1 0 0 18h1.5a2.5 2.5 0 0 0 0-5H13a1.5 1.5 0 0 1 0-3h4.5A3.5 3.5 0 0 0 21 9.5C21 5.9 17 3 12 3Z"/></svg>',
    login: '<svg viewBox="0 0 24 24"><path d="M10 17l5-5-5-5"/><path d="M15 12H3"/><path d="M14 4h5a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2h-5"/></svg>',
    logout: '<svg viewBox="0 0 24 24"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><path d="M16 17l5-5-5-5"/><path d="M21 12H9"/></svg>',
    hotel: '<svg viewBox="0 0 24 24"><path d="M4 20V8l8-4 8 4v12"/><path d="M8 20v-5h8v5"/></svg>'
  };
  return icons[name] || icons.search;
}

function baseCommandActions() {
  return [
    {
      group: "Действия",
      label: "Главная",
      hint: "Вдохновение и подборки",
      icon: "home",
      run: () => showView("home")
    },
    {
      group: "Действия",
      label: "Найти отель",
      hint: "Поиск по направлениям и вайбу",
      icon: "search",
      run: () => {
        showView("search");
        performSearch();
      }
    },
    {
      group: "Действия",
      label: "Открыть AI-консьержа",
      hint: "Локальная модель Qwen 2.5 3B",
      icon: "spark",
      run: () => openAssistant()
    },
    {
      group: "Действия",
      label: "Мои поездки",
      hint: "Брони и избранное",
      icon: "trips",
      run: () => showTrips("upcoming")
    },
    {
      group: "Действия",
      label: "Настроить цвета меню",
      hint: "Акцент и фон навигации",
      icon: "theme",
      run: () => openThemePanel()
    },
    state.user
      ? {
          group: "Аккаунт",
          label: "Выйти из аккаунта",
          hint: state.user.email,
          icon: "logout",
          run: () => logout()
        }
      : {
          group: "Аккаунт",
          label: "Войти в аккаунт",
          hint: "Демо-доступ доступен сразу",
          icon: "login",
          run: () => openAuth("login")
        }
  ];
}

function commandHotelItem(hotel) {
  return {
    group: "Отели",
    label: hotel.name,
    hint: `${hotel.city} · от ${money(hotel.priceFrom)}`,
    icon: "hotel",
    image: hotel.image,
    run: () => openHotel(hotel.slug)
  };
}

function renderCommandResults(query) {
  const normalized = String(query || "").trim().toLowerCase();
  const actions = baseCommandActions().filter((item) => {
    if (!normalized) return true;
    return `${item.label} ${item.hint}`.toLowerCase().includes(normalized);
  });
  const destinations = state.destinations
    .filter((destination) => {
      if (!normalized) return true;
      return `${destination.city} ${destination.country} ${destination.tagline}`.toLowerCase().includes(normalized);
    })
    .map((destination) => ({
      group: "Направления",
      label: destination.city,
      hint: destination.tagline,
      icon: "search",
      image: destination.image,
      run: () => searchByDestination(destination.slug)
    }));
  const featured = state.featured
    .filter((hotel) => {
      if (!normalized) return false;
      return `${hotel.name} ${hotel.city} ${(hotel.tags || []).join(" ")}`.toLowerCase().includes(normalized);
    })
    .map(commandHotelItem);
  const extra = (state.commandExtra || []).filter((item) => {
    if (!normalized) return false;
    return `${item.label} ${item.hint}`.toLowerCase().includes(normalized);
  });

  state.commandItems = [...actions, ...destinations, ...featured, ...extra];
  state.commandIndex = Math.min(state.commandIndex, Math.max(state.commandItems.length - 1, 0));

  if (!state.commandItems.length) {
    commandResults.innerHTML = '<div class="command-empty">Ничего не нашлось. Попробуйте название города или отеля.</div>';
    return;
  }

  let lastGroup = "";
  commandResults.innerHTML = state.commandItems
    .map((item, index) => {
      const groupHeading = item.group !== lastGroup ? `<div class="command-group">${escapeHTML(item.group)}</div>` : "";
      lastGroup = item.group;
      const iconMarkup = item.image
        ? `<img src="${escapeHTML(item.image)}" alt="">`
        : commandIcon(item.icon);
      return `
        ${groupHeading}
        <button type="button" class="command-result${index === state.commandIndex ? " is-selected" : ""}" data-command-index="${index}" role="option" aria-selected="${index === state.commandIndex}">
          <span class="command-result-icon">${iconMarkup}</span>
          <span class="command-result-copy">
            <strong>${escapeHTML(item.label)}</strong>
            <span>${escapeHTML(item.hint)}</span>
          </span>
          <span class="command-result-key">↵</span>
        </button>
      `;
    })
    .join("");

  window.requestAnimationFrame(() => {
    const selected = $(".command-result.is-selected", commandResults);
    if (selected) selected.scrollIntoView({ block: "nearest" });
  });
}

async function searchCommandHotels(query) {
  if (state.commandAbort) state.commandAbort.abort();
  if (!query || query.length < 2) return;
  state.commandAbort = new AbortController();
  const currentQuery = query;
  try {
    const result = await api(`/api/search?q=${encodeURIComponent(query)}`);
    if (!commandPalette.classList.contains("is-open") || currentQuery !== state.commandQuery) return;
    const existing = new Set(
      state.commandItems.filter((item) => item.group === "Отели").map((item) => item.label)
    );
    const hotels = (result.hotels || [])
      .filter((hotel) => !existing.has(hotel.name))
      .map(commandHotelItem);
    state.commandExtra = hotels;
    renderCommandResults(currentQuery);
  } catch (error) {
  }
}

function executeCommand(index) {
  const item = state.commandItems[index];
  if (!item) return;
  closeCommandPalette();
  window.setTimeout(() => item.run(), 30);
}

function addAssistantMessage(message) {
  state.assistantMessages.push(message);
  renderAssistantMessages();
}

function renderAssistantMessages() {
  assistantMessages.innerHTML = state.assistantMessages
    .map((message) => {
      const isUser = message.role === "user";
      const hotels = message.hotels || [];
      const places = message.places || [];
      const recommendations = hotels.length || places.length
        ? `
          <div class="assistant-recommendations">
            ${hotels
              .map(
                (hotel) => `
                  <button type="button" class="assistant-hotel" data-hotel-slug="${escapeHTML(hotel.slug)}">
                    <img src="${escapeHTML(hotel.image)}" alt="">
                    <div>
                      <strong>${escapeHTML(hotel.name)}</strong>
                      <span>${escapeHTML(hotel.city)} · от ${money(hotel.priceFrom)}</span>
                    </div>
                    <span class="command-result-key">→</span>
                  </button>
                `
              )
              .join("")}
            ${places
              .map(
                (place) => `
                  <div class="assistant-place">
                    <span>${escapeHTML(categoryLabels[place.category] || place.category)}</span>
                    <strong>${escapeHTML(place.name)}</strong>
                    <p>${escapeHTML(place.address)} · ${escapeHTML(place.distance)}</p>
                  </div>
                `
              )
              .join("")}
          </div>
        `
        : "";

      return `
        <div class="assistant-message ${isUser ? "user" : "ai"}">
          <div class="assistant-bubble">${formatAssistantText(message.content)}</div>
          ${message.meta ? `<div class="assistant-message-meta">${escapeHTML(message.meta)}</div>` : ""}
          ${!isUser ? recommendations : ""}
        </div>
      `;
    })
    .join("");

  window.requestAnimationFrame(() => {
    assistantMessages.scrollTop = assistantMessages.scrollHeight;
  });
}

function openAssistant(prompt = "") {
  closeAccountMenu();
  closeThemePanel();
  closeMobileMenu();
  if (!state.assistantMessages.length) {
    addAssistantMessage({
      role: "assistant",
      content: "Привет! Я Odin AI. Помогу выбрать отель под ваш вайб, найти тихую кофейню или собрать маршрут. С чего начнём?",
      meta: "Odin AI · локальная модель"
    });
  }
  assistantDrawer.classList.add("is-open");
  assistantDrawer.setAttribute("aria-hidden", "false");
  assistantScrim.classList.add("is-open");
  state.assistantOpen = true;
  syncBodyLock();
  window.setTimeout(() => assistantInput.focus(), 120);
  if (prompt) {
    window.setTimeout(() => sendAssistant(prompt), 160);
  }
}

function closeAssistant() {
  if (!assistantDrawer.classList.contains("is-open")) return;
  assistantDrawer.classList.remove("is-open");
  assistantDrawer.setAttribute("aria-hidden", "true");
  assistantScrim.classList.remove("is-open");
  state.assistantOpen = false;
  syncBodyLock();
}

function assistantDestination() {
  if (state.currentView === "hotel" && state.activeHotel) {
    return state.activeHotel.destination?.slug || state.activeHotel.destinationSlug || "";
  }
  return state.search.destination || "";
}

function setAssistantBusy(busy) {
  state.assistantBusy = busy;
  assistantInput.disabled = busy;
  assistantSend.disabled = busy;
  $$(".suggestion-chip").forEach((chip) => {
    chip.disabled = busy;
  });
}

function assistantContextLabel(response) {
  if (response.source === "llama") return "Odin AI · Qwen 2.5 3B";
  if (response.source === "concierge") return "Odin AI · на связи";
  return "Odin AI · консьерж";
}

async function sendAssistant(rawText) {
  const text = String(rawText || "").trim();
  if (!text || state.assistantBusy) return;
  if (!assistantDrawer.classList.contains("is-open")) openAssistant();
  if (!state.assistantMessages.length) {
    addAssistantMessage({
      role: "assistant",
      content: "Привет! Я Odin AI. Помогу выбрать отель под ваш вайб, найти тихую кофейню или собрать маршрут. С чего начнём?",
      meta: "Odin AI · локальная модель"
    });
  }

  const history = [...state.assistantHistory];
  addAssistantMessage({ role: "user", content: text });
  state.assistantHistory.push({ role: "user", content: text });
  assistantInput.value = "";
  setAssistantBusy(true);

  const typing = document.createElement("div");
  typing.className = "assistant-message ai";
  typing.dataset.typing = "true";
  typing.innerHTML = '<div class="assistant-bubble"><span class="typing"><span></span><span></span><span></span></span></div>';
  assistantMessages.appendChild(typing);
  assistantMessages.scrollTop = assistantMessages.scrollHeight;

  try {
    const response = await api("/api/assistant/chat", {
      method: "POST",
      body: {
        message: text,
        destinationSlug: assistantDestination(),
        history
      }
    });
    typing.remove();
    addAssistantMessage({
      role: "assistant",
      content: response.reply,
      meta: assistantContextLabel(response),
      hotels: response.hotels || [],
      places: response.places || []
    });
    state.assistantHistory.push({ role: "assistant", content: response.reply });
    if (state.assistantHistory.length > 12) {
      state.assistantHistory = state.assistantHistory.slice(-12);
    }
  } catch (error) {
    typing.remove();
    addAssistantMessage({
      role: "assistant",
      content: "Не получилось получить ответ. Попробуйте ещё раз — если модель только загружается, консьерж ответит из локальной базы знаний.",
      meta: "Odin AI · ошибка связи"
    });
    showToast(error.message, "error");
  } finally {
    setAssistantBusy(false);
    assistantInput.focus();
  }
}

async function loadAssistantStatus() {
  try {
    const status = await api("/api/assistant/status");
    state.assistantStatus = status;
    const ready = Boolean(status.ready);
    const warming = status.state === "warming" || status.state === "starting";
    assistantStatusText.textContent = ready
      ? "на связи · Qwen 2.5 3B"
      : warming
        ? "загружаю локальную модель..."
        : "резервный режим";
    assistantStatusDot.style.background = ready ? "#3a9b6d" : warming ? "#b98236" : "#a29a8f";
    assistantStatusDot.style.boxShadow = ready ? "0 0 0 4px rgba(58,155,109,.12)" : "none";
    if (!ready && warming) {
      window.setTimeout(loadAssistantStatus, 2200);
    }
  } catch (error) {
    assistantStatusText.textContent = "резервный режим";
    assistantStatusDot.style.background = "#a29a8f";
  }
}

function openThemeSwatch(type, value) {
  const next = {
    accent: type === "accent" ? value : state.preferences.accent,
    menuTint: type === "menu" ? value : state.preferences.menuTint
  };
  applyTheme(next, { persist: true, saveServer: true });
}

function renderTrips() {
  const content = $("#tripsContent");
  $$("[data-trips-tab]").forEach((tab) => {
    tab.classList.toggle("is-active", tab.dataset.tripsTab === state.tripsTab);
  });

  if (!state.user) {
    content.innerHTML = `
      <div class="login-prompt">
        <h3>Войдите, чтобы увидеть поездки</h3>
        <p>Брони, коды подтверждения и избранные отели сохраняются в аккаунте.</p>
        <button type="button" class="primary-button" data-login-prompt>Войти или создать аккаунт</button>
      </div>
    `;
    return;
  }

  if (state.tripsTab === "favorites") {
    if (state.favorites === null) {
      content.innerHTML = '<div class="hotel-grid">' + Array.from({ length: 3 }, () => '<div class="hotel-card skeleton-card"></div>').join("") + "</div>";
      return;
    }
    if (!state.favorites.length) {
      content.innerHTML = `
        <div class="trips-empty">
          <h3>В избранном пока пусто</h3>
          <p>Нажимайте на сердечко у отеля, чтобы вернуться к нему позже.</p>
          <button type="button" class="secondary-button" data-nav="search">Найти отель</button>
        </div>
      `;
      return;
    }
    content.innerHTML = '<div class="hotel-grid"></div>';
    renderHotelCards(state.favorites, $(".hotel-grid", content));
    return;
  }

  if (state.trips === null) {
    content.innerHTML = '<div class="trip-list">' + Array.from({ length: 2 }, () => '<div class="trip-card skeleton-card"></div>').join("") + "</div>";
    return;
  }

  const today = todayISO();
  const trips = state.trips.filter((trip) => {
    const upcoming = trip.status === "confirmed" && trip.checkIn >= today;
    return state.tripsTab === "upcoming" ? upcoming : !upcoming;
  });

  if (!trips.length) {
    content.innerHTML = `
      <div class="trips-empty">
        <h3>${state.tripsTab === "upcoming" ? "Пока нет предстоящих поездок" : "Здесь появятся прошедшие поездки"}</h3>
        <p>${state.tripsTab === "upcoming" ? "Выберите отель — и он появится здесь сразу после бронирования." : "После поездки брони останутся в этом разделе."}</p>
        <button type="button" class="secondary-button" data-nav="search">Подобрать отель</button>
      </div>
    `;
    return;
  }

  content.innerHTML = `<div class="trip-list">${trips
    .map((trip) => {
      const statusClass = trip.status === "cancelled" ? "is-cancelled" : trip.status !== "confirmed" || trip.checkIn < today ? "is-past" : "";
      const statusLabel = trip.status === "cancelled" ? "Отменена" : trip.status !== "confirmed" || trip.checkIn < today ? "Завершена" : "Подтверждена";
      return `
        <article class="trip-card">
          <div class="trip-image"><img src="${escapeHTML(trip.image)}" alt="${escapeHTML(trip.hotelName)}" loading="lazy"></div>
          <div class="trip-body">
            <span class="trip-status ${statusClass}"><span class="status-dot"></span>${statusLabel}</span>
            <h3>${escapeHTML(trip.hotelName)}</h3>
            <div class="trip-meta">
              <span>${escapeHTML(trip.city)}</span>
              <span>${escapeHTML(trip.roomName)}</span>
              <span>${formatDateRange(trip.checkIn, trip.checkOut)}</span>
              <span>${trip.guests} ${plural(trip.guests, "гость", "гостя", "гостей")}</span>
            </div>
            <p class="trip-code">Код брони · ${escapeHTML(trip.code)}</p>
          </div>
          <div class="trip-actions">
            <div class="trip-total">
              <strong>${money(trip.total)}</strong>
              <span>${trip.nights} ${plural(trip.nights, "ночь", "ночи", "ночей")}</span>
            </div>
            ${trip.canCancel ? `<button type="button" class="trip-cancel" data-cancel-trip="${trip.id}">Отменить бронь</button>` : ""}
          </div>
        </article>
      `;
    })
    .join("")}</div>`;
}

async function loadTrips() {
  if (!state.user) return;
  try {
    state.trips = await api("/api/trips");
  } catch (error) {
    state.trips = [];
    showToast(error.message, "error");
  }
}

async function loadFavorites() {
  if (!state.user) return;
  try {
    state.favorites = await api("/api/favorites");
  } catch (error) {
    state.favorites = [];
    showToast(error.message, "error");
  }
}

function showTrips(tab = "upcoming") {
  state.tripsTab = tab;
  showView("trips");
  if (!state.user) {
    renderTrips();
    return;
  }
  renderTrips();
  if (tab === "favorites" && state.favorites === null) {
    loadFavorites().then(renderTrips);
  }
  if (tab !== "favorites" && state.trips === null) {
    loadTrips().then(renderTrips);
  }
}

async function toggleFavorite(hotelId) {
  if (!state.user) {
    state.pendingFavoriteId = hotelId;
    openAuth("login", "Войдите, чтобы сохранять отели в избранное.");
    return;
  }
  try {
    const result = await api(`/api/favorites/${hotelId}`, { method: "POST" });
    $$(`[data-favorite="${hotelId}"]`).forEach((button) => {
      button.classList.toggle("is-active", result.favorited);
      button.setAttribute("aria-label", result.favorited ? "Убрать из избранного" : "Добавить в избранное");
      const label = $("span", button);
      if (label && button.classList.contains("detail-favorite")) {
        label.textContent = result.favorited ? "В избранном" : "В избранное";
      }
    });
    state.featured = state.featured.map((hotel) => (hotel.id === hotelId ? { ...hotel, favorited: result.favorited } : hotel));
    if (state.activeHotel && state.activeHotel.id === hotelId) state.activeHotel.favorited = result.favorited;
    state.favorites = null;
    showToast(result.favorited ? "Отель добавлен в избранное." : "Отель убран из избранного.", "info");
    if (state.currentView === "trips" && state.tripsTab === "favorites") {
      await loadFavorites();
      renderTrips();
    }
  } catch (error) {
    showToast(error.message, "error");
  }
}

async function cancelTrip(bookingId, button) {
  if (button.dataset.confirm !== "1") {
    button.dataset.confirm = "1";
    button.textContent = "Точно отменить?";
    window.setTimeout(() => {
      button.dataset.confirm = "";
      button.textContent = "Отменить бронь";
    }, 3200);
    return;
  }
  try {
    await api(`/api/bookings/${bookingId}/cancel`, { method: "POST" });
    state.trips = null;
    showToast("Бронь отменена.", "info");
    await loadTrips();
    renderTrips();
  } catch (error) {
    showToast(error.message, "error");
  }
}

function scrollToRooms() {
  const section = $("#roomSection");
  if (section) section.scrollIntoView({ behavior: "smooth", block: "start" });
}

function bindEvents() {
  siteHeader.addEventListener("transitionend", () => {});
  window.addEventListener(
    "scroll",
    () => {
      siteHeader.classList.toggle("is-scrolled", window.scrollY > 8);
    },
    { passive: true }
  );

  searchCheckIn.addEventListener("change", () => {
    if (!searchCheckIn.value) return;
    searchCheckOut.min = searchCheckIn.value;
    if (searchCheckOut.value <= searchCheckIn.value) {
      searchCheckOut.value = toISODate(addDays(new Date(`${searchCheckIn.value}T12:00:00`), 3));
    }
    state.search.checkIn = searchCheckIn.value;
    state.search.checkOut = searchCheckOut.value;
    persistSearch();
  });
  searchCheckOut.addEventListener("change", () => {
    if (!searchCheckOut.value) return;
    state.search.checkOut = searchCheckOut.value;
    persistSearch();
  });
  searchGuests.addEventListener("change", () => {
    state.search.guests = Number(searchGuests.value);
    persistSearch();
  });

  heroSearch.addEventListener("submit", (event) => {
    event.preventDefault();
    openSearchFromForm();
  });

  $("#destinationsPrev").addEventListener("click", () => destinationsTrack.scrollBy({ left: -320, behavior: "smooth" }));
  $("#destinationsNext").addEventListener("click", () => destinationsTrack.scrollBy({ left: 320, behavior: "smooth" }));

  $("#aiPromptForm").addEventListener("submit", (event) => {
    event.preventDefault();
    const input = $("#aiPromptInput");
    const prompt = input.value.trim();
    if (!prompt) {
      input.focus();
      return;
    }
    input.value = "";
    openAssistant(prompt);
  });

  $("#searchOpen").addEventListener("click", openCommandPalette);
  $("#mobileSearchOpen").addEventListener("click", () => {
    closeMobileMenu();
    openCommandPalette();
  });
  $("#mobileThemeOpen").addEventListener("click", () => {
    closeMobileMenu();
    openThemePanel();
  });
  $("#mobileAssistantOpen").addEventListener("click", () => {
    closeMobileMenu();
    openAssistant();
  });

  themeButton.addEventListener("click", () => {
    if (themePanel.classList.contains("is-open")) closeThemePanel();
    else openThemePanel();
  });
  $("#themeClose").addEventListener("click", closeThemePanel);
  $("#themeReset").addEventListener("click", () => {
    applyTheme({ accent: DEFAULT_ACCENT, menuTint: DEFAULT_MENU }, { persist: true, saveServer: true });
    themeStatus.textContent = state.user ? "Сохраняем..." : "Сброшено";
  });

  $("#accentSwatches").addEventListener("click", (event) => {
    const swatch = event.target.closest("[data-accent]");
    if (swatch) openThemeSwatch("accent", swatch.dataset.accent);
  });
  $("#menuSwatches").addEventListener("click", (event) => {
    const swatch = event.target.closest("[data-menu]");
    if (swatch) openThemeSwatch("menu", swatch.dataset.menu);
  });
  accentCustom.addEventListener("input", () => openThemeSwatch("accent", accentCustom.value));
  menuCustom.addEventListener("input", () => openThemeSwatch("menu", menuCustom.value));

  accountButton.addEventListener("click", () => {
    if (!state.user) {
      openAuth("login");
      return;
    }
    const isOpen = accountMenu.classList.toggle("is-open");
    accountButton.setAttribute("aria-expanded", String(isOpen));
  });

  $("#mobileMenuButton").addEventListener("click", openMobileMenu);
  $("#mobileMenuClose").addEventListener("click", closeMobileMenu);

  $("#searchBack").addEventListener("click", () => showView("home"));
  $("#hotelBack").addEventListener("click", () => {
    if (state.previousView === "search") showView("search");
    else showView("home");
  });

  $("#filterChips").addEventListener("click", (event) => {
    const chip = event.target.closest("[data-filter-tag]");
    if (!chip) return;
    state.search.tag = chip.dataset.filterTag;
    performSearch();
  });
  $("#priceFilter").addEventListener("change", (event) => {
    state.search.maxPrice = Number(event.target.value);
    performSearch();
  });
  $("#sortSelect").addEventListener("change", (event) => {
    state.search.sort = event.target.value;
    performSearch();
  });
  $("#resetFilters").addEventListener("click", () => {
    state.search.tag = "";
    state.search.maxPrice = 0;
    state.search.sort = "";
    performSearch();
  });

  $("#tripsTabs").addEventListener("click", (event) => {
    const tab = event.target.closest("[data-trips-tab]");
    if (!tab) return;
    showTrips(tab.dataset.tripsTab);
  });

  $("#loginForm").addEventListener("submit", async (event) => {
    event.preventDefault();
    if (state.authBusy) return;
    const email = $("#loginEmail").value.trim();
    const password = $("#loginPassword").value;
    const errorBox = $("#authError");
    errorBox.classList.add("hidden");
    if (!email || !password) {
      errorBox.textContent = "Введите email и пароль.";
      errorBox.classList.remove("hidden");
      return;
    }
    const button = $("#loginSubmit");
    setAuthBusy(true, button);
    try {
      const payload = await api("/api/auth/login", { method: "POST", body: { email, password } });
      await completeAuth(payload);
    } catch (error) {
      errorBox.textContent = error.status === 401 ? "Неверный email или пароль." : error.message;
      errorBox.classList.remove("hidden");
    } finally {
      setAuthBusy(false);
    }
  });

  $("#registerForm").addEventListener("submit", async (event) => {
    event.preventDefault();
    if (state.authBusy) return;
    const name = $("#registerName").value.trim();
    const email = $("#registerEmail").value.trim();
    const password = $("#registerPassword").value;
    const errorBox = $("#authError");
    errorBox.classList.add("hidden");
    if (name.length < 2) {
      errorBox.textContent = "Укажите имя.";
      errorBox.classList.remove("hidden");
      $("#registerName").focus();
      return;
    }
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      errorBox.textContent = "Укажите корректный email.";
      errorBox.classList.remove("hidden");
      $("#registerEmail").focus();
      return;
    }
    if (password.length < 8) {
      errorBox.textContent = "Пароль должен содержать минимум 8 символов.";
      errorBox.classList.remove("hidden");
      $("#registerPassword").focus();
      return;
    }
    const button = $("#registerSubmit");
    setAuthBusy(true, button);
    try {
      const payload = await api("/api/auth/register", { method: "POST", body: { name, email, password } });
      await completeAuth(payload);
    } catch (error) {
      errorBox.textContent = error.message;
      errorBox.classList.remove("hidden");
    } finally {
      setAuthBusy(false);
    }
  });

  $("#demoLoginButton").addEventListener("click", async () => {
    if (state.authBusy) return;
    const button = $("#demoLoginButton");
    const errorBox = $("#authError");
    errorBox.classList.add("hidden");
    setAuthBusy(true, button);
    try {
      const payload = await api("/api/auth/demo", { method: "POST", body: {} });
      await completeAuth(payload);
    } catch (error) {
      errorBox.textContent = error.message;
      errorBox.classList.remove("hidden");
    } finally {
      setAuthBusy(false);
    }
  });

  $("#demoFill").addEventListener("click", () => {
    setAuthMode("login");
    $("#loginEmail").value = "demo@odin.travel";
    $("#loginPassword").value = "odin-demo-2026";
    $("#loginPassword").focus();
  });

  $("#authClose").addEventListener("click", () => closeAuth(true));

  $("#bookingClose").addEventListener("click", () => closeModal("bookingModal"));
  $("#bookingNext1").addEventListener("click", () => {
    if (!validateBookingDates()) return;
    setBookingStep(2);
  });
  $("#bookingNext2").addEventListener("click", () => {
    if (!validateGuestDetails()) return;
    updateBookingTotal();
    setBookingStep(3);
  });
  $("#bookingBack2").addEventListener("click", () => setBookingStep(1));
  $("#bookingBack3").addEventListener("click", () => setBookingStep(2));
  $("#bookingCheckIn").addEventListener("change", () => {
    const minOut = toISODate(addDays(new Date(`${$("#bookingCheckIn").value}T12:00:00`), 1));
    $("#bookingCheckOut").min = minOut;
    if ($("#bookingCheckOut").value <= $("#bookingCheckIn").value) {
      $("#bookingCheckOut").value = toISODate(addDays(new Date(`${$("#bookingCheckIn").value}T12:00:00`), 3));
    }
    updateBookingTotal();
  });
  $("#bookingCheckOut").addEventListener("change", updateBookingTotal);
  $("#bookingGuests").addEventListener("change", updateBookingTotal);
  $("#bookingForm").addEventListener("submit", (event) => {
    event.preventDefault();
    submitBooking();
  });
  $("#bookingGoTrips").addEventListener("click", () => {
    closeModal("bookingModal");
    showTrips("upcoming");
  });

  $("#cardNumber").addEventListener("input", (event) => {
    const digits = event.target.value.replace(/\D/g, "").slice(0, 16);
    event.target.value = digits.replace(/(\d{4})(?=\d)/g, "$1 ");
  });
  $("#cardExpiry").addEventListener("input", (event) => {
    const digits = event.target.value.replace(/\D/g, "").slice(0, 4);
    event.target.value = digits.length > 2 ? `${digits.slice(0, 2)} / ${digits.slice(2)}` : digits;
  });
  $("#cardCvc").addEventListener("input", (event) => {
    event.target.value = event.target.value.replace(/\D/g, "").slice(0, 4);
  });

  assistantScrim.addEventListener("click", closeAssistant);
  $("#assistantClose").addEventListener("click", closeAssistant);
  $("#assistantButton").addEventListener("click", () => openAssistant());
  $("#assistantFab").addEventListener("click", () => openAssistant());
  $("#assistantClear").addEventListener("click", () => {
    state.assistantMessages = [];
    state.assistantHistory = [];
    addAssistantMessage({
      role: "assistant",
      content: "Диалог очищен. Куда хочется поехать?",
      meta: "Odin AI · локальная модель"
    });
  });
  $("#assistantForm").addEventListener("submit", (event) => {
    event.preventDefault();
    sendAssistant(assistantInput.value);
  });

  commandInput.addEventListener("input", () => {
    state.commandQuery = commandInput.value;
    state.commandExtra = [];
    state.commandIndex = 0;
    renderCommandResults(state.commandQuery);
    searchCommandHotels(state.commandQuery.trim());
  });
  commandInput.addEventListener("keydown", (event) => {
    if (event.key === "ArrowDown") {
      event.preventDefault();
      state.commandIndex = (state.commandIndex + 1) % Math.max(state.commandItems.length, 1);
      renderCommandResults(state.commandQuery);
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      state.commandIndex = (state.commandIndex - 1 + Math.max(state.commandItems.length, 1)) % Math.max(state.commandItems.length, 1);
      renderCommandResults(state.commandQuery);
    } else if (event.key === "Enter") {
      event.preventDefault();
      executeCommand(state.commandIndex);
    }
  });
  commandResults.addEventListener("click", (event) => {
    const item = event.target.closest("[data-command-index]");
    if (!item) return;
    executeCommand(Number(item.dataset.commandIndex));
  });

  document.addEventListener("click", handleDocumentClick);
  document.addEventListener("keydown", handleDocumentKeydown);
  window.addEventListener("resize", () => {
    if (window.innerWidth > 820) closeMobileMenu();
  });

  document.addEventListener("keydown", (event) => {
    if ((event.key === "Enter" || event.key === " ") && event.target.matches(".hotel-card[data-hotel-slug]")) {
      event.preventDefault();
      openHotel(event.target.dataset.hotelSlug);
    }
  });
}

function handleDocumentClick(event) {
  const closeTrigger = event.target.closest("[data-close]");
  if (closeTrigger) {
    const key = closeTrigger.dataset.close;
    if (key === "auth") closeAuth(true);
    if (key === "booking") closeModal("bookingModal");
    if (key === "command") closeCommandPalette();
    return;
  }

  const nav = event.target.closest("[data-nav]");
  if (nav) {
    navigate(nav.dataset.nav);
    return;
  }

  const destination = event.target.closest("[data-destination]");
  if (destination) {
    searchByDestination(destination.dataset.destination);
    return;
  }

  const favorite = event.target.closest("[data-favorite]");
  if (favorite) {
    event.stopPropagation();
    toggleFavorite(Number(favorite.dataset.favorite));
    return;
  }

  const hotelLink = event.target.closest("[data-hotel-slug]");
  if (hotelLink) {
    openHotel(hotelLink.dataset.hotelSlug);
    return;
  }

  const accountAction = event.target.closest("[data-account-action]");
  if (accountAction) {
    const action = accountAction.dataset.accountAction;
    closeAccountMenu();
    if (action === "trips") showTrips("upcoming");
    else if (action === "favorites") showTrips("favorites");
    else if (action === "theme") openThemePanel();
    else if (action === "logout") logout();
    return;
  }

  const authMode = event.target.closest("[data-auth-mode]");
  if (authMode) {
    setAuthMode(authMode.dataset.authMode);
    return;
  }

  const vibe = event.target.closest("[data-vibe]");
  if (vibe) {
    applyVibe(vibe.dataset.vibe);
    return;
  }

  const prompt = event.target.closest("[data-prompt]");
  if (prompt) {
    openAssistant(prompt.dataset.prompt);
    return;
  }

  const loginPrompt = event.target.closest("[data-login-prompt]");
  if (loginPrompt) {
    openAuth("login");
    return;
  }

  const bookRoom = event.target.closest("[data-book-room]");
  if (bookRoom && state.activeHotel) {
    const room = state.activeHotel.rooms.find((item) => item.id === Number(bookRoom.dataset.bookRoom));
    if (room) openBooking(state.activeHotel, room);
    return;
  }

  const roomScroll = event.target.closest("[data-room-scroll]");
  if (roomScroll) {
    scrollToRooms();
    return;
  }

  const cancel = event.target.closest("[data-cancel-trip]");
  if (cancel) {
    cancelTrip(Number(cancel.dataset.cancelTrip), cancel);
    return;
  }

  if (accountMenu.classList.contains("is-open") && !accountMenu.contains(event.target) && !accountButton.contains(event.target)) {
    closeAccountMenu();
  }
  if (themePanel.classList.contains("is-open") && !themePanel.contains(event.target) && !themeButton.contains(event.target)) {
    closeThemePanel();
  }
}

function handleDocumentKeydown(event) {
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "k") {
    event.preventDefault();
    if (commandPalette.classList.contains("is-open")) closeCommandPalette();
    else openCommandPalette();
    return;
  }
  if (event.key === "Escape") {
    if (commandPalette.classList.contains("is-open")) closeCommandPalette();
    if (authModal.classList.contains("is-open")) closeAuth(true);
    if (bookingModal.classList.contains("is-open")) closeModal("bookingModal");
    if (assistantDrawer.classList.contains("is-open")) closeAssistant();
    if (themePanel.classList.contains("is-open")) closeThemePanel();
    if (mobileMenu.classList.contains("is-open")) closeMobileMenu();
    closeAccountMenu();
  }
}

async function init() {
  setDefaultDates();
  loadStoredTheme();
  bindEvents();
  updateAccountUI();
  showView("home", { scroll: false });
  loadAssistantStatus();
  await Promise.all([loadHome(), restoreSession()]);
}

init();
