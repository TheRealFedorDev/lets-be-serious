package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	sessionCookieName = "odin_session"
	sessionTTL        = 7 * 24 * time.Hour
	maxBodyBytes      = 1 << 20
)

var hexColorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

type Config struct {
	Port          string
	DBPath        string
	StaticPath    string
	AssetsPath    string
	SecureCookies bool
	OpenBrowser   bool
	DemoEmail     string
	DemoPassword  string
	DemoName      string
}

type API struct {
	store         *Store
	ai            *AIManager
	secureCookies bool
	sessionTTL    time.Duration
	cfg           Config
}

type contextKey string

const userContextKey contextKey = "user"

type credentialsRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type preferencesRequest struct {
	Accent   string `json:"accent"`
	MenuTint string `json:"menuTint"`
}

// flexibleString tolerates clients that accidentally send a non-string value
// (for example a click event object) instead of failing the whole request.
type flexibleString string

func (value *flexibleString) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || string(trimmed) == "null" || trimmed[0] != '"' {
		*value = ""
		return nil
	}
	var decoded string
	if err := json.Unmarshal(trimmed, &decoded); err != nil {
		return err
	}
	*value = flexibleString(decoded)
	return nil
}

type chatRequest struct {
	Message         string         `json:"message"`
	DestinationSlug flexibleString `json:"destinationSlug"`
	HotelSlug       flexibleString `json:"hotelSlug"`
	History         []ChatTurn     `json:"history"`
	Nights          int            `json:"nights"`
	Guests          int            `json:"guests"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (recorder *responseRecorder) WriteHeader(status int) {
	recorder.status = status
	recorder.ResponseWriter.WriteHeader(status)
}

func (recorder *responseRecorder) Write(data []byte) (int, error) {
	if recorder.status == 0 {
		recorder.status = http.StatusOK
	}
	return recorder.ResponseWriter.Write(data)
}

func main() {
	cfg := loadConfig()

	store, err := OpenStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer store.Close()

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancelStartup()
	if err := seedBaseData(startupCtx, store); err != nil {
		log.Fatalf("seed content: %v", err)
	}
	if err := ensureDemoUser(startupCtx, store, cfg); err != nil {
		log.Fatalf("seed demo account: %v", err)
	}

	aiManager := NewAIManager(LoadAIConfig())
	aiManager.Start()
	defer aiManager.Close()

	api := &API{
		store:         store,
		ai:            aiManager,
		secureCookies: cfg.SecureCookies,
		sessionTTL:    sessionTTL,
		cfg:           cfg,
	}

	handler := recoverMiddleware(logMiddleware(securityHeaders(routes(api, cfg))))
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	appURL := fmt.Sprintf("http://localhost:%s", cfg.Port)
	go func() {
		log.Printf("ODIN Travel running at %s", appURL)
		log.Printf("demo traveler: %s", cfg.DemoEmail)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	if cfg.OpenBrowser {
		go func() {
			time.Sleep(600 * time.Millisecond)
			openBrowser(appURL)
		}()
	}

	<-ctx.Done()
	log.Println("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

func loadConfig() Config {
	baseDir := appBaseDir()
	return Config{
		Port:          env("PORT", "8080"),
		DBPath:        env("ODIN_DB_PATH", filepath.Join(baseDir, "odin.db")),
		StaticPath:    env("ODIN_STATIC_PATH", filepath.Join(baseDir, "index.html")),
		AssetsPath:    env("ODIN_ASSETS_PATH", filepath.Join(baseDir, "assets")),
		SecureCookies: envBool("ODIN_SECURE_COOKIE", false),
		OpenBrowser:   envBool("ODIN_OPEN_BROWSER", true),
		DemoEmail:     strings.ToLower(env("ODIN_DEMO_EMAIL", "demo@odin.travel")),
		DemoPassword:  env("ODIN_DEMO_PASSWORD", "odin-demo-2026"),
		DemoName:      env("ODIN_DEMO_NAME", "Анна Смирнова"),
	}
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func appBaseDir() string {
	if executable, err := os.Executable(); err == nil {
		dir := filepath.Dir(executable)
		if _, statErr := os.Stat(filepath.Join(dir, "index.html")); statErr == nil {
			return dir
		}
	}
	if workingDir, err := os.Getwd(); err == nil {
		return workingDir
	}
	return "."
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func routes(api *API, cfg Config) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", api.handleHealth)
	mux.HandleFunc("GET /api/home", api.handleHome)
	mux.HandleFunc("GET /api/destinations", api.handleDestinations)
	mux.HandleFunc("GET /api/search", api.handleSearch)
	mux.HandleFunc("POST /api/trip/plan", api.handleTripPlan)
	mux.HandleFunc("POST /api/walk/plan", api.handleWalkPlan)
	mux.HandleFunc("GET /api/demo/scenarios", api.handleDemoScenarios)
	mux.HandleFunc("GET /api/demo/scenario/{id}", api.handleDemoScenario)
	mux.HandleFunc("POST /api/match/what-if", api.handleWhatIf)
	mux.HandleFunc("POST /api/trips/share", api.handleShareCreate)
	mux.HandleFunc("GET /api/share/{slug}", api.handleShareJSON)
	mux.HandleFunc("GET /share/{slug}", api.handleSharePage)
	mux.HandleFunc("GET /api/moods", api.handleMoods)
	mux.HandleFunc("GET /api/weather", api.handleWeather)
	mux.HandleFunc("GET /api/hotels/{slug}", api.handleHotel)
	mux.HandleFunc("GET /api/pois/{id}", api.handlePOI)
	mux.HandleFunc("GET /api/trips", api.requireAuth(api.handleTrips))
	mux.HandleFunc("POST /api/bookings", api.requireAuth(api.handleCreateBooking))
	mux.HandleFunc("POST /api/trip/checkout", api.requireAuth(api.handleTripCheckout))
	mux.HandleFunc("POST /api/trip/surprise", api.handleTripSurprise)
	mux.HandleFunc("POST /api/bookings/{id}/cancel", api.requireAuth(api.handleCancelBooking))
	mux.HandleFunc("GET /api/favorites", api.requireAuth(api.handleFavorites))
	mux.HandleFunc("POST /api/favorites/{id}", api.requireAuth(api.handleToggleFavorite))
	mux.HandleFunc("GET /api/assistant/status", api.handleAssistantStatus)
	mux.HandleFunc("POST /api/assistant/chat", api.handleAssistantChat)

	mux.HandleFunc("POST /api/auth/register", api.handleRegister)
	mux.HandleFunc("POST /api/auth/login", api.handleLogin)
	mux.HandleFunc("POST /api/auth/demo", api.handleDemoLogin)
	mux.HandleFunc("POST /api/auth/logout", api.handleLogout)
	mux.HandleFunc("GET /api/auth/me", api.requireAuth(api.handleMe))
	mux.HandleFunc("PUT /api/preferences", api.requireAuth(api.handlePreferences))

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "API-метод не найден")
	})

	assets := http.FileServer(http.Dir(cfg.AssetsPath))
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", cacheControl(assets)))

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		serveIndex(w, r, cfg.StaticPath)
	})
	mux.HandleFunc("GET /index.html", func(w http.ResponseWriter, r *http.Request) {
		serveIndex(w, r, cfg.StaticPath)
	})

	return mux
}

func serveIndex(w http.ResponseWriter, r *http.Request, path string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, path)
}

func cacheControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".js") || strings.HasSuffix(r.URL.Path, ".css") {
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=86400")
		}
		next.ServeHTTP(w, r)
	})
}

func (api *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
		"ai":     api.ai.Status(),
	})
}

func (api *API) handleHome(w http.ResponseWriter, r *http.Request) {
	userID := api.optionalUserID(r)
	home, err := api.store.Home(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось загрузить подборки")
		return
	}
	writeJSON(w, http.StatusOK, home)
}

func (api *API) handleDestinations(w http.ResponseWriter, r *http.Request) {
	destinations, err := api.store.Destinations(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось загрузить направления")
		return
	}
	writeJSON(w, http.StatusOK, destinations)
}

func (api *API) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	guests := 0
	if value := query.Get("guests"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > 10 {
			writeError(w, http.StatusBadRequest, guestsError)
			return
		}
		guests = n
	}
	if msg := validateStay(query.Get("checkIn"), query.Get("checkOut")); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	maxPrice := 0
	if value := query.Get("maxPrice"); value != "" {
		maxPrice, _ = strconv.Atoi(value)
	}
	minPrice := 0
	if value := query.Get("minPrice"); value != "" {
		minPrice, _ = strconv.Atoi(value)
	}
	minRating := 0.0
	if value := query.Get("minRating"); value != "" {
		minRating, _ = strconv.ParseFloat(value, 64)
	}
	lat, _ := strconv.ParseFloat(query.Get("lat"), 64)
	lng, _ := strconv.ParseFloat(query.Get("lng"), 64)
	radius, _ := strconv.ParseFloat(query.Get("radius"), 64)

	tags := append([]string{}, query["tags"]...)
	if len(query["tags"]) == 0 && len(query["tag[]"]) > 0 {
		tags = append(tags, query["tag[]"]...)
	}

	result, err := api.store.Search(r.Context(), api.optionalUserID(r), SearchParams{
		Query:       query.Get("q"),
		Destination: query.Get("destination"),
		Guests:      guests,
		MaxPrice:    maxPrice,
		MinPrice:    minPrice,
		MinRating:   minRating,
		Tag:         query.Get("tag"),
		Tags:        tags,
		Mood:        query.Get("mood"),
		Sort:        query.Get("sort"),
		Insured:     queryBool(query, "insured"),
		Guides:      queryBool(query, "guides"),
		Parking:     queryBool(query, "parking"),
		Pool:        queryBool(query, "pool"),
		Soundproof:  queryBool(query, "soundproof"),
		Week:        queryBool(query, "week"),
		Discount:    queryBool(query, "discount"),
		Lat:         lat,
		Lng:         lng,
		RadiusKm:    radius,
		CheckIn:     query.Get("checkIn"),
		CheckOut:    query.Get("checkOut"),
		Needs:       ParseNeeds(query["needs"]),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось выполнить поиск")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (api *API) handleTripPlan(w http.ResponseWriter, r *http.Request) {
	var req TripRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Проверьте параметры поездки")
		return
	}
	if req.Guests != 0 && (req.Guests < 1 || req.Guests > 10) {
		writeError(w, http.StatusBadRequest, guestsError)
		return
	}
	if msg := validateStay(req.From, req.To); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	key := tripCacheKey(req)
	if cached, ok := tripFromCache(key); ok {
		cached.Cached = true
		writeJSON(w, http.StatusOK, cached)
		return
	}
	plan, err := api.store.BuildTrip(r.Context(), api.optionalUserID(r), req)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "Не нашли отели для этой поездки — попробуйте другое направление")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось собрать маршрут")
		return
	}
	tripToCache(key, plan)
	writeJSON(w, http.StatusOK, plan)
}

func (api *API) handleTripSurprise(w http.ResponseWriter, r *http.Request) {
	var req SurpriseRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Проверьте параметры поездки")
		return
	}
	if req.Guests != 0 && (req.Guests < 1 || req.Guests > 10) {
		writeError(w, http.StatusBadRequest, guestsError)
		return
	}
	if msg := validateStay(req.From, req.To); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	result, err := api.store.Surprise(r.Context(), api.optionalUserID(r), req)
	var empty *SurpriseEmptyError
	if errors.As(err, &empty) {
		writeError(w, http.StatusNotFound, empty.Hint)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось собрать сюрприз")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func queryBool(query url.Values, key string) bool {
	value := strings.ToLower(strings.TrimSpace(query.Get(key)))
	return value == "1" || value == "true" || value == "on" || value == "yes"
}

func (api *API) handleWalkPlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		HotelSlug string `json:"hotelSlug"`
		Pace      string `json:"pace"`
		Rainy     bool   `json:"rainy"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Проверьте параметры прогулки")
		return
	}
	detail, err := api.store.HotelBySlug(r.Context(), api.optionalUserID(r), req.HotelSlug, "", "", 2)
	if err != nil {
		writeError(w, http.StatusNotFound, "Отель не найден")
		return
	}
	pois, _ := api.store.poisForDestination(r.Context(), detail.DestinationSlug, "", 32)
	if req.Pace != "active" {
		req.Pace = "calm"
	}
	walk := planWalkingRoute(detail.HotelCard, pois, req.Pace, req.Rainy, 0)
	writeJSON(w, http.StatusOK, walk)
}

