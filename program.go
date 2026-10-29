package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Программа поездки под предпочтения: интересы, «что близко» и погода.
// Без предпочтений порядок точек не меняется, поэтому старые маршруты и демо остаются прежними.

// interestRules - как интерес узнаётся в точке: по категории (сильнее) и по словам в названии/описании/тегах.
var interestRules = map[string]struct {
	categories []string
	words      []string
}{
	"museums":  {[]string{"culture", "excursion"}, []string{"музей", "истори", "собор", "кремл", "галере", "усадьб", "храм"}},
	"nature":   {[]string{"nature", "park"}, []string{"парк", "озер", "гор", "лес", "рек", "тропа", "сад"}},
	"gastro":   {[]string{"food", "coffee"}, []string{"кухн", "гастро", "рынок", "ресторан", "кофе", "дегустац"}},
	"night":    {[]string{"walk"}, []string{"вечер", "ночн", "набережн", "бар", "закат", "огни"}},
	"kids":     {[]string{"park"}, []string{"дет", "зоопарк", "аквапарк", "океанариум", "семь", "аттракцион"}},
	"active":   {[]string{"nature", "excursion"}, []string{"тропа", "поход", "актив", "катер", "сап", "велос", "лыж", "пробежк"}},
	"shopping": {nil, []string{"рынок", "сувенир", "торгов", "пассаж", "лавк"}},
	"views":    {nil, []string{"вид", "смотров", "панорам", "закат", "башн", "колесо", "вершин"}},
}

const walkMetersPerMinute = walkSpeedKmh * 1000 / 60

// poiInterestScore - насколько точка совпадает с интересами: категория +2, каждое слово +1 (не больше 2).
func poiInterestScore(poi POI, interests []string) int {
	if len(interests) == 0 {
		return 0
	}
	text := strings.ToLower(poi.Name + " " + poi.Description + " " + strings.Join(poi.Tags, " "))
	score := 0
	for _, interest := range interests {
		rule, ok := interestRules[interest]
		if !ok {
			continue
		}
		if containsString(rule.categories, poi.Category) {
			score += 2
		}
		hits := 0
		for _, word := range rule.words {
			if strings.Contains(text, word) {
				hits++
			}
		}
		score += min(hits, 2)
	}
	return score
}

// poiWalkMinutes - минуты пешком от отеля по улицам; -1, если координат нет.
func poiWalkMinutes(hotel HotelCard, poi POI) int {
	if (hotel.Meta.Lat == 0 && hotel.Meta.Lng == 0) || (poi.Lat == 0 && poi.Lng == 0) {
		return -1
	}
	meters := haversineMeters(hotel.Meta.Lat, hotel.Meta.Lng, poi.Lat, poi.Lng) * walkDetourFactor
	return int(math.Ceil(meters / walkMetersPerMinute))
}

// poiDistanceLabel - «12 мин пешком» или «20 мин на такси» (если пешком дольше получаса).
func poiDistanceLabel(hotel HotelCard, poi POI) string {
	walk := poiWalkMinutes(hotel, poi)
	switch {
	case walk < 0:
		return ""
	case walk <= 30:
		return fmt.Sprintf("%d мин пешком", max(walk, 1))
	default:
		km := float64(walk) * walkMetersPerMinute / 1000
		return fmt.Sprintf("%d мин на такси", int(math.Round(km/25*60))+5)
	}
}

// rankPOIs переставляет точки: сначала совпадающие с интересами, потом ближе к отелю.
// Точки дальше maxWalk минут (если задан) уходят в конец, но не выбрасываются.
func rankPOIs(list []POI, hotel HotelCard, req TripRequest) []POI {
	if len(req.Interests) == 0 && req.MaxWalkMin == 0 {
		return list
	}
	type ranked struct {
		poi   POI
		score int
		walk  int
	}
	items := make([]ranked, len(list))
	for index, poi := range list {
		walk := poiWalkMinutes(hotel, poi)
		score := poiInterestScore(poi, req.Interests) * 10
		if req.MaxWalkMin > 0 && walk > req.MaxWalkMin {
			score -= 100
		}
		items[index] = ranked{poi: poi, score: score, walk: walk}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].score != items[j].score {
			return items[i].score > items[j].score
		}
		return items[i].walk >= 0 && (items[j].walk < 0 || items[i].walk < items[j].walk)
	})
	out := make([]POI, len(items))
	for index, item := range items {
		out[index] = item.poi
	}
	return out
}

