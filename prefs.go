package main

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Предпочтения поездки: то, что пользователь задаёт в мастере или ассистент
// извлекает из текста. Один формат для обоих входов.

// RoomPrefs - пожелания к номеру. Пустые поля означают «не важно».
type RoomPrefs struct {
	Bed     string `json:"bed,omitempty"`  // double | twin
	View    string `json:"view,omitempty"` // sea | city | mountain | nature
	MinSize int    `json:"minSize,omitempty"`
	Kitchen bool   `json:"kitchen,omitempty"`
	KidsCot bool   `json:"kidsCot,omitempty"`
}

const (
	maxTripInterests = 8
	maxWalkMinutes   = 60
)

// tripInterests - допустимые интересы программы и их подписи.
var tripInterests = map[string]string{
	"museums":  "Музеи и история",
	"nature":   "Природа",
	"gastro":   "Гастрономия",
	"night":    "Вечерняя жизнь",
	"kids":     "С детьми",
	"active":   "Активный отдых",
	"shopping": "Шопинг и сувениры",
	"views":    "Виды и смотровые",
}

var tripWeatherPrefs = map[string]string{
	"warm": "Тепло",
	"cool": "Прохладно, без жары",
	"dry":  "Без дождя",
	"snow": "Снег и зима",
}

// normalizePrefs приводит пользовательский ввод к допустимым значениям:
// неизвестное отбрасывается, а не превращается в ошибку.
func normalizePrefs(req *TripRequest) {
	req.Weather = strings.TrimSpace(strings.ToLower(req.Weather))
	if _, ok := tripWeatherPrefs[req.Weather]; !ok {
		req.Weather = ""
	}
	seen := map[string]bool{}
	interests := make([]string, 0, len(req.Interests))
	for _, raw := range req.Interests {
		key := strings.TrimSpace(strings.ToLower(raw))
		if _, ok := tripInterests[key]; ok && !seen[key] && len(interests) < maxTripInterests {
			seen[key] = true
			interests = append(interests, key)
		}
	}
	sort.Strings(interests)
	req.Interests = interests

	req.Room.Bed = strings.TrimSpace(strings.ToLower(req.Room.Bed))
	if req.Room.Bed != "double" && req.Room.Bed != "twin" {
		req.Room.Bed = ""
	}
	req.Room.View = strings.TrimSpace(strings.ToLower(req.Room.View))
	switch req.Room.View {
	case "sea", "city", "mountain", "nature":
	default:
		req.Room.View = ""
	}
	req.Room.MinSize = clampInt(req.Room.MinSize, 0, 200)
	req.MaxWalkMin = clampInt(req.MaxWalkMin, 0, maxWalkMinutes)
	if len(req.Needs) > 400 {
		req.Needs = req.Needs[:400]
	}
}

// hotelView - какой вид из окна типичен для отеля: по тегам и расстоянию до пляжа.
func hotelView(meta HotelMeta) string {
	switch {
	case containsString(meta.TagSlugs, "vid-na-more") || containsString(meta.TagSlugs, "more") ||
		(meta.BeachMinutes > 0 && meta.BeachMinutes <= 10):
		return "sea"
	case containsString(meta.TagSlugs, "gory"):
		return "mountain"
	case containsString(meta.TagSlugs, "les") || meta.Farm:
		return "nature"
	default:
		return "city"
	}
}

var viewLabels = map[string]string{"sea": "вид на море", "city": "вид на город", "mountain": "вид на горы", "nature": "вид на природу"}

// annotateRooms выводит атрибуты номера из его типа и профиля отеля.
// Номера в каталоге типовые (Studio / Deluxe / Suite), поэтому атрибуты не хранятся в БД.
func annotateRooms(rooms []Room, meta HotelMeta) {
	view := hotelView(meta)
	for index := range rooms {
		room := &rooms[index]
		name := strings.ToLower(room.Name)
		switch {
		case strings.Contains(name, "suite"):
			room.BedTypes = []string{"double", "twin"}
			room.View = view
			room.Kitchen = true
			room.KidsCot = true
		case strings.Contains(name, "deluxe"):
			room.BedTypes = []string{"double", "twin"}
			room.View = view
			room.KidsCot = true
			room.Kitchen = meta.Kitchen
			// в сиде у Deluxe везде «вид на город» - подписи приводим к фактическому виду
			if view != "city" {
				room.Description = strings.ReplaceAll(room.Description, "вид на город", viewLabels[view])
				for i, amenity := range room.Amenities {
					if amenity == "Вид на город" {
						label := viewLabels[view]
						room.Amenities[i] = strings.ToUpper(label[:2]) + label[2:]
					}
				}
			}
		default:
			room.BedTypes = []string{"double"}
			room.View = "city"
			if view == "nature" || view == "mountain" {
				room.View = view
			}
			room.Kitchen = meta.Kitchen && room.Size >= 30
		}
	}
}