func (api *API) handleMoods(w http.ResponseWriter, r *http.Request) {
	hotels, err := api.store.allHotelCards(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось загрузить подборки")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"moods": MoodsWithCounts(hotels),
		"tags":  TagCatalog,
		"needs": NeedCatalog(),
	})
}

func (api *API) handleWeather(w http.ResponseWriter, r *http.Request) {
	destination := strings.TrimSpace(r.URL.Query().Get("destination"))
	date := strings.TrimSpace(r.URL.Query().Get("date"))
	info := api.store.Weather(destination, date)
	writeJSON(w, http.StatusOK, info)
}

func (api *API) handleHotel(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimSpace(r.PathValue("slug"))
	if slug == "" {
		writeError(w, http.StatusBadRequest, "Отель не найден")
		return
	}
	hq := r.URL.Query()
	guests := 2
	if value := hq.Get("guests"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > 10 {
			writeError(w, http.StatusBadRequest, guestsError)
			return
		}
		guests = n
	}
	maxPrice := 0
	if value := hq.Get("maxPrice"); value != "" {
		maxPrice, _ = strconv.Atoi(value)
	}
	needs := ParseNeeds(hq["needs"])
	detail, err := api.store.HotelBySlug(
		r.Context(),
		api.optionalUserID(r),
		slug,
		r.URL.Query().Get("checkIn"),
		r.URL.Query().Get("checkOut"),
		guests,
	)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "Отель не найден")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось загрузить отель")
		return
	}
	mood := strings.TrimSpace(r.URL.Query().Get("mood"))
	tags := append([]string{}, r.URL.Query()["tags"]...)
	inferred := NormalizeTags(tags)
	searchParams := SearchParams{Guests: guests, Mood: mood, Tags: tags, MaxPrice: maxPrice, Needs: needs}
	// Балл считаем той же цепочкой, что Search: настроение/теги/логистика, затем смесь с потребностями.
	card := detail.HotelCard
	card.Needs = nil
	finalScore := card.Match
	if moodDef, ok := moodBySlug[mood]; ok {
		if score := MoodScore(card.Meta.TagSlugs, moodDef); score > finalScore {
			finalScore = score
		}
	}
	if len(inferred) > 0 {
		if score := overlapPercent(card.Meta.TagSlugs, inferred); score > finalScore {
			finalScore = score
		}
	}
	if lr := logisticRecommendation(card, searchParams, mood, inferred); lr > finalScore {
		finalScore = lr
	}
	if len(needs) > 0 {
		checks, _, _ := EvaluateNeeds(detail.HotelCard, needs)
		detail.HotelCard.Needs = checks
		if score := NeedsScore(checks); score >= 0 {
			if finalScore > 0 {
				finalScore = clampInt(int(0.6*float64(finalScore)+0.4*float64(score)+0.5), 1, 100)
			} else {
				finalScore = clampInt(score, 1, 100)
			}
		}
	}
	breakdown := BuildMatchBreakdown(detail.HotelCard, searchParams, mood, inferred)
	if finalScore > 0 && finalScore != breakdown.Score && len(breakdown.Factors) > 0 {
		delta := finalScore - breakdown.Score
		breakdown.Factors[0].Points += delta // factors отсортированы: гасим разницу в главном вкладе
		breakdown.Score = finalScore
		breakdown.Verdict = tripVerdict(finalScore)
	}
	detail.MatchDetail = &breakdown
	writeJSON(w, http.StatusOK, detail)
}

