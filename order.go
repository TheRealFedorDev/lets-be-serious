package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Единая оплата поездки: бронь номера + выбранные платные активности программы.
// Сервер не доверяет ценам клиента: пересобирает план по тому же запросу и берёт цены из него.

const maxOrderItems = 40

var ErrPlanChanged = errors.New("trip plan changed")

// OrderItem - оплаченная позиция программы.
type OrderItem struct {
	ItemID string `json:"itemId"`
	Date   string `json:"date"`
	Time   string `json:"time"`
	Title  string `json:"title"`
	Place  string `json:"place,omitempty"`
	Price  int    `json:"price"`
	Status string `json:"status"`
}

type CheckoutInput struct {
	Trip       TripRequest `json:"trip"`
	RoomID     int64       `json:"roomId"`
	ItemIDs    []string    `json:"itemIds"`
	GuestName  string      `json:"guestName"`
	GuestEmail string      `json:"guestEmail"`
	GuestPhone string      `json:"guestPhone"`
}

type CheckoutResult struct {
	Trip  Trip        `json:"trip"`
	Items []OrderItem `json:"items"`
	Total int         `json:"total"`
}

// ItemInputError - запрошенной позиции нет в плане или их слишком много (400).
type ItemInputError struct{ Message string }

func (e *ItemInputError) Error() string { return e.Message }

func (s *Store) ensureOrderTables() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			booking_id INTEGER NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
			status TEXT NOT NULL,
			total INTEGER NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_booking ON orders(booking_id)`,
		`CREATE TABLE IF NOT EXISTS order_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
			item_id TEXT NOT NULL,
			day_date TEXT NOT NULL,
			time TEXT NOT NULL,
			title TEXT NOT NULL,
			place TEXT NOT NULL DEFAULT '',
			price INTEGER NOT NULL,
			status TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_order_items_order ON order_items(order_id)`,
	}
	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("order migration: %w", err)
		}
	}
	return nil
}

// markBookableItems даёт платным активностям программы стабильный id для корзины.
func markBookableItems(days []TripDay) {
	for dayIndex := range days {
		for blockIndex := range days[dayIndex].Blocks {
			block := &days[dayIndex].Blocks[blockIndex]
			if block.Type == "activity" && block.Price > 0 && block.POIID > 0 {
				block.Bookable = true
				block.ItemID = fmt.Sprintf("d%d-%d-%s", dayIndex+1, block.POIID, strings.ReplaceAll(block.Time, ":", ""))
			}
		}
	}
}

// selectOrderItems находит в плане запрошенные позиции; неизвестная позиция - ошибка, а не пропуск.
func selectOrderItems(plan TripPlan, ids []string) ([]OrderItem, error) {
	if len(ids) > maxOrderItems {
		return nil, &ItemInputError{Message: fmt.Sprintf("Слишком много позиций (максимум %d)", maxOrderItems)}
	}
	byID := map[string]OrderItem{}
	for _, day := range plan.Days {
		for _, block := range day.Blocks {
			if block.Bookable {
				byID[block.ItemID] = OrderItem{ItemID: block.ItemID, Date: day.Date, Time: block.Time,
					Title: block.Title, Place: block.Place, Price: block.Price, Status: "confirmed"}
			}
		}
	}
	seen := map[string]bool{}
	items := make([]OrderItem, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if seen[id] {
			continue
		}
		item, ok := byID[id]
		if !ok {
			return nil, &ItemInputError{Message: "Позиции нет в программе — обновите маршрут"}
		}
		seen[id] = true
		items = append(items, item)
	}
	return items, nil
}

// Checkout бронирует номер и оплачивает выбранные позиции одним заказом.
func (s *Store) Checkout(ctx context.Context, userID int64, input CheckoutInput) (CheckoutResult, error) {
	plan, err := s.BuildTrip(ctx, userID, input.Trip)
	if err != nil {
		return CheckoutResult{}, err
	}
	// План собирается под запрошенный отель; если сервер подобрал другой - клиент видел не то.
	if slug := strings.TrimSpace(input.Trip.HotelSlug); slug == "" || plan.Hotel.Slug != slug {
		return CheckoutResult{}, ErrPlanChanged
	}
	items, err := selectOrderItems(plan, input.ItemIDs)
	if err != nil {
		return CheckoutResult{}, err
	}

	trip, err := s.CreateBooking(ctx, userID, BookingInput{
		HotelID: plan.Hotel.ID, RoomID: input.RoomID, CheckIn: plan.CheckIn, CheckOut: plan.CheckOut,
		Guests: plan.Guests, GuestName: input.GuestName, GuestEmail: input.GuestEmail, GuestPhone: input.GuestPhone,
	})
	if err != nil {
		return CheckoutResult{}, err
	}

	total := trip.Total
	for _, item := range items {
		total += item.Price
	}
	if err := s.saveOrder(ctx, userID, trip.ID, total, items); err != nil {
		// Заказ без позиций не оставляем: откатываем и бронь номера.
		_, _ = s.db.ExecContext(ctx, `UPDATE bookings SET status = 'cancelled' WHERE id = ?`, trip.ID)
		return CheckoutResult{}, err
	}
	trip.Items, trip.OrderTotal = items, total
	return CheckoutResult{Trip: trip, Items: items, Total: total}, nil
}

func (s *Store) saveOrder(ctx context.Context, userID, bookingID int64, total int, items []OrderItem) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := tx.ExecContext(ctx,
		`INSERT INTO orders (user_id, booking_id, status, total, created_at) VALUES (?, ?, 'confirmed', ?, ?)`,
		userID, bookingID, total, now)
	if err != nil {
		return err
	}
	orderID, err := result.LastInsertId()
	if err != nil {
		return err
	}
	for _, item := range items {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO order_items (order_id, item_id, day_date, time, title, place, price, status)
			 VALUES (?, ?, ?, ?, ?, ?, ?, 'confirmed')`,
			orderID, item.ItemID, item.Date, item.Time, item.Title, item.Place, item.Price); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// attachOrders дописывает к поездкам оплаченные позиции и общую сумму заказа.