// pickRoom выбирает номер под гостей и пожелания. Вместимость и наличие - жёсткие условия,
// остальное даёт очки. why - человеческое объяснение выбора, ok=false - подходящего номера нет.
func pickRoom(rooms []Room, guests int, prefs RoomPrefs) (Room, []string, bool) {
	best := -1
	bestScore := -1
	for index, room := range rooms {
		if room.Capacity < guests || room.Available <= 0 {
			continue
		}
		score := 0
		if prefs.Bed != "" && containsString(room.BedTypes, prefs.Bed) {
			score += 4
		}
		if prefs.View != "" && room.View == prefs.View {
			score += 4
		}
		if prefs.MinSize > 0 && room.Size >= prefs.MinSize {
			score += 3
		}
		if prefs.Kitchen && room.Kitchen {
			score += 3
		}
		if prefs.KidsCot && room.KidsCot {
			score += 3
		}
		// при равенстве остаётся более дешёвый: rooms отсортированы по цене
		if score > bestScore {
			best, bestScore = index, score
		}
	}
	if best < 0 {
		return Room{}, nil, false
	}
	room := rooms[best]
	return room, roomWhy(room, prefs), true
}

// tripRoom загружает номера отеля с наличием на даты маршрута и выбирает подходящий.
func (s *Store) tripRoom(ctx context.Context, hotel HotelCard, req TripRequest) (Room, []string, bool) {
	rooms, err := s.rooms(ctx, hotel.ID, req.Guests)
	if err != nil || len(rooms) == 0 {
		return Room{}, nil, false
	}
	booked, err := s.bookedRoomCounts(ctx, hotel.ID, req.From, req.To)
	if err != nil {
		booked = map[int64]int{}
	}
	for index := range rooms {
		rooms[index].Available = max(0, rooms[index].TotalUnits-booked[rooms[index].ID])
	}
	annotateRooms(rooms, hotel.Meta)
	return pickRoom(rooms, req.Guests, req.Room)
}

func roomWhy(room Room, prefs RoomPrefs) []string {
	why := []string{fmt.Sprintf("до %d гостей · %d м²", room.Capacity, room.Size)}
	mark := func(ok bool, yes, no string) {
		if ok {
			why = append(why, "✓ "+yes)
		} else {
			why = append(why, "✗ "+no)
		}
	}
	if prefs.Bed != "" {
		label := map[string]string{"double": "двуспальная кровать", "twin": "две отдельные кровати"}[prefs.Bed]
		mark(containsString(room.BedTypes, prefs.Bed), label, label+" — в этом отеле нет")
	}
	if prefs.View != "" {
		mark(room.View == prefs.View, viewLabels[prefs.View], viewLabels[prefs.View]+" — нет, "+viewLabels[room.View])
	}
	if prefs.MinSize > 0 {
		mark(room.Size >= prefs.MinSize, fmt.Sprintf("от %d м²", prefs.MinSize), fmt.Sprintf("меньше %d м²", prefs.MinSize))
	}
	if prefs.Kitchen {
		mark(room.Kitchen, "кухня", "кухни нет")
	}
	if prefs.KidsCot {
		mark(room.KidsCot, "детская кроватка", "детской кроватки нет")
	}
	if len(why) == 1 {
		why = append(why, "самый доступный номер, где всем хватит места")
	}
	return why
}

// AssistantTripPrefs - параметры поездки, понятые из свободного текста; мастер сразу собирает по ним вариант.
type AssistantTripPrefs struct {
	Destination string    `json:"destination,omitempty"`
	From        string    `json:"from,omitempty"`
	To          string    `json:"to,omitempty"`
	Guests      int       `json:"guests,omitempty"`
	Budget      int       `json:"budget,omitempty"`
	Weather     string    `json:"weather,omitempty"`
	Interests   []string  `json:"interests,omitempty"`
	Needs       string    `json:"needs,omitempty"`
	Room        RoomPrefs `json:"room"`
	MaxWalkMin  int       `json:"maxWalkMin,omitempty"`
	Pace        string    `json:"pace,omitempty"`
}

var (
	budgetPattern = regexp.MustCompile(`(\d[\d\s]{0,8})\s*(тыс|к(?:[^а-яё]|$)|k(?:[^a-z]|$)|000|₽|руб)`)
	walkPattern   = regexp.MustCompile(`(\d{1,2})\s*мин`)
	interestWords = map[string][]string{
		"museums":  {"музе", "истори", "архитектур", "экскурс"},
		"nature":   {"природ", "лес", "гор", "озер", "парк"},
		"gastro":   {"гастро", "поесть", "еда", "ресторан", "кухн", "вкусн"},
		"night":    {"вечерн", "ночн", "бар", "тусов"},
		"kids":     {"с детьми", "с ребён", "с ребен", "детск", "семь"},
		"active":   {"актив", "поход", "спорт", "велосип", "лыж", "сап"},
		"shopping": {"шопинг", "магазин", "сувенир", "рынок"},
		"views":    {"виды", "панорам", "закат", "смотров"}, // не «вид»: «вид на море» - про номер
	}
	nearbyNeedWords = map[string][]string{
		"beach": {"пляж", "у моря", "к морю", "до моря"},
		"metro": {"метро"},
		"food":  {"кафе рядом", "еда рядом", "поесть рядом"},
		"park":  {"парк рядом"},
	}
)

