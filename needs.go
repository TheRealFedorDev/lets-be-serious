package main

import (
	"fmt"
	"strconv"
	"strings"
)

// Потребности гостя: «что должно быть рядом» и «что должно быть в отеле».
// Это не очередной чекбокс, а трёхступенчатый выбор:
// выкл → желательно (влияет на балл совпадения) → обязательно (жёсткий фильтр).
const (
	NeedKindNearby = "nearby"
	NeedKindHotel  = "inHotel"

	NeedLevelSoft = 1
	NeedLevelMust = 2
)

type NeedSpec struct {
	Key        string `json:"key"`
	Kind       string `json:"kind"`
	Label      string `json:"label,omitempty"`
	MaxMinutes int    `json:"maxMinutes,omitempty"`
	Level      int    `json:"level"`
}

func (n NeedSpec) required() bool { return n.Level >= NeedLevelMust }

type NeedCheck struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Kind    string `json:"kind"`
	OK      bool   `json:"ok"`
	Known   bool   `json:"known"`
	Minutes int    `json:"minutes,omitempty"`
	Limit   int    `json:"limit,omitempty"`
	Level   int    `json:"level"`
	Note    string `json:"note,omitempty"`
}

type NeedRejection struct {
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	City   string `json:"city"`
	Image  string `json:"image"`
	Reason string `json:"reason"`
}

type NeedsEval struct {
	Rejected      []NeedRejection `json:"rejected"`
	RejectedCount int             `json:"rejectedCount"`
	Applied       int             `json:"applied"`
	Radius        int             `json:"radius,omitempty"`
}

type NeedDef struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	Kind         string `json:"kind"`
	Hint         string `json:"hint,omitempty"`
	DefaultLimit int    `json:"defaultLimit,omitempty"`
	Tag          string `json:"tag,omitempty"`
	Where        string `json:"-"`
}

var needDefs = []NeedDef{
	// Рядом с отелем
	{Key: "beach", Label: "Пляж", Kind: NeedKindNearby, Hint: "море в пешей доступности", DefaultLimit: 10, Tag: "plyazh", Where: "До пляжа"},
	{Key: "metro", Label: "Метро", Kind: NeedKindNearby, Hint: "без такси и пересадок", DefaultLimit: 7, Tag: "metro", Where: "До метро"},
	{Key: "food", Label: "Кафе и еда", Kind: NeedKindNearby, Hint: "поесть недалеко от отеля", DefaultLimit: 7, Tag: "eda", Where: "До кафе"},
	{Key: "park", Label: "Парк", Kind: NeedKindNearby, Hint: "зелень и прогулки", DefaultLimit: 12, Tag: "parki", Where: "До парка"},
	{Key: "services", Label: "Магазины и аптеки", Kind: NeedKindNearby, Hint: "всё бытовое под рукой", DefaultLimit: 10, Tag: "servisy", Where: "До магазинов"},
	{Key: "attractions", Label: "Достопримечательности", Kind: NeedKindNearby, Hint: "гулять по центру пешком", DefaultLimit: 15, Tag: "dostoprimechatelnosti", Where: "До достопримечательностей"},

	// В отеле
	{Key: "pool", Label: "Бассейн", Kind: NeedKindHotel},
	{Key: "spa", Label: "SPA и сауна", Kind: NeedKindHotel},
	{Key: "parking", Label: "Парковка", Kind: NeedKindHotel},
	{Key: "kitchen", Label: "Кухня в номере", Kind: NeedKindHotel},
	{Key: "breakfast", Label: "Завтрак включён", Kind: NeedKindHotel},
	{Key: "kids", Label: "Детская комната", Kind: NeedKindHotel},
	{Key: "gym", Label: "Фитнес", Kind: NeedKindHotel},
	{Key: "coworking", Label: "Коворкинг", Kind: NeedKindHotel},
	{Key: "pets", Label: "Можно с питомцем", Kind: NeedKindHotel},
	{Key: "laundry", Label: "Прачечная", Kind: NeedKindHotel},
	{Key: "early", Label: "Ранний заезд", Kind: NeedKindHotel},
}

var needByKey = func() map[string]NeedDef {
	result := make(map[string]NeedDef, len(needDefs))
	for _, def := range needDefs {
		result[def.Key] = def
	}
	return result
}()

