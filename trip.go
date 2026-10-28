package main

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"time"
)

// AI-режиссёр поездки: один запрос — готовый маршрут по дням,
// с отелем, бюджетом и погодой. Работает детерминированно и без LLM,
// чтобы демо не зависело от модели и сети.

type TripRequest struct {
	Destination string   `json:"destination"`
	HotelSlug   string   `json:"hotelSlug"`
	From        string   `json:"from"`
	To          string   `json:"to"`
	Guests      int      `json:"guests"`
	Budget      int      `json:"budget"`
	Mood        string   `json:"mood"`
	Tags        []string `json:"tags"`
	Pace        string   `json:"pace"`
}

type TripBlock struct {
	Time    string `json:"time"`
	Type    string `json:"type"`
	Title   string `json:"title"`
	Place   string `json:"place,omitempty"`
	Note    string `json:"note,omitempty"`
	Price   int    `json:"price"`
	Minutes int    `json:"minutes,omitempty"`
	POIID   int64  `json:"poiId,omitempty"`
}

type TripDay struct {
	Date    string        `json:"date"`
	Title   string        `json:"title"`
	Weather WeatherInfo   `json:"weather"`
	Blocks  []TripBlock   `json:"blocks"`
	Total   int           `json:"total"`
	Walk    *WalkingRoute `json:"walk,omitempty"`
}

type TripBudget struct {
	Total     int            `json:"total"`
	PerDay    int            `json:"perDay"`
	Split     map[string]int `json:"split"`
	Fits      bool           `json:"fits"`
	Delta     int            `json:"delta"`
	PerPerson int            `json:"perPerson"`
}

type TripMatch struct {
	Score   int    `json:"score"`
	Verdict string `json:"verdict"`
	Reason  string `json:"reason"`
}

type TripPlan struct {
	Destination string     `json:"destination"`
	City        string     `json:"city"`
	Image       string     `json:"image"`
	CheckIn     string     `json:"checkIn"`
	CheckOut    string     `json:"checkOut"`
	Nights      int        `json:"nights"`
	Guests      int        `json:"guests"`
	Pace        string     `json:"pace"`
	Hotel       HotelCard  `json:"hotel"`
	Match       TripMatch  `json:"match"`
	Days        []TripDay  `json:"days"`
	Budget      TripBudget `json:"budget"`
	Tips        []string   `json:"tips"`
	Cached      bool       `json:"cached"`
}

type tripCacheEntry struct {
	plan    TripPlan
	expires time.Time
}

var (
	tripCacheMu sync.Mutex
	tripCache   = map[string]tripCacheEntry{}
)

func tripCacheKey(req TripRequest) string {
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(strings.Join([]string{
		req.Destination, req.HotelSlug, req.From, req.To,
		fmt.Sprint(req.Guests), fmt.Sprint(req.Budget), req.Mood, req.Pace,
		strings.Join(req.Tags, ","),
	}, "|")))
	return fmt.Sprint(hasher.Sum64())
}

