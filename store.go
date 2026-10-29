package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	_ "modernc.org/sqlite"
)

var (
	ErrNotFound    = errors.New("not found")
	ErrUnavailable = errors.New("unavailable")
	ErrCantCancel  = errors.New("cannot cancel")
)

const (
	defaultAccent   = "#1F4BDB"
	defaultMenuTint = "#FFFFFF"
)

type Store struct {
	db        *sql.DB
	bookingMu sync.Mutex
}

func OpenStore(path string) (*Store, error) {
	if path == "" {
		path = "odin.db"
	}

	if path != ":memory:" {
		dir := filepath.Dir(path)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("create database directory: %w", err)
			}
		}
	}

	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if path == ":memory:" {
		db.SetMaxOpenConns(1)
	} else {
		db.SetMaxOpenConns(6)
	}
	db.SetMaxIdleConns(6)
	db.SetConnMaxLifetime(0)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT NOT NULL COLLATE NOCASE UNIQUE,
			password_hash TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token_hash TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_expiry ON sessions(expires_at)`,
		`CREATE TABLE IF NOT EXISTS preferences (
			user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
			accent TEXT NOT NULL,
			menu_tint TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS destinations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT NOT NULL UNIQUE,
			city TEXT NOT NULL,
			country TEXT NOT NULL,
			tagline TEXT NOT NULL,
			description TEXT NOT NULL,
			image TEXT NOT NULL,
			price_from INTEGER NOT NULL,
			tags_json TEXT NOT NULL,
			aliases_json TEXT NOT NULL,
			sort_order INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS hotels (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			destination_id INTEGER NOT NULL REFERENCES destinations(id) ON DELETE CASCADE,
			slug TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			short_description TEXT NOT NULL,
			description TEXT NOT NULL,
			address TEXT NOT NULL,
			image TEXT NOT NULL,
			gallery_json TEXT NOT NULL,
			rating REAL NOT NULL,
			reviews_count INTEGER NOT NULL,
			price_per_night INTEGER NOT NULL,
			vibe_tags_json TEXT NOT NULL,
			amenities_json TEXT NOT NULL,
			featured INTEGER NOT NULL DEFAULT 0,
			sort_order INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_hotels_destination ON hotels(destination_id)`,
		`CREATE TABLE IF NOT EXISTS rooms (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			hotel_id INTEGER NOT NULL REFERENCES hotels(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			description TEXT NOT NULL,
			price INTEGER NOT NULL,
			capacity INTEGER NOT NULL,
			bed TEXT NOT NULL,
			size INTEGER NOT NULL,
			amenities_json TEXT NOT NULL,
			image TEXT NOT NULL,
			total_units INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_rooms_hotel ON rooms(hotel_id)`,
		`CREATE TABLE IF NOT EXISTS bookings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			code TEXT NOT NULL UNIQUE,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			hotel_id INTEGER NOT NULL REFERENCES hotels(id),
			room_id INTEGER NOT NULL REFERENCES rooms(id),
			check_in TEXT NOT NULL,
			check_out TEXT NOT NULL,
			guests INTEGER NOT NULL,
			nights INTEGER NOT NULL,
			total INTEGER NOT NULL,
			status TEXT NOT NULL,
			guest_name TEXT NOT NULL,
			guest_email TEXT NOT NULL,
			guest_phone TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_bookings_user ON bookings(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_bookings_room_dates ON bookings(room_id, check_in, check_out)`,
		`CREATE TABLE IF NOT EXISTS favorites (
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			hotel_id INTEGER NOT NULL REFERENCES hotels(id) ON DELETE CASCADE,
			created_at TEXT NOT NULL,
			PRIMARY KEY (user_id, hotel_id)
		)`,
		`CREATE TABLE IF NOT EXISTS pois (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			destination_slug TEXT NOT NULL,
			name TEXT NOT NULL,
			category TEXT NOT NULL,
			description TEXT NOT NULL,
			address TEXT NOT NULL,
			distance TEXT NOT NULL,
			tags_json TEXT NOT NULL,
			price_level INTEGER NOT NULL,
			sort_order INTEGER NOT NULL DEFAULT 0,
			lat REAL NOT NULL DEFAULT 0,
			lng REAL NOT NULL DEFAULT 0,
			schematic INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS seed_meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
	}

	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("migration: %w", err)
		}
	}

	if err := s.ensureColumn("hotels", "meta_json", "TEXT NOT NULL DEFAULT '{}'"); err != nil {
		return err
	}
	if err := s.ensureColumn("pois", "minutes", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn("pois", "lat", "REAL NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn("pois", "lng", "REAL NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn("pois", "schematic", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn("hotels", "capacity_hint", "INTEGER NOT NULL DEFAULT 2"); err != nil {
		return err
	}
	return s.ensureOrderTables()
}

func (s *Store) ensureColumn(table, column, definition string) error {
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var cid, notNull, pk int
		var name, columnType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			rows.Close()
			return err
		}
		if name == column {
			found = true
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if found {
		return nil
	}
	_, err = s.db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` ` + definition)
	return err
}

func (s *Store) FindUserByEmail(ctx context.Context, email string) (User, error) {
	var user User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, email, created_at FROM users WHERE email = ?`,
		strings.ToLower(strings.TrimSpace(email)),
	).Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return user, err
}

func (s *Store) UserPasswordHash(ctx context.Context, email string) (int64, string, error) {
	var id int64
	var hash string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, password_hash FROM users WHERE email = ?`,
		strings.ToLower(strings.TrimSpace(email)),
	).Scan(&id, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", ErrNotFound
	}
	return id, hash, err
}

func (s *Store) CreateUser(ctx context.Context, name, email, passwordHash string) (User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := tx.ExecContext(ctx,
		`INSERT INTO users (name, email, password_hash, created_at) VALUES (?, ?, ?, ?)`,
		name, strings.ToLower(strings.TrimSpace(email)), passwordHash, now,
	)
	if err != nil {
		return User{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return User{}, err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO preferences (user_id, accent, menu_tint, updated_at) VALUES (?, ?, ?, ?)`,
		id, defaultAccent, defaultMenuTint, now,
	); err != nil {
		return User{}, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, err
	}

	return User{ID: id, Name: name, Email: strings.ToLower(strings.TrimSpace(email)), CreatedAt: now}, nil
}

func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	var user User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, email, created_at FROM users WHERE id = ?`,
		id,
	).Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return user, err
}

func (s *Store) CreateSession(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error {
	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, now.Format(time.RFC3339)); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`,
		tokenHash, userID, expiresAt.UTC().Format(time.RFC3339), now.Format(time.RFC3339),
	)
	return err
}

func (s *Store) UserBySession(ctx context.Context, tokenHash string) (User, error) {
	var user User
	err := s.db.QueryRowContext(ctx,
		`SELECT u.id, u.name, u.email, u.created_at
		 FROM sessions s JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = ? AND s.expires_at > ?`,
		tokenHash, time.Now().UTC().Format(time.RFC3339),
	).Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return user, err
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (s *Store) Preferences(ctx context.Context, userID int64) (Preferences, error) {
	var preferences Preferences
	err := s.db.QueryRowContext(ctx,
		`SELECT accent, menu_tint FROM preferences WHERE user_id = ?`,
		userID,
	).Scan(&preferences.Accent, &preferences.MenuTint)
	if errors.Is(err, sql.ErrNoRows) {
		preferences = Preferences{Accent: defaultAccent, MenuTint: defaultMenuTint}
		_, insertErr := s.db.ExecContext(ctx,
			`INSERT INTO preferences (user_id, accent, menu_tint, updated_at) VALUES (?, ?, ?, ?)`,
			userID, preferences.Accent, preferences.MenuTint, time.Now().UTC().Format(time.RFC3339),
		)
		return preferences, insertErr
	}
	return preferences, err
}

func (s *Store) SavePreferences(ctx context.Context, userID int64, preferences Preferences) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO preferences (user_id, accent, menu_tint, updated_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET
		   accent = excluded.accent,
		   menu_tint = excluded.menu_tint,
		   updated_at = excluded.updated_at`,
		userID, preferences.Accent, preferences.MenuTint, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

func (s *Store) Home(ctx context.Context, userID int64) (HomePayload, error) {
	destinations, err := s.destinations(ctx)
	if err != nil {
		return HomePayload{}, err
	}
	favoriteIDs, _ := s.favoriteSet(ctx, userID)
	featured, err := s.featuredHotels(ctx, favoriteIDs)
	if err != nil {
		return HomePayload{}, err
	}

	var stats HomeStats
	stats.DestinationCount = len(destinations)
	stats.ConciergeCount = 12480
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(AVG(rating), 0) FROM hotels`,
	).Scan(&stats.HotelCount, &stats.AverageRating); err != nil {
		return HomePayload{}, err
	}

	allHotels, err := s.allHotelCards(ctx, favoriteIDs)
	if err != nil {
		return HomePayload{}, err
	}
	for index := range featured {
		if featured[index].Meta.TagSlugs == nil {
			featured[index].Meta = decodeHotelMeta("{}")
		}
	}

	deals := make([]HotelCard, 0, 6)
	for _, hotel := range allHotels {
		if hotel.Meta.Discount > 0 {
			deals = append(deals, hotel)
		}
	}
	sort.SliceStable(deals, func(i, j int) bool {
		if deals[i].Meta.Discount != deals[j].Meta.Discount {
			return deals[i].Meta.Discount > deals[j].Meta.Discount
		}
		return deals[i].Rating > deals[j].Rating
	})
	if len(deals) > 6 {
		deals = deals[:6]
	}

	return HomePayload{
		Destinations: destinations,
		Featured:     featured,
		Deals:        deals,
		Moods:        MoodsWithCounts(allHotels),
		Stats:        stats,
	}, nil
}

func (s *Store) destinations(ctx context.Context) ([]Destination, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, slug, city, country, tagline, description, image, price_from, tags_json
		 FROM destinations ORDER BY sort_order, id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	destinations := make([]Destination, 0)
	for rows.Next() {
		var destination Destination
		var tagsJSON string
		if err := rows.Scan(
			&destination.ID, &destination.Slug, &destination.City, &destination.Country,
			&destination.Tagline, &destination.Description, &destination.Image,
			&destination.PriceFrom, &tagsJSON,
		); err != nil {
			return nil, err
		}
		destination.Tags = decodeStrings(tagsJSON)
		destinations = append(destinations, destination)
	}
	return destinations, rows.Err()
}

func (s *Store) Destinations(ctx context.Context) ([]Destination, error) {
	return s.destinations(ctx)
}

func (s *Store) featuredHotels(ctx context.Context, favoriteIDs map[int64]bool) ([]HotelCard, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT h.id, h.slug, h.name, d.city, d.country, d.slug, h.image, h.address,
		        h.short_description, h.rating, h.reviews_count,
		        COALESCE(MIN(r.price), h.price_per_night), h.vibe_tags_json, h.featured,
		        h.meta_json, h.amenities_json, COALESCE(MAX(r.capacity), 2)
		 FROM hotels h
		 JOIN destinations d ON d.id = h.destination_id
		 LEFT JOIN rooms r ON r.hotel_id = h.id
		 WHERE h.featured = 1
		 GROUP BY h.id
		 ORDER BY h.sort_order, h.rating DESC
		 LIMIT 6`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanHotelCards(rows, favoriteIDs)
}

func (s *Store) allHotelCards(ctx context.Context, favoriteIDs map[int64]bool) ([]HotelCard, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT h.id, h.slug, h.name, d.city, d.country, d.slug, h.image, h.address,
		        h.short_description, h.rating, h.reviews_count,
		        COALESCE(MIN(r.price), h.price_per_night), h.vibe_tags_json, h.featured,
		        h.meta_json, h.amenities_json, COALESCE(MAX(r.capacity), 2)
		 FROM hotels h
		 JOIN destinations d ON d.id = h.destination_id
		 LEFT JOIN rooms r ON r.hotel_id = h.id
		 GROUP BY h.id
		 ORDER BY h.sort_order, h.id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanHotelCards(rows, favoriteIDs)
}

func (s *Store) Search(ctx context.Context, userID int64, params SearchParams) (SearchResult, error) {
	favoriteIDs, _ := s.favoriteSet(ctx, userID)
	hotels, err := s.allHotelCards(ctx, favoriteIDs)
	if err != nil {
		return SearchResult{}, err
	}

	explicitTags := NormalizeTags(append([]string{params.Tag}, params.Tags...))
	inferredTags := InferTags(params.Query)
	moodSlug := strings.TrimSpace(params.Mood)
	if _, ok := moodBySlug[moodSlug]; !ok {
		moodSlug = ""
	}
	if moodSlug == "" && params.Query != "" {
		moodSlug = InferMood(params.Query)
	}
	mood := moodBySlug[moodSlug]

	destinationSlug := strings.TrimSpace(params.Destination)
	if destinationSlug == "" && strings.TrimSpace(params.Query) != "" {
		if detected, detectErr := s.DetectDestination(ctx, params.Query); detectErr == nil {
			destinationSlug = detected.Slug
		}
	}

	queryTokens := searchTokens(params.Query)
	hasIntent := len(inferredTags) > 0 || moodSlug != "" || destinationSlug != ""

	base := make([]HotelCard, 0, len(hotels))
	for _, hotel := range hotels {
		if destinationSlug != "" && hotel.DestinationSlug != destinationSlug {
			continue
		}
		if params.Guests > 0 && hotel.Capacity < params.Guests {
			continue
		}
		if params.MaxPrice > 0 && hotel.PriceFrom > params.MaxPrice {
			continue
		}
		if params.MinPrice > 0 && hotel.PriceFrom < params.MinPrice {
			continue
		}
		if params.MinRating > 0 && hotel.Rating < params.MinRating {
			continue
		}
		if params.Insured && !hotel.Meta.Insured {
			continue
		}
		if params.Guides && !hotel.Meta.Guides {
			continue
		}
		if params.Parking && !hotel.Meta.Parking {
			continue
		}
		if params.Pool && !hotel.Meta.Pool {
			continue
		}
		if params.Soundproof && !hotel.Meta.Soundproof {
			continue
		}
		if params.Discount && !containsString(hotel.Meta.TagSlugs, "skidka") {
			continue
		}
		if params.Lat != 0 || params.Lng != 0 {
			hotel.DistanceKm = haversine(params.Lat, params.Lng, hotel.Meta.Lat, hotel.Meta.Lng)
			if params.RadiusKm > 0 && hotel.DistanceKm > params.RadiusKm {
				continue
			}
		}
		if len(queryTokens) > 0 {
			literal := hotelMatchesQueryTokens(hotel, queryTokens)
			if !literal {
				tagOverlap := tagOverlap(hotel.Meta.TagSlugs, inferredTags)
				moodOverlap := 0
				if moodSlug != "" {
					moodOverlap = MoodScore(hotel.Meta.TagSlugs, mood)
				}
				if !hasIntent || (tagOverlap == 0 && moodOverlap == 0 && (destinationSlug == "" || hotel.DestinationSlug != destinationSlug)) {
					continue
				}
			}
		}
		base = append(base, hotel)
	}

	facets := buildFacets(base)
	needsEval := NeedsEval{Rejected: make([]NeedRejection, 0, 8), Applied: len(params.Needs)}
	for _, need := range params.Needs {
		if need.Kind == NeedKindNearby && need.MaxMinutes > needsEval.Radius {
			needsEval.Radius = need.MaxMinutes
		}
	}
	filtered := make([]HotelCard, 0, len(base))
	for _, hotel := range base {
		if len(explicitTags) > 0 && !containsAllTags(hotel.Meta.TagSlugs, explicitTags) {
			continue
		}
		if moodSlug != "" {
			score := MoodScore(hotel.Meta.TagSlugs, mood)
			if score == 0 {
				continue
			}
			hotel.Match = score
		}
		if len(inferredTags) > 0 {
			tagScore := overlapPercent(hotel.Meta.TagSlugs, inferredTags)
			if tagScore > hotel.Match {
				hotel.Match = tagScore
			}
		}
		if lr := logisticRecommendation(hotel, params, moodSlug, inferredTags); lr > hotel.Match {
			hotel.Match = lr
		}
		if hotel.Match == 0 && len(queryTokens) > 0 && hotelMatchesQueryTokens(hotel, queryTokens) {
			hotel.Match = 92
		}
		if len(params.Needs) > 0 {
			checks, pass, reason := EvaluateNeeds(hotel, params.Needs)
			if !pass {
				needsEval.RejectedCount++
				if len(needsEval.Rejected) < 12 {
					needsEval.Rejected = append(needsEval.Rejected, NeedRejection{
						Slug:   hotel.Slug,
						Name:   hotel.Name,
						City:   hotel.City,
						Image:  hotel.Image,
						Reason: reason,
					})
				}
				continue
			}
			hotel.Needs = checks
			if score := NeedsScore(checks); score >= 0 {
				if hotel.Match > 0 {
					hotel.Match = clampInt(int(0.6*float64(hotel.Match)+0.4*float64(score)+0.5), 1, 100)
				} else {
					hotel.Match = clampInt(score, 1, 100)
				}
			}
		}
		filtered = append(filtered, hotel)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		left, right := filtered[i], filtered[j]
		sortMode := params.Sort
		if sortMode == "" && (moodSlug != "" || len(inferredTags) > 0 || len(queryTokens) > 0 || len(params.Needs) > 0) {
			sortMode = "match"
		}
		switch sortMode {
		case "price_asc":
			return left.PriceFrom < right.PriceFrom
		case "price_desc":
			return left.PriceFrom > right.PriceFrom
		case "rating":
			return left.Rating > right.Rating
		case "distance":
			return left.DistanceKm < right.DistanceKm
		case "match":
			if left.Match != right.Match {
				return left.Match > right.Match
			}
			if left.Rating != right.Rating {
				return left.Rating > right.Rating
			}
		}
		if left.Featured != right.Featured {
			return left.Featured
		}
		return left.Rating > right.Rating
	})

	guests := params.Guests
	if guests <= 0 {
		guests = 2
	}
	for i := range filtered {
		filtered[i].PriceBreakdown = buildPriceBreakdown(filtered[i].PriceFrom, guests)
	}
	total := len(filtered)
	if len(filtered) > 60 {
		filtered = filtered[:60]
	}
	return SearchResult{
		Hotels: filtered,
		Total:  total,
		Query:  params.Query,
		Facets: facets,
		Mood:   moodSlug,
		Tags:   explicitTags,
		Nearby: params.Lat != 0 || params.Lng != 0,
		Needs:  needsEval,
	}, nil
}

func hotelMatchesQuery(hotel HotelCard, query string) bool {
	return hotelMatchesQueryTokens(hotel, searchTokens(query))
}

var searchStopWords = map[string]bool{
	"отель": true, "отели": true, "гостиница": true, "гостиницы": true, "номер": true,
	"для": true, "у": true, "с": true, "и": true, "в": true, "на": true, "рядом": true,
	"хочу": true, "хочется": true, "мне": true, "нужен": true, "нужна": true,
	"поехать": true, "поездка": true, "поездку": true, "отдых": true, "отдыхать": true,
	"где": true, "что": true, "как": true, "the": true, "a": true, "to": true,
	"in": true, "with": true, "near": true, "hotel": true, "гостиницу": true,
	"подбери": true, "подобрать": true, "найди": true, "найти": true,
}

func searchTokens(query string) []string {
	lower := strings.ToLower(strings.TrimSpace(query))
	if lower == "" {
		return nil
	}
	fields := strings.FieldsFunc(lower, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= 'а' && r <= 'я') && r != 'ё' && !(r >= '0' && r <= '9') && r != '-'
	})
	tokens := make([]string, 0, len(fields))
	for _, token := range fields {
		if len([]rune(token)) < 2 || searchStopWords[token] {
			continue
		}
		tokens = append(tokens, token)
	}
	return tokens
}

func hotelMatchesQueryTokens(hotel HotelCard, tokens []string) bool {
	if len(tokens) == 0 {
		return true
	}
	haystack := strings.ToLower(strings.Join([]string{
		hotel.Name,
		hotel.City,
		hotel.Country,
		hotel.Address,
		hotel.ShortDescription,
		strings.Join(hotel.Tags, " "),
		strings.Join(hotel.TagSlugs, " "),
		strings.Join(hotel.Amenities, " "),
		hotel.Meta.Highlight,
	}, " "))
	for _, token := range tokens {
		if !strings.Contains(haystack, token) {
			return false
		}
	}
	return true
}

func tagOverlap(haystack, needles []string) int {
	have := make(map[string]bool, len(haystack))
	for _, value := range haystack {
		have[value] = true
	}
	matched := 0
	for _, needle := range needles {
		if have[needle] {
			matched++
		}
	}
	return matched
}

func containsAllTags(haystack, needles []string) bool {
	have := make(map[string]bool, len(haystack))
	for _, value := range haystack {
		have[value] = true
	}
	for _, needle := range needles {
		if !have[needle] {
			return false
		}
	}
	return true
}

func overlapPercent(haystack, needles []string) int {
	if len(needles) == 0 {
		return 0
	}
	have := make(map[string]bool, len(haystack))
	for _, value := range haystack {
		have[value] = true
	}
	matched := 0
	for _, needle := range needles {
		if have[needle] {
			matched++
		}
	}
	if matched == 0 {
		return 0
	}
	return 55 + int(float64(matched)/float64(len(needles))*43)
}

func haversine(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadius = 6371.0
	if lat1 == 0 || lng1 == 0 || lat2 == 0 || lng2 == 0 {
		return 0
	}
	toRad := math.Pi / 180
	dLat := (lat2 - lat1) * toRad
	dLng := (lng2 - lng1) * toRad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*toRad)*math.Cos(lat2*toRad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return math.Round(earthRadius*c*10) / 10
}

func scanHotelCards(rows *sql.Rows, favoriteIDs map[int64]bool) ([]HotelCard, error) {
	hotels := make([]HotelCard, 0)
	for rows.Next() {
		var hotel HotelCard
		var tagsJSON, metaJSON, amenitiesJSON string
		if err := rows.Scan(
			&hotel.ID, &hotel.Slug, &hotel.Name, &hotel.City, &hotel.Country,
			&hotel.DestinationSlug, &hotel.Image, &hotel.Address, &hotel.ShortDescription,
			&hotel.Rating, &hotel.Reviews, &hotel.PriceFrom, &tagsJSON, &hotel.Featured,
			&metaJSON, &amenitiesJSON, &hotel.Capacity,
		); err != nil {
			return nil, err
		}
		hotel.Meta = decodeHotelMeta(metaJSON)
		hotel.Amenities = decodeStrings(amenitiesJSON)
		if len(hotel.Meta.TagSlugs) > 0 {
			hotel.TagSlugs = hotel.Meta.TagSlugs
			hotel.Tags = TagsToLabels(hotel.Meta.TagSlugs)
		} else {
			hotel.Tags = decodeStrings(tagsJSON)
			hotel.TagSlugs = []string{}
		}
		hotel.Favorited = favoriteIDs != nil && favoriteIDs[hotel.ID]
		deriveAmenities(&hotel)
		// Число гостей запроса неизвестно -> 2 (по умолчанию), но не вместимость.
		hotel.PriceBreakdown = buildPriceBreakdown(hotel.PriceFrom, 2)
		hotels = append(hotels, hotel)
	}
	return hotels, rows.Err()
}

func (s *Store) HotelBySlug(ctx context.Context, userID int64, slug, checkIn, checkOut string, guests int) (HotelDetail, error) {
	var detail HotelDetail
	var tagsJSON, destinationTagsJSON, aliasesJSON, galleryJSON, amenitiesJSON, metaJSON string
	var destinationID int64

	err := s.db.QueryRowContext(ctx,
		`SELECT h.id, h.slug, h.name, d.city, d.country, d.slug, h.image, h.address,
		        h.short_description, h.description, h.rating, h.reviews_count, h.price_per_night,
		        h.vibe_tags_json, h.amenities_json, h.gallery_json, h.featured, h.meta_json,
		        d.id, d.tagline, d.description, d.image, d.price_from, d.tags_json, d.aliases_json
		 FROM hotels h JOIN destinations d ON d.id = h.destination_id
		 WHERE h.slug = ?`,
		slug,
	).Scan(
		&detail.ID, &detail.Slug, &detail.Name, &detail.City, &detail.Country, &detail.DestinationSlug,
		&detail.Image, &detail.Address, &detail.ShortDescription, &detail.Description,
		&detail.Rating, &detail.Reviews, &detail.PriceFrom, &tagsJSON, &amenitiesJSON, &galleryJSON,
		&detail.Featured, &metaJSON, &destinationID, &detail.Destination.Tagline, &detail.Destination.Description,
		&detail.Destination.Image, &detail.Destination.PriceFrom, &destinationTagsJSON, &aliasesJSON,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return HotelDetail{}, ErrNotFound
	}
	if err != nil {
		return HotelDetail{}, err
	}
	detail.Destination.ID = destinationID
	detail.Destination.Slug = detail.DestinationSlug
	detail.Destination.City = detail.City
	detail.Destination.Country = detail.Country
	detail.Destination.Tags = decodeStrings(destinationTagsJSON)
	detail.Meta = decodeHotelMeta(metaJSON)
	detail.TagSlugs = detail.Meta.TagSlugs
	if len(detail.TagSlugs) > 0 {
		detail.Tags = TagsToLabels(detail.TagSlugs)
	} else {
		detail.Tags = decodeStrings(tagsJSON)
		detail.TagSlugs = []string{}
	}
	detail.Amenities = decodeStrings(amenitiesJSON)
	deriveAmenities(&detail.HotelCard)
	if len(detail.Amenities) == 0 {
		detail.Amenities = detail.HotelCard.Amenities
	}
	detail.Gallery = decodeStrings(galleryJSON)
	detail.CheckIn = checkIn
	detail.CheckOut = checkOut
	detail.Guests = guests

	rooms, err := s.rooms(ctx, detail.ID, guests)
	if err != nil {
		return HotelDetail{}, err
	}
	if len(rooms) > 0 {
		minPrice := rooms[0].Price
		for _, room := range rooms {
			if room.Price < minPrice {
				minPrice = room.Price
			}
		}
		detail.PriceFrom = minPrice
	}
	detail.PriceBreakdown = buildPriceBreakdown(detail.PriceFrom, guests)

	if checkIn != "" && checkOut != "" {
		booked, err := s.bookedRoomCounts(ctx, detail.ID, checkIn, checkOut)
		if err == nil {
			for index := range rooms {
				rooms[index].Available = max(0, rooms[index].TotalUnits-booked[rooms[index].ID])
			}
		}
	} else {
		for index := range rooms {
			rooms[index].Available = rooms[index].TotalUnits
		}
	}
	annotateRooms(rooms, detail.Meta)
	detail.Rooms = rooms

	allPOIs, err := s.poisForDestination(ctx, detail.DestinationSlug, "", 24)
	if err != nil {
		return HotelDetail{}, err
	}
	detail.Nearby = make([]POI, 0, len(allPOIs))
	detail.Excursions = make([]POI, 0, 3)
	for index := range allPOIs {
		poi := allPOIs[index]
		if poi.Minutes == 0 {
			poi.Minutes = parseMinutes(poi.Distance)
		}
		if poi.Minutes > 0 && poi.Minutes < 45 && !strings.Contains(poi.Distance, "такси") {
			poi.Minutes += int(detail.ID % 3)
			poi.Distance = fmt.Sprintf("%d мин пешком", poi.Minutes)
		}
		if poi.Category == "excursion" {
			detail.Excursions = append(detail.Excursions, poi)
			continue
		}
		detail.Nearby = append(detail.Nearby, poi)
	}
	if len(detail.Nearby) > 9 {
		detail.Nearby = detail.Nearby[:9]
	}

	detail.Walk = planWalkingRoute(detail.HotelCard, allPOIs, "calm", false, 0)

	favoriteIDs, _ := s.favoriteSet(ctx, userID)
	detail.Favorited = favoriteIDs[detail.ID]
	return detail, nil
}

func (s *Store) rooms(ctx context.Context, hotelID int64, guests int) ([]Room, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, description, price, capacity, bed, size, amenities_json, image, total_units
		 FROM rooms WHERE hotel_id = ? ORDER BY price`,
		hotelID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rooms := make([]Room, 0)
	for rows.Next() {
		var room Room
		var amenitiesJSON string
		if err := rows.Scan(
			&room.ID, &room.Name, &room.Description, &room.Price, &room.Capacity,
			&room.Bed, &room.Size, &amenitiesJSON, &room.Image, &room.TotalUnits,
		); err != nil {
			return nil, err
		}
		room.Amenities = decodeStrings(amenitiesJSON)
		room.PriceBreakdown = buildPriceBreakdown(room.Price, guests)
		rooms = append(rooms, room)
	}
	return rooms, rows.Err()
}

func (s *Store) bookedRoomCounts(ctx context.Context, hotelID int64, checkIn, checkOut string) (map[int64]int, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT room_id, COUNT(*) FROM bookings
		 WHERE hotel_id = ? AND status = 'confirmed' AND check_in < ? AND check_out > ?
		 GROUP BY room_id`,
		hotelID, checkOut, checkIn,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[int64]int)
	for rows.Next() {
		var roomID int64
		var count int
		if err := rows.Scan(&roomID, &count); err != nil {
			return nil, err
		}
		counts[roomID] = count
	}
	return counts, rows.Err()
}

func (s *Store) poisForDestination(ctx context.Context, destinationSlug, category string, limit int) ([]POI, error) {
	args := []any{destinationSlug}
	query := `SELECT id, name, category, description, address, distance, minutes, tags_json, price_level, lat, lng, schematic
	          FROM pois WHERE destination_slug = ?`
	if category != "" {
		query += ` AND category = ?`
		args = append(args, category)
	}
	query += ` ORDER BY sort_order, id LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pois := make([]POI, 0)
	for rows.Next() {
		var poi POI
		var tagsJSON string
		var schematic int
		if err := rows.Scan(
			&poi.ID, &poi.Name, &poi.Category, &poi.Description,
			&poi.Address, &poi.Distance, &poi.Minutes, &tagsJSON, &poi.PriceLevel,
			&poi.Lat, &poi.Lng, &schematic,
		); err != nil {
			return nil, err
		}
		poi.Tags = decodeStrings(tagsJSON)
		poi.DestinationSlug = destinationSlug
		poi.Schematic = schematic == 1
		if poi.Lat == 0 && poi.Lng == 0 {
			poi.Lat, poi.Lng = fallbackCityCoord(destinationSlug, poi.Name)
			poi.Schematic = true
		}
		pois = append(pois, poi)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return []POI{}, nil
	}
	return pois, rows.Err()
}

func (s *Store) POIDetail(ctx context.Context, userID, poiID int64) (POIDetail, error) {
	var poi POI
	var tagsJSON string
	var schematicVar int
	err := s.db.QueryRowContext(ctx,
		`SELECT id, destination_slug, name, category, description, address, distance, minutes, tags_json, price_level, lat, lng, schematic
		 FROM pois WHERE id = ?`, poiID,
	).Scan(
		&poi.ID, &poi.DestinationSlug, &poi.Name, &poi.Category, &poi.Description,
		&poi.Address, &poi.Distance, &poi.Minutes, &tagsJSON, &poi.PriceLevel,
		&poi.Lat, &poi.Lng, &schematicVar,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return POIDetail{}, ErrNotFound
	}
	if err != nil {
		return POIDetail{}, err
	}
	poi.Tags = decodeStrings(tagsJSON)
	poi.Schematic = schematicVar == 1

	favoriteIDs, _ := s.favoriteSet(ctx, userID)
	hotels, err := s.allHotelCards(ctx, favoriteIDs)
	if err != nil {
		return POIDetail{}, err
	}
	nearby := make([]HotelCard, 0, len(hotels))
	for _, hotel := range hotels {
		if hotel.DestinationSlug != poi.DestinationSlug {
			continue
		}
		nearby = append(nearby, hotel)
	}
	sort.SliceStable(nearby, func(i, j int) bool {
		if nearby[i].Rating != nearby[j].Rating {
			return nearby[i].Rating > nearby[j].Rating
		}
		return nearby[i].PriceFrom < nearby[j].PriceFrom
	})
	if len(nearby) > 6 {
		nearby = nearby[:6]
	}

	city := ""
	if len(nearby) > 0 {
		city = nearby[0].City
	}
	return POIDetail{POI: poi, Hotels: nearby, City: city}, nil
}

func (s *Store) Weather(destination, date string) WeatherInfo {
	seed := 0
	for _, ch := range destination + date {
		seed = seed*31 + int(ch)
	}
	if seed < 0 {
		seed = -seed
	}
	month := 6
	if parsed, err := time.Parse("2006-01-02", date); err == nil {
		month = int(parsed.Month())
	}
	// Детерминированный «прогноз» по климатической норме города: средняя температура месяца
	// ± 4 °C и вероятность осадков по месяцу. Ниже нуля осадки - снег.
	normals, ok := cityClimate[strings.TrimSpace(destination)]
	if !ok {
		normals = cityClimate["Москва"]
	}
	temp := normals[month-1] + seed%9 - 4
	precip := []int{35, 30, 30, 30, 30, 30, 30, 30, 30, 35, 40, 40}[month-1]
	condition := "mild"
	switch {
	case (seed/9)%100 < precip && temp <= 0:
		condition = "snow"
	case (seed/9)%100 < precip:
		condition = "rain"
	case temp >= 26:
		condition = "heat"
	case temp < 10:
		condition = "cold"
	}
	descriptions := map[string]string{
		"rain": "Возможен дождь — стоит выбрать отель с крытыми зонами",
		"heat": "Жарко — пригодятся кондиционер и пляж рядом",
		"cold": "Холодно — выбирайте отели с отоплением и сауной",
		"snow": "Снег и мороз — зимние развлечения рядом",
		"mild": "Комфортная погода для прогулок",
	}
	tags := map[string][]string{
		"rain": {"Крытый бассейн", "Спа", "Кинотеатр рядом", "Кондиционер"},
		"heat": {"Кондиционер", "Пляж в 2 минутах", "Открытый бассейн", "Терраса"},
		"cold": {"Отопление", "Сауна", "Камин", "Зимние развлечения"},
		"snow": {"Лыжи рядом", "Сауна", "Камин", "Горячие напитки"},
		"mild": {"Прогулки", "Веранда", "Велопрокат", "Кафе на террасе"},
	}
	icons := map[string]string{
		"rain": "🌧️", "heat": "☀️", "cold": "❄️", "snow": "🌨️", "mild": "🌤️",
	}
	return WeatherInfo{
		Condition:   condition,
		Description: descriptions[condition],
		TempC:       temp,
		Icon:        icons[condition],
		Tags:        tags[condition],
	}
}

// cityClimate - средняя температура по месяцам (январь..декабрь), °C.
var cityClimate = map[string][12]int{
	"Москва":          {-6, -6, -1, 6, 13, 17, 19, 17, 11, 5, -1, -5},
	"Санкт-Петербург": {-6, -6, -2, 5, 11, 16, 19, 17, 11, 5, 0, -4},
	"Сочи":            {6, 6, 9, 13, 17, 21, 24, 24, 20, 16, 11, 8},
	"Казань":          {-11, -10, -4, 6, 13, 18, 20, 18, 11, 4, -3, -8},
	"Калининград":     {-1, -1, 2, 7, 12, 16, 18, 18, 13, 8, 3, 0},
	"Алтай":           {-14, -12, -5, 4, 11, 16, 18, 16, 10, 3, -6, -12},
	"Екатеринбург":    {-13, -11, -4, 4, 11, 16, 18, 15, 9, 2, -5, -11},
	"Нижний Новгород": {-9, -8, -3, 6, 13, 17, 19, 17, 11, 4, -2, -7},
	"Иркутск":         {-18, -15, -6, 3, 10, 16, 18, 16, 9, 1, -9, -16},
	"Владивосток":     {-12, -9, -2, 5, 10, 14, 18, 20, 16, 9, -1, -9},
	"Мурманск":        {-10, -10, -7, -2, 4, 9, 13, 11, 7, 1, -4, -8},
	"Суздаль":         {-9, -8, -3, 5, 12, 16, 18, 16, 10, 4, -2, -6},
}

// buildPriceBreakdown - единая формула цены за ночь (источник истины для
// карточек, поиска и CreateBooking).
// Решение по завтраку: завтрак ВКЛЮЧЁН в цену проживания, если breakfast > 0
// (900 руб. x число гостей запроса; если гостей неизвестно - 2, не вместимость).
// Клиенты не должны добавлять завтрак отдельной строкой поверх Total.
func buildPriceBreakdown(price, guests int) PriceBreakdown {
	if guests < 1 {
		guests = 2
	}
	taxes := int(math.Round(float64(price) * 0.12))
	fees := int(math.Round(float64(price) * 0.05))
	breakfast := 900 * guests
	total := price + taxes + fees + breakfast
	return PriceBreakdown{
		Room:      price,
		Taxes:     taxes,
		Fees:      fees,
		Breakfast: breakfast,
		Total:     total,
	}
}

func (s *Store) Trips(ctx context.Context, userID int64) ([]Trip, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT b.id, b.code, b.status, h.name, h.slug, d.city, h.image, r.name,
		        b.check_in, b.check_out, b.nights, b.guests, b.total,
		        b.guest_name, b.guest_email, b.guest_phone, b.created_at
		 FROM bookings b
		 JOIN hotels h ON h.id = b.hotel_id
		 JOIN destinations d ON d.id = h.destination_id
		 JOIN rooms r ON r.id = b.room_id
		 WHERE b.user_id = ?
		 ORDER BY b.check_in DESC, b.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	today := time.Now().UTC().Format("2006-01-02")
	trips := make([]Trip, 0)
	for rows.Next() {
		var trip Trip
		if err := rows.Scan(
			&trip.ID, &trip.Code, &trip.Status, &trip.HotelName, &trip.HotelSlug,
			&trip.City, &trip.Image, &trip.RoomName, &trip.CheckIn, &trip.CheckOut,
			&trip.Nights, &trip.Guests, &trip.Total, &trip.GuestName,
			&trip.GuestEmail, &trip.GuestPhone, &trip.CreatedAt,
		); err != nil {
			return nil, err
		}
		trip.CanCancel = trip.Status == "confirmed" && trip.CheckIn > today
		trips = append(trips, trip)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if err := s.attachOrders(ctx, userID, trips); err != nil {
		return nil, err
	}
	return trips, nil
}

func (s *Store) CreateBooking(ctx context.Context, userID int64, input BookingInput) (Trip, error) {
	checkIn, err := time.Parse("2006-01-02", input.CheckIn)
	if err != nil {
		return Trip{}, errors.New("invalid check-in date")
	}
	checkOut, err := time.Parse("2006-01-02", input.CheckOut)
	if err != nil {
		return Trip{}, errors.New("invalid check-out date")
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if checkIn.Before(today) || !checkOut.After(checkIn) {
		return Trip{}, errors.New("invalid stay dates")
	}
	nights := int(checkOut.Sub(checkIn).Hours() / 24)
	if nights > 30 {
		return Trip{}, errors.New("stay is too long")
	}
	if input.Guests < 1 || input.Guests > 8 {
		return Trip{}, errors.New("invalid guest count")
	}

	s.bookingMu.Lock()
	defer s.bookingMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Trip{}, err
	}
	defer tx.Rollback()

	var hotelID, roomID int64
	var hotelName, city, hotelSlug, image, roomName string
	var price, capacity, totalUnits int
	err = tx.QueryRowContext(ctx,
		`SELECT h.id, h.name, d.city, h.slug, h.image, r.id, r.name, r.price, r.capacity, r.total_units
		 FROM rooms r
		 JOIN hotels h ON h.id = r.hotel_id
		 JOIN destinations d ON d.id = h.destination_id
		 WHERE r.id = ? AND h.id = ?`,
		input.RoomID, input.HotelID,
	).Scan(&hotelID, &hotelName, &city, &hotelSlug, &image, &roomID, &roomName, &price, &capacity, &totalUnits)
	if errors.Is(err, sql.ErrNoRows) {
		return Trip{}, ErrNotFound
	}
	if err != nil {
		return Trip{}, err
	}
	if input.Guests > capacity {
		return Trip{}, &CapacityError{Capacity: capacity}
	}

	var booked int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM bookings
		 WHERE room_id = ? AND status = 'confirmed' AND check_in < ? AND check_out > ?`,
		roomID, input.CheckOut, input.CheckIn,
	).Scan(&booked); err != nil {
		return Trip{}, err
	}
	if booked >= totalUnits {
		return Trip{}, ErrUnavailable
	}

	// Та же формула, что «Полная стоимость проживания» на карточке.
	total := buildPriceBreakdown(price, input.Guests).Total * nights
	code, err := bookingCode()
	if err != nil {
		return Trip{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := tx.ExecContext(ctx,
		`INSERT INTO bookings
		 (code, user_id, hotel_id, room_id, check_in, check_out, guests, nights, total, status,
		  guest_name, guest_email, guest_phone, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'confirmed', ?, ?, ?, ?)`,
		code, userID, hotelID, roomID, input.CheckIn, input.CheckOut, input.Guests, nights, total,
		input.GuestName, input.GuestEmail, input.GuestPhone, now,
	)
	if err != nil {
		return Trip{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Trip{}, err
	}
	if err := tx.Commit(); err != nil {
		return Trip{}, err
	}

	return Trip{
		ID: id, Code: code, Status: "confirmed", HotelName: hotelName, HotelSlug: hotelSlug,
		City: city, Image: image, RoomName: roomName, CheckIn: input.CheckIn, CheckOut: input.CheckOut,
		Nights: nights, Guests: input.Guests, Total: total, GuestName: input.GuestName,
		GuestEmail: input.GuestEmail, GuestPhone: input.GuestPhone, CanCancel: true, CreatedAt: now,
	}, nil
}

func (s *Store) CancelBooking(ctx context.Context, userID, bookingID int64) error {
	today := time.Now().UTC().Format("2006-01-02")
	result, err := s.db.ExecContext(ctx,
		`UPDATE bookings SET status = 'cancelled'
		 WHERE id = ? AND user_id = ? AND status = 'confirmed' AND check_in > ?`,
		bookingID, userID, today,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrCantCancel
	}
	s.cancelOrderFor(ctx, bookingID)
	return nil
}

func (s *Store) ToggleFavorite(ctx context.Context, userID, hotelID int64) (bool, error) {
	var exists int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM favorites WHERE user_id = ? AND hotel_id = ?`,
		userID, hotelID,
	).Scan(&exists); err != nil {
		return false, err
	}
	if exists > 0 {
		_, err := s.db.ExecContext(ctx,
			`DELETE FROM favorites WHERE user_id = ? AND hotel_id = ?`,
			userID, hotelID,
		)
		return false, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO favorites (user_id, hotel_id, created_at) VALUES (?, ?, ?)`,
		userID, hotelID, now,
	)
	return true, err
}

func (s *Store) Favorites(ctx context.Context, userID int64) ([]HotelCard, error) {
	favoriteIDs, err := s.favoriteSet(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(favoriteIDs) == 0 {
		return []HotelCard{}, nil
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT h.id, h.slug, h.name, d.city, d.country, d.slug, h.image, h.address,
		        h.short_description, h.rating, h.reviews_count,
		        COALESCE(MIN(r.price), h.price_per_night), h.vibe_tags_json, h.featured,
		        h.meta_json, h.amenities_json, COALESCE(MAX(r.capacity), 2)
		 FROM hotels h
		 JOIN destinations d ON d.id = h.destination_id
		 JOIN favorites f ON f.hotel_id = h.id AND f.user_id = ?
		 LEFT JOIN rooms r ON r.hotel_id = h.id
		 GROUP BY h.id
		 ORDER BY f.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanHotelCards(rows, favoriteIDs)
}

func (s *Store) favoriteSet(ctx context.Context, userID int64) (map[int64]bool, error) {
	favorites := make(map[int64]bool)
	if userID <= 0 {
		return favorites, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT hotel_id FROM favorites WHERE user_id = ?`, userID)
	if err != nil {
		return favorites, err
	}
	defer rows.Close()
	for rows.Next() {
		var hotelID int64
		if err := rows.Scan(&hotelID); err != nil {
			return favorites, err
		}
		favorites[hotelID] = true
	}
	return favorites, rows.Err()
}

func (s *Store) DetectDestination(ctx context.Context, message string) (Destination, error) {
	lower := strings.ToLower(message)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, slug, city, country, tagline, description, image, price_from, tags_json, aliases_json
		 FROM destinations ORDER BY sort_order`,
	)
	if err != nil {
		return Destination{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var destination Destination
		var tagsJSON, aliasesJSON string
		if err := rows.Scan(
			&destination.ID, &destination.Slug, &destination.City, &destination.Country,
			&destination.Tagline, &destination.Description, &destination.Image,
			&destination.PriceFrom, &tagsJSON, &aliasesJSON,
		); err != nil {
			return Destination{}, err
		}
		destination.Tags = decodeStrings(tagsJSON)
		aliases := decodeStrings(aliasesJSON)
		for _, alias := range aliases {
			if strings.Contains(lower, strings.ToLower(alias)) {
				return destination, nil
			}
		}
	}
	if err := rows.Err(); err != nil {
		return Destination{}, err
	}
	return Destination{}, ErrNotFound
}

func (s *Store) DestinationBySlug(ctx context.Context, slug string) (Destination, error) {
	var destination Destination
	var tagsJSON, aliasesJSON string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, slug, city, country, tagline, description, image, price_from, tags_json, aliases_json
		 FROM destinations WHERE slug = ?`,
		slug,
	).Scan(
		&destination.ID, &destination.Slug, &destination.City, &destination.Country,
		&destination.Tagline, &destination.Description, &destination.Image,
		&destination.PriceFrom, &tagsJSON, &aliasesJSON,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Destination{}, ErrNotFound
	}
	destination.Tags = decodeStrings(tagsJSON)
	return destination, err
}

func (s *Store) AssistantContext(ctx context.Context, destinationSlug, hotelSlug, message string) (Destination, []POI, []AssistantHotel, error) {
	var destination Destination
	var err error

	// A selected hotel is the strongest signal: build the whole context around it.
	var focus *AssistantHotel
	var focusPOIs []POI
	if strings.TrimSpace(hotelSlug) != "" {
		if detail, detailErr := s.HotelBySlug(ctx, 0, hotelSlug, "", "", 2); detailErr == nil {
			destination = detail.Destination
			assistantHotel := assistantHotelFromDetail(detail)
			focus = &assistantHotel
			focusPOIs = append(focusPOIs, detail.Nearby...)
			focusPOIs = append(focusPOIs, detail.Excursions...)
		}
	}

	if destination.Slug == "" {
		detected, detectErr := s.DetectDestination(ctx, message)
		if detectErr == nil {
			destination = detected
		} else if strings.TrimSpace(destinationSlug) != "" {
			destination, _ = s.DestinationBySlug(ctx, destinationSlug)
		}
	}

	category := categoryFromMessage(message)
	sights := wantsSights(message)
	pois := []POI{}
	if focus != nil {
		pois = prioritizePOIs(focusPOIs, category)
	} else if destination.Slug != "" && sights {
		// «Что посмотреть»: достопримечательности, а не кофейни и рестораны.
		for _, sightCategory := range []string{"culture", "excursion", "walk"} {
			found, poiErr := s.poisForDestination(ctx, destination.Slug, sightCategory, 3)
			if poiErr != nil {
				return Destination{}, nil, nil, poiErr
			}
			pois = append(pois, found...)
		}
		if len(pois) > 3 {
			pois = pois[:3]
		}
	} else if destination.Slug != "" {
		pois, err = s.poisForDestination(ctx, destination.Slug, category, 3)
		if err != nil {
			return Destination{}, nil, nil, err
		}
		if len(pois) == 0 {
			pois, _ = s.poisForDestination(ctx, destination.Slug, "", 3)
		}
	}

	hotels := []AssistantHotel{}
	if focus != nil {
		hotels = append(hotels, *focus)
		// Add up to two alternatives from the same destination so the AI can
		// gently suggest something else without flooding the answer.
		if alternatives, altErr := s.assistantHotels(ctx, destination.Slug, ""); altErr == nil {
			for _, alternative := range alternatives {
				if alternative.Slug == focus.Slug {
					continue
				}
				hotels = append(hotels, alternative)
				if len(hotels) >= 3 {
					break
				}
			}
		}
	} else {
		hotels, err = s.assistantHotels(ctx, destination.Slug, message)
		if err != nil {
			return Destination{}, nil, nil, err
		}
	}
	return destination, pois, hotels, nil
}

// wantsSights: вопрос про достопримечательности («что посмотреть», «куда сходить»).
func wantsSights(message string) bool {
	return containsAny(strings.ToLower(message),
		"посмотреть", "достопримечат", "что интересного", "куда сходить", "куда пойти", "осмотреть", "sights")
}

// normalizeHotelName: нижний регистр, ё→е, без знаков препинания и союза and
// («Cedar & Sea» == «cedar and sea» == «cedar sea»).
func normalizeHotelName(value string) string {
	value = strings.ToLower(strings.ReplaceAll(value, "ё", "е"))
	value = strings.ReplaceAll(value, "&", " ")
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	kept := fields[:0]
	for _, field := range fields {
		if field != "and" {
			kept = append(kept, field)
		}
	}
	return strings.Join(kept, " ")
}

// FindHotelSlugByName ищет в сообщении название отеля из каталога (самое длинное совпадение).
func (s *Store) FindHotelSlugByName(ctx context.Context, message string) string {
	haystack := " " + normalizeHotelName(message) + " "
	rows, err := s.db.QueryContext(ctx, `SELECT slug, name FROM hotels`)
	if err != nil {
		return ""
	}
	defer rows.Close()
	best, bestLen := "", 0
	for rows.Next() {
		var slug, name string
		if rows.Scan(&slug, &name) != nil {
			continue
		}
		needle := normalizeHotelName(name)
		if needle == "" || len(needle) < bestLen {
			continue
		}
		if strings.Contains(haystack, " "+needle+" ") || strings.Contains(haystack, " "+needle) && len([]rune(needle)) >= 6 {
			best, bestLen = slug, len(needle)
		}
	}
	return best
}

func assistantHotelFromDetail(detail HotelDetail) AssistantHotel {
	tags := detail.Tags
	if len(tags) == 0 && len(detail.Meta.TagSlugs) > 0 {
		tags = TagsToLabels(detail.Meta.TagSlugs)
	}
	hotel := AssistantHotel{
		Slug:             detail.Slug,
		Name:             detail.Name,
		City:             detail.City,
		Image:            detail.Image,
		PriceFrom:        detail.PriceFrom,
		Rating:           detail.Rating,
		Reviews:          detail.Reviews,
		Tags:             tags,
		Address:          detail.Address,
		Amenities:        detail.Amenities,
		ShortDescription: detail.ShortDescription,
		Description:      detail.Description,
		Highlight:        detail.Meta.Highlight,
		Stars:            detail.Meta.Stars,
		PriceTotal:       detail.PriceBreakdown.Total,
	}
	for index, room := range detail.Rooms {
		if index >= 4 {
			break
		}
		hotel.Rooms = append(hotel.Rooms, AssistantRoom{
			Name:     room.Name,
			Price:    room.Price,
			Capacity: room.Capacity,
			Bed:      room.Bed,
			Size:     room.Size,
		})
	}
	for index, poi := range detail.Nearby {
		if index >= 6 {
			break
		}
		hotel.Nearby = append(hotel.Nearby, fmt.Sprintf("%s (%s, %s)", poi.Name, poi.Category, poi.Distance))
	}
	if detail.Meta.FoodMinutes > 0 {
		hotel.Distances = append(hotel.Distances, fmt.Sprintf("еда — %d мин пешком", detail.Meta.FoodMinutes))
	}
	if detail.Meta.ParkMinutes > 0 {
		hotel.Distances = append(hotel.Distances, fmt.Sprintf("парк — %d мин пешком", detail.Meta.ParkMinutes))
	}
	if detail.Meta.MetroMinutes > 0 {
		hotel.Distances = append(hotel.Distances, fmt.Sprintf("метро — %d мин пешком", detail.Meta.MetroMinutes))
	}
	if detail.Meta.BeachMinutes > 0 {
		hotel.Distances = append(hotel.Distances, fmt.Sprintf("пляж — %d мин пешком", detail.Meta.BeachMinutes))
	}
	if detail.Meta.ServicesMinutes > 0 {
		hotel.Distances = append(hotel.Distances, fmt.Sprintf("магазины и сервисы — %d мин пешком", detail.Meta.ServicesMinutes))
	}
	return hotel
}

func prioritizePOIs(pois []POI, category string) []POI {
	limit := 4
	if len(pois) <= limit {
		return pois
	}
	if category == "" {
		return pois[:limit]
	}
	result := make([]POI, 0, limit)
	for _, poi := range pois {
		if poi.Category == category && len(result) < limit {
			result = append(result, poi)
		}
	}
	for _, poi := range pois {
		if len(result) >= limit {
			break
		}
		duplicate := false
		for _, existing := range result {
			if existing.ID == poi.ID && existing.Name == poi.Name {
				duplicate = true
				break
			}
		}
		if !duplicate {
			result = append(result, poi)
		}
	}
	return result
}

func (s *Store) assistantHotels(ctx context.Context, destinationSlug, message string) ([]AssistantHotel, error) {
	where := "1 = 1"
	args := make([]any, 0)
	if destinationSlug != "" {
		where += " AND d.slug = ?"
		args = append(args, destinationSlug)
	}
	lower := strings.ToLower(message)
	tagRules := []struct {
		Stem string
		Slug string
	}{
		{Stem: "тих", Slug: "tishina"},
		{Stem: "мор", Slug: "more"},
		{Stem: "дизайн", Slug: "dizayn"},
		{Stem: "завтрак", Slug: "zavtraki"},
		{Stem: "гор", Slug: "gory"},
		{Stem: "спа", Slug: "spa"},
		{Stem: "семь", Slug: "semya"},
		{Stem: "зим", Slug: "zima"},
		{Stem: "сиян", Slug: "polyarnoe-siyanie"},
		{Stem: "закат", Slug: "zakaty"},
		{Stem: "бассейн", Slug: "bassein"},
		{Stem: "парков", Slug: "parkovka"},
		{Stem: "гид", Slug: "guides"},
		{Stem: "истори", Slug: "istoriya"},
		{Stem: "ферм", Slug: "ferma"},
		{Stem: "лес", Slug: "les"},
		{Stem: "озер", Slug: "ozera"},
		{Stem: "аквапарк", Slug: "akvapark"},
		{Stem: "животн", Slug: "s-zhivotnymi"},
		{Stem: "экскурс", Slug: "ekskursii"},
		{Stem: "кофе", Slug: "kofe"},
	}
	matchedTags := make([]string, 0)
	for _, rule := range tagRules {
		if !strings.Contains(lower, rule.Stem) {
			continue
		}
		alreadyMatched := false
		for _, tag := range matchedTags {
			if tag == rule.Slug {
				alreadyMatched = true
				break
			}
		}
		if !alreadyMatched {
			matchedTags = append(matchedTags, rule.Slug)
		}
	}
	if len(matchedTags) > 2 {
		matchedTags = matchedTags[:2]
	}
	for _, tag := range matchedTags {
		where += " AND h.meta_json LIKE ?"
		args = append(args, "%\""+tag+"\"%")
	}

	query := `
		SELECT COALESCE(MIN(r.price), h.price_per_night), h.slug, h.name, d.city, h.image, h.rating, h.meta_json
		FROM hotels h
		JOIN destinations d ON d.id = h.destination_id
		LEFT JOIN rooms r ON r.hotel_id = h.id
		WHERE ` + where + `
		GROUP BY h.id
		ORDER BY h.featured DESC, h.rating DESC
		LIMIT 3`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hotels := make([]AssistantHotel, 0)
	for rows.Next() {
		var hotel AssistantHotel
		var metaJSON string
		if err := rows.Scan(&hotel.PriceFrom, &hotel.Slug, &hotel.Name, &hotel.City, &hotel.Image, &hotel.Rating, &metaJSON); err != nil {
			return nil, err
		}
		hotel.Tags = TagsToLabels(decodeHotelMeta(metaJSON).TagSlugs)
		hotels = append(hotels, hotel)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(hotels) == 0 && len(matchedTags) > 0 {
		return s.assistantHotels(ctx, destinationSlug, "")
	}
	return hotels, nil
}

func categoryFromMessage(message string) string {
	lower := strings.ToLower(message)
	switch {
	case containsAny(lower, "кофе", "кофей", "латте", "капучино", "эспрессо"):
		return "coffee"
	case containsAny(lower, "завтрак", "ужин", "ресторан", "поесть", "обед", "еда", "матча"):
		return "food"
	case containsAny(lower, "прогул", "гулять", "парк", "набереж"):
		return "walk"
	case containsAny(lower, "музей", "культур", "эрмитаж", "кремль", "собор", "скайпарк"):
		return "culture"
	case containsAny(lower, "спа", "массаж"):
		return "spa"
	default:
		return ""
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func parseMinutes(value string) int {
	digits := strings.Builder{}
	for _, character := range value {
		if character >= '0' && character <= '9' {
			digits.WriteRune(character)
			continue
		}
		if digits.Len() > 0 {
			break
		}
	}
	if digits.Len() == 0 {
		return 0
	}
	minutes, _ := strconv.Atoi(digits.String())
	return minutes
}

func decodeStrings(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	var result []string
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return []string{}
	}
	return result
}

func bookingCode() (string, error) {
	random := make([]byte, 6)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(random)
	return "ODN-" + encoded[:8], nil
}

func max(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func parseID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}

// CapacityError - гостей больше вместимости номера (не «номер занят»).
type CapacityError struct{ Capacity int }

func (e *CapacityError) Error() string {
	return fmt.Sprintf("Номер вмещает до %d гостей", e.Capacity)
}