// extractTripPrefs разбирает фразу в параметры поездки. Город берётся из контекста ассистента.
// ok=false - во фразе нет ни одного параметра поездки, кнопку с prefill не показываем.
func extractTripPrefs(message string, destination Destination) (AssistantTripPrefs, bool) {
	lower := strings.ToLower(message)
	prefs := AssistantTripPrefs{Destination: destination.Slug}
	found := false

	switch {
	case containsAny(lower, "снег", "зим", "лыж", "сияни"):
		prefs.Weather = "snow"
	case containsAny(lower, "без дожд", "сухо", "не под дождём", "не под дождем"):
		prefs.Weather = "dry"
	case containsAny(lower, "без жары", "не жарко", "прохлад"):
		prefs.Weather = "cool"
	case containsAny(lower, "тепл", "солнц", "жар", "загор", "купат"):
		prefs.Weather = "warm"
	}
	for key, words := range interestWords {
		if containsAny(lower, words...) {
			prefs.Interests = append(prefs.Interests, key)
		}
	}
	sort.Strings(prefs.Interests)

	needs := make([]string, 0, 4)
	for key, words := range nearbyNeedWords {
		if containsAny(lower, words...) {
			needs = append(needs, key+"::1")
		}
	}
	if containsAny(lower, "бассейн") {
		needs = append(needs, "pool::2")
	}
	sort.Strings(needs)
	prefs.Needs = strings.Join(needs, ",")

	switch {
	case containsAny(lower, "две кровати", "2 кровати", "раздельн", "twin"):
		prefs.Room.Bed = "twin"
	case containsAny(lower, "двуспальн", "одна кровать", "большая кровать"):
		prefs.Room.Bed = "double"
	}
	switch {
	case containsAny(lower, "вид на море", "видом на море"):
		prefs.Room.View = "sea"
	case containsAny(lower, "вид на горы", "видом на горы"):
		prefs.Room.View = "mountain"
	case containsAny(lower, "вид на город", "видом на город"):
		prefs.Room.View = "city"
	}
	prefs.Room.Kitchen = containsAny(lower, "кухн") && !containsAny(lower, "местн", "кухня города", "национальн")
	prefs.Room.KidsCot = containsAny(lower, "кроватк", "малыш", "младен")
	if match := walkPattern.FindStringSubmatch(lower); match != nil && containsAny(lower, "пешком", "идти", "рядом", "не дальше") {
		if minutes, err := strconv.Atoi(match[1]); err == nil {
			prefs.MaxWalkMin = clampInt(minutes, 5, maxWalkMinutes)
		}
	}
	if containsAny(lower, "насыщен", "активн", "успеть всё", "успеть все") {
		prefs.Pace = "active"
	} else if containsAny(lower, "спокойн", "отдохнуть", "без спешки", "медленн") {
		prefs.Pace = "calm"
	}

	if guests := guestsFromMessage(message); guests > 0 {
		prefs.Guests = guests
	} else if containsAny(lower, "вдвоём", "вдвоем") {
		prefs.Guests = 2
	}
	if match := budgetPattern.FindStringSubmatch(lower); match != nil {
		digits := strings.ReplaceAll(match[1], " ", "")
		if value, err := strconv.Atoi(digits); err == nil && value > 0 {
			if suffix := strings.TrimSpace(match[2]); suffix != "₽" && suffix != "руб" {
				value *= 1000 // «60 тыс», «100к», «150 000» (последние три нуля ушли в суффикс)
			}
			if value >= 5000 && value <= 5000000 {
				prefs.Budget = value
			}
		}
	}
	if nights := nightsFromMessage(message); nights > 0 {
		prefs.From = time.Now().AddDate(0, 0, 14).Format("2006-01-02")
		prefs.To = addDays(prefs.From, min(nights, 14))
	}

	found = prefs.Weather != "" || len(prefs.Interests) > 0 || prefs.Needs != "" || prefs.Room != (RoomPrefs{}) ||
		prefs.MaxWalkMin > 0 || prefs.Budget > 0
	return prefs, found && destination.Slug != ""
}

// tripPrefsSummary - короткая подпись «как понял» для кнопки ассистента.
func tripPrefsSummary(p AssistantTripPrefs) string {
	parts := make([]string, 0, 6)
	if label, ok := tripWeatherPrefs[p.Weather]; ok {
		parts = append(parts, strings.ToLower(label))
	}
	for _, interest := range p.Interests {
		parts = append(parts, strings.ToLower(tripInterests[interest]))
	}
	if p.Room.View != "" {
		parts = append(parts, viewLabels[p.Room.View])
	}
	if p.Room.Bed == "twin" {
		parts = append(parts, "две кровати")
	}
	if p.Budget > 0 {
		parts = append(parts, "до "+money(p.Budget))
	}
	if len(parts) > 4 {
		parts = append(parts[:4], "…")
	}
	return strings.Join(parts, " · ")
}