func (api *API) handleWhatIf(w http.ResponseWriter, r *http.Request) {
	var req WhatIfRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Проверьте параметры расчёта")
		return
	}
	if strings.TrimSpace(req.Slug) == "" {
		writeError(w, http.StatusBadRequest, "Отель не найден")
		return
	}
	result, err := api.store.WhatIf(r.Context(), api.optionalUserID(r), req)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "Отель не найден")
		return
	}
	var inErr *WhatIfInputError
	if errors.As(err, &inErr) {
		writeError(w, http.StatusBadRequest, inErr.Message)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось пересчитать варианты")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (api *API) handlePOI(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Некорректное место")
		return
	}
	detail, err := api.store.POIDetail(r.Context(), api.optionalUserID(r), id)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "Место не найдено")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось загрузить место")
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (api *API) handleTrips(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	trips, err := api.store.Trips(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось загрузить поездки")
		return
	}
	writeJSON(w, http.StatusOK, trips)
}

func (api *API) handleCreateBooking(w http.ResponseWriter, r *http.Request) {
	var input BookingInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Проверьте данные бронирования")
		return
	}

	if msg := normalizeGuest(&input.GuestName, &input.GuestEmail, &input.GuestPhone); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	user := currentUser(r)
	trip, err := api.store.CreateBooking(r.Context(), user.ID, input)
	if writeBookingError(w, err) {
		return
	}

	writeJSON(w, http.StatusCreated, trip)
}