func (s *Store) BuildTrip(ctx context.Context, userID int64, req TripRequest) (TripPlan, error) {
	req.Destination = strings.TrimSpace(req.Destination)
	req.From = strings.TrimSpace(req.From)
	req.To = strings.TrimSpace(req.To)
	if req.Guests <= 0 {
		req.Guests = 2
	}
	if req.Pace != "active" {
		req.Pace = "calm"
	}
	if req.From == "" {
		req.From = time.Now().AddDate(0, 0, 14).Format("2006-01-02")
	}
	if req.To == "" || req.To <= req.From {
		req.To = addDays(req.From, 3)
	}
	nights := nightsBetween(req.From, req.To)
	if nights < 1 {
		nights = 1
	}
	if nights > 14 {
		nights = 14
		req.To = addDays(req.From, nights)
	}

	destination, err := s.resolveTripDestination(ctx, req.Destination)
	if err != nil {
		return TripPlan{}, err
	}

	perNightBudget := 0
	if req.Budget > 0 {
		perNightBudget = int(float64(req.Budget) / float64(nights) * 0.55)
	}

	searchParams := SearchParams{
		Destination: destination.Slug,
		Guests:      req.Guests,
		Mood:        req.Mood,
		Tags:        req.Tags,
		CheckIn:     req.From,
		CheckOut:    req.To,
		MaxPrice:    perNightBudget,
	}
	result, err := s.Search(ctx, userID, searchParams)
	if err != nil {
		return TripPlan{}, err
	}
	if len(result.Hotels) == 0 && perNightBudget > 0 {
		searchParams.MaxPrice = 0
		result, err = s.Search(ctx, userID, searchParams)
		if err != nil {
			return TripPlan{}, err
		}
	}
	if len(result.Hotels) == 0 {
		return TripPlan{}, ErrNotFound
	}

	hotel := result.Hotels[0]
	if slug := strings.TrimSpace(req.HotelSlug); slug != "" {
		for _, candidate := range result.Hotels {
			if candidate.Slug == slug {
				hotel = candidate
				break
			}
		}
	}

	pois, err := s.poisForDestination(ctx, destination.Slug, "", 32)
	if err != nil {
		return TripPlan{}, err
	}

	score := hotel.Match
	if lr := logisticRecommendation(hotel, searchParams, strings.TrimSpace(req.Mood), req.Tags); lr > score {
		score = lr
	}
	if score <= 0 {
		score = int(hotel.Rating * 10)
	}

	plan := TripPlan{
		Destination: destination.Slug,
		City:        destination.City,
		Image:       destination.Image,
		CheckIn:     req.From,
		CheckOut:    req.To,
		Nights:      nights,
		Guests:      req.Guests,
		Pace:        req.Pace,
		Hotel:       hotel,
		Match: TripMatch{
			Score:   clampInt(score, 1, 99),
			Verdict: tripVerdict(score),
			Reason:  tripReason(hotel, req, destination),
		},
		Tips: make([]string, 0, 4),
	}

	plan.Days = buildTripDays(destination, hotel, pois, req, nights, s)
	plan.Budget = buildTripBudget(hotel, req, nights, plan.Days)
	plan.Tips = buildTripTips(destination, hotel, req, plan, pois)
	return plan, nil
}

func (s *Store) resolveTripDestination(ctx context.Context, raw string) (Destination, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Destination{}, ErrNotFound
	}
	if destination, err := s.DestinationBySlug(ctx, raw); err == nil {
		return destination, nil
	}
	if destination, err := s.DetectDestination(ctx, raw); err == nil {
		return destination, nil
	}
	return Destination{}, ErrNotFound
}

func buildTripDays(destination Destination, hotel HotelCard, pois []POI, req TripRequest, nights int, s *Store) []TripDay {
	days := make([]TripDay, 0, nights+1)
	weather := make([]WeatherInfo, nights+1)
	for index := 0; index <= nights; index++ {
		weather[index] = s.Weather(destination.City, addDays(req.From, index))
	}

	sightseeing := make([]POI, 0, len(pois))
	food := make([]POI, 0, len(pois))
	indoor := make([]POI, 0, len(pois))
	for _, poi := range pois {
		switch poi.Category {
		case "food", "coffee":
			food = append(food, poi)
		case "culture", "spa":
			indoor = append(indoor, poi)
		default:
			sightseeing = append(sightseeing, poi)
		}
	}
	pick := func(list []POI, index int) (POI, bool) {
		if len(list) == 0 {
			return POI{}, false
		}
		return list[index%len(list)], true
	}

	for index := 0; index <= nights; index++ {
		date := addDays(req.From, index)
		info := weather[index]
		day := TripDay{Date: date, Weather: info, Blocks: make([]TripBlock, 0, 5)}
		rainy := info.Condition == "rain"
		departure := index == nights

		if index == 0 {
			day.Title = "Приезд и первый вечер"
			day.Blocks = append(day.Blocks,
				TripBlock{Time: "14:00", Type: "transfer", Title: "Заселение", Place: hotel.Name, Note: "Оставьте вещи и выдохните", Price: 0, Minutes: 20},
				TripBlock{Time: "16:00", Type: "walk", Title: "Первая прогулка рядом", Place: hotel.Name, Price: 0, Minutes: 60},
				TripBlock{Time: "19:30", Type: "food", Title: "Ужин без спешки", Price: 1800 * req.Guests},
			)
			if poi, ok := pick(food, 0); ok {
				day.Blocks[2].Place = poi.Name
				day.Blocks[2].POIID = poi.ID
				day.Blocks[2].Minutes = poi.Minutes
			}
		} else if departure {
			day.Title = "Завтрак и выезд"
			day.Blocks = append(day.Blocks,
				TripBlock{Time: "09:00", Type: "food", Title: "Завтрак", Price: 700 * req.Guests},
				TripBlock{Time: "11:00", Type: "walk", Title: "Последняя прогулка", Price: 0, Minutes: 50},
				TripBlock{Time: "12:00", Type: "transfer", Title: "Выезд из отеля", Place: hotel.Name, Price: 0, Minutes: 20},
			)
		} else {
			day.Title = fmt.Sprintf("День %d · %s", index+1, tripDayTheme(index, req.Pace, rainy))
			main := sightseeing
			if rainy {
				main = indoor
			}
			day.Blocks = append(day.Blocks,
				TripBlock{Time: "09:30", Type: "food", Title: "Завтрак и кофе", Price: 650 * req.Guests},
			)
			if poi, ok := pick(food, index); ok {
				day.Blocks[0].Place = poi.Name
				day.Blocks[0].POIID = poi.ID
			}
			if poi, ok := pick(main, index*2); ok {
				day.Blocks = append(day.Blocks, TripBlock{
					Time: "11:00", Type: "activity", Title: tripPOITitle(poi),
					Place: poi.Name, Note: poi.Description, Price: poiPrice(poi) * req.Guests, Minutes: poi.Minutes, POIID: poi.ID,
				})
			}
			day.Blocks = append(day.Blocks,
				TripBlock{Time: "13:30", Type: "food", Title: "Обед", Price: 1200 * req.Guests},
			)
			if poi, ok := pick(food, index+1); ok {
				day.Blocks[1].Place = poi.Name
				day.Blocks[1].POIID = poi.ID
			}
			if req.Pace == "active" {
				if poi, ok := pick(sightseeing, index*2+1); ok {
					day.Blocks = append(day.Blocks, TripBlock{
						Time: "16:00", Type: "activity", Title: "Ещё одна точка",
						Place: poi.Name, Price: poiPrice(poi) * req.Guests, Minutes: poi.Minutes, POIID: poi.ID,
					})
				}
			} else {
				day.Blocks = append(day.Blocks, TripBlock{
					Time: "16:30", Type: "rest", Title: "Тихий час", Note: "Кофе, книга или просто ничего", Price: 450 * req.Guests,
				})
			}
			day.Blocks = append(day.Blocks, TripBlock{
				Time: "19:30", Type: "food", Title: "Ужин", Place: tripDinnerPlace(food, index), Price: 1900 * req.Guests,
			})
		}

		day.Walk = planWalkingRoute(hotel, pois, req.Pace, rainy, index)
		for _, block := range day.Blocks {
			day.Total += block.Price
		}
		days = append(days, day)
	}
	return days
}