func (s *Store) attachOrders(ctx context.Context, userID int64, trips []Trip) error {
	if len(trips) == 0 {
		return nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT o.booking_id, o.total, i.item_id, i.day_date, i.time, i.title, i.place, i.price, i.status
		 FROM orders o JOIN order_items i ON i.order_id = o.id
		 WHERE o.user_id = ?
		 ORDER BY i.day_date, i.time`, userID)
	if err != nil {
		return err
	}
	defer rows.Close()
	index := make(map[int64]int, len(trips))
	for i := range trips {
		index[trips[i].ID] = i
	}
	for rows.Next() {
		var bookingID int64
		var total int
		var item OrderItem
		if err := rows.Scan(&bookingID, &total, &item.ItemID, &item.Date, &item.Time, &item.Title, &item.Place, &item.Price, &item.Status); err != nil {
			return err
		}
		if i, ok := index[bookingID]; ok {
			trips[i].Items = append(trips[i].Items, item)
			trips[i].OrderTotal = total
		}
	}
	// заказ без платных позиций: итог = бронь
	for i := range trips {
		if trips[i].OrderTotal == 0 {
			trips[i].OrderTotal = trips[i].Total
		}
	}
	return rows.Err()
}

// cancelOrderFor отменяет позиции заказа вместе с бронью.
func (s *Store) cancelOrderFor(ctx context.Context, bookingID int64) {
	_, _ = s.db.ExecContext(ctx,
		`UPDATE order_items SET status = 'cancelled' WHERE order_id IN (SELECT id FROM orders WHERE booking_id = ?)`, bookingID)
	_, _ = s.db.ExecContext(ctx, `UPDATE orders SET status = 'cancelled' WHERE booking_id = ?`, bookingID)
}
