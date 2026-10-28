package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
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
	var input ShareRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Не удалось прочитать маршрут")
		return
	}
	slug, err := api.store.CreateShare(r.Context(), input.Plan, api.optionalUserID(r))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Не получилось поделиться маршрутом")
		return
	}
	writeJSON(w, http.StatusOK, ShareResponse{
		Slug: slug,
		URL:  shareURL(r, slug),
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

func shareURL(r *http.Request, slug string) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := r.Host
	if host == "" {
		host = "localhost"
	}
	return fmt.Sprintf("%s://%s/share/%s", scheme, host, url.PathEscape(slug))
}

type sharePageData struct {
	Plan       TripPlan
	BudgetRows []shareBudgetRow
	Views      int
	Link       string
	QRData     string
}

type shareBudgetRow struct {
	Label   string
	Value   int
	Percent int
}

func shareBudgetRows(budget TripBudget) []shareBudgetRow {
	labels := []struct {
		key   string
		label string
	}{
		{"stay", "Отель"},
		{"food", "Еда"},
		{"activities", "Активности"},
		{"transport", "Транспорт"},
	}
	rows := make([]shareBudgetRow, 0, len(labels))
	for _, item := range labels {
		value := budget.Split[item.key]
		percent := 0
		if budget.Total > 0 {
			percent = int(float64(value)/float64(budget.Total)*100 + 0.5)
		}
		rows = append(rows, shareBudgetRow{Label: item.label, Value: value, Percent: percent})
	}
	return rows
}

var sharePageTemplate = template.Must(template.New("share").Funcs(template.FuncMap{
	"money":     money,
	"shortDate": shortDate,
	"plural":    pluralWord,
	"blockIcon": shareBlockIcon,
}).Parse(`<!doctype html>
<html lang="ru">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Plan.City}} · маршрут от О!дин</title>
<meta name="description" content="Маршрут поездки в {{.Plan.City}} на {{.Plan.Nights}} ночей: отель, дни, честный бюджет.">
<style>
:root{--lime:#c8f86e;--blue:#1f4bdb;--ink:#171a22;--muted:#6d7381;--line:#e7e9ef;--canvas:#f4f6f9;--paper:#fff;--success:#0d8a5d}
*{box-sizing:border-box}
body{margin:0;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Arial,sans-serif;background:var(--canvas);color:var(--ink);line-height:1.5}
.wrap{max-width:920px;margin:0 auto;padding:28px 20px 64px}
.brand{display:flex;align-items:center;gap:10px;font-weight:800;font-size:18px;text-decoration:none;color:var(--ink);margin-bottom:22px}
.brand span{display:inline-flex;align-items:center;justify-content:center;width:34px;height:34px;border-radius:12px;background:var(--lime);font-size:16px}
.kicker{font-size:11px;text-transform:uppercase;letter-spacing:.08em;color:var(--muted);font-weight:700}
h1{font-size:30px;line-height:1.15;margin:6px 0 8px;letter-spacing:-.02em}
.sub{color:var(--muted);margin:0 0 22px;font-size:14px}
.card{background:var(--paper);border:1px solid var(--line);border-radius:20px;padding:18px;margin-bottom:14px}
.hotel{display:grid;grid-template-columns:140px 1fr;gap:16px;align-items:center}
.hotel img{width:140px;height:110px;object-fit:cover;border-radius:14px}
.hotel h2{margin:2px 0 4px;font-size:19px}
.hotel p{margin:0;color:var(--muted);font-size:13px}
.meta{display:flex;flex-wrap:wrap;gap:10px;margin-top:8px;font-size:12.5px;color:#343a49;font-weight:600}
.score{display:inline-flex;align-items:center;justify-content:center;min-width:46px;height:46px;border-radius:50%;background:var(--lime);font-weight:800;font-size:14px;margin-left:auto}
.budget-top{display:flex;justify-content:space-between;gap:16px;align-items:flex-start}
.budget-top strong{display:block;font-size:28px;margin-top:2px}
.budget-top small{color:var(--muted);font-size:12.5px}
.fit{padding:6px 12px;border-radius:999px;background:#e8f8f0;color:var(--success);font-size:12.5px;font-weight:700;white-space:nowrap}
.bars{display:grid;gap:8px;margin-top:14px}
.bar{display:grid;grid-template-columns:96px 1fr 92px;gap:10px;align-items:center;font-size:12.5px;color:#343a49}
.bar i{display:block;height:8px;border-radius:999px;background:linear-gradient(90deg,var(--blue),#6f8cff)}
.bar-track{height:8px;border-radius:999px;background:var(--canvas);overflow:hidden}
.day{border:1px solid var(--line);border-radius:16px;margin-bottom:10px;overflow:hidden;background:var(--paper)}
.day-head{display:grid;grid-template-columns:88px 1fr auto auto;gap:12px;align-items:center;padding:13px 16px;background:#fbfcfe}
.day-date{font-size:11.5px;text-transform:uppercase;letter-spacing:.05em;color:var(--muted);font-weight:700}
.day-title{font-weight:650;font-size:14px}
.day-weather{font-size:12.5px;color:#343a49}
.day-total{font-size:13px;font-weight:700}
.blocks{padding:6px 16px 12px}
.block{display:grid;grid-template-columns:58px 1fr 92px;gap:12px;padding:9px 0;border-bottom:1px dashed var(--line);font-size:13px}
.block:last-child{border-bottom:0}
.block time{color:var(--blue);font-weight:700;font-size:12.5px}
.block small{display:block;color:var(--muted);font-size:12px}
.block .price{text-align:right;font-size:12.5px;font-weight:650;color:#343a49}
.tips{margin:14px 0 0;padding:14px 18px 14px 34px;border-radius:14px;background:#eef2ff;font-size:13px;color:#343a49}
.share{display:grid;grid-template-columns:1fr auto;gap:18px;align-items:center}
.share-link{font-size:12.5px;color:var(--muted);word-break:break-all;margin:6px 0 10px}
.copy{display:inline-flex;padding:10px 16px;border-radius:12px;background:var(--ink);color:#fff;font-weight:650;font-size:13px;border:0;cursor:pointer}
.copy:hover{background:#2a3040}
#qr{width:132px;height:132px;background:#fff;border:1px solid var(--line);border-radius:12px;padding:6px}
#qr svg{width:100%;height:100%;display:block}
.views{font-size:11.5px;color:var(--muted);margin-top:8px}
.cta{display:inline-flex;align-items:center;gap:8px;margin-top:6px;padding:13px 20px;border-radius:14px;background:var(--lime);color:var(--ink);font-weight:700;font-size:14px;text-decoration:none}
.foot{margin-top:26px;font-size:12px;color:var(--muted)}
@media (max-width:640px){
  .wrap{padding:20px 14px 48px}
  h1{font-size:24px}
  .hotel{grid-template-columns:88px 1fr}
  .hotel img{width:88px;height:88px}
  .score{grid-column:2;margin:0}
  .day-head{grid-template-columns:1fr auto;row-gap:3px}
  .day-date{grid-column:1/-1}
  .block{grid-template-columns:50px 1fr}
  .block .price{grid-column:2;text-align:left}
  .share{grid-template-columns:1fr}
  #qr{margin:0 auto}
}
</style>
</head>
<body>
<div class="wrap">
  <a class="brand" href="/"><span>О!</span>дин · один шаг до идеального отеля</a>
  <span class="kicker">Маршрут поездки</span>
  <h1>{{.Plan.City}} · {{.Plan.Nights}} {{plural .Plan.Nights "ночь" "ночи" "ночей"}}</h1>
  <p class="sub">{{shortDate .Plan.CheckIn}} — {{shortDate .Plan.CheckOut}} · {{.Plan.Guests}} {{plural .Plan.Guests "гость" "гостя" "гостей"}} · {{.Plan.Match.Verdict}}</p>

  <section class="card hotel">
    <img src="{{.Plan.Hotel.Image}}" alt="{{.Plan.Hotel.Name}}" loading="lazy">
    <div>
      <span class="kicker">Отель в плане</span>
      <h2>{{.Plan.Hotel.Name}}</h2>
      <p>{{.Plan.Hotel.ShortDescription}}</p>
      <div class="meta">
        <span>★ {{.Plan.Hotel.Rating}}</span>
        <span>{{money .Plan.Hotel.PriceFrom}} за ночь</span>
        <span>{{plural .Plan.Nights "ночь" "ночи" "ночей"}}</span>
      </div>
    </div>
    <div class="score">{{.Plan.Match.Score}}%</div>
  </section>

  <section class="card">
    <div class="budget-top">
      <div>
        <span class="kicker">Честная стоимость поездки</span>
        <strong>{{money .Plan.Budget.Total}}</strong>
        <small>{{money .Plan.Budget.PerPerson}} на человека · {{money .Plan.Budget.PerDay}} в день</small>
      </div>
      <span class="fit">В бюджет</span>
    </div>
    <div class="bars">
      {{range .BudgetRows}}
      <div class="bar"><span>{{.Label}}</span><span class="bar-track"><i style="width:{{.Percent}}%"></i></span><strong>{{money .Value}}</strong></div>
      {{end}}
    </div>
  </section>

  {{range .Plan.Days}}
  <article class="day">
    <div class="day-head">
      <span class="day-date">{{shortDate .Date}}</span>
      <span class="day-title">{{.Title}}</span>
      <span class="day-weather">{{.Weather.Icon}} {{.Weather.TempC}}°C</span>
      <span class="day-total">{{money .Total}}</span>
    </div>
    <div class="blocks">
      {{range .Blocks}}
      <div class="block">
        <time>{{.Time}}</time>
        <div><strong>{{.Title}}</strong>{{if .Place}} · {{.Place}}{{end}}{{if .Note}}<small>{{.Note}}</small>{{end}}</div>
        <span class="price">{{if .Price}}{{money .Price}}{{else}}—{{end}}</span>
      </div>
      {{end}}
    </div>
  </article>
  {{end}}

  {{if .Plan.Tips}}
  <ul class="tips">
    {{range .Plan.Tips}}<li>{{.}}</li>{{end}}
  </ul>
  {{end}}

  <section class="card share">
    <div>
      <span class="kicker">Поделиться маршрутом</span>
      <p class="share-link" id="shareLink">{{.Link}}</p>
      <button type="button" class="copy" id="copyLink">Скопировать ссылку</button>
      <div class="views">Ссылку открыли {{.Views}} раз</div>
    </div>
    <div id="qr" aria-label="QR-код со ссылкой на маршрут"></div>
  </section>

  <a class="cta" href="/">Собрать свой маршрут →</a>
  <p class="foot">О!дин подбирает отель по вашим потребностям и считает полную стоимость с налогами, сборами и завтраком.</p>
</div>
<script src="/assets/vendor/qrcode.js"></script>
<script>
(function () {
  var link = document.getElementById("shareLink").textContent.trim();
  try {
    var qr = qrcode(0, "M");
    qr.addData(link);
    qr.make();
    document.getElementById("qr").innerHTML = qr.createSvgTag({ cellSize: 4, margin: 4, scalable: true });
  } catch (error) {
    document.getElementById("qr").textContent = "QR недоступен";
  }
  document.getElementById("copyLink").addEventListener("click", async function () {
    var button = this;
    try {
      await navigator.clipboard.writeText(link);
      button.textContent = "Ссылка скопирована";
    } catch (error) {
      button.textContent = "Выделите и скопируйте ссылку";
    }
    setTimeout(function () { button.textContent = "Скопировать ссылку"; }, 2200);
  });
})();
</script>
</body>
</html>`))

func (api *API) handleSharePage(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimSpace(r.PathValue("slug"))
	plan, views, err := api.store.SharedPlan(r.Context(), slug)
	if errors.Is(err, ErrNotFound) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `<!doctype html><html lang="ru"><meta charset="utf-8"><title>Маршрут не найден</title><body style="font-family:sans-serif;padding:40px"><h1>Маршрут не найден</h1><p><a href="/">Вернуться в О!дин</a></p></body></html>`)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Не удалось загрузить маршрут")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := sharePageTemplate.Execute(w, sharePageData{
		Plan:       plan,
		BudgetRows: shareBudgetRows(plan.Budget),
		Views:      views,
		Link:       shareURL(r, slug),
	}); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func shareBlockIcon(kind string) string {
	switch kind {
	case "food":
		return "🍽"
	case "activity":
		return "📍"
	case "rest":
		return "☕"
	case "transfer":
		return "🧳"
	default:
		return "•"
	}
}