func tripDayTheme(index int, pace string, rainy bool) string {
	if rainy {
		return "под крышей"
	}
	themes := []string{"город пешком", "вода и набережная", "музеи и кофе", "парки и панорамы", "локальная еда", "медленное утро"}
	if pace == "active" {
		themes = []string{"максимум города", "дальние точки", "спорт и воздух", "всё пешком", "новые районы"}
	}
	return themes[index%len(themes)]
}

func tripPOITitle(poi POI) string {
	switch poi.Category {
	case "culture":
		return "Культурная точка"
	case "nature":
		return "Природа рядом"
	case "spa":
		return "Восстановление"
	case "walk":
		return "Прогулка"
	default:
		return "Место рядом"
	}
}

func tripDinnerPlace(food []POI, index int) string {
	if len(food) == 0 {
		return ""
	}
	return food[(index+2)%len(food)].Name
}

func poiPrice(poi POI) int {
	switch poi.PriceLevel {
	case 0:
		return 0
	case 1:
		return 500
	case 2:
		return 1200
	case 3:
		return 2400
	default:
		return 900
	}
}

func buildTripBudget(hotel HotelCard, req TripRequest, nights int, days []TripDay) TripBudget {
	stayPerNight := hotel.PriceBreakdown.Total
	if stayPerNight <= 0 {
		stayPerNight = hotel.PriceFrom
	}
	stay := stayPerNight * nights

	food, activities := 0, 0
	for _, day := range days {
		for _, block := range day.Blocks {
			switch block.Type {
			case "food":
				food += block.Price
			case "activity", "rest":
				activities += block.Price
			}
		}
	}
	transport := 700 * nights * req.Guests
	total := stay + food + activities + transport
	perDay := 0
	if len(days) > 0 {
		perDay = total / len(days)
	}
	budget := TripBudget{
		Total:     total,
		PerDay:    perDay,
		PerPerson: total / max(1, req.Guests),
		Split: map[string]int{
			"stay":       stay,
			"food":       food,
			"activities": activities,
			"transport":  transport,
		},
	}
	if req.Budget > 0 {
		budget.Delta = total - req.Budget
		budget.Fits = budget.Delta <= 0
	} else {
		budget.Fits = true
	}
	return budget
}

