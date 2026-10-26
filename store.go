package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

var (
	ErrNotFound    = errors.New("not found")
	ErrUnavailable = errors.New("unavailable")
	ErrCantCancel  = errors.New("cannot cancel")
)

const (
	defaultAccent   = "#D45A3A"
	defaultMenuTint = "#FBF7EF"
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
			sort_order INTEGER NOT NULL DEFAULT 0
		)`,
	}

	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("migration: %w", err)
		}
	}
	return nil
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

	return HomePayload{Destinations: destinations, Featured: featured, Stats: stats}, nil
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
		        COALESCE(MIN(r.price), h.price_per_night), h.vibe_tags_json, h.featured
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

func (s *Store) Search(ctx context.Context, userID int64, params SearchParams) (SearchResult, error) {
	favoriteIDs, _ := s.favoriteSet(ctx, userID)
	where := []string{"1 = 1"}
	args := make([]any, 0)

	if strings.TrimSpace(params.Query) != "" {
		query := "%" + strings.TrimSpace(params.Query) + "%"
		where = append(where, `(h.name LIKE ? OR d.city LIKE ? OR h.short_description LIKE ? OR h.vibe_tags_json LIKE ?)`)
		args = append(args, query, query, query, query)
	}
	if strings.TrimSpace(params.Destination) != "" {
		where = append(where, `d.slug = ?`)
		args = append(args, strings.TrimSpace(params.Destination))
	}
	if params.Guests > 0 {
		where = append(where, `EXISTS (SELECT 1 FROM rooms rr WHERE rr.hotel_id = h.id AND rr.capacity >= ?)`)
		args = append(args, params.Guests)
	}
	if params.MaxPrice > 0 {
		where = append(where, `h.price_per_night <= ?`)
		args = append(args, params.MaxPrice)
	}
	if strings.TrimSpace(params.Tag) != "" {
		where = append(where, `(h.vibe_tags_json LIKE ? OR h.amenities_json LIKE ?)`)
		tag := "%" + strings.TrimSpace(params.Tag) + "%"
		args = append(args, tag, tag)
	}

	orderBy := "h.featured DESC, h.rating DESC"
	switch params.Sort {
	case "price_asc":
		orderBy = "COALESCE(MIN(r.price), h.price_per_night) ASC"
	case "price_desc":
		orderBy = "COALESCE(MIN(r.price), h.price_per_night) DESC"
	case "rating":
		orderBy = "h.rating DESC"
	}

	query := `
		SELECT h.id, h.slug, h.name, d.city, d.country, d.slug, h.image, h.address,
		       h.short_description, h.rating, h.reviews_count,
		       COALESCE(MIN(r.price), h.price_per_night), h.vibe_tags_json, h.featured
		FROM hotels h
		JOIN destinations d ON d.id = h.destination_id
		LEFT JOIN rooms r ON r.hotel_id = h.id
		WHERE ` + strings.Join(where, " AND ") + `
		GROUP BY h.id
		ORDER BY ` + orderBy + `
		LIMIT 60`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return SearchResult{}, err
	}
	defer rows.Close()

	hotels, err := scanHotelCards(rows, favoriteIDs)
	if err != nil {
		return SearchResult{}, err
	}
	return SearchResult{Hotels: hotels, Total: len(hotels), Query: params.Query}, nil
}

func scanHotelCards(rows *sql.Rows, favoriteIDs map[int64]bool) ([]HotelCard, error) {
	hotels := make([]HotelCard, 0)
	for rows.Next() {
		var hotel HotelCard
		var tagsJSON string
		if err := rows.Scan(
			&hotel.ID, &hotel.Slug, &hotel.Name, &hotel.City, &hotel.Country,
			&hotel.DestinationSlug, &hotel.Image, &hotel.Address, &hotel.ShortDescription,
			&hotel.Rating, &hotel.Reviews, &hotel.PriceFrom, &tagsJSON, &hotel.Featured,
		); err != nil {
			return nil, err
		}
		hotel.Tags = decodeStrings(tagsJSON)
		hotel.Favorited = favoriteIDs != nil && favoriteIDs[hotel.ID]
		hotels = append(hotels, hotel)
	}
	return hotels, rows.Err()
}

func (s *Store) HotelBySlug(ctx context.Context, userID int64, slug, checkIn, checkOut string, guests int) (HotelDetail, error) {
	var detail HotelDetail
	var tagsJSON, destinationTagsJSON, aliasesJSON, galleryJSON, amenitiesJSON string
	var destinationID int64

	err := s.db.QueryRowContext(ctx,
		`SELECT h.id, h.slug, h.name, d.city, d.country, d.slug, h.image, h.address,
		        h.short_description, h.description, h.rating, h.reviews_count, h.price_per_night,
		        h.vibe_tags_json, h.amenities_json, h.gallery_json, h.featured,
		        d.id, d.tagline, d.description, d.image, d.price_from, d.tags_json, d.aliases_json
		 FROM hotels h JOIN destinations d ON d.id = h.destination_id
		 WHERE h.slug = ?`,
		slug,
	).Scan(
		&detail.ID, &detail.Slug, &detail.Name, &detail.City, &detail.Country, &detail.DestinationSlug,
		&detail.Image, &detail.Address, &detail.ShortDescription, &detail.Description,
		&detail.Rating, &detail.Reviews, &detail.PriceFrom, &tagsJSON, &amenitiesJSON, &galleryJSON,
		&detail.Featured, &destinationID, &detail.Destination.Tagline, &detail.Destination.Description,
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
	detail.Tags = decodeStrings(tagsJSON)
	detail.Amenities = decodeStrings(amenitiesJSON)
	detail.Gallery = decodeStrings(galleryJSON)
	detail.CheckIn = checkIn
	detail.CheckOut = checkOut
	detail.Guests = guests

	rooms, err := s.rooms(ctx, detail.ID)
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
	detail.Rooms = rooms

	detail.Nearby, err = s.poisForDestination(ctx, detail.DestinationSlug, "", 4)
	if err != nil {
		return HotelDetail{}, err
	}

	favoriteIDs, _ := s.favoriteSet(ctx, userID)
	detail.Favorited = favoriteIDs[detail.ID]
	return detail, nil
}

func (s *Store) rooms(ctx context.Context, hotelID int64) ([]Room, error) {
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
	query := `SELECT id, name, category, description, address, distance, tags_json, price_level
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
		if err := rows.Scan(
			&poi.ID, &poi.Name, &poi.Category, &poi.Description,
			&poi.Address, &poi.Distance, &tagsJSON, &poi.PriceLevel,
		); err != nil {
			return nil, err
		}
		poi.Tags = decodeStrings(tagsJSON)
		pois = append(pois, poi)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return []POI{}, nil
	}
	return pois, rows.Err()
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
	return trips, rows.Err()
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
		return Trip{}, ErrUnavailable
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

	total := price * nights
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
		        COALESCE(MIN(r.price), h.price_per_night), h.vibe_tags_json, h.featured
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