// weatherFits - подходит ли погода дня под пожелание. Пустое пожелание подходит всегда.
func weatherFits(w WeatherInfo, pref string) bool {
	wet := w.Condition == "rain" || w.Condition == "snow"
	switch pref {
	case "warm":
		return w.TempC >= 18 && !wet
	case "cool":
		return w.TempC <= 24 && w.Condition != "heat"
	case "dry":
		return !wet
	case "snow":
		return w.Condition == "snow" || w.TempC <= -2
	default:
		return true
	}
}

// indoorDay - перевести основную программу дня под крышу: дождь или погода против пожелания
// (для любителей снега холод - не повод уходить в помещения).
func indoorDay(w WeatherInfo, pref string) bool {
	if w.Condition == "rain" {
		return true
	}
	return pref != "" && pref != "snow" && !weatherFits(w, pref)
}

// DateHint - подсказка сдвинуть даты ради погоды.
type DateHint struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Fit   int    `json:"fit"`
	Total int    `json:"total"`
	Text  string `json:"text"`
}

// bestDates ищет сдвиг до ±7 дней той же длины, где больше дней с подходящей погодой.
// Подсказка нужна, только если в выбранных датах не подходит больше половины дней.
func (s *Store) bestDates(city, from string, nights int, pref string) *DateHint {
	if pref == "" {
		return nil
	}
	countFit := func(start string) int {
		fit := 0
		for index := 0; index <= nights; index++ {
			if weatherFits(s.Weather(city, addDays(start, index)), pref) {
				fit++
			}
		}
		return fit
	}
	total := nights + 1
	current := countFit(from)
	if current*2 >= total {
		return nil
	}
	today := time.Now().Format("2006-01-02")
	bestShift, bestFit := 0, current
	for _, shift := range []int{1, -1, 2, -2, 3, -3, 4, -4, 5, -5, 6, -6, 7, -7} {
		start := addDays(from, shift)
		if start < today {
			continue
		}
		if fit := countFit(start); fit > bestFit {
			bestShift, bestFit = shift, fit
		}
	}
	if bestShift == 0 {
		return nil
	}
	direction := "позже"
	if bestShift < 0 {
		direction = "раньше"
	}
	days := int(math.Abs(float64(bestShift)))
	return &DateHint{
		From: addDays(from, bestShift), To: addDays(from, bestShift+nights), Fit: bestFit, Total: total,
		Text: fmt.Sprintf("Сдвиньте поездку на %d %s %s — погода подойдёт в %d из %d дней (сейчас %d).",
			days, pluralWord(days, "день", "дня", "дней"), direction, bestFit, total, current),
	}
}

// NearbyPlace - точка рядом с отелем для блока «Что близко».
type NearbyPlace struct {
	POIID    int64  `json:"poiId"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Distance string `json:"distance"`
	Minutes  int    `json:"minutes"`
	Match    bool   `json:"match"`
}

// nearbyPlaces - до 6 ближайших точек в пешей доступности, совпадающие с интересами - первыми.
func nearbyPlaces(hotel HotelCard, pois []POI, req TripRequest) []NearbyPlace {
	limit := req.MaxWalkMin
	if limit == 0 {
		limit = 20
	}
	out := make([]NearbyPlace, 0, 6)
	type cand struct {
		poi   POI
		walk  int
		match int
	}
	cands := make([]cand, 0, len(pois))
	for _, poi := range pois {
		walk := poiWalkMinutes(hotel, poi)
		if walk < 0 || walk > limit {
			continue
		}
		cands = append(cands, cand{poi, walk, poiInterestScore(poi, req.Interests)})
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if (cands[i].match > 0) != (cands[j].match > 0) {
			return cands[i].match > 0
		}
		return cands[i].walk < cands[j].walk
	})
	for _, c := range cands {
		if len(out) == 6 {
			break
		}
		out = append(out, NearbyPlace{POIID: c.poi.ID, Name: c.poi.Name, Category: c.poi.Category,
			Distance: fmt.Sprintf("%d мин пешком", max(c.walk, 1)), Minutes: c.walk, Match: c.match > 0})
	}
	return out
}