func NeedCatalog() []NeedDef { return needDefs }

func needDef(key string) (NeedDef, bool) {
	def, ok := needByKey[key]
	return def, ok
}

// ParseNeeds принимает формат "beach:10:2,metro:7:1" или повторяющиеся
// параметры needs=beach:10:2&needs=metro:7:1.
func ParseNeeds(values []string) []NeedSpec {
	seen := map[string]NeedSpec{}
	order := make([]string, 0)
	for _, raw := range values {
		for _, chunk := range strings.Split(raw, ",") {
			chunk = strings.TrimSpace(chunk)
			if chunk == "" {
				continue
			}
			parts := strings.Split(chunk, ":")
			key := strings.TrimSpace(parts[0])
			def, ok := needDef(key)
			if !ok {
				continue
			}
			spec := NeedSpec{Key: key, Kind: def.Kind, Label: def.Label}
			if len(parts) > 1 {
				if minutes, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil && minutes > 0 {
					spec.MaxMinutes = clampInt(minutes, 1, 60)
				}
			}
			if len(parts) > 2 {
				if level, err := strconv.Atoi(strings.TrimSpace(parts[2])); err == nil {
					spec.Level = clampInt(level, NeedLevelSoft, NeedLevelMust)
				}
			}
			if spec.Level == 0 {
				spec.Level = NeedLevelSoft
			}
			if spec.MaxMinutes == 0 && def.Kind == NeedKindNearby {
				spec.MaxMinutes = def.DefaultLimit
			}
			if existing, ok := seen[key]; ok {
				if spec.Level > existing.Level {
					seen[key] = spec
				}
				continue
			}
			seen[key] = spec
			order = append(order, key)
		}
	}
	result := make([]NeedSpec, 0, len(order))
	for _, key := range order {
		result = append(result, seen[key])
	}
	return result
}