// normalizeGuest чистит контакты гостя и возвращает текст ошибки ("" - всё в порядке).
func normalizeGuest(name, email, phone *string) string {
	*name = strings.TrimSpace(*name)
	*email = strings.ToLower(strings.TrimSpace(*email))
	*phone = strings.TrimSpace(*phone)
	if len([]rune(*name)) < 2 || len([]rune(*name)) > 80 {
		return "Укажите имя гостя"
	}
	if !validEmail(*email) {
		return "Укажите корректный email"
	}
	if len(*phone) < 5 || len(*phone) > 30 {
		return "Укажите телефон для связи"
	}
	return ""
}

// writeBookingError отвечает на ошибку брони; true - ответ уже отправлен.
func writeBookingError(w http.ResponseWriter, err error) bool {
	var capErr *CapacityError
	switch {
	case err == nil:
		return false
	case errors.As(err, &capErr):
		writeError(w, http.StatusBadRequest, capErr.Error())
	case errors.Is(err, ErrUnavailable):
		writeError(w, http.StatusConflict, "Этот номер уже занят на выбранные даты")
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "Номер не найден")
	default:
		writeError(w, http.StatusBadRequest, "Не удалось создать бронирование. Проверьте даты и гостей.")
	}
	return true
}

func (api *API) handleTripCheckout(w http.ResponseWriter, r *http.Request) {
	var input CheckoutInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Проверьте данные заказа")
		return
	}
	if msg := normalizeGuest(&input.GuestName, &input.GuestEmail, &input.GuestPhone); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if input.Trip.Guests < 1 || input.Trip.Guests > 10 {
		writeError(w, http.StatusBadRequest, guestsError)
		return
	}
	if msg := validateStay(input.Trip.From, input.Trip.To); msg != "" || input.Trip.From == "" || input.Trip.To == "" {
		if msg == "" {
			msg = "Укажите даты поездки"
		}
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	user := currentUser(r)
	result, err := api.store.Checkout(r.Context(), user.ID, input)
	var itemErr *ItemInputError
	switch {
	case errors.As(err, &itemErr):
		writeError(w, http.StatusBadRequest, itemErr.Message)
		return
	case errors.Is(err, ErrPlanChanged):
		writeError(w, http.StatusConflict, "Маршрут изменился — соберите его заново перед оплатой")
		return
	case errors.Is(err, ErrNotFound) && result.Trip.ID == 0 && input.RoomID == 0:
		writeError(w, http.StatusBadRequest, "Выберите номер")
		return
	}
	if writeBookingError(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (api *API) handleCancelBooking(w http.ResponseWriter, r *http.Request) {
	bookingID, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Некорректная бронь")
		return
	}
	user := currentUser(r)
	if err := api.store.CancelBooking(r.Context(), user.ID, bookingID); err != nil {
		if errors.Is(err, ErrCantCancel) {
			writeError(w, http.StatusConflict, "Эту бронь уже нельзя отменить")
			return
		}
		writeError(w, http.StatusInternalServerError, "Не удалось отменить бронь")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (api *API) handleFavorites(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	favorites, err := api.store.Favorites(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось загрузить избранное")
		return
	}
	writeJSON(w, http.StatusOK, favorites)
}

func (api *API) handleToggleFavorite(w http.ResponseWriter, r *http.Request) {
	hotelID, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Некорректный отель")
		return
	}
	var exists int
	if err := api.store.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM hotels WHERE id = ?`, hotelID).Scan(&exists); err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось обновить избранное")
		return
	}
	if exists == 0 {
		writeError(w, http.StatusNotFound, "Отель не найден")
		return
	}
	user := currentUser(r)
	favorited, err := api.store.ToggleFavorite(r.Context(), user.ID, hotelID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось обновить избранное")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"favorited": favorited})
}

func (api *API) handlePreferences(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var input preferencesRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Проверьте настройки меню")
		return
	}
	if !hexColorPattern.MatchString(input.Accent) || !hexColorPattern.MatchString(input.MenuTint) {
		writeError(w, http.StatusBadRequest, "Неверный формат цвета")
		return
	}
	preferences := Preferences{Accent: strings.ToUpper(input.Accent), MenuTint: strings.ToUpper(input.MenuTint)}
	user := currentUser(r)
	if err := api.store.SavePreferences(r.Context(), user.ID, preferences); err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось сохранить настройки меню")
		return
	}
	writeJSON(w, http.StatusOK, preferences)
}

func (api *API) handleRegister(w http.ResponseWriter, r *http.Request) {
	if rateLimited(w, r, authLimiter, "Слишком много попыток входа. Подождите минуту и попробуйте снова") {
		return
	}
	if !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "Запрос отклонён")
		return
	}
	var input registerRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Проверьте данные формы")
		return
	}

	name := strings.TrimSpace(input.Name)
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if len([]rune(name)) < 2 || len([]rune(name)) > 80 {
		writeError(w, http.StatusBadRequest, "Имя должно содержать от 2 до 80 символов")
		return
	}
	if !validEmail(email) {
		writeError(w, http.StatusBadRequest, "Введите корректный email")
		return
	}
	if len(input.Password) < 8 || len(input.Password) > 72 {
		writeError(w, http.StatusBadRequest, "Пароль должен содержать от 8 до 72 символов")
		return
	}

	ctx := r.Context()
	if _, err := api.store.FindUserByEmail(ctx, email); err == nil {
		writeError(w, http.StatusConflict, "Аккаунт с таким email уже существует")
		return
	} else if !errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "Не удалось проверить аккаунт")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось создать аккаунт")
		return
	}
	user, err := api.store.CreateUser(ctx, name, email, string(hash))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось создать аккаунт")
		return
	}
	if err := api.issueSession(w, r, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось создать сессию")
		return
	}
	preferences, _ := api.store.Preferences(ctx, user.ID)
	writeJSON(w, http.StatusCreated, AuthPayload{User: user, Preferences: preferences})
}

func (api *API) handleLogin(w http.ResponseWriter, r *http.Request) {
	if rateLimited(w, r, authLimiter, "Слишком много попыток входа. Подождите минуту и попробуйте снова") {
		return
	}
	if !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "Запрос отклонён")
		return
	}
	var input credentialsRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Проверьте данные формы")
		return
	}

	email := strings.ToLower(strings.TrimSpace(input.Email))
	if !validEmail(email) || input.Password == "" {
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(input.Password))
		writeError(w, http.StatusUnauthorized, "Неверный email или пароль")
		return
	}

	ctx := r.Context()
	userID, hash, err := api.store.UserPasswordHash(ctx, email)
	if errors.Is(err, ErrNotFound) {
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(input.Password))
		writeError(w, http.StatusUnauthorized, "Неверный email или пароль")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось войти")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "Неверный email или пароль")
		return
	}

	user, err := api.store.UserByID(ctx, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось загрузить аккаунт")
		return
	}
	if err := api.issueSession(w, r, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось создать сессию")
		return
	}
	preferences, _ := api.store.Preferences(ctx, user.ID)
	writeJSON(w, http.StatusOK, AuthPayload{User: user, Preferences: preferences})
}

func (api *API) handleDemoLogin(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "Запрос отклонён")
		return
	}
	ctx := r.Context()
	user, err := api.store.FindUserByEmail(ctx, api.cfg.DemoEmail)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Демо-аккаунт недоступен")
		return
	}
	if err := api.issueSession(w, r, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось создать сессию")
		return
	}
	preferences, _ := api.store.Preferences(ctx, user.ID)
	writeJSON(w, http.StatusOK, AuthPayload{User: user, Preferences: preferences})
}

func (api *API) handleLogout(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "Запрос отклонён")
		return
	}
	if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
		_ = api.store.DeleteSession(r.Context(), hashToken(cookie.Value))
	}
	api.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (api *API) handleMe(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	preferences, _ := api.store.Preferences(r.Context(), user.ID)
	writeJSON(w, http.StatusOK, AuthPayload{User: user, Preferences: preferences})
}

func (api *API) issueSession(w http.ResponseWriter, r *http.Request, userID int64) error {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	expires := time.Now().UTC().Add(api.sessionTTL)
	if err := api.store.CreateSession(r.Context(), userID, hashToken(token), expires); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(api.sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   api.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (api *API) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   api.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

func (api *API) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead && !sameOrigin(r) {
			writeError(w, http.StatusForbidden, "Запрос отклонён")
			return
		}
		user, err := api.userFromRequest(r)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusUnauthorized, "Требуется вход")
				return
			}
			writeError(w, http.StatusInternalServerError, "Не удалось проверить сессию")
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey, user)
		next(w, r.WithContext(ctx))
	}
}

func (api *API) userFromRequest(r *http.Request) (User, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return User{}, ErrNotFound
	}
	return api.store.UserBySession(r.Context(), hashToken(cookie.Value))
}

func (api *API) optionalUserID(r *http.Request) int64 {
	user, err := api.userFromRequest(r)
	if err != nil {
		return 0
	}
	return user.ID
}

func currentUser(r *http.Request) User {
	user, _ := r.Context().Value(userContextKey).(User)
	return user
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		return errors.New("content type must be application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("body must contain a single json object")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

func validEmail(email string) bool {
	if len(email) > 254 || strings.ContainsAny(email, "\r\n") {
		return false
	}
	address, err := mail.ParseAddress(email)
	return err == nil && strings.EqualFold(address.Address, email)
}

func sameOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Host, r.Host)
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		if recorder.status == 0 {
			recorder.status = http.StatusOK
		}
		if !strings.HasPrefix(r.URL.Path, "/assets/") {
			log.Printf("%s %s %d %s", r.Method, r.URL.Path, recorder.status, time.Since(start).Round(time.Millisecond))
		}
	})
}

// script-src без 'unsafe-inline': inline-обработчики в разметке убраны,
// обработчики вешаются через addEventListener. style-src оставлен с
// 'unsafe-inline', т.к. вёрстка использует inline style.
const contentSecurityPolicy = "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; " +
	"script-src 'self'; connect-src 'self'; font-src 'self'; frame-ancestors 'none'; " +
	"base-uri 'self'; form-action 'self'"

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(self)")
		w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
		next.ServeHTTP(w, r)
	})
}

func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("panic: %v", recovered)
				writeError(w, http.StatusInternalServerError, "Внутренняя ошибка сервера")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func openBrowser(target string) {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	case "darwin":
		command = exec.Command("open", target)
	default:
		command = exec.Command("xdg-open", target)
	}
	if err := command.Start(); err != nil {
		log.Printf("open browser: %v", err)
	}
}

// ---------- rate limit ----------

type ipLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration
}

func newIPLimiter(limit int, window time.Duration) *ipLimiter {
	return &ipLimiter{hits: map[string][]time.Time{}, limit: limit, window: window}
}

// allow регистрирует попытку и возвращает false, если лимит превышен.
func (l *ipLimiter) allow(key string) bool {
	now := time.Now()
	cutoff := now.Add(-l.window)
	l.mu.Lock()
	defer l.mu.Unlock()
	kept := l.hits[key][:0]
	for _, at := range l.hits[key] {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	if len(kept) >= l.limit {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	if len(l.hits) > 10000 { // защита от разрастания карты
		for k, v := range l.hits {
			if len(v) == 0 || !v[len(v)-1].After(cutoff) {
				delete(l.hits, k)
			}
		}
	}
	return true
}

// clientIP берёт адрес TCP-соединения: X-Forwarded-For подделывается клиентом.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

var (
	authLimiter  = newIPLimiter(10, time.Minute)
	shareLimiter = newIPLimiter(20, time.Minute)
)

func rateLimited(w http.ResponseWriter, r *http.Request, limiter *ipLimiter, message string) bool {
	if limiter.allow(clientIP(r)) {
		return false
	}
	w.Header().Set("Retry-After", "60")
	writeError(w, http.StatusTooManyRequests, message)
	return true
}

// Фиктивный хэш: для несуществующего email login тратит столько же времени на bcrypt.
var dummyPasswordHash, _ = bcrypt.GenerateFromPassword([]byte("odin-dummy-password"), bcrypt.DefaultCost)

// ---------- валидация дат и гостей ----------

const maxStayNights = 30

// validateStay проверяет даты заезда/выезда. Пустые значения допустимы (дефолты).
func validateStay(checkIn, checkOut string) string {
	checkIn = strings.TrimSpace(checkIn)
	checkOut = strings.TrimSpace(checkOut)
	var in, out time.Time
	var err error
	if checkIn != "" {
		if in, err = time.Parse("2006-01-02", checkIn); err != nil {
			return "Дата заезда должна быть в формате ГГГГ-ММ-ДД"
		}
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		if in.Before(today) {
			return "Дата заезда не может быть в прошлом"
		}
	}
	if checkOut != "" {
		if out, err = time.Parse("2006-01-02", checkOut); err != nil {
			return "Дата выезда должна быть в формате ГГГГ-ММ-ДД"
		}
	}
	if checkIn != "" && checkOut != "" {
		if !out.After(in) {
			return "Дата выезда должна быть позже даты заезда"
		}
		if out.Sub(in) > maxStayNights*24*time.Hour {
			return "Поездка не может быть длиннее 30 ночей"
		}
	}
	return ""
}

const guestsError = "Число гостей должно быть от 1 до 10"