func buildTripTips(destination Destination, hotel HotelCard, req TripRequest, plan TripPlan, pois []POI) []string {
	tips := make([]string, 0, 4)
	rainyDay := ""
	for _, day := range plan.Days {
		if day.Weather.Condition == "rain" {
			rainyDay = day.Date
			break
		}
	}
	if rainyDay != "" {
		tips = append(tips, fmt.Sprintf("На %s обещают дождь — уличные точки заменены на музеи и SPA.", shortDate(rainyDay)))
	}
	if !containsString(hotel.Meta.TagSlugs, "zavtraki") {
		tips = append(tips, "Завтрак не включён: заложили кафе рядом — это честнее, чем доплата на месте.")
	}
	if hotel.Meta.BeachMinutes > 0 {
		tips = append(tips, fmt.Sprintf("До пляжа %d мин пешком — можно ходить в тапочках.", hotel.Meta.BeachMinutes))
	}
	if req.Budget > 0 && plan.Budget.Delta > 0 {
		tips = append(tips, fmt.Sprintf("Бюджет превышен на %s. Самый быстрый способ — отель на категорию ниже или убрать одно платное SPA.", money(plan.Budget.Delta)))
	}
	if len(tips) == 0 {
		tips = append(tips, "План собран без спешки: два блока в день плюс свободное время.")
	}
	return tips
}

func tripVerdict(score int) string {
	switch {
	case score >= 88:
		return "Идеальное совпадение"
	case score >= 75:
		return "Очень хорошо подходит"
	case score >= 60:
		return "Хороший вариант"
	default:
		return "Компромисс, но стоит посмотреть"
	}
}

func tripReason(hotel HotelCard, req TripRequest, destination Destination) string {
	parts := []string{fmt.Sprintf("%s в %s", hotel.Name, destination.City)}
	if hotel.Meta.BeachMinutes > 0 {
		parts = append(parts, fmt.Sprintf("до моря %d мин", hotel.Meta.BeachMinutes))
	}
	if hotel.Meta.MetroMinutes > 0 {
		parts = append(parts, fmt.Sprintf("до метро %d мин", hotel.Meta.MetroMinutes))
	}
	if hotel.Meta.FoodMinutes > 0 {
		parts = append(parts, fmt.Sprintf("еда в %d мин", hotel.Meta.FoodMinutes))
	}
	if req.Budget > 0 {
		parts = append(parts, fmt.Sprintf("под бюджет %s", money(req.Budget)))
	}
	return strings.Join(parts, ", ") + "."
}

func tripFromCache(key string) (TripPlan, bool) {
	tripCacheMu.Lock()
	defer tripCacheMu.Unlock()
	entry, ok := tripCache[key]
	if !ok {
		return TripPlan{}, false
	}
	if time.Now().After(entry.expires) {
		delete(tripCache, key)
		return TripPlan{}, false
	}
	return entry.plan, true
}

func tripToCache(key string, plan TripPlan) {
	tripCacheMu.Lock()
	defer tripCacheMu.Unlock()
	if len(tripCache) > 64 {
		for cacheKey, entry := range tripCache {
			if time.Now().After(entry.expires) {
				delete(tripCache, cacheKey)
			}
		}
	}
	tripCache[key] = tripCacheEntry{plan: plan, expires: time.Now().Add(30 * time.Minute)}
}

func addDays(iso string, days int) string {
	parsed, err := time.Parse("2006-01-02", iso)
	if err != nil {
		parsed = time.Now()
	}
	return parsed.AddDate(0, 0, days).Format("2006-01-02")
}

func nightsBetween(from, to string) int {
	start, err1 := time.Parse("2006-01-02", from)
	end, err2 := time.Parse("2006-01-02", to)
	if err1 != nil || err2 != nil {
		return 0
	}
	return int(end.Sub(start).Hours() / 24)
}

func shortDate(iso string) string {
	parsed, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return iso
	}
	months := []string{"янв", "фев", "мар", "апр", "мая", "июн", "июл", "авг", "сен", "окт", "ноя", "дек"}
	return fmt.Sprintf("%d %s", parsed.Day(), months[int(parsed.Month())-1])
}

func money(value int) string {
	negative := value < 0
	if negative {
		value = -value
	}
	digits := fmt.Sprint(value)
	for index := len(digits) - 3; index > 0; index -= 3 {
		digits = digits[:index] + " " + digits[index:]
	}
	if negative {
		return "−" + digits + " ₽"
	}
	return digits + " ₽"
}