func (s *Store) AssistantContext(ctx context.Context, destinationSlug, message string) (Destination, []POI, []AssistantHotel, error) {
	var destination Destination
	var err error
	detected, detectErr := s.DetectDestination(ctx, message)
	if detectErr == nil {
		destination = detected
	} else if strings.TrimSpace(destinationSlug) != "" {
		destination, _ = s.DestinationBySlug(ctx, destinationSlug)
	}

	category := categoryFromMessage(message)
	pois := []POI{}
	if destination.Slug != "" {
		pois, err = s.poisForDestination(ctx, destination.Slug, category, 3)
		if err != nil {
			return Destination{}, nil, nil, err
		}
		if len(pois) == 0 {
			pois, _ = s.poisForDestination(ctx, destination.Slug, "", 3)
		}
	}

	hotels, err := s.assistantHotels(ctx, destination.Slug, message)
	if err != nil {
		return Destination{}, nil, nil, err
	}
	return destination, pois, hotels, nil
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
		Tag  string
	}{
		{Stem: "тих", Tag: "тишина"},
		{Stem: "мор", Tag: "море"},
		{Stem: "дизайн", Tag: "дизайн"},
		{Stem: "завтрак", Tag: "завтраки"},
		{Stem: "гор", Tag: "горы"},
		{Stem: "спа", Tag: "спа"},
		{Stem: "семь", Tag: "семья"},
		{Stem: "центр", Tag: "центр"},
	}
	matchedTags := make([]string, 0)
	for _, rule := range tagRules {
		if !strings.Contains(lower, rule.Stem) {
			continue
		}
		alreadyMatched := false
		for _, tag := range matchedTags {
			if tag == rule.Tag {
				alreadyMatched = true
				break
			}
		}
		if !alreadyMatched {
			matchedTags = append(matchedTags, rule.Tag)
		}
	}
	for _, tag := range matchedTags {
		where += " AND h.vibe_tags_json LIKE ?"
		args = append(args, "%"+tag+"%")
	}

	query := `
		SELECT COALESCE(MIN(r.price), h.price_per_night), h.slug, h.name, d.city, h.image, h.rating, h.vibe_tags_json
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
		var tagsJSON string
		if err := rows.Scan(&hotel.PriceFrom, &hotel.Slug, &hotel.Name, &hotel.City, &hotel.Image, &hotel.Rating, &tagsJSON); err != nil {
			return nil, err
		}
		hotel.Tags = decodeStrings(tagsJSON)
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
