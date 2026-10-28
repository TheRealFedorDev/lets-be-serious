package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Публичная ссылка на маршрут: план можно отправить спутнику, открыть на
// телефоне по QR и посмотреть без входа в аккаунт.

type ShareRequest struct {
	Plan TripPlan `json:"plan"`
}

type ShareResponse struct {
	Slug  string `json:"slug"`
	URL   string `json:"url"`
	Views int    `json:"views,omitempty"`
}

const shareSlugAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"

func (s *Store) ensureShareTable(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS shared_trips (
		slug TEXT PRIMARY KEY,
		plan_json TEXT NOT NULL,
		owner_id INTEGER NOT NULL DEFAULT 0,
		city TEXT NOT NULL DEFAULT '',
		hotel_slug TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		views INTEGER NOT NULL DEFAULT 0
	)`)
	return err
}

func newShareSlug() (string, error) {
	buffer := make([]byte, 9)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	var builder strings.Builder
	for _, value := range buffer {
		builder.WriteByte(shareSlugAlphabet[int(value)%len(shareSlugAlphabet)])
	}
	return builder.String(), nil
}

func (s *Store) CreateShare(ctx context.Context, plan TripPlan, ownerID int64) (string, error) {
	if strings.TrimSpace(plan.Hotel.Slug) == "" || len(plan.Days) == 0 {
		return "", errors.New("empty plan")
	}
	if err := s.ensureShareTable(ctx); err != nil {
		return "", err
	}
	payload, err := json.Marshal(plan)
	if err != nil {
		return "", err
	}
	for attempt := 0; attempt < 5; attempt++ {
		slug, err := newShareSlug()
		if err != nil {
			return "", err
		}
		_, err = s.db.ExecContext(ctx,
			`INSERT INTO shared_trips (slug, plan_json, owner_id, city, hotel_slug, created_at, views)
			 VALUES (?, ?, ?, ?, ?, ?, 0)`,
			slug, string(payload), ownerID, plan.City, plan.Hotel.Slug, time.Now().UTC().Format(time.RFC3339),
		)
		if err == nil {
			return slug, nil
		}
		if !strings.Contains(strings.ToLower(err.Error()), "unique") {
			return "", err
		}
	}
	return "", errors.New("could not generate share slug")
}

func (s *Store) SharedPlan(ctx context.Context, slug string) (TripPlan, int, error) {
	if err := s.ensureShareTable(ctx); err != nil {
		return TripPlan{}, 0, err
	}
	var payload string
	var views int
	err := s.db.QueryRowContext(ctx,
		`SELECT plan_json, views FROM shared_trips WHERE slug = ?`, slug,
	).Scan(&payload, &views)
	if errors.Is(err, sql.ErrNoRows) {
		return TripPlan{}, 0, ErrNotFound
	}
	if err != nil {
		return TripPlan{}, 0, err
	}
	var plan TripPlan
	if err := json.Unmarshal([]byte(payload), &plan); err != nil {
		return TripPlan{}, 0, err
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE shared_trips SET views = views + 1 WHERE slug = ?`, slug)
	return plan, views + 1, nil
}

func (api *API) handleShareCreate(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "Запрос отклонён")
		return
	}
	if rateLimited(w, r, shareLimiter, "Слишком много запросов. Подождите минуту и попробуйте снова") {
		return
	}
	if r.ContentLength > maxShareBodyBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "Маршрут слишком большой (максимум 256 КБ)")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxShareBodyBytes)
	var input ShareRequest
	if err := decodeJSON(w, r, &input); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "Маршрут слишком большой (максимум 256 КБ)")
			return
		}
		writeError(w, http.StatusBadRequest, "Не удалось прочитать маршрут")
		return
	}
	if msg := validateSharePlan(input.Plan); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	slug, err := api.store.CreateShare(r.Context(), input.Plan, api.optionalUserID(r))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Не получилось поделиться маршрутом")
		return
	}
	writeJSON(w, http.StatusOK, ShareResponse{
		Slug: slug,
		URL:  shareURL(slug),
	})
}

func (api *API) handleShareJSON(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimSpace(r.PathValue("slug"))
	plan, views, err := api.store.SharedPlan(r.Context(), slug)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "Маршрут не найден")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось загрузить маршрут")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plan": plan, "views": views})
}

// shareURL отдаёт относительный путь: абсолютный адрес строился бы из
// Host/X-Forwarded-* (host-header injection), origin добавляет UI.
func shareURL(slug string) string {
	return "/assets/share.html?s=" + url.QueryEscape(slug)
}

func (api *API) handleSharePage(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimSpace(r.PathValue("slug"))
	http.Redirect(w, r, shareURL(slug), http.StatusFound)
}

const (
	maxShareBodyBytes = 256 << 10
	maxShareDays      = 30
	maxShareString    = 500
)

// validateSharePlan проверяет структуру плана перед сохранением публичной ссылки.
func validateSharePlan(plan TripPlan) string {
	if strings.TrimSpace(plan.City) == "" {
		return "В маршруте не указан город"
	}
	if len(plan.Days) == 0 {
		return "В маршруте нет дней"
	}
	if len(plan.Days) > maxShareDays {
		return "В маршруте не может быть больше 30 дней"
	}
	if plan.Budget.Total <= 0 {
		return "В маршруте не указан бюджет"
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		return "Не удалось прочитать маршрут"
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return "Не удалось прочитать маршрут"
	}
	return walkShareValue("", generic)
}

func walkShareValue(key string, value any) string {
	switch typed := value.(type) {
	case string:
		if len([]rune(typed)) > maxShareString {
			return "Слишком длинное текстовое поле в маршруте (максимум 500 символов)"
		}
		switch strings.ToLower(key) {
		case "url", "image", "href", "link", "src", "gallery":
			if !safeShareURL(typed) {
				return "Ссылки в маршруте должны быть http, https или относительными"
			}
		}
	case []any:
		for _, item := range typed {
			if msg := walkShareValue(key, item); msg != "" {
				return msg
			}
		}
	case map[string]any:
		for k, item := range typed {
			if msg := walkShareValue(k, item); msg != "" {
				return msg
			}
		}
	}
	return ""
}

func safeShareURL(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	if strings.HasPrefix(value, "//") || strings.ContainsAny(value, "\r\n\t\\") {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "":
		return true
	case "http", "https":
		return parsed.Host != ""
	}
	return false
}