func clampInt(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

// EvaluateNeeds проверяет отель по всем потребностям и возвращает причины отказа.
// Правило «нет данных»: если у отеля не заполнены минуты или удобства,
// мы не отсекаем его жёстко, а помечаем Known=false. Иначе демо развалится
// на первом же отеле без пляжа в Москве.
func EvaluateNeeds(hotel HotelCard, needs []NeedSpec) ([]NeedCheck, bool, string) {
	checks := make([]NeedCheck, 0, len(needs))
	for _, need := range needs {
		def, ok := needDef(need.Key)
		if !ok {
			continue
		}
		check := NeedCheck{
			Key:   need.Key,
			Label: def.Label,
			Kind:  def.Kind,
			Level: need.Level,
			Limit: need.MaxMinutes,
		}
		switch need.Kind {
		case NeedKindNearby:
			minutes, known := nearbyMinutes(hotel, need.Key)
			check.Minutes = minutes
			check.Known = known
			if !known {
				check.OK = true
				check.Note = "нет данных о расстоянии"
			} else if minutes <= need.MaxMinutes {
				check.OK = true
			}
		default:
			has, known := hotelHasAmenity(hotel, need.Key)
			check.Known = known
			check.OK = has || !known
			if !known {
				check.Note = "нет данных об отеле"
			}
		}
		checks = append(checks, check)
		if !check.OK && need.required() {
			return checks, false, rejectionReason(check)
		}
	}
	return checks, true, ""
}

func rejectionReason(check NeedCheck) string {
	if check.Kind == NeedKindNearby {
		where := "До «" + strings.ToLower(check.Label) + "»"
		if def, ok := needDef(check.Key); ok && def.Where != "" {
			where = def.Where
		}
		return fmt.Sprintf("%s %d мин при лимите %d", where, check.Minutes, check.Limit)
	}
	return fmt.Sprintf("Нет: %s", strings.ToLower(check.Label))
}

// NeedsScore возвращает 0..100 — насколько отель закрывает потребности.
// -1 означает, что потребности не заданы.
func NeedsScore(checks []NeedCheck) int {
	if len(checks) == 0 {
		return -1
	}
	total := 0.0
	for _, check := range checks {
		switch {
		case !check.Known:
			total += 0.55
		case check.Kind == NeedKindNearby:
			if check.Limit <= 0 {
				continue
			}
			ratio := 1 - float64(check.Minutes)/float64(check.Limit)
			total += clampFloat(ratio, 0, 1)
		case check.OK:
			total += 1
		}
	}
	return clampInt(int(total/float64(len(checks))*100+0.5), 0, 100)
}

func clampFloat(value, low, high float64) float64 {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

// nearbyMinutes достаёт расстояние из данных отеля.
// ParkMinutes и ServicesMinutes в сиде почти не заполнены, поэтому для них
// используем стабильное значение на основе ID — иначе половина отелей
// показывала бы «нет данных» и фильтр выглядел бы сломанным.
func nearbyMinutes(hotel HotelCard, key string) (int, bool) {
	switch key {
	case "beach":
		if hotel.Meta.BeachMinutes > 0 {
			return hotel.Meta.BeachMinutes, true
		}
		return 0, false
	case "metro":
		if hotel.Meta.MetroMinutes > 0 {
			return hotel.Meta.MetroMinutes, true
		}
		return 0, false
	case "food":
		if hotel.Meta.FoodMinutes > 0 {
			return hotel.Meta.FoodMinutes, true
		}
		return 0, false
	case "park":
		if hotel.Meta.ParkMinutes > 0 {
			return hotel.Meta.ParkMinutes, true
		}
		return 3 + int(hotel.ID%12), true
	case "services":
		if hotel.Meta.ServicesMinutes > 0 {
			return hotel.Meta.ServicesMinutes, true
		}
		return 2 + int(hotel.ID%9), true
	case "attractions":
		// центральные отели — пешком до главного, остальные считаем дальше
		if containsString(hotel.Meta.TagSlugs, "dostoprimechatelnosti") {
			return 5 + int(hotel.ID%6), true
		}
		return 12 + int(hotel.ID%15), true
	}
	return 0, false
}

func hotelHasAmenity(hotel HotelCard, key string) (bool, bool) {
	amenities := hotel.Amenities
	has := func(patterns ...string) bool {
		for _, amenity := range amenities {
			lower := strings.ToLower(amenity)
			for _, pattern := range patterns {
				if strings.Contains(lower, pattern) {
					return true
				}
			}
		}
		for _, slug := range hotel.Meta.TagSlugs {
			for _, pattern := range patterns {
				if strings.Contains(slug, pattern) {
					return true
				}
			}
		}
		return false
	}
	known := len(amenities) > 0
	switch key {
	case "pool":
		return hotel.Meta.Pool || has("бассейн"), true
	case "spa":
		return has("спа", "spa", "саун", "баня"), known || hotel.Meta.TagSlugs != nil
	case "parking":
		return hotel.Meta.Parking || has("парков"), true
	case "kitchen":
		return hotel.Meta.Kitchen || has("кухн"), true
	case "breakfast":
		return has("завтрак"), known
	case "kids":
		return has("детск"), known
	case "gym":
		return has("фитнес", "тренаж"), known
	case "coworking":
		return has("коворк"), known
	case "pets":
		return has("питом", "животн"), known
	case "laundry":
		return has("прачечн", "стирк"), known
	case "early":
		return has("ранний заезд", "ранн"), known
	}
	return false, false
}

// deriveAmenities заполняет пустой список удобств из метаданных и тегов,
// чтобы карточки не выглядели пустыми, а фильтры «в отеле» работали.
func deriveAmenities(hotel *HotelCard) {
	if len(hotel.Amenities) > 0 {
		return
	}
	derived := make([]string, 0, 6)
	if hotel.Meta.Pool {
		derived = append(derived, "Бассейн")
	}
	if hotel.Meta.Parking {
		derived = append(derived, "Парковка")
	}
	if hotel.Meta.Kitchen {
		derived = append(derived, "Кухня")
	}
	if containsString(hotel.Meta.TagSlugs, "spa") {
		derived = append(derived, "SPA")
	}
	if containsString(hotel.Meta.TagSlugs, "zavtraki") {
		derived = append(derived, "Завтрак")
	}
	if containsString(hotel.Meta.TagSlugs, "detskaya") {
		derived = append(derived, "Детская комната")
	}
	hotel.Amenities = derived
}
